import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/forms/dirty_form_mixin.dart';
import 'package:app/design_system/components/danger_confirm_dialog.dart';
import 'package:app/design_system/components/save_feedback.dart';
import 'package:app/design_system/components/settings_section_card.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/settings/presentation/controllers/account_settings_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Timezone dropdown options. Brazilian zones first (majority of signups),
/// then a compact international set. Add more if analytics shows demand —
/// this list is intentionally short so users don't have to scroll through
/// 400+ IANA names to find São Paulo.
const _timezoneOptions = <String>[
  'America/Sao_Paulo',
  'America/Manaus',
  'America/Fortaleza',
  'America/Recife',
  'America/Bahia',
  'America/Cuiaba',
  'America/Belem',
  'America/Argentina/Buenos_Aires',
  'America/Mexico_City',
  'America/New_York',
  'America/Los_Angeles',
  'Europe/London',
  'Europe/Lisbon',
  'Europe/Madrid',
  'Europe/Paris',
  'Africa/Luanda',
];

/// Workspace-level settings: rename the company, delete the account.
/// Separate from the Profile screen (which scopes to the authenticated
/// user's own name/email/password).
class AccountScreen extends ConsumerStatefulWidget {
  const AccountScreen({super.key});

  @override
  ConsumerState<AccountScreen> createState() => _AccountScreenState();
}

