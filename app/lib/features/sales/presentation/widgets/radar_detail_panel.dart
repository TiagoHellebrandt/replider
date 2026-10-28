import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:app/core/utils/account_currency.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/lead_detail_controller.dart';
import 'package:app/features/sales/presentation/controllers/radar_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Everything the system knows about one opportunity, in one place.
///
/// The card is a headline: two lines, then an ellipsis. That is right for a
/// list somebody scans and wrong as the ONLY place the information exists — a
/// tender for "PEÇAS DE REPOSIÇÃO ORIGINAIS PARA ATENDER ÀS NECESSIDADES DO
/// LABORATÓRIO MUNICIPAL" rendered up to the word "NECESSIDADES", and on that
/// evidence it read as out of trade.
///
/// It matters more than "more detail" because **16 of this account's 62
/// tenders carry no link at all**: PNCP returns `linkSistemaOrigem` empty when
/// the buyer published none, and some of the links that do exist are broken at
/// the source (one ends in `?id=` with nothing after it). For a quarter of the
/// feed, this panel is the only place the opportunity can be read.
///
/// It shows what the sources published and nothing else — but it shows ALL of
/// it. A public notice carries no e-mail and no telephone, so for a tender the
/// panel says the buyer's CNPJ and the control number and stops; inventing a
/// contact would be worse than admitting there is none. The health registry,
/// though, publishes a phone, an address and an e-mail for most of the
/// territory, and the radar was reading that record already and rendering the
/// device counts alone.
///
/// The other half is what the account itself knows: the card this buyer
/// already has, the column it sits in, and the products the park says fit.
/// All three existed and none of them could be reached from here — the panel
/// named a state ("já está no seu quadro") with no way to open it, and listed
/// product names as dead text while the card two columns to the left made the
/// same names clickable.
class RadarDetailPanel extends ConsumerWidget {
  const RadarDetailPanel({super.key, required this.item, this.onClose});

  final RadarItem item;

