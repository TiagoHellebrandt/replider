import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/utils/account_currency.dart';
import 'package:app/design_system/components/empty_state_card.dart';
import 'package:app/design_system/components/gradient_pill.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/contract_calendar_controller.dart';
import 'package:app/features/sales/presentation/controllers/lead_detail_controller.dart';
import 'package:app/features/sales/presentation/controllers/radar_controller.dart';
import 'package:app/features/sales/presentation/controllers/products_controller.dart';
import 'package:app/features/sales/presentation/controllers/signals_controller.dart';
import 'package:app/features/sales/presentation/widgets/radar_detail_panel.dart';
import 'package:app/features/sales/presentation/widgets/signal_watch_editor.dart';
import 'package:app/i18n/strings.g.dart';

/// What to tell the reader about a source that did not answer.
///
/// Relative, never a clock time: "há 7 h" is what makes a stale feed obvious,
/// and a formatted timestamp would also have to pick a timezone — the device's
/// or the account's — which is a decision this sentence does not need to make.
///
/// Exported for its own test: the sentence is the whole feature.
String sourceDownMessage(SignalSourceHealth s, DateTime now) {
  final since = s.lastSuccessAt;
  if (since == null) return t.radar.source_down_never(source: s.label);
  final d = now.difference(since);
  final ago = d.inHours < 1
      ? t.radar.ago_minutes(n: d.inMinutes < 1 ? 1 : d.inMinutes)
      : d.inHours < 48
          ? t.radar.ago_hours(n: d.inHours)
          : t.radar.ago_days(n: d.inDays);
  return t.radar.source_down(source: s.label, ago: ago);
}

/// The Radar tab (Ofertas phase 2): everything the system detected, one
/// list, each row ending in a next step. Replaces the licitação-only Sinais
/// tab — a tender is now one TYPE of item beside the CNES park, reorders
/// due and relationship dates.
class RadarBody extends ConsumerStatefulWidget {
  const RadarBody({super.key});

  @override
  ConsumerState<RadarBody> createState() => _RadarBodyState();
}

class _RadarBodyState extends ConsumerState<RadarBody> {
  /// The sources that read from `sales`. Until the account records a sale
  /// they are empty by definition, so their chips and the copy promising
  /// them would advertise something the feed cannot yet show.
  static const _carteiraTypes = {
    RadarItemType.recompra,
    RadarItemType.data,
    RadarItemType.crossSell,
  };

  RadarItemType? _filter;
  bool _busy = false;

  /// Which row the panel is showing. The card is a headline that truncates;
  /// for the quarter of tenders that publish no link at all, the panel is the
  /// only place the opportunity can be read.
  String? _openId;

  /// Below this the panel cannot sit beside the list, so it arrives as a
  /// sheet. Same content either way — a phone does not get less of the
  /// opportunity than a desk.
  static const _panelBreakpoint = 1000.0;

