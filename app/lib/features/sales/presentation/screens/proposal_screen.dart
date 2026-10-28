import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/utils/currency_format.dart';
import 'package:app/design_system/components/empty_state_card.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/product_model.dart';
import 'package:app/features/sales/domain/models/proposal.dart';
import 'package:app/features/sales/presentation/controllers/lead_detail_controller.dart';
import 'package:app/features/sales/presentation/controllers/products_controller.dart';
import 'package:app/features/sales/presentation/controllers/proposal_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Compose a priced offer for one customer (pillar 3).
///
/// The screen is a form with one honest ending: **Enviar para aprovação** puts
/// the offer in the queue and reaches nobody. A button that said "enviar" would
/// be a lie, and the whole product rests on the customer trusting that nothing
/// leaves without her.
class ProposalScreen extends ConsumerStatefulWidget {
  const ProposalScreen({super.key, required this.leadId});

  final String leadId;

  @override
  ConsumerState<ProposalScreen> createState() => _ProposalScreenState();
}

class _ProposalScreenState extends ConsumerState<ProposalScreen> {
  final _bodyController = TextEditingController();
  bool _sending = false;
  Proposal? _queued;

  @override
  void dispose() {
    _bodyController.dispose();
    super.dispose();
  }

  /// Products a proposal may contain.
  ///
  /// Inactive ones are excluded, not greyed out: offering something the company
  /// stopped selling is a promise it cannot keep, and a disabled row invites the
  /// question "why not?" at the moment somebody is trying to close.
  List<ProductModel> _offerable(List<ProductModel> all) =>
      all.where((p) => p.active).toList();

  Future<void> _send(List<ProductModel> catalogue) async {
    final lines = ref.read(proposalDraftLinesProvider.notifier).lines;
    if (lines.isEmpty) return;

    setState(() => _sending = true);
    final actions = ref.read(proposalActionsProvider);
    try {
      // Two calls, one gesture. Splitting them into two buttons would leave the
      // person wondering what the first one did with an offer nobody has seen.
      final created = await actions.create(
        leadId: widget.leadId,
        lines: lines,
        body: _bodyController.text,
      );
      final queued = await actions.sendForApproval(proposalId: created.id);
      if (!mounted) return;
      setState(() => _queued = queued);
      ref.read(proposalDraftLinesProvider.notifier).clear();
      ref.invalidate(leadProposalsProvider(widget.leadId));
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          key: const Key('proposal_error'),
          content: Text(parseApiError(e)),
          behavior: SnackBarBehavior.floating,
        ),
      );
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  /// Opens the form with the matcher's picks already in it.
  ///
  /// Post-frame because a notifier cannot be written during build, and
  /// unconditional because `suggest` is idempotent — it refuses to run twice,
  /// so a rebuild cannot resurrect a line the seller just removed.
  void _suggestFromLead() {
    final lead = ref.read(leadDetailProvider(widget.leadId)).asData?.value;
    if (lead == null || lead.matchedProducts.isEmpty) return;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      ref
          .read(proposalDraftLinesProvider.notifier)
          .suggest(lead.matchedProducts.map((m) => m.productId));
    });
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final productsAsync = ref.watch(productsListProvider);
    // Watched, not read: the lead usually resolves a frame or two after this
    // screen mounts, and seeding only on the first build would leave the form
    // blank exactly when the matcher had something to say.
    ref.watch(leadDetailProvider(widget.leadId));
    _suggestFromLead();

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundPrimary,
        // Reached from a lead and rendered outside the shell: no rail, no bottom
        // bar, so this is the only way out.
        leading: IconButton(
          key: const Key('proposal_back'),
          tooltip: t.sales.back_tooltip,
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () =>
              context.canPop() ? context.pop() : context.go('/funnel'),
        ),
        // WHO this offer is for, under the title. Composing a priced offer
        // with no visual confirmation of the recipient is how the wrong
        // clinic gets the proposal — the lead rides in the URL and nothing
        // on screen said its name.
        title: Builder(builder: (context) {
          final detail = widget.leadId.isEmpty
              ? null
              : ref.watch(leadDetailProvider(widget.leadId)).asData?.value;
          final recipient = detail?.recipient.trim() ?? '';
          if (recipient.isEmpty) return Text(t.proposal.title);
          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(t.proposal.title),
              Text(
                '${t.lead_detail.confirm_to}: $recipient',
                key: const Key('proposal_recipient'),
                style: TextStyle(fontSize: 12, color: colors.textSecondary),
              ),
            ],
          );
        }),
      ),
      body: productsAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => RetryableError(
          error: e,
          onRetry: () => ref.invalidate(productsListProvider),
        ),
        data: (all) {
          final catalogue = _offerable(all);
          if (catalogue.isEmpty) {
            // A picker over an empty catalogue is a dead end. Say what is
            // missing and where it lives.
            return Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                children: [
                  EmptyStateCard(
                    key: const Key('proposal_no_products'),
                    icon: Icons.inventory_2_outlined,
                    title: t.proposal.no_products,
                    subtitle: t.proposal.no_products_hint,
                  ),
                  const SizedBox(height: 12),
                  TextButton.icon(
                    key: const Key('proposal_go_products'),
                    onPressed: () => context.go('/sales/products'),
                    icon: const Icon(Icons.add),
                    label: Text(t.proposal.go_to_products),
                  ),
                ],
              ),
            );
          }
          return _Composer(
            leadId: widget.leadId,
            catalogue: catalogue,
            bodyController: _bodyController,
            sending: _sending,
            queued: _queued,
            onSend: () => _send(catalogue),
          );
        },
      ),
    );
  }
}