  /// Set on the desk layout, where the panel lives beside the list. Null
  /// inside a sheet, which already has its own way out.
  final VoidCallback? onClose;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(18, 18, 10, 0),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Text(
                  item.title,
                  key: Key('radar_panel_title_${item.id}'),
                  style: typography.title.copyWith(fontSize: 20),
                ),
              ),
              if (onClose != null)
                IconButton(
                  key: const Key('radar_panel_close'),
                  tooltip: t.radar.panel_close,
                  onPressed: onClose,
                  icon: Icon(Icons.close, size: 18, color: colors.textMuted),
                ),
            ],
          ),
        ),
        Flexible(
          child: ListView(
            shrinkWrap: true,
            padding: const EdgeInsets.fromLTRB(18, 12, 18, 20),
            children: [
              if (item.location.isNotEmpty)
                _line(context, Icons.place_outlined, item.location),
              if (item.deadline != null)
                _line(context, Icons.schedule,
                    '${t.radar.panel_deadline}: ${_date(item.deadline!)}'),
              if (item.publishedAt != null)
                _line(context, Icons.event_outlined,
                    '${t.radar.panel_published}: ${_date(item.publishedAt!)}'),
              if (item.valueCents > 0)
                _line(context, Icons.payments_outlined,
                    '${t.radar.panel_value}: ${formatAccountMoney(ref, item.valueCents)}'),

              // The object, whole. The reason this panel exists.
              if (item.detail.trim().isNotEmpty) ...[
                const SizedBox(height: 14),
                _heading(context, _detailHeading(item)),
                const SizedBox(height: 6),
                SelectableText(
                  item.detail.trim(),
                  key: Key('radar_panel_detail_${item.id}'),
                  style: typography.body.copyWith(
                      fontSize: 13.5,
                      color: colors.textSecondary,
                      height: 1.55),
                ),
              ],

              // A park count is a month-old snapshot, and the panel repeats
              // the caveat rather than assuming the card was read.
              if (item.competencia != null) ...[
                const SizedBox(height: 8),
                Text(
                  t.radar
                      .data_month(month: competenciaLabel(item.competencia!)),
                  style: TextStyle(color: colors.textMuted, fontSize: 11.5),
                ),
              ],

              if (item.cnpj.isNotEmpty) ...[
                const SizedBox(height: 14),
                _heading(context, t.radar.panel_buyer),
                const SizedBox(height: 6),
                SelectableText(
                  item.cnpj,
                  key: Key('radar_panel_cnpj_${item.id}'),
                  style: typography.body.copyWith(fontSize: 13.5),
                ),
              ],

              // Who actually buys for this hospital. A public unit is not a
              // legal person, so its own CNPJ is often blank while the
              // secretaria that maintains it signs everything — and that is
              // the organization a seller has to reach and to bid against.
              if (item.buyerEntityName.isNotEmpty) ...[
                const SizedBox(height: 14),
                _heading(context, t.radar.panel_purchaser),
                const SizedBox(height: 6),
                SelectableText(
                  item.buyerEntityName,
                  key: Key('radar_panel_purchaser_${item.id}'),
                  style: typography.body.copyWith(fontSize: 13.5),
                ),
              ],

              if (item.incumbent.isNotEmpty) ...[
                const SizedBox(height: 10),
                _line(context, Icons.badge_outlined,
                    t.calendar.holder(name: item.incumbent)),
              ],

              if (item.reference.isNotEmpty) ...[
                const SizedBox(height: 14),
                _heading(context, t.radar.panel_reference),
                const SizedBox(height: 6),
                _Reference(reference: item.reference, itemId: item.id),
              ],

              // The honest ending. For a quarter of these there is nowhere to
              // send the seller, and saying so beats a button that opens
              // nothing — the same reason a blocked send says NO_ADDRESS
              // instead of failing at the click.
              if (item.url == null && item.reference.isNotEmpty) ...[
                const SizedBox(height: 10),
                Container(
                  key: Key('radar_panel_no_link_${item.id}'),
                  padding: const EdgeInsets.all(11),
                  decoration: BoxDecoration(
                    color: colors.glassLight,
                    borderRadius: BorderRadius.circular(10),
                    border: Border.all(color: colors.borderLight),
                  ),
                  child: Text(
                    t.radar.panel_no_link,
                    style: TextStyle(
                        color: colors.textSecondary, fontSize: 12, height: 1.5),
                  ),
                ),
              ],
              // The portal, named and clickable.
              //
              // The instruction used to read "search the number on the
              // procurement portal" without saying WHICH — half an
              // instruction. It is the PNCP, and the seller pastes the number
              // above into its search. The deep link to this particular notice
              // is deliberately not built: PNCP renders client-side, so a path
              // cannot be verified from outside, and a link that opens nothing
              // is worse than a search that works.
              if (item.url == null && item.reference.isNotEmpty) ...[
                const SizedBox(height: 8),
                TextButton.icon(
                  key: Key('radar_panel_open_portal_${item.id}'),
                  onPressed: () => launchUrl(
                      Uri.parse('https://pncp.gov.br/app/editais'),
                      mode: LaunchMode.externalApplication),
                  icon: const Icon(Icons.open_in_new, size: 15),
                  label: Text(t.radar.panel_open_portal,
                      style: const TextStyle(fontSize: 13)),
                  style: TextButton.styleFrom(
                      foregroundColor: colors.bluePrimary,
                      padding: EdgeInsets.zero,
                      minimumSize: const Size(0, 32),
                      tapTargetSize: MaterialTapTargetSize.shrinkWrap),
                ),
              ],
              if (item.url != null) ...[
                const SizedBox(height: 12),
                TextButton.icon(
                  key: Key('radar_panel_open_url_${item.id}'),
                  onPressed: () => launchUrl(Uri.parse(item.url!),
                      mode: LaunchMode.externalApplication),
                  icon: const Icon(Icons.open_in_new, size: 15),
                  label: Text(t.signals.open_notice,
                      style: const TextStyle(fontSize: 13)),
                  style: TextButton.styleFrom(
                      foregroundColor: colors.bluePrimary,
                      padding: EdgeInsets.zero,
                      minimumSize: const Size(0, 32),
                      tapTargetSize: MaterialTapTargetSize.shrinkWrap),
                ),
              ],

              // Who this buyer is to US, and who is already selling to them.
              // Two facts that change the first line of the message, both
              // sitting unread in this database until now.
              if (item.buyer != null) ...[
                const SizedBox(height: 16),
                _BuyerBlock(buyer: item.buyer!, itemId: item.id),
              ],

              // What the MINISTRY FUNDS here. Stronger than anything the
              // hospital declares about itself: an ordinance pays for it and
              // requires the equipment that entitles it.
              if (item.habilitations.isNotEmpty) ...[
                const SizedBox(height: 16),
                _heading(context, t.radar.panel_habilitations),
                const SizedBox(height: 6),
                for (final h in item.habilitations.take(6))
                  Padding(
                    padding: const EdgeInsets.only(bottom: 3),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Icon(Icons.verified_outlined,
                            size: 13,
                            color: h.isNeonatal
                                ? colors.limeGlow
                                : colors.textMuted),
                        const SizedBox(width: 7),
                        Expanded(
                          child: Text(
                            // Zero beds is common — many habilitations are
                            // not bed-based — and must not read as "no
                            // capacity", so the count only shows when it is
                            // a count.
                            h.beds > 0
                                ? t.radar.panel_habilitation_beds(
                                    name: h.name, n: h.beds)
                                : h.name,
                            key: Key('radar_panel_hab_${item.id}_${h.code}'),
                            style: TextStyle(
                                color: colors.textSecondary, fontSize: 12.5),
                          ),
                        ),
                      ],
                    ),
                  ),
              ],

              // What the place is BUILT to treat. The park says what it
              // owns; this says how much care it delivers, which is what
              // sizes an order — and the two disagreeing is the opening.
              if (item.beds.isNotEmpty) ...[
                const SizedBox(height: 16),
                _heading(context, t.radar.panel_beds),
                const SizedBox(height: 6),
                for (final b in item.beds.take(8))
                  Padding(
                    padding: const EdgeInsets.only(bottom: 3),
                    child: Row(
                      children: [
                        Text(
                          '${b.qtExist}×',
                          style: TextStyle(
                            color: b.isNeonatal
                                ? colors.limeGlow
                                : (b.isIcu
                                    ? colors.cyanGlow
                                    : colors.textSecondary),
                            fontSize: 12.5,
                            fontWeight: FontWeight.w600,
                            fontFeatures: AppTypographyExtension.tabular,
                          ),
                        ),
                        const SizedBox(width: 7),
                        Expanded(
                          child: Text(
                            b.name,
                            key: Key('radar_panel_bed_${item.id}_${b.bedCode}'),
                            style: TextStyle(
                                color: colors.textSecondary, fontSize: 12.5),
                          ),
                        ),
                      ],
                    ),
                  ),
              ],

              // What it says it performs. Capped by the server; a big
              // hospital declares dozens and past a handful the section
              // stops qualifying the buyer.
              if (item.services.isNotEmpty) ...[
                const SizedBox(height: 16),
                _heading(context, t.radar.panel_services),
                const SizedBox(height: 6),
                for (final sv in item.services)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 2),
                    child: Text(
                      '• ${sv.name}',
                      key: Key('radar_panel_srv_${item.id}_${sv.code}'),
                      style: TextStyle(color: colors.textMuted, fontSize: 12),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
              ],

              // How to reach them. A published notice carries none of this;
              // the health registry carries it for most of the territory, and
              // it was being read and thrown away on every park row.
              if (_hasReach) ...[
                const SizedBox(height: 16),
                _heading(context, t.radar.panel_reach),
                const SizedBox(height: 6),
                if (item.phone.isNotEmpty)
                  _CopyLine(
                    icon: Icons.call_outlined,
                    value: item.phone,
                    fieldKey: 'radar_panel_phone_${item.id}',
                  ),
                if (item.email.isNotEmpty)
                  _CopyLine(
                    icon: Icons.alternate_email,
                    value: item.email,
                    fieldKey: 'radar_panel_email_${item.id}',
                  ),
                if (item.address.isNotEmpty)
                  _line(context, Icons.location_on_outlined, item.address),
              ],

              // What of the catalogue fits. Already on the card as chips, and
              // repeated here because the panel is where somebody decides —
              // so here, as on the card, each one opens its product. The
              // panel listed them as dead bullets while the card two columns
              // to the left made the same names clickable.
              if (item.matches.isNotEmpty) ...[
                const SizedBox(height: 16),
                _heading(context, t.radar.panel_offer_title),
                const SizedBox(height: 6),
                for (final m in item.matches) _OfferRow(match: m),
              ],
            ],
          ),
        ),
      ],
    );
  }

  bool get _hasReach =>
      item.phone.isNotEmpty || item.email.isNotEmpty || item.address.isNotEmpty;

  /// A tender's detail is the object; a park's is its inventory. Naming it
  /// wrong would make the panel read as boilerplate.
  String _detailHeading(RadarItem i) => switch (i.type) {
        RadarItemType.parque => t.radar.panel_park_items,
        _ => t.radar.panel_object,
      };

  static String _date(DateTime d) =>
      '${d.day.toString().padLeft(2, '0')}/${d.month.toString().padLeft(2, '0')}/${d.year}';

  Widget _heading(BuildContext context, String text) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Text(
      text.toUpperCase(),
      style: TextStyle(
        color: colors.textMuted,
        fontSize: 11,
        fontWeight: FontWeight.w600,
        letterSpacing: 0.7,
      ),
    );
  }

  Widget _line(BuildContext context, IconData icon, String text) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 14, color: colors.textMuted),
          const SizedBox(width: 8),
          Expanded(
            child: Text(text,
                style: TextStyle(color: colors.textSecondary, fontSize: 12.5)),
          ),
        ],
      ),
    );
  }
}