  void _open(BuildContext context, RadarItem item) {
    if (MediaQuery.of(context).size.width >= _panelBreakpoint) {
      setState(() => _openId = _openId == item.id ? null : item.id);
      return;
    }
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Theme.of(context)
          .extension<AppColorsExtension>()!
          .backgroundSecondary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      constraints: BoxConstraints(
        maxHeight: MediaQuery.of(context).size.height * 0.85,
      ),
      builder: (_) => SafeArea(child: RadarDetailPanel(item: item)),
    );
  }

  final _huntCtrl = TextEditingController();
  TerritoryHuntResult? _hunt;
  bool _hunting = false;
  bool _autoHunting = false;

  @override
  void dispose() {
    _huntCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final state = ref.watch(radarProvider);
    final watches = ref.watch(signalWatchesProvider).asData?.value;
    final downSources = (ref.watch(signalSourcesProvider).asData?.value ??
            const <SignalSourceHealth>[])
        .where((s) => !s.reachable)
        .toList();

    return state.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => RetryableError(
        error: err,
        onRetry: () => ref.invalidate(radarProvider),
      ),
      data: (feed) {
        final visible = _filter == null
            ? feed.items
            : feed.items.where((i) => i.type == _filter).toList();
        final hasCarteira =
            feed.items.any((i) => _carteiraTypes.contains(i.type));
        final open = _openId == null
            ? null
            : feed.items.where((i) => i.id == _openId).firstOrNull;
        // The row that was open can vanish under a filter change or a
        // refresh. Dropping the panel is right; keeping a stale one beside a
        // list that no longer contains it is the screen lying.
        final list = RefreshIndicator(
          onRefresh: () async => ref.invalidate(radarProvider),
          child: ListView(
            padding: const EdgeInsets.fromLTRB(20, 12, 20, 24),
            children: [
              // What the radar is currently watching, and the door to change
              // it — the same watch the licitação poller uses.
              if (watches != null && watches.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: Row(
                    children: [
                      Expanded(
                        child: Text(
                          _watchSummary(watches),
                          key: const Key('radar_watch_current'),
                          style:
                              TextStyle(color: colors.textMuted, fontSize: 12),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      TextButton(
                        key: const Key('radar_watch_edit'),
                        onPressed: () =>
                            editSignalWatch(context, ref, watches.first),
                        child: Text(t.common.edit),
                      ),
                    ],
                  ),
                ),
              // A source that did not answer says so, above everything else.
              // Without this the screen presented the last good collection as
              // the current state of the market — the reader has no way to
              // tell an outage from a week with no tenders, and acts on stale
              // deadlines believing they are today's.
              for (final down in downSources)
                Padding(
                  padding: const EdgeInsets.only(bottom: 10),
                  child: Container(
                    key: Key('radar_source_down_${down.source}'),
                    padding: const EdgeInsets.symmetric(
                        horizontal: 12, vertical: 10),
                    decoration: BoxDecoration(
                      color: colors.orangeAlert.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(8),
                      border: Border.all(
                          color: colors.orangeAlert.withValues(alpha: 0.4)),
                    ),
                    child: Row(
                      children: [
                        Icon(Icons.cloud_off_outlined,
                            size: 16, color: colors.orangeAlert),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            sourceDownMessage(down, DateTime.now()),
                            style: TextStyle(
                                color: colors.textSecondary, fontSize: 12),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              if (feed.warnings.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(bottom: 10),
                  child: Container(
                    key: const Key('radar_warnings'),
                    padding:
                        const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                    decoration: BoxDecoration(
                      color: colors.orangeAlert.withValues(alpha: 0.1),
                      borderRadius: BorderRadius.circular(8),
                      border: Border.all(
                          color: colors.orangeAlert.withValues(alpha: 0.35)),
                    ),
                    child: Text(
                      t.radar.warnings(sources: feed.warnings.join(', ')),
                      style:
                          TextStyle(color: colors.textSecondary, fontSize: 12),
                    ),
                  ),
                ),
              // How big the opportunity actually is. Without this the page
              // showed a handful of hospitals with nothing saying a handful
              // of WHAT — which, for a territory holding thousands, reads as
              // "you have five opportunities".
              if (feed.park != null && feed.park!.hasMore)
                Padding(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: Container(
                    key: const Key('radar_park_universe'),
                    padding: const EdgeInsets.symmetric(
                        horizontal: 14, vertical: 11),
                    decoration: BoxDecoration(
                      color: colors.limeGlow.withValues(alpha: 0.08),
                      borderRadius: BorderRadius.circular(10),
                      border: Border.all(
                          color: colors.limeGlow.withValues(alpha: 0.28)),
                    ),
                    child: Row(
                      children: [
                        Icon(Icons.travel_explore,
                            size: 16, color: colors.limeGlow),
                        const SizedBox(width: 9),
                        Expanded(
                          child: Text(
                            t.radar.park_universe(
                              total: '${feed.park!.establishments}',
                              scope: feed.park!.scope,
                              shown: '${feed.park!.shown}',
                            ),
                            style: TextStyle(
                                color: colors.textSecondary,
                                fontSize: 12.5,
                                height: 1.4),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              if (feed.items.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: Wrap(
                    spacing: 8,
                    children: [
                      _filterChip(null, t.radar.filter_all, feed.items.length),
                      // A chip only for a source with something in it — which
                      // keeps the three carteira chips off screen before the
                      // first sale.
                      for (final type in RadarItemType.values)
                        if (feed.items.any((i) => i.type == type))
                          _filterChip(
                            type,
                            _typeLabel(type),
                            feed.items.where((i) => i.type == type).length,
                          ),
                    ],
                  ),
                ),
              if (!hasCarteira)
                Padding(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: Text(
                    t.radar.carteira_hint,
                    key: const Key('radar_carteira_hint'),
                    style: TextStyle(color: colors.textMuted, fontSize: 12),
                  ),
                ),
              if (visible.isEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 48),
                  child: EmptyStateCard(
                    key: const Key('radar_empty'),
                    icon: Icons.radar_outlined,
                    title: t.radar.empty,
                    // The promise matches what can show up: no reorders or
                    // dates in the copy before a sale exists to derive them.
                    subtitle: hasCarteira
                        ? t.radar.empty_hint
                        : t.radar.empty_hint_prospecting,
                  ),
                )
              else
                for (final item in visible)
                  _RadarCard(
                    item: item,
                    busy: _busy,
                    selected: _openId == item.id,
                    onOpen: () => _open(context, item),
                    onParkOffer: () => _parkOffer(item),
                    onDismissSignal: item.type == RadarItemType.licitacao
                        ? () => _dismissSignal(item)
                        : null,
                  ),
              _huntSection(colors),
              Padding(
                padding: const EdgeInsets.only(top: 14),
                child: Text(
                  t.radar.footer_note,
                  key: const Key('radar_footer_note'),
                  style: TextStyle(color: colors.textMuted, fontSize: 12),
                ),
              ),
            ],
          ),
        );

        // Below the breakpoint the panel arrives as a sheet instead (see
        // _open), so the list is the whole screen.
        if (open == null ||
            MediaQuery.of(context).size.width < _panelBreakpoint) {
          return list;
        }
        return Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(child: list),
            Container(
              key: const Key('radar_detail_panel'),
              width: 420,
              margin: const EdgeInsets.fromLTRB(0, 12, 20, 24),
              decoration: BoxDecoration(
                color: colors.backgroundSecondary,
                borderRadius: BorderRadius.circular(16),
                border: Border.all(color: colors.borderMedium),
              ),
              child: RadarDetailPanel(
                item: open,
                onClose: () => setState(() => _openId = null),
              ),
            ),
          ],
        );
      },
    );
  }

  /// "Abastecer o território": the ex-Caçar, CNES-first — who in the
  /// territory declares the equipment the seller types, counts only.
  Widget _huntSection(AppColorsExtension colors) {
    final products = ref.watch(productsListProvider).asData?.value ?? const [];
    final hunt = _hunt;
    return Container(
      key: const Key('territory_hunt'),
      margin: const EdgeInsets.only(top: 18),
      padding: const EdgeInsets.all(18),
      decoration: BoxDecoration(
        color: colors.glassLight,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: colors.borderMedium),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(t.radar.hunt_title,
              style:
                  const TextStyle(fontSize: 15, fontWeight: FontWeight.w700)),
          const SizedBox(height: 4),
          Text(t.radar.hunt_hint,
              style: TextStyle(color: colors.textMuted, fontSize: 12.5)),
          const SizedBox(height: 12),
          // The mockup's search pill: input and button share one rounded
          // container, like a command line for the territory.
          // The hunt that needs no words, above the one that does.
          //
          // Typing equipment names is the opposite of a product that goes
          // looking — and it only works if you already know which of your
          // words the CNES registry happens to use. This reads the catalogue
          // and answers for all of them.
          SizedBox(
            width: double.infinity,
            child: FilledButton.icon(
              key: const Key('territory_auto_hunt'),
              onPressed: _autoHunting ? null : _runAutoHunt,
              icon: _autoHunting
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.travel_explore, size: 18),
              label: Text(t.radar.auto_hunt_cta),
            ),
          ),
          const SizedBox(height: 6),
          Text(
            t.radar.auto_hunt_hint,
            key: const Key('territory_auto_hunt_hint'),
            style: TextStyle(color: colors.textMuted, fontSize: 11.5),
          ),
          const SizedBox(height: 14),
          Container(
            padding: const EdgeInsets.only(left: 18, right: 6),
            decoration: BoxDecoration(
              color: colors.backgroundSecondary,
              borderRadius: BorderRadius.circular(999),
              border: Border.all(color: colors.borderMedium),
            ),
            child: Row(
              children: [
                Icon(Icons.search, size: 16, color: colors.textMuted),
                const SizedBox(width: 8),
                Expanded(
                  child: TextField(
                    key: const Key('territory_hunt_input'),
                    controller: _huntCtrl,
                    onSubmitted: (_) => _runHunt(),
                    decoration: InputDecoration(
                      hintText: t.radar.hunt_placeholder,
                      border: InputBorder.none,
                      enabledBorder: InputBorder.none,
                      focusedBorder: InputBorder.none,
                      isDense: true,
                      contentPadding: const EdgeInsets.symmetric(vertical: 14),
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                GradientPill(
                  key: const Key('territory_hunt_search'),
                  label: t.common.search,
                  onTap: _hunting ? null : _runHunt,
                ),
              ],
            ),
          ),
          if (products.isNotEmpty) ...[
            const SizedBox(height: 8),
            Wrap(
              spacing: 6,
              children: [
                for (final p in products.take(3))
                  ActionChip(
                    label: Text(p.name, style: const TextStyle(fontSize: 11.5)),
                    onPressed: () {
                      _huntCtrl.text = p.name;
                      _runHunt();
                    },
                  ),
              ],
            ),
          ],
          if (_hunting)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 16),
              child: Center(child: CircularProgressIndicator()),
            ),
          if (hunt != null && !_hunting) ...[
            const SizedBox(height: 12),
            // Which ground and which month: "no seu território (Cuiabá) ·
            // dados CNES de 07/2026". Silent scope reads as "the state,
            // today" — neither is true.
            if (hunt.uf.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Text(
                  [
                    hunt.cities.isEmpty
                        ? t.radar.hunt_scope_uf(uf: hunt.uf)
                        : t.radar
                            .hunt_scope_cities(cities: hunt.cities.join(', ')),
                    if (hunt.competencia != null)
                      t.radar.data_month(
                          month: competenciaLabel(hunt.competencia!)),
                  ].join(' · '),
                  key: const Key('territory_hunt_scope'),
                  style: TextStyle(color: colors.textMuted, fontSize: 11.5),
                ),
              ),
            if (hunt.holders.isEmpty)
              Text(
                hunt.note ?? t.radar.hunt_empty,
                key: const Key('territory_hunt_note'),
                style: TextStyle(color: colors.textMuted, fontSize: 12.5),
              )
            else
              for (final h in hunt.holders)
                Container(
                  key: Key('territory_holder_${h.cnes}'),
                  margin: const EdgeInsets.only(bottom: 8),
                  padding:
                      const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                  decoration: BoxDecoration(
                    color: colors.glassLight,
                    borderRadius: BorderRadius.circular(12),
                    border: Border.all(color: colors.borderLight),
                  ),
                  child: Row(
                    children: [
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              h.tradeName ?? 'CNES ${h.cnes}',
                              style: const TextStyle(
                                  fontSize: 13, fontWeight: FontWeight.w600),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                            Text(
                              [
                                t.radar.hunt_units(count: '${h.total}'),
                                if (h.phone != null) h.phone!,
                              ].join(' · '),
                              style: TextStyle(
                                  color: colors.textMuted, fontSize: 11.5),
                            ),
                          ],
                        ),
                      ),
                      if (h.alreadyKnown)
                        Text(t.radar.hunt_known,
                            key: Key('territory_known_${h.cnes}'),
                            style: TextStyle(
                                color: colors.limeGlow, fontSize: 11.5))
                      else
                        TextButton(
                          key: Key('territory_offer_${h.cnes}'),
                          onPressed: _busy
                              ? null
                              : () => _parkOffer(RadarItem(
                                    type: RadarItemType.parque,
                                    id: 'cnes-${h.cnes}',
                                    title: h.tradeName ?? h.cnes,
                                    context: '',
                                    cnes: h.cnes,
                                  )),
                          child: Text(t.radar.prepare_offer,
                              style: const TextStyle(fontSize: 12)),
                        ),
                    ],
                  ),
                ),
          ],
        ],
      ),
    );
  }

  /// Fills the funnel from the catalogue, no words typed.
  ///
  /// It creates leads and writes nothing else — no approach is drafted and
  /// nothing is sent, so the button is safe to press without a confirmation
  /// step that would just teach people to click through it.
  Future<void> _runAutoHunt() async {
    final messenger = ScaffoldMessenger.of(context);
    setState(() => _autoHunting = true);
    try {
      final res = await ref.read(radarActionsProvider).autoHunt();
      if (!mounted) return;
      // The funnel changed underneath: whatever is on screen counting leads
      // is now stale.
      ref.invalidate(radarProvider);
      messenger.showSnackBar(SnackBar(
        key: const Key('territory_auto_hunt_result'),
        content: Text(res.created > 0
            ? t.radar.auto_hunt_result(
                created: '${res.created}', found: '${res.found}')
            : t.radar.auto_hunt_none),
      ));
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: const Key('radar_error'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _autoHunting = false);
    }
  }

  Future<void> _runHunt() async {
    final term = _huntCtrl.text.trim();
    final messenger = ScaffoldMessenger.of(context);
    if (term.isEmpty) {
      // An empty search gets an answer, not a dead click.
      messenger.showSnackBar(SnackBar(content: Text(t.radar.hunt_placeholder)));
      return;
    }
    setState(() => _hunting = true);
    try {
      final res = await ref.read(radarActionsProvider).hunt(term);
      if (!mounted) return;
      setState(() => _hunt = res);
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: const Key('radar_error'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _hunting = false);
    }
  }

  /// "vigiando: luva, seringa · catálogo (12 produtos)" — what feeds the
  /// radar, in one line, with the door to change the words.
  String _watchSummary(List<SignalWatch> watches) {
    final words = watches.expand((w) => w.keywords).take(6).join(', ');
    final products = ref.read(productsListProvider).asData?.value ?? const [];
    final parts = [
      t.signals.watch_current(words: words),
      if (products.isNotEmpty)
        t.radar.watching_catalog(count: '${products.length}'),
    ];
    return parts.join(' · ');
  }

  Widget _filterChip(RadarItemType? type, String label, int count) {
    final selected = _filter == type;
    return ChoiceChip(
      key: Key('radar_filter_${type?.name ?? 'all'}'),
      label: Text('$label · $count'),
      selected: selected,
      onSelected: (_) => setState(() => _filter = type),
    );
  }

  static String _typeLabel(RadarItemType type) => switch (type) {
        RadarItemType.licitacao => t.radar.type_licitacao,
        RadarItemType.contrato => t.radar.type_contrato,
        RadarItemType.ata => t.radar.type_ata,
        RadarItemType.parque => t.radar.type_parque,
        RadarItemType.recompra => t.radar.type_recompra,
        RadarItemType.data => t.radar.type_data,
        RadarItemType.crossSell => t.radar.type_cross_sell,
      };

  Future<void> _parkOffer(RadarItem item) async {
    final cnes = item.cnes;
    if (cnes == null) return;
    final messenger = ScaffoldMessenger.of(context);
    setState(() => _busy = true);
    try {
      await ref.read(radarActionsProvider).parkOffer(cnes);
      messenger.showSnackBar(SnackBar(content: Text(t.radar.offer_queued)));
      if (mounted) context.go('/prospect?tab=queue');
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: const Key('radar_error'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _dismissSignal(RadarItem item) async {
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref.read(signalActionsProvider).dismiss(item.id);
      ref.invalidate(radarProvider);
      ref.invalidate(signalsProvider);
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: const Key('radar_error'),
        content: Text(parseApiError(e)),
      ));
    }
  }
}

/// A card names a few products and counts the rest. A dozen chips is a
/// spreadsheet, not an answer to "what do I offer them".
const _maxCardMatches = 3;

class _RadarCard extends ConsumerWidget {
  const _RadarCard({
    required this.item,
    required this.busy,
    required this.onParkOffer,
    this.onDismissSignal,
    this.selected = false,
    this.onOpen,
  });

  final RadarItem item;
  final bool busy;
  final VoidCallback onParkOffer;
  final VoidCallback? onDismissSignal;

  /// The row the panel is showing. Marked with the border rather than a fill
  /// so the urgency colours keep their meaning.
  final bool selected;

  /// Opens everything the system knows. The card shows two lines and then an
  /// ellipsis, and until this existed that ellipsis was the end of the road:
  /// a quarter of these tenders publish no link either.
  final VoidCallback? onOpen;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final deadline = item.deadline;
    final daysLeft = deadline?.difference(DateTime.now()).inDays;
    final urgent = daysLeft != null && daysLeft <= 5;

    // The mockup's row anatomy: colored icon square → title + context →
    // value → CTA pill → quiet dismiss. One glance says what, why, how
    // much, and what to do — without opening anything.
    return InkWell(
      key: Key('radar_open_${item.id}'),
      onTap: onOpen,
      borderRadius: BorderRadius.circular(14),
      child: Container(
        key: Key('radar_item_${item.id}'),
        margin: const EdgeInsets.only(bottom: 10),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
        decoration: BoxDecoration(
          color: colors.glassLight,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(
              color: selected
                  ? colors.bluePrimary.withValues(alpha: 0.75)
                  : urgent
                      ? colors.orangeAlert.withValues(alpha: 0.4)
                      : colors.borderLight),
        ),
        child: Row(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: _iconColor(colors).withValues(alpha: 0.13),
                borderRadius: BorderRadius.circular(12),
              ),
              child: Icon(_icon(), size: 18, color: _iconColor(colors)),
            ),
            const SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Flexible(
                        child: Text(
                          item.title,
                          style: const TextStyle(
                              fontSize: 14, fontWeight: FontWeight.w600),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      if (daysLeft != null) ...[
                        const SizedBox(width: 8),
                        Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 9, vertical: 2),
                          decoration: BoxDecoration(
                            color: (urgent
                                    ? colors.orangeAlert
                                    : colors.borderMedium)
                                .withValues(alpha: urgent ? 0.15 : 0.2),
                            borderRadius: BorderRadius.circular(999),
                            border: urgent
                                ? Border.all(
                                    color: colors.orangeAlert
                                        .withValues(alpha: 0.35))
                                : null,
                          ),
                          child: Text(
                            daysLeft <= 0
                                ? t.radar.deadline_today
                                : t.radar.deadline_days(count: '$daysLeft'),
                            style: TextStyle(
                              fontSize: 11,
                              fontWeight: FontWeight.w700,
                              color: urgent
                                  ? colors.orangeAlert
                                  : colors.textMuted,
                            ),
                          ),
                        ),
                      ],
                    ],
                  ),
                  const SizedBox(height: 3),
                  Text(
                    item.context,
                    style: TextStyle(
                        color: colors.textSecondary,
                        fontSize: 12.5,
                        height: 1.4),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  // The whole pitch of a forward-looking row, in two words. It
                  // is a badge and not a sentence because it is the reason to
                  // pick THIS row over the one below it.
                  if (item.edge != null) ...[
                    const SizedBox(height: 6),
                    _EdgeBadge(edge: item.edge!, itemId: item.id),
                  ],
                  // A park count is a month-old snapshot (DATASUS lags 45–75
                  // days); the card names the month rather than implying now.
                  if (item.competencia != null)
                    Text(
                      t.radar.data_month(
                          month: competenciaLabel(item.competencia!)),
                      key: Key('radar_park_month_${item.id}'),
                      style: TextStyle(color: colors.textMuted, fontSize: 11),
                    ),
                  // What of the catalogue fits what they own. The card used to
                  // say how much equipment a hospital has and stop there,
                  // leaving "so what do I sell them?" to a paid draft.
                  if (item.matches.isNotEmpty) ...[
                    const SizedBox(height: 6),
                    Wrap(
                      spacing: 6,
                      runSpacing: 4,
                      children: [
                        // The chip opens the product, for the same reason the
                        // lead panel's line does: nobody knows a hundred-item
                        // catalogue by heart, and a product named with nowhere
                        // to read it leaves the seller guessing at the spec.
                        for (final m in item.matches.take(_maxCardMatches))
                          _OfferChip(
                            key: Key('radar_offer_${item.id}_${m.productName}'),
                            match: m,
                          ),
                        if (item.matches.length > _maxCardMatches)
                          Text(
                            t.radar.offer_more(
                                count:
                                    '${item.matches.length - _maxCardMatches}'),
                            style: TextStyle(
                                color: colors.textMuted, fontSize: 11),
                          ),
                      ],
                    ),
                  ],
                ],
              ),
            ),
            // Padding on the Wrap, not a SizedBox in the Row: a fixed sibling
            // takes its 12px BEFORE the Flexible is measured, which was enough
            // to leave the cluster half a pixel over its bounds once it fit
            // otherwise.
            // The money and the verbs, in a Wrap rather than laid straight
            // into the Row.
            //
            // A Row cannot wrap, so the cluster used to overflow the moment
            // the list got narrower: opening the detail panel takes 420px off
            // the width and the card blew its bounds by 164 — a yellow-and-
            // black overflow stripe across the money figure. It only never
            // showed before because the list always had the whole screen.
            // NOT Flexible. A flex child gets at most its share of the row,
            // and a squeezed GradientPill does not shrink — its inner Row
            // overflows inside itself. Sized naturally instead, the actions
            // take what they need and the text column (Expanded) absorbs the
            // rest, which is the way round that cannot break.
            //
            // The Wrap is still a Wrap so a long cluster falls to a second
            // line rather than pushing the card out: opening the detail panel
            // takes 420px off the list, and the card used to blow its bounds
            // by 164 — a yellow-and-black stripe across the money figure. It
            // never showed before only because the list had the whole screen.
            ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 260),
              child: Padding(
                padding: const EdgeInsets.only(left: 12),
                child: Wrap(
                  alignment: WrapAlignment.end,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  spacing: 8,
                  runSpacing: 6,
                  children: [
                    if (item.valueCents > 0)
                      Text(
                        formatAccountMoney(ref, item.valueCents),
                        style: const TextStyle(
                            fontSize: 14, fontWeight: FontWeight.w700),
                      ),
                    ..._ctas(context),
                    if (onDismissSignal != null)
                      TextButton(
                        key: Key('radar_dismiss_${item.id}'),
                        onPressed: busy ? null : onDismissSignal,
                        child: Text(t.signals.dismiss,
                            style: TextStyle(
                                color: colors.textMuted, fontSize: 12)),
                      ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  List<Widget> _ctas(BuildContext context) {
    // The verb says the STAGE, before it says the kind. A row whose offer is
    // already written needs the same next step whatever produced it, and
    // saying so on the card is what let the approval queue stop being a tab
    // the seller had to remember to open.
    if (item.ready) {
      return [
        GradientPill(
          key: Key('radar_read_and_send_${item.id}'),
          label: t.radar.read_and_send,
          onTap: busy ? null : () => context.go('/prospect?tab=queue'),
        ),
      ];
    }
    switch (item.type) {
      case RadarItemType.licitacao:
        return [
          if (item.leadId != null)
            GradientPill(
              key: Key('radar_open_draft_${item.id}'),
              label: t.radar.open_draft,
              onTap: busy ? null : () => context.go('/prospect?tab=queue'),
            )
          else
            // The seller who wants to do it themselves. Without this the only
            // ways forward were "read the notice" (which leaves the product,
            // recording nothing) and "not for us" — while an expiring contract
            // beside it could be worked by hand. Nothing is drafted and the
            // account's AI key is not touched.
            _WorkSignalPill(itemId: item.id, busy: busy),
          if (item.url != null)
            GhostPill(
              key: Key('radar_open_url_${item.id}'),
              label: t.signals.open_notice,
              onTap: busy
                  ? null
                  : () => launchUrl(Uri.parse(item.url!),
                      mode: LaunchMode.externalApplication),
            ),
        ];
      case RadarItemType.parque:
        return [
          GradientPill(
            key: Key('radar_park_offer_${item.id}'),
            label: t.radar.prepare_offer,
            onTap: busy ? null : onParkOffer,
          ),
        ];
      case RadarItemType.recompra:
      case RadarItemType.data:
      case RadarItemType.crossSell:
        return [
          if (item.contactId != null)
            GhostPill(
              key: Key('radar_open_customer_${item.id}'),
              label: t.radar.open_customer,
              onTap: busy
                  ? null
                  : () => context.go('/sales/portfolio/${item.contactId}'),
            ),
        ];
      // A published commitment has no e-mail and no phone on the record, so
      // the first step is not a draft: it is putting the buyer in the funnel
      // with the one thing missing — a contact — named on the card.
      case RadarItemType.contrato:
      case RadarItemType.ata:
        return [
          if (item.pncpId.isNotEmpty)
            _WorkWindowPill(pncpId: item.pncpId, itemId: item.id, busy: busy),
          if (item.url != null)
            GhostPill(
              key: Key('radar_open_url_${item.id}'),
              label: t.calendar.open_record,
              onTap: busy
                  ? null
                  : () => launchUrl(Uri.parse(item.url!),
                      mode: LaunchMode.externalApplication),
            ),
        ];
    }
  }

  IconData _icon() => switch (item.type) {
        RadarItemType.licitacao => Icons.description_outlined,
        RadarItemType.parque => Icons.precision_manufacturing_outlined,
        RadarItemType.recompra => Icons.replay_outlined,
        RadarItemType.data => Icons.event_outlined,
        RadarItemType.crossSell => Icons.add_shopping_cart_outlined,
        RadarItemType.contrato => Icons.event_busy_outlined,
        RadarItemType.ata => Icons.receipt_long_outlined,
      };

  Color _iconColor(AppColorsExtension colors) => switch (item.type) {
        RadarItemType.licitacao => colors.orangeAlert,
        RadarItemType.parque => colors.cyanGlow,
        RadarItemType.recompra => colors.limeGlow,
        RadarItemType.data => colors.bluePrimary,
        RadarItemType.crossSell => colors.indigoDark,
        RadarItemType.contrato => colors.orangeAlert,
        RadarItemType.ata => colors.limeGlow,
      };
}

/// "Trabalhar este" on a contract/ata row: the published window becomes a card
/// in the funnel. Nothing is drafted and nothing is sent — a public body has no
/// contact on the record, so the card arrives asking for exactly that.
class _WorkWindowPill extends ConsumerStatefulWidget {
  const _WorkWindowPill({
    required this.pncpId,
    required this.itemId,
    required this.busy,
  });

  final String pncpId;
  final String itemId;
  final bool busy;

  @override
  ConsumerState<_WorkWindowPill> createState() => _WorkWindowPillState();
}

class _WorkWindowPillState extends ConsumerState<_WorkWindowPill> {
  bool _working = false;

  Future<void> _work() async {
    setState(() => _working = true);
    try {
      await ref.read(calendarActionsProvider).work(pncpId: widget.pncpId);
      if (!mounted) return;
      ref.invalidate(radarProvider);
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(t.calendar.work_queued),
        action: SnackBarAction(
          label: t.calendar.open_funnel,
          onPressed: () => context.go('/funnel'),
        ),
      ));
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(parseApiError(e))));
    } finally {
      if (mounted) setState(() => _working = false);
    }
  }

  @override
  Widget build(BuildContext context) => GradientPill(
        key: Key('radar_work_window_${widget.itemId}'),
        label: t.calendar.work,
        onTap: widget.busy || _working ? null : _work,
      );
}

