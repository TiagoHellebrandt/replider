import 'dart:math';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/forms/dirty_form_mixin.dart';
import 'package:app/design_system/components/custom_text_field.dart';
import 'package:app/design_system/components/metadata_schema_editor.dart';
import 'package:app/design_system/components/primary_pill_button.dart';
import 'package:app/design_system/components/settings_section_header.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Create / edit screen for a sales funnel (US-009). A null [funnelId] means
/// create; otherwise the funnel is loaded from the keep-alive list and edited in
/// place. Hosts the funnel name plus the reusable [MetadataSchemaEditor] as its
/// own section ("Dados a coletar"). Saves the whole funnel via
/// [FunnelsList] so the metadata schema round-trips alongside untouched fields
/// (product/agent/channel/webhook). US-010 extends this with a webhook section.
class FunnelConfigScreen extends ConsumerStatefulWidget {
  final String? funnelId;

  const FunnelConfigScreen({super.key, this.funnelId});

  bool get isEdit => funnelId != null;

  @override
  ConsumerState<FunnelConfigScreen> createState() => _FunnelConfigScreenState();
}

class _FunnelConfigScreenState extends ConsumerState<FunnelConfigScreen>
    with DirtyFormMixin<FunnelConfigScreen> {
  final _formKey = GlobalKey<FormState>();
  final _nameController = TextEditingController();
  final _webhookUrlController = TextEditingController();
  final _webhookSecretController = TextEditingController();

  /// Backing funnel loaded from the server (defaults for create). Untouched
  /// fields are preserved on save via [FunnelModel.copyWith].
  FunnelModel _loaded = const FunnelModel();
  List<MetadataFieldData> _fields = [];
  bool _triggerOnRequiredComplete = false;
  bool _triggerOnBuyIntent = false;

  bool _initialized = false;
  bool _saving = false;
  bool _testingWebhook = false;

  @override
  void initState() {
    super.initState();
    _nameController.addListener(recomputeDirty);
    _webhookUrlController.addListener(recomputeDirty);
    _webhookSecretController.addListener(recomputeDirty);
    if (!widget.isEdit) {
      _initialized = true;
      resetSnapshot();
    }
  }

  @override
  void dispose() {
    _nameController.removeListener(recomputeDirty);
    _nameController.dispose();
    _webhookUrlController.removeListener(recomputeDirty);
    _webhookUrlController.dispose();
    _webhookSecretController.removeListener(recomputeDirty);
    _webhookSecretController.dispose();
    super.dispose();
  }

  @override
  Map<String, Object?> snapshotForm() => {
        'name': _nameController.text.trim(),
        'schema': _fieldsSignature(_fields),
        'webhook_url': _webhookUrlController.text.trim(),
        'webhook_secret': _webhookSecretController.text.trim(),
        'trigger_required': _triggerOnRequiredComplete,
        'trigger_buy_intent': _triggerOnBuyIntent,
      };

  /// Stable, value-based signature of the schema so [DirtyFormMixin] can detect
  /// edits without deep-equality on mutable [MetadataFieldData].
  String _fieldsSignature(List<MetadataFieldData> fields) => fields
      .map((f) =>
          '${f.key}|${f.label}|${f.type}|${f.required}|${f.aiHint}|${f.enumOptions.join(",")}')
      .join(';');

  void _hydrate(FunnelModel f) {
    if (_initialized) return;
    _initialized = true;
    _loaded = f;
    _nameController.text = f.name;
    _webhookUrlController.text = f.webhookUrl;
    _webhookSecretController.text = f.webhookSecret;
    _triggerOnRequiredComplete = f.triggerOnRequiredComplete;
    _triggerOnBuyIntent = f.triggerOnBuyIntent;
    _fields = f.metadataSchema
        .map((m) => MetadataFieldData(
              key: m.key,
              label: m.label,
              type: m.type,
              required: m.isRequired,
              aiHint: m.aiHint,
              enumOptions: m.enumOptions ?? [],
            ))
        .toList();
    resetSnapshot();
  }

  Future<void> _save() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;

    final schemaError = validateMetadataFields(_fields);
    if (schemaError != null) {
      _toast(schemaError, false);
      return;
    }

    setState(() => _saving = true);

    final updated = _loaded.copyWith(
      name: _nameController.text.trim(),
      metadataSchema: _fields
          .map((d) => MetadataFieldModel(
                key: d.key,
                label: d.label,
                type: d.type,
                isRequired: d.required,
                aiHint: d.aiHint,
                enumOptions: d.type == 'enum' ? d.enumOptions : null,
              ))
          .toList(),
      webhookUrl: _webhookUrlController.text.trim(),
      webhookSecret: _webhookSecretController.text.trim(),
      triggerOnRequiredComplete: _triggerOnRequiredComplete,
      triggerOnBuyIntent: _triggerOnBuyIntent,
    );

    final notifier = ref.read(funnelsListProvider.notifier);
    final String? err = widget.isEdit
        ? await notifier.updateFunnel(updated)
        : await notifier.create(updated);

    if (!mounted) return;
    setState(() => _saving = false);

    if (err != null) {
      _toast(err, false);
      return;
    }
    resetSnapshot();
    _toast(
      widget.isEdit
          ? t.sales.funnels.updated_success
          : t.sales.funnels.created_success,
      true,
    );
    context.go('/sales/funnels');
  }

  /// Generates a 32-byte hex secret client-side for HMAC signing. Uses a
  /// crypto-grade RNG; the value is only revealed in the (masked) field and
  /// persisted on the next save.
  void _generateSecret() {
    final rng = Random.secure();
    final bytes = List<int>.generate(32, (_) => rng.nextInt(256));
    _webhookSecretController.text =
        bytes.map((b) => b.toRadixString(16).padLeft(2, '0')).join();
    recomputeDirty();
  }

  String? _validateWebhookUrl(String? v) {
    final s = (v ?? '').trim();
    if (s.isEmpty) return null; // webhook is optional
    final uri = Uri.tryParse(s);
    if (uri == null ||
        !uri.hasScheme ||
        (uri.scheme != 'http' && uri.scheme != 'https') ||
        !uri.hasAuthority) {
      return t.sales.webhook.url_invalid;
    }
    return null;
  }

  Future<void> _sendTestEvent() async {
    if (!widget.isEdit || _loaded.id.isEmpty) {
      _toast(t.sales.webhook.test_save_first, false);
      return;
    }
    if (_webhookUrlController.text.trim().isEmpty) {
      _toast(t.sales.webhook.test_url_first, false);
      return;
    }

    setState(() => _testingWebhook = true);
    final result =
        await ref.read(funnelsListProvider.notifier).testWebhook(_loaded.id);

    if (!mounted) return;
    setState(() => _testingWebhook = false);

    if (result.ok) {
      _toast(t.sales.webhook.test_success(status: '${result.status ?? '-'}'),
          true);
    } else {
      _toast('${t.sales.webhook.test_failed}: ${result.error}', false);
    }
  }

  void _toast(String msg, bool success) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(msg),
      backgroundColor: success ? Colors.green.shade700 : Colors.red.shade700,
      behavior: SnackBarBehavior.floating,
    ));
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    // In edit mode, hydrate from the keep-alive list once it's available.
    //
    // THREE states, not two. The code only left the loading state when it found
    // the funnel — if the list arrived without it (wrong id, another account's
    // funnel, a deleted funnel), `_initialized` stayed false and the screen spun
    // forever, with no message and no way out but the back button.
    var missing = false;
    if (widget.isEdit && !_initialized) {
      final listAsync = ref.watch(funnelsListProvider);
      final list = listAsync.value;
      if (list != null) {
        final found = list.where((FunnelModel f) => f.id == widget.funnelId);
        if (found.isNotEmpty) {
          _hydrate(found.first);
        } else {
          // The list arrived and the funnel is not in it: that is an answer, not
          // a wait.
          missing = true;
        }
      }
    }

    final loading = widget.isEdit && !_initialized && !missing;

    return buildDirtyGuard(
      onDiscard: () => context.go('/sales/funnels'),
      child: Scaffold(
        backgroundColor: colors.backgroundPrimary,
        appBar: AppBar(
          backgroundColor: colors.backgroundSecondary,
          elevation: 0,
          leading: IconButton(
            tooltip: t.sales.back_tooltip,
            icon: Icon(Icons.arrow_back, color: colors.textSecondary),
            onPressed: () => context.go('/sales/funnels'),
          ),
          title: Text(
            widget.isEdit
                ? t.sales.funnels.edit_funnel
                : t.sales.funnels.new_funnel,
            style: TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.w700,
              color: colors.textPrimary,
            ),
          ),
        ),
        body: loading
            ? const Center(child: CircularProgressIndicator())
            : missing
                ? Center(
                    child: Padding(
                      padding: const EdgeInsets.all(32),
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(Icons.filter_alt_off_outlined,
                              size: 44, color: colors.textMuted),
                          const SizedBox(height: 14),
                          Text(
                            t.sales.funnels.not_found,
                            key: const Key('funnel_not_found'),
                            style: TextStyle(
                                color: colors.textPrimary,
                                fontSize: 16,
                                fontWeight: FontWeight.w700),
                          ),
                          const SizedBox(height: 6),
                          Text(
                            t.sales.funnels.not_found_hint,
                            textAlign: TextAlign.center,
                            style: TextStyle(
                                color: colors.textMuted, fontSize: 13),
                          ),
                          const SizedBox(height: 20),
                          FilledButton(
                            onPressed: () => context.go('/sales/funnels'),
                            child: Text(t.sales.funnels.title),
                          ),
                        ],
                      ),
                    ),
                  )
                : Center(
                    child: ConstrainedBox(
                      constraints: const BoxConstraints(maxWidth: 640),
                      child: Form(
                        key: _formKey,
                        child: ListView(
                          padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
                          children: [
                            _label(colors, t.sales.funnels.field_name),
                            CustomTextField(
                              hint: t.sales.funnels.field_name_hint,
                              controller: _nameController,
                              autofocus: !widget.isEdit,
                              validator: (v) => (v == null || v.trim().isEmpty)
                                  ? t.sales.funnels.name_required
                                  : null,
                            ),
                            const SizedBox(height: 24),
                            SettingsSectionHeader(
                              label: t.sales.schema.section_title,
                              subtitle: t.sales.schema.section_subtitle,
                            ),
                            const SizedBox(height: 12),
                            MetadataSchemaEditor(
                              initialFields: _fields,
                              onChanged: (fields) {
                                _fields = fields;
                                recomputeDirty();
                              },
                            ),
                            const SizedBox(height: 24),
                            SettingsSectionHeader(
                              label: t.sales.webhook.section_title,
                              subtitle: t.sales.webhook.section_subtitle,
                            ),
                            const SizedBox(height: 12),
                            _buildWebhookSection(colors),
                            const SizedBox(height: 28),
                            PrimaryPillButton(
                              text: t.sales.funnels.save,
                              isLoading: _saving,
                              onPressed: _saving ? null : _save,
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
      ),
    );
  }

  Widget _buildWebhookSection(AppColorsExtension colors) {
    final hasSecret = _webhookSecretController.text.trim().isNotEmpty;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _label(colors, t.sales.webhook.field_url),
        CustomTextField(
          hint: t.sales.webhook.field_url_hint,
          controller: _webhookUrlController,
          keyboardType: TextInputType.url,
          validator: _validateWebhookUrl,
        ),
        const SizedBox(height: 16),
        _label(colors, t.sales.webhook.field_secret),
        Row(
          children: [
            Expanded(
              child: CustomTextField(
                hint: t.sales.webhook.field_secret_hint,
                controller: _webhookSecretController,
              ),
            ),
            if (hasSecret)
              IconButton(
                tooltip: t.common.copy,
                icon: Icon(Icons.copy_outlined, color: colors.textSecondary),
                onPressed: () {
                  Clipboard.setData(ClipboardData(
                      text: _webhookSecretController.text.trim()));
                  _toast(t.sales.webhook.secret_copied, true);
                },
              ),
            TextButton(
              onPressed: _generateSecret,
              child: Text(hasSecret
                  ? t.sales.webhook.regenerate_secret
                  : t.sales.webhook.generate_secret),
            ),
          ],
        ),
        const SizedBox(height: 8),
        SwitchListTile(
          contentPadding: EdgeInsets.zero,
          value: _triggerOnRequiredComplete,
          activeThumbColor: colors.whatsappGreen,
          onChanged: (v) {
            setState(() => _triggerOnRequiredComplete = v);
            recomputeDirty();
          },
          title: Text(
            t.sales.webhook.trigger_required_complete,
            style: TextStyle(
                color: colors.textPrimary,
                fontSize: 14,
                fontWeight: FontWeight.w600),
          ),
          subtitle: Text(
            t.sales.webhook.trigger_required_complete_hint,
            style: TextStyle(color: colors.textMuted, fontSize: 12),
          ),
        ),
        SwitchListTile(
          contentPadding: EdgeInsets.zero,
          value: _triggerOnBuyIntent,
          activeThumbColor: colors.whatsappGreen,
          onChanged: (v) {
            setState(() => _triggerOnBuyIntent = v);
            recomputeDirty();
          },
          title: Text(
            t.sales.webhook.trigger_buy_intent,
            style: TextStyle(
                color: colors.textPrimary,
                fontSize: 14,
                fontWeight: FontWeight.w600),
          ),
          subtitle: Text(
            t.sales.webhook.trigger_buy_intent_hint,
            style: TextStyle(color: colors.textMuted, fontSize: 12),
          ),
        ),
        const SizedBox(height: 12),
        OutlinedButton.icon(
          onPressed:
              (_testingWebhook || !widget.isEdit) ? null : _sendTestEvent,
          icon: _testingWebhook
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Icon(Icons.send_outlined, size: 18),
          label: Text(t.sales.webhook.send_test),
        ),
      ],
    );
  }

  Widget _label(AppColorsExtension colors, String text) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Text(
        text,
        style: TextStyle(
          color: colors.textSecondary,
          fontSize: 13,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}