/// The control number, copyable.
///
/// For a notice with no link this is the whole address: the seller pastes it
/// into the procurement portal's search. Selecting a 28-character string by
/// dragging is exactly the kind of friction that ends with somebody retyping
/// it wrong.
class _Reference extends StatelessWidget {
  const _Reference({required this.reference, required this.itemId});

  final String reference;
  final String itemId;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Row(
      children: [
        Flexible(
          child: SelectableText(
            reference,
            key: Key('radar_panel_reference_$itemId'),
            style: TextStyle(
                color: colors.textPrimary,
                fontSize: 13,
                fontFeatures: AppTypographyExtension.tabular),
          ),
        ),
        const SizedBox(width: 6),
        IconButton(
          key: Key('radar_panel_copy_$itemId'),
          tooltip: t.radar.panel_copy_reference,
          visualDensity: VisualDensity.compact,
          onPressed: () async {
            await Clipboard.setData(ClipboardData(text: reference));
            if (!context.mounted) return;
            ScaffoldMessenger.of(context)
                .showSnackBar(SnackBar(content: Text(t.radar.panel_copied)));
          },
          icon: Icon(Icons.copy, size: 15, color: colors.textMuted),
        ),
      ],
    );
  }
}

/// One product this establishment's park says we can sell it — and the way
/// into it.
///
/// The card's chips already opened the product; this list repeated the same
/// names as dead text, in the place where somebody actually decides. A seller
/// who wants the price or the spec had to leave the screen and search the
/// catalogue for a name they could see.
class _OfferRow extends StatelessWidget {
  const _OfferRow({required this.match});

