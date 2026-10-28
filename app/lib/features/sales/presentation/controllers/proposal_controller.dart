import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/core/utils/file_download.dart';
import 'package:app/features/sales/domain/models/proposal.dart';

/// Priced offers for one customer.
///
/// There is no "send" here, and that is the point rather than an omission: a
/// proposal reaches the customer only through the approval queue, so the last
/// action this controller exposes is `sendForApproval` — which puts the offer in
/// front of a person and delivers nothing.
///
/// Nor is there an EDIT. The screen composes a fresh offer each time, so the
/// API's `PUT /proposals/:id/{lines,body}` have no client caller yet. Declaring
/// them here anyway would be a capability nobody using the product can reach —
/// which the repo's own `capability_has_ui_test` refuses, correctly.

/// What this customer has already been offered, newest first.
final leadProposalsProvider =
    FutureProvider.autoDispose.family<List<Proposal>, String>(
  (ref, leadId) async {
    final dio = ref.watch(dioProvider);
    final res = await dio.get<List<dynamic>>('/leads/$leadId/proposals');
    return (res.data ?? const <dynamic>[])
        .whereType<Map<String, dynamic>>()
        .map(Proposal.fromJson)
        .toList();
  },
);

class ProposalActions {
  const ProposalActions(this._ref);

  final Ref _ref;

  /// Prices the lines against the account's catalogue and stores a draft.
  ///
  /// The server owns the pricing: it snapshots each product's name and price so
  /// a later reprice cannot restate an offer already made, and it refuses a
  /// mixed-currency set rather than inventing a total.
  Future<Proposal> create({
    required String leadId,
    required List<ProposalLine> lines,
    String body = '',
  }) async {
    final dio = _ref.read(dioProvider);
    final res = await dio.post<Map<String, dynamic>>(
      '/proposals',
      data: <String, dynamic>{
        'lead_id': leadId,
        'lines': lines.map((l) => l.toJson()).toList(),
        'body': body,
      },
    );
    return Proposal.fromJson(res.data ?? const <String, dynamic>{});
  }

  /// Downloads the offer as a PDF the seller can forward.
  ///
  /// Deliberately NOT a send: it hands over a file. The only way a proposal
  /// reaches a customer is still the approval queue — this is for the buyer who
  /// asked for the price on WhatsApp and is waiting right now, which is the
  /// moment this market is actually won.
  ///
  /// Returns false when the platform refused to write the file; the caller
  /// says so rather than pretending it saved.
  Future<bool> downloadPdf({required String proposalId}) async {
    final dio = _ref.read(dioProvider);
    final res = await dio.get<List<int>>(
      '/proposals/$proposalId/pdf',
      options: downloadOptions(),
    );
    final bytes = res.data ?? const <int>[];
    if (bytes.isEmpty) return false;
    final name = filenameFrom(res.headers.map) ?? 'proposta.pdf';
    return saveDownloadedFile(name, bytes);
  }

  /// Puts the offer in the approval queue. Reaches nobody: a person still reads
  /// the draft and decides, which is the same gate every cold approach passes.
  Future<Proposal> sendForApproval({required String proposalId}) async {
    final dio = _ref.read(dioProvider);
    final res = await dio.post<Map<String, dynamic>>(
      '/proposals/$proposalId/send-for-approval',
    );
    return Proposal.fromJson(res.data ?? const <String, dynamic>{});
  }
}

final proposalActionsProvider = Provider<ProposalActions>(ProposalActions.new);

/// The lines being composed, keyed by product id.
///
/// Held as quantities rather than as a list of [ProposalLine] so tapping a
/// product twice is "two of them" and not two rows of one — the second reads as
/// a mistake the person then has to clean up.
class ProposalDraftLines extends Notifier<Map<String, int>> {
  @override
  Map<String, int> build() => const <String, int>{};

  /// True once the matcher's picks have been offered, whatever became of
  /// them. A suggestion that reappears after the seller deleted it is not a
  /// suggestion, it is an argument — and this screen rebuilds often enough
  /// for that to happen within seconds.
  bool _suggested = false;

  /// Opens the form with what the matcher already chose for this lead, one of
  /// each, every line editable and removable.
  ///
  /// The screen used to open blank against a catalogue of a hundred items, so
  /// pricing an offer meant remembering which products fit a hospital the
  /// system had already decided about. The picks are a starting point and
  /// nothing more: the seller changes quantities, drops lines and adds others
  /// exactly as before.
  void suggest(Iterable<String> productIds) {
    if (_suggested) return;
    final next = <String, int>{};
    for (final id in productIds) {
      if (id.trim().isEmpty) continue;
      next[id] = 1;
    }
    // Nothing to suggest does not spend the one shot: the lead resolves a
    // frame or two after this screen mounts, and burning it on the empty
    // first read would lock the form blank for good.
    if (next.isEmpty) return;
    _suggested = true;
    state = next;
  }

  /// Private on purpose: `add`/`remove` are the whole vocabulary a stepper
  /// needs, and a public setter with no caller is a capability nobody reaches.
  void _setQuantity(String productId, int quantity) {
    final next = Map<String, int>.from(state);
    if (quantity <= 0) {
      next.remove(productId);
    } else {
      next[productId] = quantity;
    }
    state = next;
  }

  void add(String productId) =>
      _setQuantity(productId, (state[productId] ?? 0) + 1);

  void remove(String productId) =>
      _setQuantity(productId, (state[productId] ?? 0) - 1);

  void clear() => state = const <String, int>{};

  List<ProposalLine> get lines => state.entries
      .map((e) => ProposalLine(productId: e.key, quantity: e.value))
      .toList();
}

final proposalDraftLinesProvider =
    NotifierProvider.autoDispose<ProposalDraftLines, Map<String, int>>(
        ProposalDraftLines.new);
