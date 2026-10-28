import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/components/custom_text_field.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/settings/presentation/controllers/email_dns_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Sending-domain authentication for an email channel (US-M006 / US-M022).
///
/// Shown on the email channel's card because it is not a nice-to-have: until
/// SPF, DKIM and DMARC are live, cold mail from this domain is dropped by the
/// big providers — silently, with a successful SMTP handoff. The card lists
/// every record with its own status and a one-tap copy, so the customer can
/// work through it inside their registrar without guessing.
class EmailDnsCard extends ConsumerStatefulWidget {
  const EmailDnsCard({super.key, required this.channelId});

  final String channelId;

  @override
  ConsumerState<EmailDnsCard> createState() => _EmailDnsCardState();
}

class _EmailDnsCardState extends ConsumerState<EmailDnsCard> {
  Timer? _poll;
  int _polls = 0;
  bool _applying = false;

  /// DNS propagation is minutes, not seconds, and the user has just come
  /// back from their registrar. Making them hunt for a refresh button while
  /// they wait is how a correct setup gets abandoned as broken — so the card
  /// re-checks itself, and stops the moment it goes green.
  static const _pollEvery = Duration(seconds: 20);

  /// Roughly ten minutes. Past that the problem is not propagation, it is a
  /// wrong record — and polling forever would keep a DNS lookup running
  /// server-side for every abandoned tab.
  static const _maxPolls = 30;