/// "Trabalhar este" on an open tender: the buyer lands in the customer list
/// with the notice attached, and nothing is written or sent.
///
/// A public notice publishes no e-mail and no phone, so the card arrives
/// asking for exactly the thing the record cannot supply — who to call. That
/// is honest work to hand someone, and better than a send button that
/// discovers at the click that there is no address.
class _WorkSignalPill extends ConsumerStatefulWidget {
  const _WorkSignalPill({required this.itemId, required this.busy});

  final String itemId;
  final bool busy;

  @override
  ConsumerState<_WorkSignalPill> createState() => _WorkSignalPillState();
}

class _WorkSignalPillState extends ConsumerState<_WorkSignalPill> {
  bool _working = false;

  Future<void> _work() async {
    setState(() => _working = true);
    try {
      await ref.read(radarActionsProvider).workSignal(widget.itemId);
      if (!mounted) return;
      ref.invalidate(radarProvider);
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(t.calendar.work_queued),
        action: SnackBarAction(
          label: t.calendar.open_funnel,
          onPressed: () => context.go('/funnel'),
        ),
      ));
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(parseApiError(e))));
    } finally {
      if (mounted) setState(() => _working = false);
    }
  }

  @override
  Widget build(BuildContext context) => GradientPill(
        key: Key('radar_work_signal_${widget.itemId}'),
        label: t.calendar.work,
        onTap: widget.busy || _working ? null : _work,
      );
}