class _Composer extends ConsumerWidget {
  const _Composer({
    required this.leadId,
    required this.catalogue,
    required this.bodyController,
    required this.sending,
    required this.queued,
    required this.onSend,
  });

  final String leadId;
  final List<ProductModel> catalogue;
  final TextEditingController bodyController;
  final bool sending;
  final Proposal? queued;
  final VoidCallback onSend;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final quantities = ref.watch(proposalDraftLinesProvider);
    final chosen = catalogue.where((p) => (quantities[p.id] ?? 0) > 0).toList();

    // The currencies actually in play. Checked HERE rather than only on the
    // server: the server is the authority, but letting somebody build an offer
    // it will refuse wastes the work they just did.
    final currencies =
        chosen.map((p) => p.currency.trim().toUpperCase()).toSet();
    final mixed = currencies.length > 1;
    final currency = currencies.length == 1 ? currencies.first : '';

    final totalCents = chosen.fold<int>(
      0,
      (sum, p) => sum + p.priceCents * (quantities[p.id] ?? 0),
    );

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        if (queued != null) _QueuedNotice(proposal: queued!),
        // Said out loud, because a form that filled itself is unsettling until
        // someone tells you it is only a draft. Rendered only when the picks
        // actually came from the matcher — otherwise it would take credit for
        // a form the seller filled themselves.
        if (ref
                .watch(leadDetailProvider(leadId))
                .asData
                ?.value
                .matchedProducts
                .isNotEmpty ??
            false)
          Padding(
            padding: const EdgeInsets.only(bottom: 10),
            child: Text(
              t.proposal.suggested_hint,
              key: const Key('proposal_suggested_hint'),
              style: TextStyle(color: colors.textSecondary, fontSize: 12.5),
            ),
          ),
        Text(t.proposal.pick_products,
            style: TextStyle(color: colors.textMuted, fontSize: 13)),
        const SizedBox(height: 8),
        for (final product in catalogue)
          _ProductRow(product: product, quantity: quantities[product.id] ?? 0),
        const SizedBox(height: 16),

