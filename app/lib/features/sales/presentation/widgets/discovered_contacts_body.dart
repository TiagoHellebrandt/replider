import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/empty_state_card.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/discovered_contact.dart';
import 'package:app/features/sales/domain/models/discovered_contact.dart'
    show DiscoveredContact;
import 'package:app/features/sales/presentation/controllers/discovered_contacts_controller.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';
import 'package:app/features/settings/presentation/controllers/channels_controller.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// "Traz os clientes que você já tem."
///
/// The screen is a REVIEW, and every decision on it belongs to the person
/// reading. The scan reads her Sent folder's envelopes — who she wrote to and
/// when, never what she said — and everything it finds arrives here as a
/// candidate. Nothing reaches the CRM until she ticks it.
///
/// There is deliberately no "import everything" button. The value of this
/// screen is that a human looked at the list; a bulk action would make looking
/// optional, and the names it would import are people she will then be emailing.
class DiscoveredContactsBody extends ConsumerStatefulWidget {
  const DiscoveredContactsBody({super.key, this.channelId});

  /// The email channel to scan. Usually left null: the body resolves the
  /// account's email channel itself, because being asked for a channel id is not
  /// a question anybody reviewing their own customers can answer.
  ///
  /// With no email channel at all there is nothing to dial, so the scan button
  /// hides rather than failing on tap.
  final String? channelId;

  @override
  ConsumerState<DiscoveredContactsBody> createState() =>
      _DiscoveredContactsBodyState();
}

class _DiscoveredContactsBodyState
    extends ConsumerState<DiscoveredContactsBody> {
  final _selected = <String>{};
  String? _funnelId;
  bool _busy = false;
  bool _scanning = false;
  SentScanResult? _lastScan;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final state = ref.watch(discoveredContactsProvider);

    // No Scaffold and no AppBar: this is a VIEW of the Clientes destination, so
    // the shell's rail and bottom bar are already on screen. It used to mount
    // outside the shell at `/sales/my-customers` and had to invent its own back
    // button to avoid being a dead end.
    return SafeArea(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(20, 16, 20, 8),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Said before anything is read, not buried in a settings
                // page: she is about to let a tool open her mailbox, and what
                // it does and does not read is the first thing she should
                // know.
                Text(
                  t.my_customers.subtitle,
                  key: const Key('discovered_privacy_note'),
                  style: typography.body
                      .copyWith(color: colors.textSecondary, fontSize: 13),
                ),
                if (_emailChannelId != null) ...[
                  const SizedBox(height: 12),
                  _ScanButton(
                    scanning: _scanning,
                    onScan: _scan,
                  ),
                ],
                if (_lastScan != null) ...[
                  const SizedBox(height: 8),
                  _ScanSummary(result: _lastScan!),
                ],
              ],
            ),
          ),
          Expanded(
            child: state.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (err, _) => RetryableError(
                error: err,
                onRetry: () => ref.invalidate(discoveredContactsProvider),
              ),
              data: (page) => page.items.isEmpty
                  ? Center(
                      child: Padding(
                        padding: const EdgeInsets.all(24),
                        child: EmptyStateCard(
                          key: const Key('discovered_empty'),
                          icon: Icons.mark_email_read_outlined,
                          title: t.my_customers.empty_title,
                          subtitle: t.my_customers.empty_hint,
                        ),
                      ),
                    )
                  : _CandidateList(
                      page: page,
                      selected: _selected,
                      onToggle: (address, on) => setState(() => on
                          ? _selected.add(address)
                          : _selected.remove(address)),
                      onSelectAll: () => setState(() => _selected
                        ..clear()
                        ..addAll(page.items.map((c) => c.address))),
                      onClear: () => setState(_selected.clear),
                    ),
            ),
          ),
          if (_selected.isNotEmpty)
            _DecisionBar(
              count: _selected.length,
              busy: _busy,
              funnelId: _funnelId,
              onFunnelChanged: (id) => setState(() => _funnelId = id),
              onImport: _import,
              onDismiss: _dismiss,
            ),
        ],
      ),
    );
  }

  /// The email channel whose Sent folder gets read.
  ///
  /// Resolved here rather than demanded from the caller: "which channel?" is not
  /// a question somebody reviewing their own customers can answer, and the answer
  /// is always the account's email channel. An explicit [DiscoveredContactsBody.channelId]
  /// still wins, for the callers that already know.
  String? get _emailChannelId {
    final explicit = widget.channelId;
    if (explicit != null && explicit.isNotEmpty) return explicit;
    final channels = ref.watch(channelsListProvider).asData?.value;
    if (channels == null) return null;
    for (final c in channels) {
      if (c.type == ChannelType.email) return c.id;
    }
    return null;
  }

  Future<void> _scan() async {
    final channelId = _emailChannelId;
    if (channelId == null || _scanning) return;

    final messenger = ScaffoldMessenger.of(context);
    setState(() => _scanning = true);
    try {
      final result = await ref
          .read(discoveredContactActionsProvider)
          .scan(channelId: channelId);
      if (!mounted) return;
      setState(() => _lastScan = result);
      ref.invalidate(discoveredContactsProvider);
    } catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(
        key: const Key('discovered_scan_error'),
        content: Text(t.my_customers.scan_failed(error: parseApiError(e))),
      ));
    } finally {
      if (mounted) setState(() => _scanning = false);
    }
  }

  Future<void> _import() async {
    if (_busy) return;
    final funnelId = _funnelId;
    final messenger = ScaffoldMessenger.of(context);
    // A lead with no funnel is a row nobody sees, so the button refuses rather
    // than importing into nowhere.
    if (funnelId == null || funnelId.isEmpty) {
      messenger.showSnackBar(SnackBar(
        key: const Key('discovered_funnel_required'),
        content: Text(t.my_customers.funnel_required),
      ));
      return;
    }

    setState(() => _busy = true);
    try {
      final result = await ref.read(discoveredContactActionsProvider).import(
            addresses: _selected.toList(),
            funnelId: funnelId,
          );
      if (!mounted) return;
      setState(_selected.clear);
      ref.invalidate(discoveredContactsProvider);
      messenger.showSnackBar(SnackBar(
        key: const Key('discovered_import_result'),
        // Named when some were skipped: a number that does not match what she
        // ticked is a question, and answering it here saves a support message.
        content: Text(result.skipped == 0
            ? t.my_customers.imported(count: result.imported.toString())
            : t.my_customers.imported_partial(
                count: result.imported.toString(),
                skipped: result.skipped.toString())),
      ));
    } catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(
        key: const Key('discovered_import_error'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _dismiss() async {
    if (_busy) return;
    final messenger = ScaffoldMessenger.of(context);
    final count = _selected.length;

    setState(() => _busy = true);
    try {
      await ref
          .read(discoveredContactActionsProvider)
          .dismiss(addresses: _selected.toList());
      if (!mounted) return;
      setState(_selected.clear);
      ref.invalidate(discoveredContactsProvider);
      messenger.showSnackBar(SnackBar(
        key: const Key('discovered_dismiss_result'),
        content: Text(t.my_customers.dismissed(count: count.toString())),
      ));
    } catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(parseApiError(e))));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }
}

