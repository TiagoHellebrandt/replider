import 'dart:async';

import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/design_system/components/whatsapp_text.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/settings/domain/models/whatsapp_template_model.dart';
import 'package:app/features/settings/domain/models/whatsapp_error.dart';
import 'package:app/features/settings/presentation/controllers/whatsapp_config_controller.dart';
import 'package:app/features/settings/presentation/widgets/whatsapp_embedded_signup_button.dart';
import 'package:app/i18n/strings.g.dart';

const _kMaxWidth = 720.0;

class WhatsAppSettingsScreen extends ConsumerWidget {
  final String channelId;

  const WhatsAppSettingsScreen({super.key, required this.channelId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // Templates only (since 2026-09-01). The business-profile tab, the
    // webhook diagnostics and the sibling phone numbers belonged to the v1
    // inbox product; the SDR needs this screen for one thing — the approved
    // templates the lead panel's template picker offers.
    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        leading: IconButton(
          tooltip: t.settings.wa_settings_back_tooltip,
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () => context.go('/settings/channels'),
        ),
        title: Text(
          t.settings.wa_settings_tab_templates,
          style: TextStyle(
            fontSize: 18,
            fontWeight: FontWeight.w700,
            color: colors.textPrimary,
          ),
        ),
      ),
      body: _TemplatesTab(channelId: channelId),
    );
  }
}

// ────────────────────────────────────────────────────────────────────────────
// TEMPLATES TAB
// ────────────────────────────────────────────────────────────────────────────

class _TemplatesTab extends ConsumerStatefulWidget {
  final String channelId;
  const _TemplatesTab({required this.channelId});

  @override
  ConsumerState<_TemplatesTab> createState() => _TemplatesTabState();
}

class _TemplatesTabState extends ConsumerState<_TemplatesTab> {
  /// Polls Meta every 30s while at least one template is PENDING. Meta
  /// template review takes minutes-to-hours; the user opens this tab once
  /// right after submitting and wants to see status flip to APPROVED
  /// without manually refreshing. We stop polling when every template is
  /// in a terminal state (approved/rejected/paused/disabled) — no point
  /// hammering the Graph API after that.
  Timer? _poll;

