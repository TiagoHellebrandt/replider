import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/save_feedback.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/lgpd/domain/models/audit_log_model.dart';
import 'package:app/features/lgpd/presentation/controllers/lgpd_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Admin screen for LGPD / privacy actions. Two panels:
///   1. "Titular rights" — search a contact by external_id (phone) then
///      export or anonymize their data.
///   2. "Trilha de auditoria" — paginated view of every LGPD + admin
///      action recorded in audit_logs.
///
/// Admin-only. The server enforces role on both /lgpd and /audit, but
/// this screen is also hidden from the settings menu for non-admins.
class ComplianceScreen extends ConsumerStatefulWidget {
  const ComplianceScreen({super.key});

  @override
  ConsumerState<ComplianceScreen> createState() => _ComplianceScreenState();
}

class _ComplianceScreenState extends ConsumerState<ComplianceScreen> {
  final _searchCtrl = TextEditingController();
  ExportResult? _export;
  String? _searchedId;
  bool _busy = false;
  bool _smtpBusy = false;

  @override
  void dispose() {
    _searchCtrl.dispose();
    super.dispose();
  }

  Future<void> _doExport() async {
    final id = _searchCtrl.text.trim();
    if (id.isEmpty) return;
    setState(() {
      _busy = true;
      _export = null;
      _searchedId = id;
    });
    try {
      final res =
          await ref.read(lgpdControllerProvider.notifier).exportContact(id);
      if (!mounted) return;
      setState(() => _export = res);
    } catch (e) {
      if (!mounted) return;
      _toast(parseApiError(e), success: false);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _downloadJson() async {
    if (_export == null || _searchedId == null) return;
    // Put the raw JSON in the system clipboard. On Flutter Web,
    // this is the only cross-browser-friendly way to "save" without
    // pulling in html/web deps.
    final pretty = const JsonEncoder.withIndent('  ').convert(_export!.bundle);
    await Clipboard.setData(ClipboardData(text: pretty));
    _toast(t.lgpd.json_copied_hint(id: _searchedId!), success: true);
    // A richer implementation would trigger an actual file download via
    // the `file_saver` package; copying to clipboard is a pragmatic MVP.
  }

  Future<void> _doAnonymize() async {
    final id = _searchCtrl.text.trim();
    if (id.isEmpty) return;
    final confirmed = await _confirmAnonymize(id);
    if (!confirmed) return;
    setState(() => _busy = true);
    final err =
        await ref.read(lgpdControllerProvider.notifier).anonymizeContact(id);
    if (!mounted) return;
    setState(() {
      _busy = false;
      _export = null;
    });
    if (err == null) {
      _toast(t.lgpd.anonymize_success, success: true);
      // Surface the new audit entry right away.
      ref.invalidate(auditLogListProvider);
    } else {
      _toast(err, success: false);
    }
  }

  Future<void> _sendSMTPTest() async {
    setState(() => _smtpBusy = true);
    final err =
        await ref.read(lgpdControllerProvider.notifier).sendSMTPTestEmail();
    if (!mounted) return;
    setState(() => _smtpBusy = false);
    if (err == null) {
      _toast(t.settings.smtp_test_success, success: true);
    } else {
      _toast(err, success: false);
    }
  }

  Future<bool> _confirmAnonymize(String externalId) async {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final result = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: colors.backgroundSecondary,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: Text(t.lgpd.anonymize_title,
            style: TextStyle(color: colors.textPrimary)),
        content: Text(
          t.lgpd.anonymize_confirm_body(externalId: externalId),
          style:
              TextStyle(color: colors.textSecondary, fontSize: 14, height: 1.4),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: Text(t.common.cancel,
                style: TextStyle(color: colors.textMuted)),
          ),
          ElevatedButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: ElevatedButton.styleFrom(
              backgroundColor: colors.redAlert,
              foregroundColor: Colors.white,
            ),
            child: Text(t.lgpd.anonymize),
          ),
        ],
      ),
    );
    return result ?? false;
  }

  void _toast(String msg, {required bool success}) {
    // Thin wrapper around the shared SaveFeedback helper so the rest of
    // this file stays call-site-stable. SaveFeedback owns the color +
    // behavior contract — change it there and every screen adopts it.
    if (success) {
      SaveFeedback.showSuccess(context, msg);
    } else {
      SaveFeedback.showError(context, msg);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        leading: IconButton(
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () => context.go('/settings'),
        ),
        title: Text(
          t.lgpd.screen_title,
          style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.w700,
              color: colors.textPrimary),
        ),
      ),
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 900),
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              _introCard(colors),
              const SizedBox(height: 24),
              _sectionLabel(colors, t.lgpd.section_data_subject_rights,
                  Icons.shield_outlined),
              const SizedBox(height: 6),
              _titularCard(colors),
              const SizedBox(height: 24),
              _sectionLabel(
                  colors, t.lgpd.section_infrastructure, Icons.dns_outlined),
              const SizedBox(height: 6),
              _smtpCard(colors),
              const SizedBox(height: 24),
              _sectionLabel(colors, t.lgpd.section_audit_trail,
                  Icons.fact_check_outlined),
              const SizedBox(height: 6),
              _auditCard(colors),
              const SizedBox(height: 24),
            ],
          ),
        ),
      ),
    );
  }

  Widget _smtpCard(AppColorsExtension colors) {
    return Container(
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      padding: const EdgeInsets.all(16),
      child: Row(
        children: [
          Icon(Icons.mail_outline, color: colors.bluePrimary),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  t.settings.smtp_test_title,
                  style: TextStyle(
                      color: colors.textPrimary,
                      fontSize: 14,
                      fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: 2),
                Text(
                  t.settings.smtp_test_subtitle,
                  style: TextStyle(
                      color: colors.textMuted, fontSize: 12, height: 1.4),
                ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          ElevatedButton.icon(
            onPressed: _smtpBusy ? null : _sendSMTPTest,
            icon: _smtpBusy
                ? const SizedBox(
                    width: 14,
                    height: 14,
                    child: CircularProgressIndicator(
                        strokeWidth: 2, color: Colors.white),
                  )
                : const Icon(Icons.send_outlined, size: 16),
            label: Text(t.settings.smtp_test_cta),
            style: ElevatedButton.styleFrom(
              backgroundColor: colors.bluePrimary,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            ),
          ),
        ],
      ),
    );
  }

  Widget _titularCard(AppColorsExtension colors) {
    return Container(
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.lgpd.titular_search_hint,
            style:
                TextStyle(color: colors.textMuted, fontSize: 12, height: 1.4),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _searchCtrl,
                  // The two buttons below follow this field.
                  onChanged: (_) => setState(() {}),
                  style: TextStyle(color: colors.textPrimary),
                  inputFormatters: [
                    FilteringTextInputFormatter.allow(
                        RegExp(r'[A-Za-z0-9_\-.+]')),
                  ],
                  decoration: InputDecoration(
                    hintText: '5511999999999',
                    hintStyle:
                        TextStyle(color: colors.textDisabled, fontSize: 13),
                  ),
                ),
              ),
              const SizedBox(width: 8),
              ElevatedButton.icon(
                // With no identifier there is nothing to export, and the handler
                // already knew that — silently.
                onPressed:
                    _busy || _searchCtrl.text.trim().isEmpty ? null : _doExport,
                icon: const Icon(Icons.search, size: 16),
                label: Text(t.common.search),
                style: ElevatedButton.styleFrom(
                  backgroundColor: colors.bluePrimary,
                  foregroundColor: Colors.white,
                  padding:
                      const EdgeInsets.symmetric(horizontal: 18, vertical: 14),
                ),
              ),
            ],
          ),
          if (_export != null) ...[
            const SizedBox(height: 16),
            _exportPreview(colors),
          ],
        ],
      ),
    );
  }

  Widget _exportPreview(AppColorsExtension colors) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.whatsappGreen.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: colors.whatsappGreen.withValues(alpha: 0.3)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.check_circle, size: 16, color: colors.whatsappGreen),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  t.lgpd.data_found_for(id: _searchedId ?? ''),
                  style: TextStyle(
                      color: colors.textPrimary,
                      fontWeight: FontWeight.w700,
                      fontSize: 13),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 16,
            runSpacing: 6,
            children: [
              _stat(colors, t.lgpd.stat_conversations, _export!.conversations),
              _stat(colors, t.lgpd.stat_messages, _export!.messages),
              _stat(colors, t.lgpd.stat_campaigns, _export!.campaigns),
            ],
          ),
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              OutlinedButton.icon(
                onPressed: _busy ? null : _downloadJson,
                icon: const Icon(Icons.copy, size: 14),
                label: Text(t.lgpd.copy_json),
                style: OutlinedButton.styleFrom(
                  foregroundColor: colors.bluePrimary,
                  side: BorderSide(color: colors.borderMedium),
                ),
              ),
              ElevatedButton.icon(
                onPressed: _busy || _searchCtrl.text.trim().isEmpty
                    ? null
                    : _doAnonymize,
                icon: const Icon(Icons.delete_forever, size: 14),
                label: Text(t.lgpd.anonymize),
                style: ElevatedButton.styleFrom(
                  backgroundColor: colors.redAlert,
                  foregroundColor: Colors.white,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _stat(AppColorsExtension colors, String label, int value) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          width: 6,
          height: 6,
          decoration: BoxDecoration(
              color: colors.whatsappGreen, shape: BoxShape.circle),
        ),
        const SizedBox(width: 6),
        Text('$label: $value',
            style: TextStyle(color: colors.textSecondary, fontSize: 12)),
      ],
    );
  }

  Widget _auditCard(AppColorsExtension colors) {
    final async = ref.watch(auditLogListProvider);
    return Container(
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _AuditFilterBar(colors: colors),
          const SizedBox(height: 12),
          async.when(
            data: (page) {
              if (page.entries.isEmpty) {
                return Text(t.lgpd.no_actions,
                    style: TextStyle(color: colors.textMuted, fontSize: 13));
              }
              return Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    t.lgpd.audit_records_summary(
                        shown: page.entries.length, total: page.total),
                    style: TextStyle(color: colors.textMuted, fontSize: 12),
                  ),
                  const SizedBox(height: 8),
                  ...page.entries.map((e) => _auditRow(colors, e)),
                  if (page.entries.length < page.total) ...[
                    const SizedBox(height: 8),
                    Align(
                      alignment: Alignment.center,
                      child: TextButton.icon(
                        onPressed: () =>
                            ref.read(auditLogListProvider.notifier).loadMore(),
                        icon: const Icon(Icons.expand_more, size: 14),
                        label: Text(t.common.load_more),
                      ),
                    ),
                  ],
                ],
              );
            },
            loading: () => const Center(
                child: Padding(
                    padding: EdgeInsets.all(16),
                    child: CircularProgressIndicator())),
            error: (err, _) => Row(
              children: [
                Icon(Icons.error_outline, size: 16, color: colors.redAlert),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(parseApiError(err),
                      style: TextStyle(color: colors.redAlert, fontSize: 12)),
                ),
                TextButton(
                  onPressed: () =>
                      ref.read(auditLogListProvider.notifier).refresh(),
                  child: Text(t.common.retry),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _auditRow(AppColorsExtension colors, AuditLogEntry e) {
    return Container(
      margin: const EdgeInsets.only(bottom: 6),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: colors.glassMedium,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(_iconForAction(e.action),
              size: 16, color: _colorForAction(colors, e.action)),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '${e.actionLabel} · ${e.entityType}: ${e.entityId}',
                  style: TextStyle(
                      color: colors.textPrimary,
                      fontWeight: FontWeight.w600,
                      fontSize: 13),
                ),
                Text(
                  _formatLocal(e.createdAt),
                  style: TextStyle(color: colors.textMuted, fontSize: 11),
                ),
                if (e.payload.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: Text(
                      _payloadSummary(e.payload),
                      style: TextStyle(
                          color: colors.textSecondary,
                          fontSize: 11,
                          fontFamily: 'monospace'),
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  IconData _iconForAction(String action) {
    if (action == 'lgpd.anonymize') return Icons.delete_forever;
    if (action == 'lgpd.export') return Icons.download;
    return Icons.history;
  }

  Color _colorForAction(AppColorsExtension colors, String action) {
    if (action == 'lgpd.anonymize') return colors.redAlert;
    if (action == 'lgpd.export') return colors.bluePrimary;
    return colors.textMuted;
  }

  String _formatLocal(DateTime d) {
    final local = d.toLocal();
    final dd = local.day.toString().padLeft(2, '0');
    final mm = local.month.toString().padLeft(2, '0');
    final hh = local.hour.toString().padLeft(2, '0');
    final mi = local.minute.toString().padLeft(2, '0');
    return '$dd/$mm/${local.year} $hh:$mi';
  }

  String _payloadSummary(Map<String, dynamic> payload) {
    final parts = <String>[];
    payload.forEach((k, v) {
      parts.add('$k=$v');
    });
    final s = parts.join(' · ');
    return s.length > 120 ? '${s.substring(0, 120)}…' : s;
  }

  Widget _sectionLabel(AppColorsExtension colors, String text, IconData icon) {
    return Row(
      children: [
        Icon(icon, size: 14, color: colors.textMuted),
        const SizedBox(width: 6),
        Text(
          text,
          style: TextStyle(
            color: colors.textMuted,
            fontSize: 11,
            fontWeight: FontWeight.w700,
            letterSpacing: 0.8,
          ),
        ),
      ],
    );
  }

  /// Top-of-page explainer card. Covers what the LGPD, infra and audit
  /// sections are for so the admin doesn't have to guess — this screen
  /// is the primary evidence surface for compliance audits, so a
  /// plain-language intro beats a naked list of controls.
  Widget _introCard(AppColorsExtension colors) {
    return Container(
      decoration: BoxDecoration(
        color: colors.bluePrimary.withValues(alpha: 0.06),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.bluePrimary.withValues(alpha: 0.25)),
      ),
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.verified_user_outlined,
                  size: 20, color: colors.bluePrimary),
              const SizedBox(width: 8),
              Text(
                t.lgpd.intro_title,
                style: TextStyle(
                  color: colors.textPrimary,
                  fontSize: 15,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            t.lgpd.intro_body,
            style: TextStyle(
                color: colors.textSecondary, fontSize: 13, height: 1.45),
          ),
          const SizedBox(height: 12),
          _introBullet(
            colors,
            Icons.shield_outlined,
            t.lgpd.intro_bullet_rights_title,
            t.lgpd.intro_bullet_rights_body,
          ),
          _introBullet(
            colors,
            Icons.dns_outlined,
            t.lgpd.intro_bullet_infra_title,
            t.lgpd.intro_bullet_infra_body,
          ),
          _introBullet(
            colors,
            Icons.fact_check_outlined,
            t.lgpd.intro_bullet_audit_title,
            t.lgpd.intro_bullet_audit_body,
          ),
        ],
      ),
    );
  }

  Widget _introBullet(
    AppColorsExtension colors,
    IconData icon,
    String title,
    String body,
  ) {
    return Padding(
      padding: const EdgeInsets.only(top: 10),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 16, color: colors.bluePrimary),
          const SizedBox(width: 10),
          Expanded(
            child: RichText(
              text: TextSpan(
                style: TextStyle(
                    color: colors.textSecondary, fontSize: 12, height: 1.45),
                children: [
                  TextSpan(
                    text: '$title — ',
                    style: TextStyle(
                      color: colors.textPrimary,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  TextSpan(text: body),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Filter bar + CSV export CTA for the audit log. Owns local input state
/// so the parent screen doesn't rebuild on every keystroke. Applying
/// dispatches to AuditLogList.applyFilters; clearing resets inputs + calls
/// applyFilters(const AuditLogFilter()).
class _AuditFilterBar extends ConsumerStatefulWidget {
  final AppColorsExtension colors;
  const _AuditFilterBar({required this.colors});

  @override
  ConsumerState<_AuditFilterBar> createState() => _AuditFilterBarState();
}

class _AuditFilterBarState extends ConsumerState<_AuditFilterBar> {
  final _actionCtrl = TextEditingController();
  final _entityCtrl = TextEditingController();
  final _actorCtrl = TextEditingController();
  bool _exporting = false;

  @override
  void dispose() {
    _actionCtrl.dispose();
    _entityCtrl.dispose();
    _actorCtrl.dispose();
    super.dispose();
  }

  String? _nonEmpty(String raw) {
    final v = raw.trim();
    return v.isEmpty ? null : v;
  }

  void _apply() {
    ref.read(auditLogListProvider.notifier).applyFilters(
          AuditLogFilter(
            action: _nonEmpty(_actionCtrl.text),
            entityType: _nonEmpty(_entityCtrl.text),
            actorId: _nonEmpty(_actorCtrl.text),
          ),
        );
  }

  void _clear() {
    _actionCtrl.clear();
    _entityCtrl.clear();
    _actorCtrl.clear();
    ref
        .read(auditLogListProvider.notifier)
        .applyFilters(const AuditLogFilter());
  }

  Future<void> _export() async {
    setState(() => _exporting = true);
    final f = ref.read(auditLogListProvider.notifier).filter;
    final res =
        await ref.read(lgpdControllerProvider.notifier).exportAuditCSV(f);
    if (!mounted) return;
    setState(() => _exporting = false);
    if (res.error != null || res.csv == null) {
      SaveFeedback.showError(
          context, res.error ?? t.settings.audit_export_failed);
      return;
    }
    await Clipboard.setData(ClipboardData(text: res.csv!));
    if (!mounted) return;
    SaveFeedback.showSuccess(context, t.settings.audit_export_copied);
  }

  @override
  Widget build(BuildContext context) {
    final colors = widget.colors;
    final fieldStyle = TextStyle(color: colors.textPrimary, fontSize: 12);
    final labelStyle = TextStyle(color: colors.textMuted, fontSize: 12);
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        SizedBox(
          width: 180,
          child: TextField(
            controller: _actionCtrl,
            style: fieldStyle,
            decoration: InputDecoration(
              isDense: true,
              labelText: t.settings.audit_filter_action,
              labelStyle: labelStyle,
            ),
          ),
        ),
        SizedBox(
          width: 160,
          child: TextField(
            controller: _entityCtrl,
            style: fieldStyle,
            decoration: InputDecoration(
              isDense: true,
              labelText: t.settings.audit_filter_entity,
              labelStyle: labelStyle,
            ),
          ),
        ),
        SizedBox(
          width: 180,
          child: TextField(
            controller: _actorCtrl,
            style: fieldStyle,
            decoration: InputDecoration(
              isDense: true,
              labelText: t.settings.audit_filter_actor,
              labelStyle: labelStyle,
            ),
          ),
        ),
        ElevatedButton.icon(
          onPressed: _apply,
          icon: const Icon(Icons.filter_alt, size: 16),
          label: Text(t.settings.audit_filter_apply),
          style: ElevatedButton.styleFrom(
            backgroundColor: colors.bluePrimary,
            foregroundColor: Colors.white,
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
          ),
        ),
        TextButton(
          onPressed: _clear,
          child: Text(t.settings.audit_filter_clear,
              style: TextStyle(color: colors.textSecondary)),
        ),
        const SizedBox(width: 8),
        OutlinedButton.icon(
          onPressed: _exporting ? null : _export,
          icon: _exporting
              ? const SizedBox(
                  width: 14,
                  height: 14,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Icon(Icons.download, size: 16),
          label: Text(t.settings.audit_export_cta),
        ),
      ],
    );
  }
}