class _ScanButton extends StatelessWidget {
  const _ScanButton({required this.scanning, required this.onScan});

  final bool scanning;
  final VoidCallback onScan;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    if (scanning) {
      // The scan walks a mailbox and takes real seconds. A button that just
      // goes quiet reads as broken, so it says what it is doing.
      return Row(
        key: const Key('discovered_scanning'),
        children: [
          const SizedBox(
            width: 16,
            height: 16,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
          const SizedBox(width: 10),
          Text(t.my_customers.scanning,
              style: TextStyle(color: colors.textSecondary, fontSize: 13)),
        ],
      );
    }
    return FilledButton.icon(
      key: const Key('discovered_scan'),
      onPressed: onScan,
      icon: const Icon(Icons.search, size: 16),
      label: Text(t.my_customers.scan_action),
    );
  }
}

/// What the scan read, in the customer's own terms.
///
/// Reports the folder and the counts rather than a bare "done": a long history
/// that yields a short list needs an explanation, and "read 4000, found 62, 300
/// robots left out" is it.
class _ScanSummary extends StatelessWidget {
  const _ScanSummary({required this.result});

  final SentScanResult result;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Column(
      key: const Key('discovered_scan_summary'),
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          t.my_customers.scan_result(
            scanned: result.scanned.toString(),
            mailbox: result.mailbox,
            found: result.found.toString(),
          ),
          style: typography.body.copyWith(color: Colors.white, fontSize: 13),
        ),
        if (result.skipped > 0)
          Text(
            t.my_customers.scan_skipped(count: result.skipped.toString()),
            style:
                typography.body.copyWith(color: colors.textMuted, fontSize: 12),
          ),
        // Said out loud: the cap stopped the walk, so this is not everybody she
        // has ever written to, and implying otherwise would make her think a
        // missing customer was never there.
        if (!result.complete)
          Padding(
            padding: const EdgeInsets.only(top: 4),
            child: Text(
              t.my_customers.scan_partial,
              key: const Key('discovered_scan_partial'),
              style: typography.body
                  .copyWith(color: colors.orangeAlert, fontSize: 12),
            ),
          ),
      ],
    );
  }
}

class _CandidateList extends StatelessWidget {
  const _CandidateList({
    required this.page,
    required this.selected,
    required this.onToggle,
    required this.onSelectAll,
    required this.onClear,
  });