  String get channelId => widget.channelId;

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }

  void _ensurePolling(List<WhatsAppTemplateModel> templates) {
    final hasPending =
        templates.any((t) => t.status == WhatsAppTemplateStatus.pending);
    if (hasPending && _poll == null) {
      _poll = Timer.periodic(const Duration(seconds: 30), (_) {
        if (!mounted) return;
        ref.invalidate(whatsAppTemplatesProvider(channelId));
      });
    } else if (!hasPending && _poll != null) {
      _poll!.cancel();
      _poll = null;
    }
  }

  Color _statusColor(WhatsAppTemplateStatus s, AppColorsExtension c) {
    switch (s) {
      case WhatsAppTemplateStatus.approved:
        return c.whatsappGreen;
      case WhatsAppTemplateStatus.pending:
        return c.orangeAlert;
      case WhatsAppTemplateStatus.rejected:
        return c.redAlert;
      case WhatsAppTemplateStatus.paused:
      case WhatsAppTemplateStatus.disabled:
      case WhatsAppTemplateStatus.unknown:
        return c.textMuted;
    }
  }

  Widget _pendingBanner(AppColorsExtension colors, int count) {
    final headline = count == 1
        ? t.settings.wa_templates_pending_banner_one
        : t.settings.wa_templates_pending_banner_other(count: count);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: colors.orangeAlert.withValues(alpha: 0.12),
        border: Border.all(color: colors.orangeAlert.withValues(alpha: 0.4)),
        borderRadius: BorderRadius.circular(10),
      ),
      child: Row(
        children: [
          Icon(Icons.hourglass_top, size: 18, color: colors.orangeAlert),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  headline,
                  style: TextStyle(
                      color: colors.orangeAlert,
                      fontSize: 13,
                      fontWeight: FontWeight.w700),
                ),
                const SizedBox(height: 2),
                Text(
                  t.settings.wa_templates_polling_hint,
                  style: TextStyle(
                      color: colors.textMuted, fontSize: 11, height: 1.3),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final async = ref.watch(whatsAppTemplatesProvider(channelId));

    return Scaffold(
      backgroundColor: Colors.transparent,
      // With no template, the empty state already offers "Create first template"
      // — the two buttons opened the same dialog, one on top of the other.
      floatingActionButton: (async.asData?.value ?? const []).isEmpty
          ? null
          : FloatingActionButton.extended(
              onPressed: () => _showCreateDialog(context, ref, colors),
              backgroundColor: colors.whatsappGreen,
              foregroundColor: colors.textInverted,
              icon: const Icon(Icons.add),
              label: Text(t.whatsapp.new_template),
            ),
      body: async.when(
        data: (templates) {
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (mounted) _ensurePolling(templates);
          });
          if (templates.isEmpty) return _empty(context, ref, colors);
          final pending = templates
              .where((t) => t.status == WhatsAppTemplateStatus.pending)
              .length;
          return Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: _kMaxWidth),
              child: RefreshIndicator(
                onRefresh: () async =>
                    ref.invalidate(whatsAppTemplatesProvider(channelId)),
                child: ListView.separated(
                  padding: const EdgeInsets.fromLTRB(16, 16, 16, 96),
                  itemCount: templates.length + (pending > 0 ? 1 : 0),
                  separatorBuilder: (_, __) => const SizedBox(height: 10),
                  itemBuilder: (_, i) {
                    if (pending > 0 && i == 0) {
                      return _pendingBanner(colors, pending);
                    }
                    final idx = pending > 0 ? i - 1 : i;
                    return _card(context, ref, colors, templates[idx]);
                  },
                ),
              ),
            ),
          );
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => _buildTemplateError(context, ref, colors, err),
      ),
    );
  }

  /// Richer error screen for the templates tab. When the backend
  /// returns a structured WhatsApp error (e.g. TOKEN_EXPIRED,
  /// MISSING_WABA_ID), we surface the human copy from
  /// `WhatsAppError.humanize()` and pin a "Reconectar via Meta" button
  /// — that's the real remediation, not "Tentar novamente".
  Widget _buildTemplateError(
    BuildContext context,
    WidgetRef ref,
    AppColorsExtension colors,
    Object err,
  ) {
    WhatsAppError? parsed;
    if (err is DioException) parsed = WhatsAppError.fromDio(err);
    final needsReconnect = parsed != null &&
        (parsed.isTokenExpired ||
            parsed.isMissingCredentials ||
            parsed.isMissingPermission);
    final message = parsed?.humanize() ?? '$err';

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              needsReconnect ? Icons.link_off : Icons.error_outline,
              color: needsReconnect ? colors.orangeAlert : colors.redAlert,
              size: 48,
            ),
            const SizedBox(height: 12),
            Text(
              needsReconnect
                  ? t.settings.wa_templates_create_loading_error_title
                  : t.settings.error_loading_templates,
              style: TextStyle(color: colors.textPrimary, fontSize: 16),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 6),
            Text(
              message,
              style:
                  TextStyle(color: colors.textMuted, fontSize: 12, height: 1.4),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                if (needsReconnect)
                  ElevatedButton.icon(
                    onPressed: () => _openReconnectDialog(context, ref, colors),
                    icon: const Icon(Icons.chat, size: 16),
                    label: Text(t.whatsapp.reconnect_meta),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: colors.whatsappGreen,
                      foregroundColor: colors.textInverted,
                    ),
                  )
                else
                  TextButton.icon(
                    onPressed: () =>
                        ref.invalidate(whatsAppTemplatesProvider(channelId)),
                    icon: const Icon(Icons.refresh),
                    label: Text(t.common.retry),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _empty(
      BuildContext context, WidgetRef ref, AppColorsExtension colors) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.article_outlined, size: 64, color: colors.textDisabled),
            const SizedBox(height: 16),
            Text(t.whatsapp.no_templates,
                style: TextStyle(
                    color: colors.textPrimary,
                    fontSize: 18,
                    fontWeight: FontWeight.w700)),
            const SizedBox(height: 8),
            Text(
              t.settings.wa_templates_no_templates_subtitle,
              textAlign: TextAlign.center,
              style:
                  TextStyle(color: colors.textMuted, fontSize: 13, height: 1.5),
            ),
            const SizedBox(height: 20),
            ElevatedButton.icon(
              onPressed: () => _showCreateDialog(context, ref, colors),
              icon: const Icon(Icons.add),
              label: Text(t.whatsapp.create_first_template),
              style: ElevatedButton.styleFrom(
                backgroundColor: colors.whatsappGreen,
                foregroundColor: colors.textInverted,
                padding:
                    const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(10)),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _card(BuildContext context, WidgetRef ref, AppColorsExtension colors,
      WhatsAppTemplateModel t) {
    final statusColor = _statusColor(t.status, colors);
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  t.name,
                  style: TextStyle(
                      color: colors.textPrimary,
                      fontWeight: FontWeight.w700,
                      fontSize: 15),
                ),
              ),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                decoration: BoxDecoration(
                  color: statusColor.withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(6),
                ),
                child: Text(t.status.label,
                    style: TextStyle(
                        color: statusColor,
                        fontSize: 11,
                        fontWeight: FontWeight.w700)),
              ),
              IconButton(
                icon: Icon(Icons.visibility_outlined,
                    color: colors.textSecondary, size: 20),
                onPressed: () => _showPreview(context, colors, t),
                splashRadius: 18,
                tooltip: Translations.of(context)
                    .settings
                    .wa_templates_visualize_tooltip,
              ),
              IconButton(
                icon: Icon(Icons.delete_outline,
                    color: colors.redAlert, size: 20),
                onPressed: () => _confirmDelete(context, ref, colors, t),
                splashRadius: 18,
                tooltip: Translations.of(context)
                    .settings
                    .wa_templates_delete_tooltip,
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            '${t.category?.label ?? t.rawCategory ?? '—'}  ·  ${t.languageLabel}',
            style: TextStyle(color: colors.textMuted, fontSize: 12),
          ),
          // Show Meta's rejection reason inline so the user doesn't
          // have to open the Business Manager to learn why the template
          // was refused. Only renders on REJECTED templates with a
          // populated `rejected_reason` (backend fetches this via the
          // &fields=rejected_reason query param).
          if (t.status == WhatsAppTemplateStatus.rejected &&
              t.rejectedReason != null) ...[
            const SizedBox(height: 6),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
              decoration: BoxDecoration(
                color: colors.redAlert.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(6),
                border: Border.all(
                    color: colors.redAlert.withValues(alpha: 0.3), width: 0.5),
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(Icons.report_gmailerrorred,
                      size: 14, color: colors.redAlert),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      Translations.of(context)
                          .settings
                          .wa_templates_rejected_reason(
                              reason: t.rejectedReason ?? ''),
                      style: TextStyle(
                          color: colors.redAlert,
                          fontSize: 11,
                          fontWeight: FontWeight.w600),
                    ),
                  ),
                ],
              ),
            ),
          ],
          if (t.bodyPreview.isNotEmpty) ...[
            const SizedBox(height: 10),
            Container(
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: colors.glassMedium,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Text(
                t.bodyPreview,
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                    color: colors.textSecondary, fontSize: 13, height: 1.4),
              ),
            ),
          ],
        ],
      ),
    );
  }

  void _confirmDelete(BuildContext context, WidgetRef ref,
      AppColorsExtension colors, WhatsAppTemplateModel tmpl) {
    final tr = Translations.of(context);
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: colors.backgroundSecondary,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: Text(tr.whatsapp.delete_template,
            style: TextStyle(color: colors.textPrimary)),
        content: Text(
          tr.settings.wa_templates_delete_confirm(name: tmpl.name),
          style: TextStyle(color: colors.textSecondary, fontSize: 14),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text(tr.common.cancel,
                style: TextStyle(color: colors.textMuted)),
          ),
          ElevatedButton(
            onPressed: () async {
              Navigator.pop(ctx);
              final err = await ref
                  .read(whatsAppTemplatesProvider(channelId).notifier)
                  .delete(tmpl.name);
              if (context.mounted) {
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                  content: Text(err ?? tr.settings.wa_templates_delete_success),
                  backgroundColor:
                      err != null ? Colors.red.shade700 : Colors.green.shade700,
                  behavior: SnackBarBehavior.floating,
                ));
              }
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: colors.redAlert,
              foregroundColor: Colors.white,
            ),
            child: Text(tr.common.delete),
          ),
        ],
      ),
    );
  }

  void _showCreateDialog(
      BuildContext context, WidgetRef ref, AppColorsExtension colors) {
    showDialog<void>(
      context: context,
      builder: (_) => _CreateTemplateDialog(channelId: channelId),
    );
  }

  // Preview modal: renders the template the same way the customer will
  // see it on WhatsApp — green outbound bubble, placeholder values
  // substituted, footer muted, interactive buttons stacked below.
  // Read-only; edits require deleting and recreating (Meta rule).
  void _showPreview(BuildContext context, AppColorsExtension colors,
      WhatsAppTemplateModel tmpl) {
    final tr = Translations.of(context);
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: colors.backgroundSecondary,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: Text(tmpl.name,
            style: TextStyle(color: colors.textPrimary, fontSize: 16)),
        content: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 340),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                // Bubble simulating WhatsApp outbound.
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: colors.whatsappGreen.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(10),
                    border: Border.all(
                        color: colors.whatsappGreen.withValues(alpha: 0.3)),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        tmpl.renderedBody.isEmpty ? '—' : tmpl.renderedBody,
                        style: TextStyle(
                            color: colors.textPrimary,
                            fontSize: 14,
                            height: 1.4),
                      ),
                      if (tmpl.footerText != null) ...[
                        const SizedBox(height: 8),
                        Text(
                          tmpl.footerText!,
                          style: TextStyle(
                              color: colors.textMuted,
                              fontSize: 11,
                              height: 1.3),
                        ),
                      ],
                    ],
                  ),
                ),
                if (tmpl.buttons.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  // Buttons render as rows of pill-style actions — same
                  // visual weight WhatsApp uses, minus the native ripple.
                  ...tmpl.buttons.map((b) {
                    final label = (b['text'] as String?) ?? '';
                    final type = (b['type'] as String?)?.toUpperCase() ?? '';
                    final icon = type == 'URL'
                        ? Icons.open_in_new
                        : type == 'PHONE_NUMBER'
                            ? Icons.call
                            : Icons.reply;
                    return Container(
                      margin: const EdgeInsets.only(top: 6),
                      padding: const EdgeInsets.symmetric(
                          horizontal: 12, vertical: 10),
                      decoration: BoxDecoration(
                        color: colors.backgroundPrimary,
                        borderRadius: BorderRadius.circular(8),
                        border: Border.all(color: colors.borderLight),
                      ),
                      child: Row(
                        children: [
                          Icon(icon, size: 14, color: colors.bluePrimary),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Text(
                              label,
                              style: TextStyle(
                                  color: colors.bluePrimary,
                                  fontSize: 13,
                                  fontWeight: FontWeight.w600),
                            ),
                          ),
                        ],
                      ),
                    );
                  }),
                ],
                const SizedBox(height: 14),
                Text(
                  '${tmpl.category?.label ?? tmpl.rawCategory ?? '—'}  ·  ${tmpl.languageLabel}  ·  ${tmpl.status.label}',
                  style: TextStyle(color: colors.textMuted, fontSize: 11),
                ),
                if (tmpl.bodyExamples.isEmpty &&
                    tmpl.bodyText.contains('{{')) ...[
                  const SizedBox(height: 6),
                  Text(
                    tr.settings.wa_templates_no_examples_warning,
                    style: TextStyle(
                        color: colors.orangeAlert,
                        fontSize: 11,
                        fontStyle: FontStyle.italic),
                  ),
                ],
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text(tr.common.close,
                style: TextStyle(color: colors.textSecondary)),
          ),
        ],
      ),
    );
  }

  /// Inline reconnect dialog that shows the Embedded Signup button so
  /// the user can re-authorise Meta without bouncing through the
  /// channels list and hunting for the right row. Invalidates the
  /// templates/profile providers on success so data refreshes.
  void _openReconnectDialog(
    BuildContext context,
    WidgetRef ref,
    AppColorsExtension colors,
  ) {
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: colors.backgroundSecondary,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: Text(
          t.whatsapp.reconnect_meta,
          style:
              TextStyle(color: colors.textPrimary, fontWeight: FontWeight.w700),
        ),
        content: SizedBox(
          width: (MediaQuery.of(ctx).size.width * 0.9).clamp(280.0, 420.0),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                t.whatsapp.reconnect_meta_help,
                style: TextStyle(color: colors.textMuted, fontSize: 13),
              ),
              const SizedBox(height: 16),
              WhatsAppEmbeddedSignupButton(
                onConnected: () {
                  Navigator.of(ctx).pop();
                  ref.invalidate(whatsAppTemplatesProvider(channelId));
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(t.whatsapp.reconnect_success)),
                  );
                },
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text(t.common.cancel,
                style: TextStyle(color: colors.textMuted)),
          ),
        ],
      ),
    );
  }
}