        // Mixed currencies have no total. Saying so is the only honest answer:
        // summing them invents a number and picking one misstates the rest.
        if (mixed)
          Container(
            key: const Key('proposal_mixed_currency'),
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: colors.redAlert.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Text(
              t.proposal.mixed_currency,
              style: TextStyle(color: colors.textPrimary, fontSize: 13),
            ),
          )
        else if (chosen.isNotEmpty)
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(t.proposal.total,
                  style: TextStyle(
                      color: colors.textSecondary,
                      fontWeight: FontWeight.w600)),
              Text(
                // Never a bare number: the amount and its currency travel
                // together everywhere in this codebase.
                formatCurrencyCents(totalCents, currency),
                key: const Key('proposal_total'),
                style: TextStyle(
                  color: colors.textPrimary,
                  fontSize: 18,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),

        const SizedBox(height: 16),
        TextField(
          key: const Key('proposal_body'),
          controller: bodyController,
          maxLines: 4,
          minLines: 3,
          style: TextStyle(color: colors.textPrimary),
          decoration: InputDecoration(
            labelText: t.proposal.body_label,
            helperText: t.proposal.body_hint,
            helperMaxLines: 2,
            labelStyle: TextStyle(color: colors.textMuted),
            helperStyle: TextStyle(color: colors.textMuted),
          ),
        ),
        const SizedBox(height: 20),

        // The label names the destination, and the line under it names what does
        // NOT happen. Everything about this product rests on that being true.
        FilledButton.icon(
          key: const Key('proposal_send'),
          onPressed: (chosen.isEmpty || mixed || sending) ? null : onSend,
          icon: sending
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Icon(Icons.rule_outlined),
          label: Text(t.proposal.send_for_approval),
        ),
        const SizedBox(height: 6),
        Text(
          t.proposal.approval_note,
          key: const Key('proposal_approval_note'),
          style: TextStyle(color: colors.textMuted, fontSize: 12),
        ),

        const SizedBox(height: 24),
        _History(leadId: leadId),
      ],
    );
  }
}

class _ProductRow extends ConsumerWidget {
  const _ProductRow({required this.product, required this.quantity});

  final ProductModel product;
  final int quantity;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final notifier = ref.read(proposalDraftLinesProvider.notifier);

    return ListTile(
      key: Key('proposal_product_${product.id}'),
      contentPadding: EdgeInsets.zero,
      title: Text(product.name,
          style: TextStyle(
              color: colors.textPrimary, fontWeight: FontWeight.w600)),
      // The unit price is on the row because the decision is "is this worth
      // offering", and that question needs the number next to the name.
      subtitle: Text(
        formatCurrencyCents(product.priceCents, product.currency),
        style: TextStyle(color: colors.textMuted, fontSize: 13),
      ),
      trailing: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          IconButton(
            key: Key('proposal_minus_${product.id}'),
            onPressed: quantity == 0 ? null : () => notifier.remove(product.id),
            icon: const Icon(Icons.remove_circle_outline),
            color: colors.textMuted,
          ),
          SizedBox(
            width: 24,
            child: Text(
              '$quantity',
              key: Key('proposal_qty_${product.id}'),
              textAlign: TextAlign.center,
              style: TextStyle(
                  color: colors.textPrimary, fontWeight: FontWeight.w700),
            ),
          ),
          IconButton(
            key: Key('proposal_plus_${product.id}'),
            onPressed: () => notifier.add(product.id),
            icon: const Icon(Icons.add_circle_outline),
            color: colors.bluePrimary,
          ),
        ],
      ),
    );
  }
}

/// What the approval queue will show, after the offer is queued.
///
/// The rendered text comes from the server so what she reads here is byte for
/// byte what the reviewer will read — a second renderer in the client would
/// eventually disagree with the one that actually sends.
class _QueuedNotice extends ConsumerStatefulWidget {
  const _QueuedNotice({required this.proposal});

  final Proposal proposal;

  @override
  ConsumerState<_QueuedNotice> createState() => _QueuedNoticeState();
}

class _QueuedNoticeState extends ConsumerState<_QueuedNotice> {
  bool _saving = false;

  Proposal get proposal => widget.proposal;