  final DiscoveredContactsPage page;
  final Set<String> selected;
  final void Function(String address, bool on) onToggle;
  final VoidCallback onSelectAll;
  final VoidCallback onClear;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 8),
          child: Row(
            children: [
              TextButton(
                key: const Key('discovered_select_all'),
                onPressed: onSelectAll,
                child: Text(t.my_customers.select_all),
              ),
              if (selected.isNotEmpty)
                TextButton(
                  key: const Key('discovered_clear'),
                  onPressed: onClear,
                  child: Text(t.my_customers.clear_selection),
                ),
              const Spacer(),
              // The page is capped; the total is not. Saying so keeps "select
              // all" from reading as "all of them".
              if (page.total > page.items.length)
                Text(
                  t.my_customers.showing_partial(
                    shown: page.items.length.toString(),
                    total: page.total.toString(),
                  ),
                  key: const Key('discovered_showing_partial'),
                  style: typography.body
                      .copyWith(color: colors.textMuted, fontSize: 12),
                ),
            ],
          ),
        ),
        Expanded(
          child: ListView.builder(
            key: const Key('discovered_list'),
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
            itemCount: page.items.length,
            itemBuilder: (context, i) {
              final c = page.items[i];
              return _CandidateRow(
                contact: c,
                checked: selected.contains(c.address),
                onChanged: (on) => onToggle(c.address, on),
              );
            },
          ),
        ),
      ],
    );
  }
}

class _CandidateRow extends StatelessWidget {
  const _CandidateRow({
    required this.contact,
    required this.checked,
    required this.onChanged,
  });

  final DiscoveredContact contact;
  final bool checked;
  final ValueChanged<bool> onChanged;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Container(
      key: Key('discovered_row_${contact.address}'),
      margin: const EdgeInsets.only(bottom: 8),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: checked ? colors.bluePrimary : colors.borderLight,
        ),
      ),
      child: CheckboxListTile(
        value: checked,
        onChanged: (v) => onChanged(v ?? false),
        controlAffinity: ListTileControlAffinity.leading,
        title: Text(
          contact.label,
          style: typography.body.copyWith(
              color: Colors.white, fontSize: 14, fontWeight: FontWeight.w600),
        ),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // The address is always shown, even when a name exists: two people
            // share a name, nobody shares an address, and she is about to email
            // whoever this is.
            Text(contact.address,
                style: typography.body
                    .copyWith(color: colors.textSecondary, fontSize: 12)),
            Text(
              _history(contact),
              key: Key('discovered_history_${contact.address}'),
              style: typography.body
                  .copyWith(color: colors.textMuted, fontSize: 12),
            ),
          ],
        ),
      ),
    );
  }

  /// "40 e-mails enviados · desde mar/2024 · último em jul/2026".
  ///
  /// This line IS the decision: it is how she tells a customer of two years from
  /// somebody who asked one question. Parts with no data are left out rather
  /// than rendered empty.
  static String _history(DiscoveredContact c) {
    final parts = <String>[
      t.my_customers.sent_count(count: c.sentCount.toString()),
      if (c.firstSentAt != null)
        t.my_customers.since(date: _monthYear(c.firstSentAt!)),
      if (c.lastSentAt != null)
        t.my_customers.last(date: _monthYear(c.lastSentAt!)),
    ];
    return parts.join(' · ');
  }

  static String _monthYear(DateTime d) =>
      '${d.month.toString().padLeft(2, '0')}/${d.year}';
}

/// The bar that appears once something is ticked.
class _DecisionBar extends ConsumerWidget {
  const _DecisionBar({
    required this.count,
    required this.busy,
    required this.funnelId,
    required this.onFunnelChanged,
    required this.onImport,
    required this.onDismiss,
  });

  final int count;
  final bool busy;
  final String? funnelId;
  final ValueChanged<String?> onFunnelChanged;
  final VoidCallback onImport;
  final VoidCallback onDismiss;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final funnels = ref.watch(funnelsListProvider);

    return Container(
      key: const Key('discovered_decision_bar'),
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 12),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        border: Border(top: BorderSide(color: colors.borderMedium)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.my_customers.selected(count: count.toString()),
            key: const Key('discovered_selected_count'),
            style: typography.body
                .copyWith(color: colors.textSecondary, fontSize: 13),
          ),
          const SizedBox(height: 8),
          funnels.when(
            loading: () => const LinearProgressIndicator(),
            error: (_, __) => Text(t.my_customers.no_funnels,
                style: TextStyle(color: colors.textMuted, fontSize: 12)),
            data: (list) => list.isEmpty
                // Without a funnel there is nowhere for a customer to land, so
                // the screen says what to do instead of offering a dead button.
                ? Text(
                    t.my_customers.no_funnels,
                    key: const Key('discovered_no_funnels'),
                    style: TextStyle(color: colors.orangeAlert, fontSize: 12),
                  )
                : DropdownButtonFormField<String>(
                    key: const Key('discovered_funnel_picker'),
                    initialValue: funnelId,
                    isExpanded: true,
                    decoration: InputDecoration(
                      labelText: t.my_customers.funnel_label,
                      isDense: true,
                    ),
                    items: [
                      for (final f in list)
                        DropdownMenuItem(value: f.id, child: Text(f.name)),
                    ],
                    onChanged: busy ? null : onFunnelChanged,
                  ),
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: FilledButton.icon(
                  key: const Key('discovered_import'),
                  onPressed: busy ? null : onImport,
                  icon: const Icon(Icons.person_add_alt, size: 16),
                  label: Text(t.my_customers.import_action),
                ),
              ),
              const SizedBox(width: 8),
              TextButton(
                key: const Key('discovered_dismiss'),
                onPressed: busy ? null : onDismiss,
                child: Text(t.my_customers.dismiss_action),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