// ────────────────────────────────────────────────────────────────────────────
// CREATE TEMPLATE DIALOG
// ────────────────────────────────────────────────────────────────────────────

class _CreateTemplateDialog extends ConsumerStatefulWidget {
  final String channelId;
  const _CreateTemplateDialog({required this.channelId});

  @override
  ConsumerState<_CreateTemplateDialog> createState() =>
      _CreateTemplateDialogState();
}

class _CreateTemplateDialogState extends ConsumerState<_CreateTemplateDialog> {
  final _nameCtrl = TextEditingController();
  final _bodyCtrl = TextEditingController();
  final _footerCtrl = TextEditingController();
  // Optional HEADER. Supports plain text + Meta media handles for
  // IMAGE / VIDEO / DOCUMENT. Text uses _headerCtrl + a single
  // optional `{{1}}` example. Media uses _headerMediaHandle (set by
  // the Resumable Upload flow) plus a local preview/filename for the
  // operator's confirmation.
  final _headerCtrl = TextEditingController();
  final _headerExampleCtrl = TextEditingController();
  _HeaderType _headerType = _HeaderType.none;
  String? _headerMediaHandle;
  String? _headerMediaFilename;
  Uint8List? _headerMediaBytes;
  bool _uploadingHeaderMedia = false;
  final List<TextEditingController> _exampleCtrls = [];
  final List<_ButtonDraft> _buttons = [];
  WhatsAppTemplateCategory _category = WhatsAppTemplateCategory.utility;
  String _language = 'pt_BR';
  bool _submitting = false;

  @override
  void initState() {
    super.initState();
    _bodyCtrl.addListener(_onBodyChanged);
    // Header textbox needs a rebuild trigger so the {{1}} example field
    // shows / hides as the user types, and so the preview reflects the
    // current header text live.
    _headerCtrl.addListener(_rebuild);
    _footerCtrl.addListener(_rebuild);
  }

  void _rebuild() {
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _bodyCtrl.removeListener(_onBodyChanged);
    _bodyCtrl.dispose();
    _footerCtrl.removeListener(_rebuild);
    _footerCtrl.dispose();
    _headerCtrl.removeListener(_rebuild);
    _headerCtrl.dispose();
    _headerExampleCtrl.dispose();
    for (final c in _exampleCtrls) {
      c.dispose();
    }
    super.dispose();
  }