  final ParkMatch match;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final label = Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('• ',
            style: typography.body
                .copyWith(fontSize: 13, color: colors.textSecondary)),
        Expanded(
          child: Text(
            match.productName,
            style: typography.body.copyWith(
              fontSize: 13,
              color: match.productId.isEmpty
                  ? colors.textSecondary
                  : colors.bluePrimary,
            ),
          ),
        ),
        if (match.productId.isNotEmpty)
          Icon(Icons.chevron_right, size: 15, color: colors.textMuted),
      ],
    );

    // A product the matcher named but could not identify has nowhere to go.
    // Rendering it as a link would be a tap that does nothing — the same rule
    // the card's chips already follow.
    if (match.productId.isEmpty) {
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: label,
      );
    }
    return InkWell(
      key: Key('radar_panel_offer_open_${match.productId}'),
      borderRadius: BorderRadius.circular(6),
      onTap: () => context.go('/sales/products/${match.productId}'),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: label,
      ),
    );
  }
}

/// A reachable address, copyable.
///
/// Copy rather than a `tel:`/`mailto:` launch: this is a desk tool, the
/// seller's mail client and dialler are their own, and a launch that silently
/// does nothing on web is worse than a value they can paste.
class _CopyLine extends StatelessWidget {
  const _CopyLine({
    required this.icon,
    required this.value,
    required this.fieldKey,
  });