/// "antes do edital" / "sem licitação" — why this row beats the market to the
/// purchase. Two values, two colours, never one shared badge: reaching a buyer
/// before the edital exists and buying with no edital at all are different
/// facts, and a seller who confuses them pitches the wrong thing.
class _EdgeBadge extends StatelessWidget {
  const _EdgeBadge({required this.edge, required this.itemId});

  final RadarEdge edge;
  final String itemId;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final (label, tone) = switch (edge) {
      RadarEdge.beforeTender => (
          t.radar.edge_before_tender,
          colors.orangeAlert
        ),
      RadarEdge.noTender => (t.radar.edge_no_tender, colors.limeGlow),
    };
    return Container(
      key: Key('radar_edge_$itemId'),
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 3),
      decoration: BoxDecoration(
        color: tone.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: tone.withValues(alpha: 0.38)),
      ),
      child: Text(
        label,
        style:
            TextStyle(color: tone, fontSize: 11, fontWeight: FontWeight.w600),
      ),
    );
  }
}

/// One catalogue match on a radar card, as a chip that opens the product.
class _OfferChip extends StatelessWidget {
  const _OfferChip({super.key, required this.match});

  final ParkMatch match;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final chip = Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: colors.limeGlow.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(6),
        // The border is the affordance: a coloured chip that does nothing and
        // a coloured chip that opens something must not look identical.
        border: match.productId.isEmpty
            ? null
            : Border.all(color: colors.limeGlow.withValues(alpha: 0.4)),
      ),
      child: Text(
        match.productName,
        style: TextStyle(
            color: colors.limeGlow, fontSize: 11, fontWeight: FontWeight.w600),
      ),
    );

    // An older server sends no id; the chip stays a label rather than becoming
    // a tap that goes nowhere.
    if (match.productId.isEmpty) return chip;

    return Tooltip(
      message: t.board.panel_offer_open(product: match.productName),
      child: InkWell(
        key: Key('radar_offer_open_${match.productId}'),
        borderRadius: BorderRadius.circular(6),
        onTap: () => context.go('/sales/products/${match.productId}'),
        child: chip,
      ),
    );
  }
}