  void _onBodyChanged() {
    final variableCount = _variablesInBody().length;
    setState(() {
      while (_exampleCtrls.length < variableCount) {
        _exampleCtrls.add(TextEditingController());
      }
      while (_exampleCtrls.length > variableCount) {
        _exampleCtrls.removeLast().dispose();
      }
    });
  }

  List<int> _variablesInBody() {
    final text = _bodyCtrl.text;
    final matches = RegExp(r'\{\{(\d+)\}\}').allMatches(text);
    final nums = matches.map((m) => int.parse(m.group(1)!)).toSet().toList()
      ..sort();
    return nums;
  }

  String? _validate() {
    final name = _nameCtrl.text.trim();
    if (!RegExp(r'^[a-z0-9_]{1,512}$').hasMatch(name)) {
      return t.settings.wa_create_validation_name;
    }
    final body = _bodyCtrl.text.trim();
    if (body.isEmpty) return t.settings.wa_create_validation_body_required;
    // Meta rejects templates containing double-asterisk markdown pairs —
    // the validator bounces them as 2388299 "Invalid parameter" without
    // telling the user what's wrong. Catch it here so the round-trip
    // error isn't the first signal.
    if (body.contains('**')) {
      return t.settings.wa_create_validation_double_asterisk;
    }
    // Meta rejects templates with variables at the very start or end.
    // The rule: variables must be bracketed by literal text on both
    // sides, otherwise the template reads as "unfinished."
    final trimmed = body.trim();
    if (RegExp(r'^\s*\{\{\d+\}\}').hasMatch(trimmed)) {
      return t.settings.wa_create_validation_no_leading_var;
    }
    if (RegExp(r'\{\{\d+\}\}\s*$').hasMatch(trimmed)) {
      return t.settings.wa_create_validation_no_trailing_var;
    }
    for (final c in _exampleCtrls) {
      if (c.text.trim().isEmpty)
        return t.settings.wa_create_validation_examples_required;
    }
    if (_headerType == _HeaderType.text) {
      final hdr = _headerCtrl.text.trim();
      if (hdr.isEmpty) return t.settings.wa_create_validation_header_empty;
      if (hdr.length > 60) {
        return t.settings.wa_create_validation_header_too_long;
      }
      // Meta allows at most ONE variable in the header text. Reject
      // multiples up-front so the round-trip to Graph API fails fast.
      final hdrVars = RegExp(r'\{\{\d+\}\}').allMatches(hdr).length;
      if (hdrVars > 1) {
        return t.settings.wa_create_validation_header_one_var;
      }
      if (hdrVars == 1 && _headerExampleCtrl.text.trim().isEmpty) {
        return t.settings.wa_create_validation_header_example;
      }
    }
    if (_headerType.isMedia) {
      if (_headerMediaHandle == null) {
        if (_uploadingHeaderMedia) {
          return t.settings.wa_create_validation_media_uploading;
        }
        return t.settings.wa_create_validation_media_required;
      }
    }
    for (final b in _buttons) {
      if (b.text.trim().isEmpty)
        return t.settings.wa_create_validation_button_text_required;
      if (b.type == _ButtonType.url && b.url.trim().isEmpty) {
        return t.settings.wa_create_validation_button_url;
      }
      if (b.type == _ButtonType.phone) {
        final p = b.phoneNumber.trim();
        // E.164 format with leading "+", 8-15 digits — Meta rejects
        // numbers without the country prefix.
        if (!RegExp(r'^\+\d{8,15}$').hasMatch(p)) {
          return t.settings.wa_create_validation_button_phone;
        }
      }
    }
    return null;
  }

  List<Map<String, dynamic>> _buildComponents() {
    final comps = <Map<String, dynamic>>[];

    if (_headerType == _HeaderType.text) {
      final hdrText = _headerCtrl.text.trim();
      final hdr = <String, dynamic>{
        'type': 'HEADER',
        'format': 'TEXT',
        'text': hdrText,
      };
      if (RegExp(r'\{\{1\}\}').hasMatch(hdrText)) {
        // Meta only allows ONE variable in the header; the example is
        // a single-string array, NOT the matrix used by the body.
        hdr['example'] = {
          'header_text': [_headerExampleCtrl.text.trim()],
        };
      }
      comps.add(hdr);
    } else if (_headerType.isMedia && _headerMediaHandle != null) {
      // Image / video / document headers ride on Meta's media handle
      // (Resumable Upload). Meta requires a SAMPLE handle on the
      // example field at create time even though the actual send
      // re-uploads/re-references media.
      comps.add({
        'type': 'HEADER',
        'format': _headerType.metaFormat,
        'example': {
          'header_handle': [_headerMediaHandle],
        },
      });
    }

    final body = <String, dynamic>{
      'type': 'BODY',
      'text': _bodyCtrl.text.trim(),
    };
    if (_exampleCtrls.isNotEmpty) {
      body['example'] = {
        'body_text': [_exampleCtrls.map((c) => c.text.trim()).toList()],
      };
    }
    comps.add(body);

    final footer = _footerCtrl.text.trim();
    if (footer.isNotEmpty) {
      comps.add({'type': 'FOOTER', 'text': footer});
    }

    if (_buttons.isNotEmpty) {
      comps.add({
        'type': 'BUTTONS',
        'buttons': _buttons.map((b) {
          switch (b.type) {
            case _ButtonType.url:
              return {
                'type': 'URL',
                'text': b.text.trim(),
                'url': b.url.trim(),
              };
            case _ButtonType.phone:
              return {
                'type': 'PHONE_NUMBER',
                'text': b.text.trim(),
                'phone_number': b.phoneNumber.trim(),
              };
            case _ButtonType.quickReply:
              return {'type': 'QUICK_REPLY', 'text': b.text.trim()};
          }
        }).toList(),
      });
    }
    return comps;
  }