  bool get _pollsExhausted => _polls >= _maxPolls;

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }

  void _schedulePoll(bool verified) {
    if (verified || _polls >= _maxPolls) {
      _poll?.cancel();
      _poll = null;
      return;
    }
    _poll ??= Timer.periodic(_pollEvery, (_) {
      if (!mounted) return;
      _polls++;
      ref.invalidate(emailDnsProvider(widget.channelId));
    });
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final state = ref.watch(emailDnsProvider(widget.channelId));
    state.whenData((s) => _schedulePoll(s.verified));

    return Container(
      key: const Key('email_dns_card'),
      margin: const EdgeInsets.only(top: 12),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderMedium),
      ),
      child: state.when(
        loading: () => const Padding(
          padding: EdgeInsets.symmetric(vertical: 12),
          child: Center(child: CircularProgressIndicator()),
        ),
        error: (err, _) => Row(
          children: [
            Expanded(
              child: Text(
                parseApiError(err),
                key: const Key('email_dns_error'),
                style: TextStyle(color: colors.redAlert, fontSize: 12),
              ),
            ),
            TextButton(
              onPressed: () =>
                  ref.invalidate(emailDnsProvider(widget.channelId)),
              child: Text(t.common.retry),
            ),
          ],
        ),
        data: (setup) => Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(
                  setup.verified
                      ? Icons.verified_outlined
                      : Icons.warning_amber,
                  size: 18,
                  color: setup.verified
                      ? colors.whatsappGreen
                      : colors.orangeAlert,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    setup.providerManaged
                        ? t.channels.email_dns_provider_managed
                        : setup.verified
                            ? t.channels.email_dns_verified
                            : _pollsExhausted
                                ? t.channels.email_dns_poll_stopped
                                : t.channels.email_dns_pending,
                    key: const Key('email_dns_headline'),
                    style: TextStyle(
                      color: colors.textPrimary,
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                IconButton(
                  key: const Key('email_dns_recheck'),
                  tooltip: t.channels.email_dns_recheck,
                  onPressed: () {
                    // A manual check re-arms the auto-poll: the person just
                    // FIXED something, and the exhausted state was about the
                    // previous attempt.
                    _polls = 0;
                    _poll?.cancel();
                    _poll = null;
                    ref.invalidate(emailDnsProvider(widget.channelId));
                  },
                  icon: Icon(Icons.refresh,
                      size: 18, color: colors.textSecondary),
                ),
              ],
            ),
            if (setup.domain.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(top: 2, bottom: 8),
                child: Text(
                  setup.domain,
                  style: TextStyle(color: colors.textMuted, fontSize: 12),
                ),
              ),
            if (!setup.verified) _buildHelp(setup, colors),
            if (!setup.verified) _buildAskForHelp(setup, colors),
            for (final record in setup.records)
              _RecordRow(record: record, colors: colors),
          ],
        ),
      ),
    );
  }

  /// Where to go and, when we can, an offer to do it for them.
  ///
  /// The detected nameserver is worth nothing as a slug — it does not tell
  /// anyone which menu to open. Turned into the provider's own documentation
  /// link, plus the one-click button where the zone is one we can write to.
  ///
  /// An UNRECOGNISED provider is a normal case, not a corner: professional
  /// domains live on Locaweb, KingHost, whatever the web agency picked —
  /// rendering nothing here left exactly those customers with a list of
  /// records and no idea where they go. They get the generic walkthrough.
  Widget _buildHelp(EmailDnsSetup setup, AppColorsExtension colors) {
    final guide = dnsGuideFor(setup.nameserverHint);

    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (guide != null)
            Row(
              children: [
                Icon(Icons.dns_outlined, size: 15, color: colors.textSecondary),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    t.channels.email_dns_detected(provider: guide.name),
                    key: const Key('email_dns_provider'),
                    style: TextStyle(color: colors.textSecondary, fontSize: 12),
                  ),
                ),
                TextButton(
                  key: const Key('email_dns_guide'),
                  onPressed: () => launchUrl(Uri.parse(guide.url),
                      mode: LaunchMode.externalApplication),
                  child: Text(t.channels.email_dns_open_guide),
                ),
              ],
            )
          else
            // No provider assumed. Says where DNS usually LIVES (the panel
            // of whoever sold the domain or hosts the site) and what to do
            // with the records below — the whole path, not just the data.
            Container(
              key: const Key('email_dns_generic_help'),
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: colors.glassMedium,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Text(
                t.channels.email_dns_generic_steps,
                style: TextStyle(
                    color: colors.textSecondary, fontSize: 12, height: 1.45),
              ),
            ),
          // The universal instruction, provider known or not: what to DO
          // with the record rows below. "Here are three records" without
          // "create each one in your DNS zone using the copy buttons" is
          // data, not a task.
          Padding(
            padding: const EdgeInsets.only(top: 6),
            child: Text(
              t.channels.email_dns_how_to,
              key: const Key('email_dns_how_to'),
              style: TextStyle(color: colors.textMuted, fontSize: 11.5),
            ),
          ),
          if (guide != null && guide.canAutoConfigure)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: FilledButton.icon(
                key: const Key('email_dns_auto'),
                onPressed: _applying ? null : () => _autoConfigure(guide),
                icon: _applying
                    ? const SizedBox(
                        width: 14,
                        height: 14,
                        child: CircularProgressIndicator(strokeWidth: 2))
                    : const Icon(Icons.bolt, size: 18),
                label: Text(t.channels.email_dns_auto_configure),
              ),
            ),
        ],
      ),
    );
  }

  /// Opens support with the problem already described.
  ///
  /// The card knows the domain, the detected provider and exactly which
  /// records are missing. Making someone retype that is how a support thread
  /// starts with half the facts and takes three round trips to get going.
  Widget _buildAskForHelp(EmailDnsSetup setup, AppColorsExtension colors) {
    final missing = setup.records
        .where((r) => r.required_ && r.status != DnsRecordStatus.ok)
        .map((r) => r.kind.toUpperCase())
        .join(', ');
    final guide = dnsGuideFor(setup.nameserverHint);
    final draft = t.channels.email_dns_help_draft(
      domain: setup.domain,
      provider: guide?.name ?? t.channels.email_dns_provider_unknown,
      records: missing.isEmpty ? '—' : missing,
    );

    return Align(
      alignment: Alignment.centerLeft,
      child: TextButton.icon(
        key: const Key('email_dns_help'),
        onPressed: () =>
            context.go('/support?draft=${Uri.encodeComponent(draft)}'),
        icon: Icon(Icons.support_agent, size: 16, color: colors.textSecondary),
        label: Text(t.channels.email_dns_need_help,
            style: TextStyle(color: colors.textSecondary, fontSize: 12)),
      ),
    );
  }

  Future<void> _autoConfigure(DnsProviderGuide guide) async {
    final token = await _askForToken(guide);
    if (token == null || token.isEmpty) return;

    final messenger = ScaffoldMessenger.of(context);
    setState(() => _applying = true);
    try {
      final report = await ref.read(dnsApplyActionsProvider).apply(
            channelId: widget.channelId,
            provider: guide.slug,
            apiToken: token,
          );
      ref.invalidate(emailDnsProvider(widget.channelId));
      // Named record by record, not "pronto": the outcomes that matter are
      // the ones where we found something of theirs — a merged SPF, a DMARC
      // policy we refused to weaken — and a bare success message would hide
      // exactly those.
      if (mounted) await _showReport(report);
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: const Key('email_dns_auto_error'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _applying = false);
    }
  }

  Future<String?> _askForToken(DnsProviderGuide guide) {
    final controller = TextEditingController();
    return showDialog<String>(
      context: context,
      builder: (ctx) {
        final colors = Theme.of(ctx).extension<AppColorsExtension>()!;
        return AlertDialog(
          backgroundColor: colors.backgroundSecondary,
          // The provider's NAME in the title — a fixed "Cloudflare" asked
          // Registro.br customers for a credential from a company they have
          // no account with.
          title: Text(
              t.channels.email_dns_token_title_named(provider: guide.name)),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Says plainly that the token is not kept. Asking someone to
              // paste a credential that can redirect their company's mail
              // without saying what happens to it is not a request anyone
              // should grant.
              Text(
                t.channels.email_dns_token_help(provider: guide.name),
                style: TextStyle(color: colors.textSecondary, fontSize: 12),
              ),
              const SizedBox(height: 12),
              CustomTextField(
                key: const Key('email_dns_token_field'),
                controller: controller,
                hint: t.channels.email_dns_token_hint,
                // Obscured: this is a credential that can redirect a
                // company's mail, and it is being pasted on a screen someone
                // may well be sharing.
                isPassword: true,
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: Text(t.common.cancel),
            ),
            FilledButton(
              key: const Key('email_dns_token_confirm'),
              onPressed: () => Navigator.of(ctx).pop(controller.text.trim()),
              child: Text(t.channels.email_dns_auto_configure),
            ),
          ],
        );
      },
    );
  }

  Future<void> _showReport(DnsApplyReport report) {
    final lines = report.results
        .map((r) => '${r.kind.toUpperCase()}: ${_actionLabel(r.action)}'
            '${r.detail.isEmpty ? '' : '\n   ${r.detail}'}')
        .join('\n');
    // Confirm-shaped because it is the one dialog that has to be READ: the
    // outcomes that matter are the ones where we found something of theirs
    // and did not destroy it.
    return showConfirmDialog(
      context,
      title: report.anyFailed
          ? t.channels.email_dns_auto_partial
          : t.channels.email_dns_auto_done,
      message: '${report.zone}\n\n$lines',
      confirmText: t.common.close,
      cancelText: '',
    );
  }

  String _actionLabel(String action) => switch (action) {
        'created' => t.channels.dns_action_created,
        'updated' => t.channels.dns_action_updated,
        'merged' => t.channels.dns_action_merged,
        'kept' => t.channels.dns_action_kept,
        _ => t.channels.dns_action_failed,
      };
}