  final IconData icon;
  final String value;
  final String fieldKey;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Padding(
      padding: const EdgeInsets.only(bottom: 2),
      child: Row(
        children: [
          Icon(icon, size: 14, color: colors.textMuted),
          const SizedBox(width: 8),
          Flexible(
            child: SelectableText(
              value,
              key: Key(fieldKey),
              style: TextStyle(color: colors.textSecondary, fontSize: 12.5),
            ),
          ),
          IconButton(
            key: Key('${fieldKey}_copy'),
            tooltip: t.common.copy,
            visualDensity: VisualDensity.compact,
            onPressed: () async {
              await Clipboard.setData(ClipboardData(text: value));
              if (!context.mounted) return;
              ScaffoldMessenger.of(context)
                  .showSnackBar(SnackBar(content: Text(t.radar.panel_copied)));
            },
            icon: Icon(Icons.copy, size: 14, color: colors.textMuted),
          ),
        ],
      ),
    );
  }
}

/// The buyer, as this account knows them.
class _BuyerBlock extends StatelessWidget {
  const _BuyerBlock({required this.buyer, required this.itemId});

  final BuyerContext buyer;
  final String itemId;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Warm, written-to, or cold — said plainly and kept apart. Opening a
        // stranger's letter to an existing customer loses one; claiming a sale
        // that never happened loses the next one.
        Container(
          key: Key('radar_panel_buyer_known_$itemId'),
          padding: const EdgeInsets.symmetric(horizontal: 11, vertical: 8),
          decoration: BoxDecoration(
            color: _tint(colors).withValues(alpha: 0.10),
            borderRadius: BorderRadius.circular(999),
            border: Border.all(color: _tint(colors).withValues(alpha: 0.4)),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(_icon, size: 14, color: _tint(colors)),
              const SizedBox(width: 7),
              Flexible(
                child: Text(
                  _label,
                  style: TextStyle(
                    color: _tint(colors),
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
          ),
        ),

        // WHERE the card sits, and the way into it.
        //
        // "Já está no seu quadro" named a state and offered no way to act on
        // it: the seller went looking for the hospital by name across the
        // whole board. And the state alone is not a position — a card in
        // "Ganhos" and one nobody has touched are both "no quadro", and they
        // call for opposite next moves.
        if (buyer.isOnBoard) ...[
          const SizedBox(height: 10),
          Row(
            children: [
              if (buyer.stage.isNotEmpty) ...[
                Icon(Icons.view_column_outlined,
                    size: 14, color: colors.textMuted),
                const SizedBox(width: 7),
                Flexible(
                  child: Text(
                    t.radar.panel_stage(stage: buyer.stage),
                    key: Key('radar_panel_stage_$itemId'),
                    style:
                        TextStyle(color: colors.textSecondary, fontSize: 12.5),
                  ),
                ),
              ],
            ],
          ),
          const SizedBox(height: 4),
          TextButton.icon(
            key: Key('radar_panel_open_board_$itemId'),
            onPressed: () => context.go('/funnel?lead=${buyer.leadId}'),
            icon: const Icon(Icons.open_in_new, size: 15),
            label: Text(t.radar.panel_open_board,
                style: const TextStyle(fontSize: 13)),
            style: TextButton.styleFrom(
                foregroundColor: colors.bluePrimary,
                padding: EdgeInsets.zero,
                minimumSize: const Size(0, 32),
                tapTargetSize: MaterialTapTargetSize.shrinkWrap),
          ),
        ],

        // What the published record says they have contracted. The only place
        // a competitor's name may honestly come from — never the installed
        // park, which publishes counts and never a brand.
        if (buyer.hasHistory) ...[
          const SizedBox(height: 12),
          Text(
            t.radar.panel_history.toUpperCase(),
            style: TextStyle(
              color: colors.textMuted,
              fontSize: 11,
              fontWeight: FontWeight.w600,
              letterSpacing: 0.7,
            ),
          ),
          const SizedBox(height: 6),
          Text(
            // Whose record this is, said out loud when it is not the
            // hospital's. Seven hospitals share one secretaria, so an
            // unqualified "este comprador já contratou" would be the same
            // false sentence on all seven cards.
            buyer.historyVia.isEmpty
                ? t.radar.panel_history_count(n: buyer.contracts)
                : t.radar.panel_history_via(
                    n: buyer.contracts, org: buyer.historyVia),
            key: Key('radar_panel_history_$itemId'),
            style: TextStyle(color: colors.textSecondary, fontSize: 12.5),
          ),
          if (buyer.lastSupplier.isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(
              t.calendar.holder(name: buyer.lastSupplier),
              key: Key('radar_panel_last_supplier_$itemId'),
              style: TextStyle(
                  color: colors.textPrimary,
                  fontSize: 12.5,
                  fontWeight: FontWeight.w600),
            ),
          ],
          if (buyer.lastObject.isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(
              buyer.lastObject,
              style: TextStyle(color: colors.textMuted, fontSize: 12),
              maxLines: 3,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ] else if (buyer.scanned) ...[
          // Read and found nothing of ours. A different sentence from silence:
          // silence would let the seller assume we simply had not looked.
          const SizedBox(height: 10),
          Text(
            t.radar.panel_history_none,
            key: Key('radar_panel_history_none_$itemId'),
            style: TextStyle(color: colors.textMuted, fontSize: 12.5),
          ),
        ],
      ],
    );
  }

  Color _tint(AppColorsExtension colors) => switch (buyer.relationship) {
        BuyerRelationship.sold => colors.limeGlow,
        BuyerRelationship.contacted => colors.cyanGlow,
        BuyerRelationship.queued => colors.orangeAlert,
        BuyerRelationship.listed => colors.textSecondary,
        BuyerRelationship.cold => colors.textMuted,
      };

  IconData get _icon => switch (buyer.relationship) {
        BuyerRelationship.sold => Icons.handshake_outlined,
        BuyerRelationship.contacted => Icons.mark_email_read_outlined,
        BuyerRelationship.queued => Icons.drafts_outlined,
        BuyerRelationship.listed => Icons.view_kanban_outlined,
        BuyerRelationship.cold => Icons.person_outline,
      };

  String get _label => switch (buyer.relationship) {
        BuyerRelationship.sold => t.radar.panel_buyer_known,
        BuyerRelationship.contacted => t.radar.panel_buyer_contacted,
        BuyerRelationship.queued => t.radar.panel_buyer_queued,
        BuyerRelationship.listed => t.radar.panel_buyer_listed,
        BuyerRelationship.cold => t.radar.panel_buyer_cold,
      };
}