  Future<void> _submit() async {
    final err = _validate();
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(err),
        backgroundColor: Colors.red.shade700,
        behavior: SnackBarBehavior.floating,
      ));
      return;
    }
    setState(() => _submitting = true);

    final result = await ref
        .read(whatsAppTemplatesProvider(widget.channelId).notifier)
        .create(
          name: _nameCtrl.text.trim(),
          category: _category,
          language: _language,
          components: _buildComponents(),
        );

    if (!mounted) return;
    setState(() => _submitting = false);

    if (result == null) {
      Navigator.pop(context);
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(t.settings.wa_template_submit_success),
        backgroundColor: Colors.green.shade700,
        behavior: SnackBarBehavior.floating,
        duration: const Duration(seconds: 5),
      ));
    } else {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(result),
        backgroundColor: Colors.red.shade700,
        behavior: SnackBarBehavior.floating,
      ));
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final width = MediaQuery.of(context).size.width;
    final maxW = width < 560 ? width * 0.95 : 520.0;

    return Dialog(
      backgroundColor: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: ConstrainedBox(
        constraints: BoxConstraints(
          maxWidth: maxW,
          maxHeight: MediaQuery.of(context).size.height * 0.9,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Header
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 20, 8, 8),
              child: Row(
                children: [
                  Expanded(
                    child: Text(t.whatsapp.new_template,
                        style: TextStyle(
                            color: colors.textPrimary,
                            fontWeight: FontWeight.w700,
                            fontSize: 18)),
                  ),
                  IconButton(
                    icon: Icon(Icons.close, color: colors.textMuted),
                    onPressed: () => Navigator.pop(context),
                  ),
                ],
              ),
            ),
            Divider(height: 1, color: colors.borderLight),

            Expanded(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(20),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _sectionLabel(colors, t.settings.wa_create_section_name),
                    const SizedBox(height: 6),
                    _field(
                      colors: colors,
                      controller: _nameCtrl,
                      label: t.settings.wa_create_name_hint,
                      hint: '',
                      inputFormatters: [
                        FilteringTextInputFormatter.allow(RegExp(r'[a-z0-9_]')),
                      ],
                    ),
                    _helpHint(colors, t.settings.wa_create_name_helper),
                    const SizedBox(height: 20),
                    _sectionLabel(
                        colors, t.settings.wa_create_section_category),
                    const SizedBox(height: 6),
                    _categoryChips(colors),
                    if (_categoryMismatchWarning(_category, _bodyCtrl.text)
                        case final warn?) ...[
                      const SizedBox(height: 8),
                      Container(
                        padding: const EdgeInsets.all(10),
                        decoration: BoxDecoration(
                          color: colors.orangeAlert.withValues(alpha: 0.12),
                          border: Border.all(
                              color: colors.orangeAlert.withValues(alpha: 0.4)),
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Icon(Icons.warning_amber_rounded,
                                color: colors.orangeAlert, size: 16),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Text(
                                warn,
                                style: TextStyle(
                                    color: colors.orangeAlert, fontSize: 12),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                    const SizedBox(height: 20),
                    _sectionLabel(
                        colors, t.settings.wa_create_section_language),
                    const SizedBox(height: 6),
                    _languageDropdown(colors),
                    const SizedBox(height: 20),
                    _sectionLabel(colors, t.settings.wa_create_section_header),
                    const SizedBox(height: 6),
                    _headerEditor(colors),
                    const SizedBox(height: 20),
                    _sectionLabel(colors, t.settings.wa_create_section_message),
                    const SizedBox(height: 6),
                    _field(
                      colors: colors,
                      controller: _bodyCtrl,
                      label: '',
                      hint: t.settings.wa_create_message_hint,
                      maxLines: 5,
                    ),
                    _helpHint(colors, t.settings.wa_create_message_helper),
                    if (_bodyCtrl.text.contains('**')) ...[
                      const SizedBox(height: 6),
                      Row(
                        children: [
                          Icon(Icons.info_outline,
                              size: 14, color: colors.orangeAlert),
                          const SizedBox(width: 6),
                          Expanded(
                            child: Text(
                              t.settings.wa_create_double_asterisk_warn,
                              style: TextStyle(
                                  color: colors.orangeAlert, fontSize: 11),
                            ),
                          ),
                        ],
                      ),
                    ],
                    if (_exampleCtrls.isNotEmpty) ...[
                      const SizedBox(height: 16),
                      _sectionLabel(
                          colors, t.settings.wa_create_section_examples),
                      const SizedBox(height: 6),
                      for (int i = 0; i < _exampleCtrls.length; i++) ...[
                        _field(
                          colors: colors,
                          controller: _exampleCtrls[i],
                          label: t.settings
                              .wa_create_example_for(n: (i + 1).toString()),
                          hint: i == 0
                              ? t.settings.wa_create_example_first
                              : t.settings.wa_create_example_other,
                        ),
                        const SizedBox(height: 10),
                      ],
                      _helpHint(colors, t.settings.wa_create_examples_helper),
                    ],
                    const SizedBox(height: 20),
                    _sectionLabel(colors, t.settings.wa_create_section_footer),
                    const SizedBox(height: 6),
                    _limitedField(
                      colors: colors,
                      controller: _footerCtrl,
                      label: '',
                      hint: t.settings.wa_create_footer_hint,
                      maxLength: 60,
                    ),
                    const SizedBox(height: 20),
                    _sectionLabel(colors, t.settings.wa_create_section_buttons),
                    const SizedBox(height: 6),
                    ..._buttons
                        .asMap()
                        .entries
                        .map((e) => _buttonEditor(colors, e.key, e.value)),
                    if (_buttons.length < 3)
                      Align(
                        alignment: Alignment.centerLeft,
                        child: TextButton.icon(
                          onPressed: () => setState(() => _buttons
                              .add(_ButtonDraft(type: _ButtonType.quickReply))),
                          icon: Icon(Icons.add, color: colors.whatsappGreen),
                          label: Text(t.whatsapp.add_button,
                              style: TextStyle(color: colors.whatsappGreen)),
                        ),
                      ),
                    const SizedBox(height: 24),
                    _sectionLabel(colors, t.settings.wa_create_section_preview),
                    const SizedBox(height: 6),
                    _preview(colors),
                  ],
                ),
              ),
            ),

            Divider(height: 1, color: colors.borderLight),
            Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  Expanded(
                    child: TextButton(
                      onPressed:
                          _submitting ? null : () => Navigator.pop(context),
                      child: Text(t.common.cancel,
                          style: TextStyle(color: colors.textMuted)),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    flex: 2,
                    child: ElevatedButton.icon(
                      onPressed: _submitting ? null : _submit,
                      icon: _submitting
                          ? const SizedBox(
                              width: 18,
                              height: 18,
                              child: CircularProgressIndicator(
                                  strokeWidth: 2, color: Colors.white))
                          : const Icon(Icons.send),
                      label: Text(_submitting
                          ? t.settings.wa_create_submitting
                          : t.settings.wa_create_submit),
                      style: ElevatedButton.styleFrom(
                        backgroundColor: colors.whatsappGreen,
                        foregroundColor: colors.textInverted,
                        padding: const EdgeInsets.symmetric(vertical: 14),
                        shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(10)),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  /// Heuristic pre-submit check. Meta rejects templates where the text
  /// clashes with the declared category (subcode 2388299). The common
  /// trap is promotional copy submitted as UTILITY; catching it here
  /// saves the user a round-trip and the vague Meta error.
  String? _categoryMismatchWarning(WhatsAppTemplateCategory cat, String body) {
    final lower = body.toLowerCase();
    bool has(Iterable<String> words) => words.any(lower.contains);
    final promoHits = has([
      'novidade',
      'promoção',
      'promocao',
      'oferta',
      'desconto',
      'melhoria',
      'lançamento',
      'lancamento',
      'cupom',
      'saiba das',
      'confira',
    ]);
    final txHits = has([
      'pedido',
      'confirmação',
      'confirmacao',
      'código',
      'codigo',
      'agendamento',
      'entrega',
      'pagamento',
      'fatura',
    ]);
    if (cat == WhatsAppTemplateCategory.utility && promoHits) {
      return t.settings.wa_create_warning_promo_for_utility;
    }
    if (cat == WhatsAppTemplateCategory.marketing && txHits && !promoHits) {
      return t.settings.wa_create_warning_tx_for_marketing;
    }
    return null;
  }

  Widget _categoryChips(AppColorsExtension colors) {
    final options = [
      (
        WhatsAppTemplateCategory.marketing,
        '🛍️',
        t.settings.wa_category_marketing
      ),
      (WhatsAppTemplateCategory.utility, '🔔', t.settings.wa_category_utility),
    ];
    return Column(
      children: options.map((o) {
        final selected = _category == o.$1;
        return GestureDetector(
          onTap: () => setState(() => _category = o.$1),
          child: Container(
            margin: const EdgeInsets.only(bottom: 8),
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: selected
                  ? colors.whatsappGreen.withValues(alpha: 0.12)
                  : colors.glassMedium,
              borderRadius: BorderRadius.circular(10),
              border: Border.all(
                color: selected ? colors.whatsappGreen : colors.borderLight,
                width: selected ? 1.5 : 1,
              ),
            ),
            child: Row(
              children: [
                Text(o.$2, style: const TextStyle(fontSize: 20)),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(o.$1.label,
                          style: TextStyle(
                              color: colors.textPrimary,
                              fontWeight: FontWeight.w700,
                              fontSize: 14)),
                      Text(o.$3,
                          style:
                              TextStyle(color: colors.textMuted, fontSize: 12)),
                    ],
                  ),
                ),
                if (selected)
                  Icon(Icons.check_circle, color: colors.whatsappGreen),
              ],
            ),
          ),
        );
      }).toList(),
    );
  }

  Widget _languageDropdown(AppColorsExtension colors) {
    final langs = [
      ('pt_BR', t.settings.wa_lang_pt_br),
      ('en_US', t.settings.wa_lang_en_us),
      ('es', t.settings.wa_lang_es),
    ];
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: colors.borderMedium),
      ),
      child: DropdownButtonHideUnderline(
        child: DropdownButton<String>(
          value: _language,
          isExpanded: true,
          dropdownColor: colors.backgroundSecondary,
          style: TextStyle(color: colors.textPrimary, fontSize: 14),
          items: langs
              .map((l) => DropdownMenuItem(value: l.$1, child: Text(l.$2)))
              .toList(),
          onChanged: (v) => setState(() => _language = v ?? 'pt_BR'),
        ),
      ),
    );
  }

  Widget _headerEditor(AppColorsExtension colors) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Three-row segmented to fit narrow phones; no segment uses
        // its icon slot to keep the row compact.
        Wrap(
          spacing: 6,
          runSpacing: 6,
          children: [
            for (final headerType in _HeaderType.values)
              ChoiceChip(
                label: Text(_headerLabel(headerType)),
                selected: _headerType == headerType,
                onSelected: (sel) {
                  if (!sel) return;
                  setState(() {
                    _headerType = headerType;
                    if (!headerType.isMedia) {
                      _headerMediaBytes = null;
                      _headerMediaFilename = null;
                      _headerMediaHandle = null;
                    }
                  });
                },
                labelStyle: TextStyle(color: colors.textPrimary, fontSize: 12),
                selectedColor: colors.bluePrimary.withValues(alpha: 0.2),
              ),
          ],
        ),
        if (_headerType == _HeaderType.text) ...[
          const SizedBox(height: 8),
          _limitedField(
            colors: colors,
            controller: _headerCtrl,
            label: '',
            hint: t.settings.wa_header_text_hint,
            maxLength: 60,
          ),
          _helpHint(colors, t.settings.wa_header_text_helper),
          if (RegExp(r'\{\{1\}\}').hasMatch(_headerCtrl.text)) ...[
            const SizedBox(height: 6),
            _field(
              colors: colors,
              controller: _headerExampleCtrl,
              label: t.settings.wa_header_example_label,
              hint: t.settings.wa_header_example_hint,
            ),
          ],
        ] else if (_headerType.isMedia) ...[
          const SizedBox(height: 8),
          _headerMediaPicker(colors),
        ],
      ],
    );
  }

  String _headerLabel(_HeaderType headerType) {
    switch (headerType) {
      case _HeaderType.none:
        return t.settings.wa_header_none;
      case _HeaderType.text:
        return t.settings.wa_header_text;
      case _HeaderType.image:
        return t.settings.wa_header_image;
      case _HeaderType.video:
        return t.settings.wa_header_video;
      case _HeaderType.document:
        return t.settings.wa_header_document;
    }
  }

  /// Inline picker + Resumable Upload trigger for media headers.
  /// Renders a "Selecionar arquivo" button when nothing is staged, and
  /// a filename + clear/replace controls once the operator picks
  /// something. Calls /channels/{id}/whatsapp/templates/upload-media
  /// inline so the handle is in hand BEFORE template submit — the
  /// alternative (uploading at submit time) doubles the perceived
  /// latency on the create button.
  Widget _headerMediaPicker(AppColorsExtension colors) {
    final filename = _headerMediaFilename;
    if (filename == null) {
      return OutlinedButton.icon(
        onPressed: _uploadingHeaderMedia ? null : _pickHeaderMedia,
        icon: _uploadingHeaderMedia
            ? const SizedBox(
                width: 14,
                height: 14,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            : Icon(Icons.upload_file, size: 16, color: colors.bluePrimary),
        label: Text(
          _uploadingHeaderMedia
              ? t.settings.wa_header_uploading
              : t.settings.wa_header_pick_file,
          style: TextStyle(color: colors.textPrimary, fontSize: 13),
        ),
        style: OutlinedButton.styleFrom(
          side: BorderSide(color: colors.borderMedium),
          shape:
              RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
        ),
      );
    }
    return Container(
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: colors.glassMedium,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: colors.borderLight),
      ),
      child: Row(
        children: [
          Icon(
              _headerType == _HeaderType.image
                  ? Icons.image_outlined
                  : _headerType == _HeaderType.video
                      ? Icons.videocam_outlined
                      : Icons.description_outlined,
              size: 18,
              color: colors.bluePrimary),
          const SizedBox(width: 8),
          Expanded(
            child: Text(filename,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                    color: colors.textPrimary,
                    fontSize: 12,
                    fontWeight: FontWeight.w600)),
          ),
          if (_headerMediaHandle != null) ...[
            Icon(Icons.check_circle, size: 14, color: colors.whatsappGreen),
            const SizedBox(width: 4),
          ] else if (_uploadingHeaderMedia)
            const SizedBox(
              width: 14,
              height: 14,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
          IconButton(
            icon: Icon(Icons.close, size: 16, color: colors.textMuted),
            onPressed: _uploadingHeaderMedia
                ? null
                : () => setState(() {
                      _headerMediaBytes = null;
                      _headerMediaFilename = null;
                      _headerMediaHandle = null;
                    }),
            splashRadius: 14,
            tooltip: t.settings.wa_header_remove_tooltip,
          ),
        ],
      ),
    );
  }

  Widget _headerMediaPlaceholder() {
    final icon = _headerType == _HeaderType.video
        ? Icons.play_circle_outline
        : _headerType == _HeaderType.document
            ? Icons.description_outlined
            : Icons.image_outlined;
    return Container(
      height: 100,
      width: double.infinity,
      color: Colors.white.withValues(alpha: 0.1),
      alignment: Alignment.center,
      child: Icon(icon, color: Colors.white70, size: 36),
    );
  }

  Future<void> _pickHeaderMedia() async {
    final extensions = switch (_headerType) {
      _HeaderType.image => ['jpg', 'jpeg', 'png'],
      _HeaderType.video => ['mp4'],
      _HeaderType.document => ['pdf'],
      _ => null,
    };
    final result = await FilePicker.platform.pickFiles(
      type: extensions == null ? FileType.any : FileType.custom,
      allowedExtensions: extensions,
      withData: true,
    );
    if (result == null || result.files.isEmpty) return;
    final f = result.files.first;
    final bytes = f.bytes;
    if (bytes == null) return;
    if (!mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    setState(() {
      _headerMediaBytes = bytes;
      _headerMediaFilename = f.name;
      _headerMediaHandle = null;
      _uploadingHeaderMedia = true;
    });
    try {
      final ds = ref.read(whatsAppConfigRemoteDataSourceProvider);
      final res = await ds.uploadTemplateMedia(
        widget.channelId,
        MultipartFile.fromBytes(bytes, filename: f.name),
      );
      final body = res.data;
      String? handle;
      if (body is Map && body['handle'] is String) {
        handle = body['handle'] as String;
      }
      if (!mounted) return;
      if (handle == null) {
        messenger.showSnackBar(SnackBar(
          content: Text(t.settings.wa_header_upload_failed_processing),
        ));
        setState(() {
          _uploadingHeaderMedia = false;
          _headerMediaBytes = null;
          _headerMediaFilename = null;
        });
        return;
      }
      setState(() {
        _headerMediaHandle = handle;
        _uploadingHeaderMedia = false;
      });
    } on DioException catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(
        content: Text(WhatsAppError.fromDio(e)?.humanize() ??
            t.settings.wa_header_upload_failed_generic(error: e.message ?? '')),
        backgroundColor: Colors.red.shade700,
      ));
      setState(() {
        _uploadingHeaderMedia = false;
        _headerMediaBytes = null;
        _headerMediaFilename = null;
        _headerMediaHandle = null;
      });
    }
  }

  Widget _buttonEditor(
      AppColorsExtension colors, int index, _ButtonDraft draft) {
    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: colors.glassMedium,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: colors.borderLight),
      ),
      child: Column(
        children: [
          Row(
            children: [
              Expanded(
                child: DropdownButton<_ButtonType>(
                  value: draft.type,
                  isExpanded: true,
                  dropdownColor: colors.backgroundSecondary,
                  style: TextStyle(color: colors.textPrimary, fontSize: 13),
                  underline: const SizedBox.shrink(),
                  items: [
                    DropdownMenuItem(
                        value: _ButtonType.quickReply,
                        child: Text(t.whatsapp.quick_reply)),
                    DropdownMenuItem(
                        value: _ButtonType.url,
                        child: Text(t.whatsapp.open_url)),
                    DropdownMenuItem(
                        value: _ButtonType.phone,
                        child: Text(t.settings.wa_button_call)),
                  ],
                  onChanged: (v) =>
                      setState(() => draft.type = v ?? draft.type),
                ),
              ),
              IconButton(
                icon: Icon(Icons.close, color: colors.textMuted, size: 18),
                onPressed: () => setState(() => _buttons.removeAt(index)),
                splashRadius: 16,
              ),
            ],
          ),
          const SizedBox(height: 4),
          TextField(
            decoration: InputDecoration(
              labelText: t.settings.wa_button_text_label,
              labelStyle: TextStyle(color: colors.textMuted, fontSize: 12),
              isDense: true,
            ),
            maxLength: 25,
            style: TextStyle(color: colors.textPrimary, fontSize: 13),
            onChanged: (v) => draft.text = v,
          ),
          if (draft.type == _ButtonType.url)
            TextField(
              decoration: InputDecoration(
                labelText: t.settings.wa_button_url_label,
                labelStyle: TextStyle(color: colors.textMuted, fontSize: 12),
                isDense: true,
                hintText: t.settings.wa_button_url_hint,
              ),
              style: TextStyle(color: colors.textPrimary, fontSize: 13),
              onChanged: (v) => draft.url = v,
            ),
          if (draft.type == _ButtonType.phone)
            TextField(
              decoration: InputDecoration(
                labelText: t.settings.wa_button_phone_label,
                labelStyle: TextStyle(color: colors.textMuted, fontSize: 12),
                isDense: true,
                hintText: t.settings.wa_button_phone_hint,
              ),
              keyboardType: TextInputType.phone,
              style: TextStyle(color: colors.textPrimary, fontSize: 13),
              onChanged: (v) => draft.phoneNumber = v,
            ),
        ],
      ),
    );
  }

  Widget _preview(AppColorsExtension colors) {
    // Substitute {{N}} with the example values
    var body = _bodyCtrl.text;
    for (int i = 0; i < _exampleCtrls.length; i++) {
      final val = _exampleCtrls[i].text;
      if (val.isNotEmpty) {
        body = body.replaceAll('{{${i + 1}}}', val);
      }
    }
    final footer = _footerCtrl.text.trim();

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFF005C4B), // WhatsApp outgoing bubble
        borderRadius: const BorderRadius.only(
          topLeft: Radius.circular(12),
          topRight: Radius.circular(12),
          bottomLeft: Radius.circular(12),
          bottomRight: Radius.circular(2),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (_headerType == _HeaderType.text &&
              _headerCtrl.text.trim().isNotEmpty) ...[
            Text(
              _headerCtrl.text.trim().replaceAll(
                  '{{1}}',
                  _headerExampleCtrl.text.trim().isEmpty
                      ? '{{1}}'
                      : _headerExampleCtrl.text.trim()),
              style: const TextStyle(
                  color: Colors.white,
                  fontSize: 15,
                  fontWeight: FontWeight.w700,
                  height: 1.3),
            ),
            const SizedBox(height: 6),
          ] else if (_headerType.isMedia && _headerMediaBytes != null) ...[
            ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: _headerType == _HeaderType.image
                  ? Image.memory(
                      _headerMediaBytes!,
                      height: 140,
                      width: double.infinity,
                      fit: BoxFit.cover,
                      errorBuilder: (_, __, ___) => _headerMediaPlaceholder(),
                    )
                  : _headerMediaPlaceholder(),
            ),
            const SizedBox(height: 6),
          ],
          body.isEmpty
              ? Text(
                  t.settings.wa_preview_message_placeholder,
                  style: const TextStyle(
                      color: Colors.white, fontSize: 14, height: 1.35),
                )
              : WhatsAppText(
                  body,
                  baseStyle: const TextStyle(
                      color: Colors.white, fontSize: 14, height: 1.35),
                ),
          if (footer.isNotEmpty) ...[
            const SizedBox(height: 6),
            Text(footer,
                style: TextStyle(
                    color: Colors.white.withValues(alpha: 0.6),
                    fontSize: 12,
                    fontStyle: FontStyle.italic)),
          ],
          if (_buttons.isNotEmpty) ...[
            const SizedBox(height: 8),
            for (final b in _buttons)
              Container(
                margin: const EdgeInsets.only(top: 4),
                width: double.infinity,
                padding:
                    const EdgeInsets.symmetric(vertical: 8, horizontal: 10),
                decoration: BoxDecoration(
                  color: Colors.white.withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(6),
                ),
                alignment: Alignment.center,
                child: Text(
                  b.text.isEmpty
                      ? t.settings.wa_preview_button_placeholder
                      : b.text,
                  style: const TextStyle(
                      color: Colors.white,
                      fontSize: 13,
                      fontWeight: FontWeight.w600),
                ),
              ),
          ],
        ],
      ),
    );
  }
}