class _AccountScreenState extends ConsumerState<AccountScreen>
    with DirtyFormMixin<AccountScreen> {
  final _nameController = TextEditingController();
  bool _initialized = false;
  bool _saving = false;

  // Branding form (separate dirty set so it has its own Save button).
  String? _initialLogoUrl;
  String? _currentLogoUrl; // null = no logo displayed
  String? _pendingLogoPath; // object_path waiting to be persisted
  String _timezone = 'America/Sao_Paulo';
  String _initialTimezone = 'America/Sao_Paulo';
  bool _uploadingLogo = false;
  bool _savingBranding = false;

  bool get _brandingDirty =>
      _currentLogoUrl != _initialLogoUrl || _timezone != _initialTimezone;

  @override
  Map<String, Object?> snapshotForm() => {
        'accountName': _nameController.text.trim(),
      };

  @override
  void dispose() {
    _nameController.removeListener(recomputeDirty);
    _nameController.dispose();
    super.dispose();
  }

  void _initFields(String name, String? logoUrl, String timezone) {
    if (_initialized) return;
    _nameController.text = name;
    _nameController.addListener(recomputeDirty);
    _initialLogoUrl = logoUrl;
    _currentLogoUrl = logoUrl;
    _initialTimezone = timezone;
    _timezone = timezone;
    resetSnapshot();
    _initialized = true;
  }

  Future<void> _pickLogo() async {
    final res = await FilePicker.platform.pickFiles(
      type: FileType.image,
      withData: true,
    );
    if (res == null || res.files.isEmpty) return;
    final file = res.files.first;
    if (file.bytes == null) return;
    if (!mounted) return;
    if (file.bytes!.length > 2 * 1024 * 1024) {
      SaveFeedback.showError(context, t.settings.branding_logo_too_large);
      return;
    }

    setState(() => _uploadingLogo = true);
    final result = await ref
        .read(accountSettingsControllerProvider.notifier)
        .uploadLogo(file.bytes!, file.name);
    if (!mounted) return;
    setState(() => _uploadingLogo = false);

    if (result.error != null || result.path == null) {
      SaveFeedback.showError(
          context, result.error ?? t.settings.branding_logo_upload_failed);
      return;
    }
    setState(() {
      _pendingLogoPath = result.path;
      _currentLogoUrl = result.path;
    });
  }

  void _removeLogo() {
    setState(() {
      _pendingLogoPath = null;
      _currentLogoUrl = null;
    });
  }

  Future<void> _saveBranding() async {
    setState(() => _savingBranding = true);
    final err = await ref
        .read(accountSettingsControllerProvider.notifier)
        .updateBranding(
          logoObjectPath: _pendingLogoPath,
          clearLogo: _currentLogoUrl == null && _initialLogoUrl != null,
          timezone: _timezone != _initialTimezone ? _timezone : null,
        );
    if (!mounted) return;
    setState(() => _savingBranding = false);
    if (err == null) {
      setState(() {
        _initialLogoUrl = _currentLogoUrl;
        _initialTimezone = _timezone;
        _pendingLogoPath = null;
      });
    }
    SaveFeedback.showResult(context, err,
        onSuccess: t.settings.branding_save_success);
  }

  Future<void> _save() async {
    setState(() => _saving = true);
    final err = await ref
        .read(accountSettingsControllerProvider.notifier)
        .updateAccountName(_nameController.text.trim());
    if (!mounted) return;
    setState(() => _saving = false);
    if (err == null) resetSnapshot();
    SaveFeedback.showResult(context, err,
        onSuccess: t.settings.company_save_success);
  }

  Future<void> _confirmDelete(String companyName) async {
    final deleted = await DangerConfirmDialog.show(
      context,
      title: t.settings.delete_account_dialog_title(company: companyName),
      body: t.settings.delete_account_dialog_body,
      confirmString: companyName,
      confirmHint: t.settings.delete_account_type_hint(company: companyName),
      confirmButton: t.settings.delete_account_confirm_button,
      onConfirm: () async {
        final err = await ref
            .read(accountSettingsControllerProvider.notifier)
            .deleteAccount();
        return err;
      },
    );
    if (!mounted) return;
    if (deleted) {
      SaveFeedback.showSuccess(context, t.settings.delete_account_success);
      context.go('/');
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final stateAsync = ref.watch(accountSettingsControllerProvider);

    return buildDirtyGuard(
      onDiscard: () => context.go('/settings'),
      child: Scaffold(
        backgroundColor: colors.backgroundPrimary,
        appBar: AppBar(
          backgroundColor: colors.backgroundSecondary,
          elevation: 0,
          leading: IconButton(
            tooltip: t.common.back,
            icon: Icon(Icons.arrow_back, color: colors.textSecondary),
            onPressed: () async {
              if (!isDirty) {
                context.go('/settings');
                return;
              }
              if (await confirmDiscard() && context.mounted) {
                context.go('/settings');
              }
            },
          ),
          title: Text(
            t.settings.account,
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.w700,
              color: colors.textPrimary,
            ),
          ),
        ),
        body: stateAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              // Parsed, never '$e': the interpolation dodged the CI guard
              // and painted a raw DioException on a settings screen.
              child: Text(
                parseApiError(e),
                style: TextStyle(color: colors.textSecondary),
                textAlign: TextAlign.center,
              ),
            ),
          ),
          data: (state) {
            _initFields(state.accountName, state.logoUrl, state.timezone);
            return SingleChildScrollView(
              padding: const EdgeInsets.all(24),
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 640),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      SettingsSectionCard(
                        title: t.settings.company_section_title,
                        icon: Icons.business,
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            TextField(
                              controller: _nameController,
                              style: TextStyle(color: colors.textPrimary),
                              textCapitalization: TextCapitalization.words,
                              decoration: InputDecoration(
                                labelText: t.settings.company_name_label,
                                labelStyle: TextStyle(color: colors.textMuted),
                                helperText: t.settings.company_name_helper,
                                helperMaxLines: 2,
                                helperStyle: TextStyle(color: colors.textMuted),
                              ),
                            ),
                            const SizedBox(height: 16),
                            Align(
                              alignment: Alignment.centerRight,
                              child: ElevatedButton(
                                onPressed: (!isDirty || _saving) ? null : _save,
                                style: ElevatedButton.styleFrom(
                                  backgroundColor: colors.bluePrimary,
                                  foregroundColor: Colors.white,
                                  padding: const EdgeInsets.symmetric(
                                      horizontal: 24, vertical: 12),
                                ),
                                child: _saving
                                    ? const SizedBox(
                                        width: 16,
                                        height: 16,
                                        child: CircularProgressIndicator(
                                            strokeWidth: 2,
                                            color: Colors.white),
                                      )
                                    : Text(t.settings.company_save_cta),
                              ),
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(height: 24),
                      SettingsSectionCard(
                        title: t.settings.branding_section_title,
                        icon: Icons.palette_outlined,
                        child: _BrandingForm(
                          colors: colors,
                          logoUrl: _currentLogoUrl,
                          pendingPath: _pendingLogoPath,
                          uploading: _uploadingLogo,
                          timezone: _timezone,
                          dirty: _brandingDirty,
                          saving: _savingBranding,
                          onPickLogo: _pickLogo,
                          onRemoveLogo: _removeLogo,
                          onTimezoneChanged: (tz) =>
                              setState(() => _timezone = tz),
                          onSave: _saveBranding,
                        ),
                      ),
                      if (state.isAdmin) ...[
                        const SizedBox(height: 24),
                        _ReplyReminderCard(
                          colors: colors,
                          enabled: state.replyReminderEnabled,
                          thresholdMinutes: state.replyReminderThresholdMinutes,
                          onChanged: (enabled, minutes) async {
                            final err = await ref
                                .read(
                                    accountSettingsControllerProvider.notifier)
                                .updateReplyReminder(
                                    enabled: enabled,
                                    thresholdMinutes: minutes);
                            if (err != null && context.mounted) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(content: Text(err)),
                              );
                            }
                          },
                        ),
                      ],
                      if (state.isAdmin) ...[
                        const SizedBox(height: 24),
                        SettingsSectionCard(
                          title: t.settings.danger_zone_title,
                          icon: Icons.warning_amber_rounded,
                          accent: colors.redAlert,
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.stretch,
                            children: [
                              Text(
                                t.settings.danger_zone_subtitle,
                                style: TextStyle(
                                    color: colors.textMuted, fontSize: 13),
                              ),
                              const SizedBox(height: 16),
                              Align(
                                alignment: Alignment.centerLeft,
                                child: OutlinedButton.icon(
                                  onPressed: () =>
                                      _confirmDelete(state.accountName),
                                  icon: Icon(Icons.delete_forever,
                                      color: colors.redAlert),
                                  label: Text(t.settings.delete_account_cta,
                                      style: TextStyle(
                                          color: colors.redAlert,
                                          fontWeight: FontWeight.w600)),
                                  style: OutlinedButton.styleFrom(
                                    side: BorderSide(color: colors.redAlert),
                                    padding: const EdgeInsets.symmetric(
                                        horizontal: 20, vertical: 14),
                                  ),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }
}

class _BrandingForm extends StatelessWidget {
  final AppColorsExtension colors;
  final String? logoUrl;
  final String? pendingPath;
  final bool uploading;
  final String timezone;
  final bool dirty;
  final bool saving;
  final VoidCallback onPickLogo;
  final VoidCallback onRemoveLogo;
  final ValueChanged<String> onTimezoneChanged;
  final VoidCallback onSave;

  const _BrandingForm({
    required this.colors,
    required this.logoUrl,
    required this.pendingPath,
    required this.uploading,
    required this.timezone,
    required this.dirty,
    required this.saving,
    required this.onPickLogo,
    required this.onRemoveLogo,
    required this.onTimezoneChanged,
    required this.onSave,
  });

  @override
  Widget build(BuildContext context) {
    final options = {..._timezoneOptions, timezone}.toList()..sort();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          t.settings.branding_section_subtitle,
          style: TextStyle(color: colors.textMuted, fontSize: 13),
        ),
        const SizedBox(height: 20),
        Text(
          t.settings.branding_logo_label,
          style: TextStyle(
              color: colors.textSecondary,
              fontWeight: FontWeight.w600,
              fontSize: 13),
        ),
        const SizedBox(height: 4),
        Text(
          t.settings.branding_logo_helper,
          style: TextStyle(color: colors.textMuted, fontSize: 12),
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            Container(
              width: 72,
              height: 72,
              decoration: BoxDecoration(
                color: colors.backgroundPrimary,
                border: Border.all(color: colors.borderLight),
                borderRadius: BorderRadius.circular(12),
              ),
              alignment: Alignment.center,
              child: uploading
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : (logoUrl != null
                      ? Icon(Icons.image, color: colors.textMuted)
                      : Icon(Icons.business, color: colors.textDisabled)),
            ),
            const SizedBox(width: 16),
            Expanded(
              child: Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  TextButton.icon(
                    onPressed: uploading ? null : onPickLogo,
                    icon: const Icon(Icons.upload_outlined, size: 18),
                    label: Text(logoUrl == null
                        ? t.settings.branding_logo_upload_cta
                        : t.settings.branding_logo_change_cta),
                  ),
                  if (logoUrl != null)
                    TextButton.icon(
                      onPressed: uploading ? null : onRemoveLogo,
                      icon: Icon(Icons.delete_outline,
                          size: 18, color: colors.redAlert),
                      label: Text(
                        t.settings.branding_logo_remove_cta,
                        style: TextStyle(color: colors.redAlert),
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
        const SizedBox(height: 24),
        Text(
          t.settings.branding_timezone_label,
          style: TextStyle(
              color: colors.textSecondary,
              fontWeight: FontWeight.w600,
              fontSize: 13),
        ),
        const SizedBox(height: 4),
        Text(
          t.settings.branding_timezone_helper,
          style: TextStyle(color: colors.textMuted, fontSize: 12),
        ),
        const SizedBox(height: 8),
        DropdownButtonFormField<String>(
          initialValue: options.contains(timezone) ? timezone : null,
          isExpanded: true,
          dropdownColor: colors.backgroundSecondary,
          style: TextStyle(color: colors.textPrimary),
          decoration: InputDecoration(),
          items: [
            for (final tz in options)
              DropdownMenuItem<String>(value: tz, child: Text(tz)),
          ],
          onChanged: (v) {
            if (v != null) onTimezoneChanged(v);
          },
        ),
        const SizedBox(height: 20),
        Align(
          alignment: Alignment.centerRight,
          child: ElevatedButton(
            onPressed: (!dirty || saving || uploading) ? null : onSave,
            style: ElevatedButton.styleFrom(
              backgroundColor: colors.bluePrimary,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 12),
            ),
            child: saving
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(
                        strokeWidth: 2, color: Colors.white),
                  )
                : Text(t.settings.branding_save_cta),
          ),
        ),
      ],
    );
  }
}

/// Modal that requires the user to type the company name verbatim before
/// enabling the destructive Delete button — deliberate friction to
/// Prior to Sprint 3 this file also defined its own `_DeleteAccountDialog`
/// with copy-the-company-name friction. That's now the shared
/// [DangerConfirmDialog] in design_system/components.

class _ReplyReminderCard extends StatefulWidget {
  final AppColorsExtension colors;
  final bool enabled;
  final int thresholdMinutes;
  final Future<void> Function(bool enabled, int thresholdMinutes) onChanged;

  const _ReplyReminderCard({
    required this.colors,
    required this.enabled,
    required this.thresholdMinutes,
    required this.onChanged,
  });

  @override
  State<_ReplyReminderCard> createState() => _ReplyReminderCardState();
}

class _ReplyReminderCardState extends State<_ReplyReminderCard> {
  late int _minutes = widget.thresholdMinutes;
  bool _saving = false;

  static const _options = <int>[10, 15, 30, 60, 120];

  @override
  void didUpdateWidget(covariant _ReplyReminderCard oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.thresholdMinutes != widget.thresholdMinutes) {
      _minutes = widget.thresholdMinutes;
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = widget.colors;
    return SettingsSectionCard(
      title: t.settings.reply_reminder_title,
      icon: Icons.alarm_outlined,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      t.settings.reply_reminder_subtitle,
                      style: TextStyle(
                          color: colors.textPrimary,
                          fontSize: 14,
                          fontWeight: FontWeight.w500),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      t.settings.reply_reminder_helper,
                      style: TextStyle(color: colors.textMuted, fontSize: 12),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 16),
              Switch.adaptive(
                value: widget.enabled,
                activeThumbColor: colors.bluePrimary,
                onChanged: _saving
                    ? null
                    : (v) async {
                        setState(() => _saving = true);
                        await widget.onChanged(v, _minutes);
                        if (mounted) setState(() => _saving = false);
                      },
              ),
            ],
          ),
          if (widget.enabled) ...[
            const SizedBox(height: 16),
            Text(
              t.settings.reply_reminder_threshold_label,
              style: TextStyle(color: colors.textSecondary, fontSize: 12),
            ),
            const SizedBox(height: 8),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: _options.map((m) {
                final selected = _minutes == m;
                return ChoiceChip(
                  label: Text(
                      m < 60 ? '${m}min' : '${(m / 60).toStringAsFixed(0)}h'),
                  selected: selected,
                  onSelected: _saving
                      ? null
                      : (sel) async {
                          if (!sel) return;
                          setState(() {
                            _minutes = m;
                            _saving = true;
                          });
                          await widget.onChanged(widget.enabled, m);
                          if (mounted) setState(() => _saving = false);
                        },
                );
              }).toList(),
            ),
          ],
        ],
      ),
    );
  }
}