  /// Hands the seller the file.
  ///
  /// Separate from the queue on purpose: the queue is how an offer REACHES a
  /// customer, and this is for the buyer who asked for the price on WhatsApp
  /// and is waiting right now. Being second with the price is the commonest
  /// way to lose a deal in this market.
  Future<void> _download() async {
    setState(() => _saving = true);
    try {
      final ok = await ref
          .read(proposalActionsProvider)
          .downloadPdf(proposalId: proposal.id);
      if (!mounted) return;
      if (!ok) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(t.proposal.download_failed)),
        );
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(parseApiError(e))),
      );
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Container(
      key: const Key('proposal_queued'),
      margin: const EdgeInsets.only(bottom: 20),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.whatsappGreen.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(t.proposal.queued_title,
              style: TextStyle(
                  color: colors.textPrimary, fontWeight: FontWeight.w700)),
          const SizedBox(height: 6),
          Text(proposal.rendered,
              style: TextStyle(color: colors.textSecondary, fontSize: 13)),
          const SizedBox(height: 8),
          Text(t.proposal.download_hint,
              style: TextStyle(color: colors.textMuted, fontSize: 12)),
          const SizedBox(height: 4),
          Wrap(
            spacing: 4,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              // The file, first. Everything else on this card is about a
              // queue; this is the thing the customer is waiting for.
              TextButton.icon(
                key: const Key('proposal_download_pdf'),
                onPressed: _saving ? null : _download,
                icon: _saving
                    ? const SizedBox(
                        width: 15,
                        height: 15,
                        child: CircularProgressIndicator(strokeWidth: 2))
                    : const Icon(Icons.picture_as_pdf_outlined, size: 16),
                label: Text(t.proposal.download_pdf),
              ),
              // "It went to the queue" without a door to the queue is a dead
              // end — the person had to navigate there by memory.
              TextButton.icon(
                key: const Key('proposal_open_queue'),
                onPressed: () => context.go('/prospect?tab=queue'),
                icon: const Icon(Icons.rule_outlined, size: 16),
                label: Text(t.lead_detail.queue_title),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

/// Offers already made to this customer.
///
/// The reason the object has a history at all: the last offer is what shapes the
/// next one, and repeating an offer somebody already declined reads as amnesia.
class _History extends ConsumerWidget {
  const _History({required this.leadId});

  final String leadId;

  /// The same file, from an offer made weeks ago.
  ///
  /// A buyer who asks for the price again does not want a new proposal — they
  /// want the one you already sent. Without this the seller had to rebuild it
  /// line by line, at which point the numbers drift and the two documents
  /// disagree about what was offered.
  Future<void> _download(BuildContext context, WidgetRef ref, String id) async {
    try {
      final ok =
          await ref.read(proposalActionsProvider).downloadPdf(proposalId: id);
      if (!context.mounted || ok) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(t.proposal.download_failed)));
    } catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(parseApiError(e))));
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final async = ref.watch(leadProposalsProvider(leadId));

    return async.maybeWhen(
      data: (list) {
        if (list.isEmpty) return const SizedBox.shrink();
        return Column(
          key: const Key('proposal_history'),
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(t.proposal.history,
                style: TextStyle(
                    color: colors.textMuted,
                    fontSize: 13,
                    fontWeight: FontWeight.w700)),
            const SizedBox(height: 8),
            for (final p in list)
              ListTile(
                key: Key('proposal_history_${p.id}'),
                contentPadding: EdgeInsets.zero,
                title: Text(
                  formatCurrencyCents(p.totalCents, p.currency),
                  style: TextStyle(
                      color: colors.textPrimary, fontWeight: FontWeight.w600),
                ),
                subtitle: Text(
                  p.sentAt == null
                      ? t.proposal.status_draft
                      : t.proposal.status_sent(
                          date: _shortDate(p.sentAt!),
                        ),
                  style: TextStyle(color: colors.textMuted, fontSize: 12),
                ),
                trailing: IconButton(
                  key: Key('proposal_history_pdf_${p.id}'),
                  tooltip: t.proposal.download_pdf,
                  icon: const Icon(Icons.picture_as_pdf_outlined, size: 18),
                  onPressed: () => _download(context, ref, p.id),
                ),
              ),
          ],
        );
      },
      // A history that failed to load says nothing rather than claiming there is
      // none: "no previous offers" is a claim, and a failed query has not earned
      // the right to make it.
      orElse: () => const SizedBox.shrink(),
    );
  }

  String _shortDate(DateTime d) =>
      '${d.day.toString().padLeft(2, '0')}/${d.month.toString().padLeft(2, '0')}/${d.year}';
}