enum _ButtonType { quickReply, url, phone }

class _ButtonDraft {
  _ButtonType type;
  String text = '';
  String url = '';
  String phoneNumber = '';
  _ButtonDraft({required this.type});
}

// HEADER type for templates. Meta accepts NONE / TEXT / IMAGE / VIDEO /
// DOCUMENT / LOCATION. We expose NONE + TEXT + IMAGE / VIDEO /
// DOCUMENT (the latter three require the Resumable Upload handle from
// `POST /channels/{id}/whatsapp/templates/upload-media`). LOCATION
// is rare and skipped — adds a coordinate input that few customers
// would actually use.
enum _HeaderType { none, text, image, video, document }

extension _HeaderTypeExt on _HeaderType {
  /// Meta's `format` enum value for the HEADER component.
  String get metaFormat {
    switch (this) {
      case _HeaderType.text:
        return 'TEXT';
      case _HeaderType.image:
        return 'IMAGE';
      case _HeaderType.video:
        return 'VIDEO';
      case _HeaderType.document:
        return 'DOCUMENT';
      case _HeaderType.none:
        return '';
    }
  }

  /// Whether this header type expects a Meta media handle (uploaded
  /// via Resumable Upload) instead of inline text.
  bool get isMedia =>
      this == _HeaderType.image ||
      this == _HeaderType.video ||
      this == _HeaderType.document;
}