class _RecordRow extends StatelessWidget {
  const _RecordRow({required this.record, required this.colors});

  final EmailDnsRecord record;
  final AppColorsExtension colors;

  @override
  Widget build(BuildContext context) {
    final (icon, tint) = switch (record.status) {
      DnsRecordStatus.ok => (Icons.check_circle, colors.whatsappGreen),
      DnsRecordStatus.mismatch => (Icons.error_outline, colors.redAlert),
      DnsRecordStatus.pending => (Icons.schedule, colors.textMuted),
    };

    return Padding(
      key: Key('email_dns_record_${record.kind}'),
      padding: const EdgeInsets.only(top: 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(icon, size: 15, color: tint),
              const SizedBox(width: 6),
              Text(
                '${record.kind.toUpperCase()} · ${record.type}',
                style: TextStyle(
                  color: colors.textPrimary,
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                ),
              ),
              if (!record.required_) ...[
                const SizedBox(width: 6),
                Text(
                  t.channels.email_dns_optional,
                  style: TextStyle(color: colors.textMuted, fontSize: 11),
                ),
              ],
            ],
          ),
          if (record.purpose.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(left: 21, top: 2),
              child: Text(
                record.purpose,
                style: TextStyle(color: colors.textMuted, fontSize: 11),
              ),
            ),
          // A green row still showing "publish this" copy reads as an
          // instruction — the DMARC-with-a-check confusion. Say plainly that
          // this one is done.
          if (record.status == DnsRecordStatus.ok)
            Padding(
              padding: const EdgeInsets.only(left: 21, top: 2),
              child: Text(
                t.channels.email_dns_already_published,
                key: Key('email_dns_done_${record.kind}'),
                style: TextStyle(color: colors.whatsappGreen, fontSize: 11),
              ),
            ),
          Padding(
            padding: const EdgeInsets.only(left: 21, top: 6),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Both spellings of the host: registrars disagree about
                // whether they want the FQDN or only the subdomain, and
                // guessing wrong is the most common setup mistake.
                _CopyLine(
                  label: t.channels.email_dns_host,
                  value: record.host,
                  secondary:
                      record.hostShort != record.host ? record.hostShort : null,
                  colors: colors,
                ),
                const SizedBox(height: 4),
                _CopyLine(
                  label: t.channels.email_dns_value,
                  value: record.value,
                  colors: colors,
                ),
                // A published-but-wrong record is only fixable if the user
                // can see what is actually out there.
                if (record.status == DnsRecordStatus.mismatch &&
                    record.found.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: Text(
                      '${t.channels.email_dns_found}: ${record.found}',
                      key: Key('email_dns_found_${record.kind}'),
                      style: TextStyle(color: colors.redAlert, fontSize: 11),
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _CopyLine extends StatelessWidget {
  const _CopyLine({
    required this.label,
    required this.value,
    required this.colors,
    this.secondary,
  });

  final String label;
  final String value;
  final String? secondary;
  final AppColorsExtension colors;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 48,
          child: Text(label,
              style: TextStyle(color: colors.textMuted, fontSize: 11)),
        ),
        Expanded(
          child: SelectableText(
            secondary == null ? value : '$value  ($secondary)',
            style: TextStyle(
              color: colors.textSecondary,
              fontSize: 11,
              fontFamily: 'monospace',
            ),
          ),
        ),
        InkWell(
          onTap: () async {
            await Clipboard.setData(ClipboardData(text: value));
            if (!context.mounted) return;
            ScaffoldMessenger.of(context).showSnackBar(SnackBar(
              content: Text(t.channels.email_dns_copied),
              behavior: SnackBarBehavior.floating,
            ));
          },
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 4),
            child: Icon(Icons.copy, size: 14, color: colors.textMuted),
          ),
        ),
      ],
    );
  }
}