// ────────────────────────────────────────────────────────────────────────────
// Shared helpers
// ────────────────────────────────────────────────────────────────────────────

Widget _sectionLabel(AppColorsExtension colors, String text) {
  return Text(
    text,
    style: TextStyle(
      color: colors.textMuted,
      fontSize: 11,
      fontWeight: FontWeight.w700,
      letterSpacing: 0.8,
    ),
  );
}

Widget _helpHint(AppColorsExtension colors, String text) {
  return Padding(
    padding: const EdgeInsets.only(top: 6),
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(Icons.info_outline, size: 12, color: colors.textMuted),
        const SizedBox(width: 4),
        Expanded(
          child: Text(text,
              style: TextStyle(
                  color: colors.textMuted, fontSize: 11, height: 1.3)),
        ),
      ],
    ),
  );
}

Widget _field({
  required AppColorsExtension colors,
  required TextEditingController controller,
  required String label,
  required String hint,
  int maxLines = 1,
  TextInputType? keyboardType,
  List<TextInputFormatter>? inputFormatters,
}) {
  return TextField(
    controller: controller,
    maxLines: maxLines,
    keyboardType: keyboardType,
    inputFormatters: inputFormatters,
    style: TextStyle(color: colors.textPrimary),
    decoration: InputDecoration(
      labelText: label.isEmpty ? null : label,
      hintText: hint,
      labelStyle: TextStyle(color: colors.textMuted),
      hintStyle: TextStyle(color: colors.textDisabled, fontSize: 13),
    ),
  );
}

Widget _limitedField({
  required AppColorsExtension colors,
  required TextEditingController controller,
  required String label,
  required String hint,
  required int maxLength,
  int maxLines = 1,
}) {
  return TextField(
    controller: controller,
    maxLines: maxLines,
    maxLength: maxLength,
    style: TextStyle(color: colors.textPrimary),
    decoration: InputDecoration(
      labelText: label.isEmpty ? null : label,
      hintText: hint,
      labelStyle: TextStyle(color: colors.textMuted),
      hintStyle: TextStyle(color: colors.textDisabled, fontSize: 13),
      // Bumped contrast (textDisabled → textMuted) + font size so the
      // "X/139" counter is actually readable. With the old muted color
      // it blended into the background and users thought the field had
      // no limit — the TextField silently truncated their typing.
      counterStyle: TextStyle(
          color: colors.textMuted, fontSize: 12, fontWeight: FontWeight.w500),
    ),
  );
}
