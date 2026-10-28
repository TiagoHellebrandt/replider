import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/utils/currency_format.dart';
import 'package:app/design_system/components/empty_state_card.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/contract_calendar_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// The buying calendar.
///
/// Every other screen in this product reacts to something that already
/// happened: a tender was published, a lead replied, a draft is waiting. This
/// one is the only screen that looks forward — a contract with a published end
/// date is a purchase that MUST happen again, on a date anyone can read, three
/// to six months before a tender exists.
///
/// Ordered by date and nothing else. The question it answers is "what do I
/// work on this month", and any other ordering answers a different question.
class ContractCalendarBody extends ConsumerWidget {
  const ContractCalendarBody({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final async = ref.watch(contractCalendarProvider);

    return async.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (e, _) => RetryableError(
        error: e,
        onRetry: () => ref.invalidate(contractCalendarProvider),
      ),
      data: (calendar) {
        if (calendar.entries.isEmpty) {
          return _EmptyCalendar(calendar: calendar);
        }
        return ListView.builder(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 28),
          itemCount: calendar.entries.length + 1,
          itemBuilder: (context, i) {
            if (i == 0) {
              return Padding(
                padding: const EdgeInsets.only(bottom: 14),
                child: Text(
                  t.calendar.subtitle(days: calendar.windowDays),
                  key: const Key('calendar_subtitle'),
                  style: TextStyle(fontSize: 13, color: colors.textSecondary),
                ),
              );
            }
            return _WindowCard(window: calendar.entries[i - 1]);
          },
        );
      },
    );
  }
}

/// An empty calendar that says WHICH kind of empty it is.
///
/// Three very different facts render as zero rows: no territory, no words, and
/// a sweep that has not reached this state yet. Showing the same blank for all
/// three is how a working feature gets reported as broken.
class _EmptyCalendar extends StatelessWidget {
  const _EmptyCalendar({required this.calendar});

  final ContractCalendar calendar;

  @override
  Widget build(BuildContext context) {
    final (key, title, body) = switch (calendar) {
      final c when !c.hasTerritory => (
          'calendar_empty_no_territory',
          t.calendar.no_territory_title,
          t.calendar.no_territory_body,
        ),
      final c when !c.hasKeywords => (
          'calendar_empty_no_keywords',
          t.calendar.no_keywords_title,
          t.calendar.no_keywords_body,
        ),
      final c when c.scanning => (
          'calendar_empty_scanning',
          t.calendar.scanning_title,
          t.calendar.scanning_body,
        ),
      _ => (
          'calendar_empty',
          t.calendar.empty_title,
          t.calendar.empty_body(days: calendar.windowDays),
        ),
    };
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: EmptyStateCard(
          key: Key(key),
          icon: Icons.event_available_outlined,
          title: title,
          subtitle: body,
        ),
      ),
    );
  }
}

class _WindowCard extends ConsumerStatefulWidget {
  const _WindowCard({required this.window});

  final ContractWindow window;

  @override
  ConsumerState<_WindowCard> createState() => _WindowCardState();
}

class _WindowCardState extends ConsumerState<_WindowCard> {
  bool _working = false;

  ContractWindow get window => widget.window;

  /// From a date to a card in the funnel.
  ///
  /// Without this the calendar is a list you read once. Nothing is drafted and
  /// nothing is sent — a public body has no e-mail on the record, so the card
  /// arrives asking for the one thing it needs: the buyer's contact.
  Future<void> _work() async {
    setState(() => _working = true);
    try {
      await ref.read(calendarActionsProvider).work(pncpId: window.pncpId);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(t.calendar.worked(name: window.buyerName)),
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
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    final days = window.daysLeft;
    // The clock is the whole point of the card, so it carries the colour.
    final tone = window.hasClosed
        ? colors.textMuted
        : window.isNear
            ? colors.orangeAlert
            : colors.cyanGlow;
    final when = days == null
        ? ''
        : window.hasClosed
            ? t.calendar.days_past(count: -days)
            : days == 0
                ? t.calendar.days_today
                : t.calendar.days_left(count: days);

    return Container(
      key: Key('calendar_card_${window.pncpId}'),
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: colors.borderMedium),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Text(
                  window.buyerName,
                  style: typography.body.copyWith(
                      color: Colors.white,
                      fontSize: 15,
                      fontWeight: FontWeight.w700),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              if (when.isNotEmpty) ...[
                const SizedBox(width: 10),
                Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(
                    color: tone.withValues(alpha: 0.14),
                    borderRadius: BorderRadius.circular(999),
                    border: Border.all(color: tone.withValues(alpha: 0.45)),
                  ),
                  child: Text(
                    when,
                    key: Key('calendar_when_${window.pncpId}'),
                    style: TextStyle(
                        fontSize: 11.5,
                        fontWeight: FontWeight.w700,
                        color: tone),
                  ),
                ),
              ],
            ],
          ),
          const SizedBox(height: 6),
          Text(
            window.object,
            style: typography.body
                .copyWith(color: colors.textSecondary, fontSize: 13),
            maxLines: 3,
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(height: 10),
          Wrap(
            spacing: 8,
            runSpacing: 6,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              // Who holds it today. The ONLY place a competitor's name may
              // come from is this public record — never a guess off the
              // installed park, which publishes counts and never a brand.
              _Chip(
                icon: Icons.badge_outlined,
                label: window.supplierName.trim().isEmpty
                    ? t.calendar.holder_unknown
                    : t.calendar.holder(name: window.supplierName),
                muted: window.supplierName.trim().isEmpty,
              ),
              _Chip(
                icon: Icons.payments_outlined,
                label: window.valueCents > 0
                    ? formatCurrencyCents(window.valueCents, 'brl')
                    : t.calendar.value_unknown,
                muted: window.valueCents <= 0,
              ),
              if (window.location.isNotEmpty)
                _Chip(
                    icon: Icons.place_outlined,
                    label: window.location,
                    muted: true),
              if (window.priceRegistry)
                _Chip(
                    icon: Icons.receipt_long_outlined,
                    label: t.calendar.price_registry),
              // The most actionable bit in the whole record, so it gets a
              // sentence rather than a word.
              if (window.ridesOthers)
                Tooltip(
                  message: t.calendar.rides_others_hint,
                  child: _Chip(
                      icon: Icons.merge_type_outlined,
                      label: t.calendar.rides_others),
                ),
            ],
          ),
          const SizedBox(height: 10),
          Wrap(
            spacing: 14,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              // The action, first. A calendar whose only verb is "open the
              // public record" gets read once and never again.
              TextButton.icon(
                key: Key('calendar_work_${window.pncpId}'),
                onPressed: _working ? null : _work,
                icon: _working
                    ? const SizedBox(
                        width: 14,
                        height: 14,
                        child: CircularProgressIndicator(strokeWidth: 2))
                    : const Icon(Icons.playlist_add, size: 16),
                label: Text(t.calendar.work,
                    style: const TextStyle(fontSize: 12.5)),
                style: TextButton.styleFrom(
                  foregroundColor: colors.cyanGlow,
                  padding: EdgeInsets.zero,
                  minimumSize: const Size(0, 30),
                  tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                ),
              ),
              if (window.url.isNotEmpty)
                TextButton.icon(
                  key: Key('calendar_open_${window.pncpId}'),
                  onPressed: () => launchUrl(Uri.parse(window.url),
                      mode: LaunchMode.externalApplication),
                  icon: const Icon(Icons.open_in_new, size: 15),
                  label: Text(t.calendar.open_record,
                      style: const TextStyle(fontSize: 12.5)),
                  style: TextButton.styleFrom(
                    foregroundColor: colors.bluePrimary,
                    padding: EdgeInsets.zero,
                    minimumSize: const Size(0, 30),
                    tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  ),
                ),
            ],
          ),
        ],
      ),
    );
  }
}

class _Chip extends StatelessWidget {
  const _Chip({required this.icon, required this.label, this.muted = false});

  final IconData icon;
  final String label;
  final bool muted;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final tone = muted ? colors.textMuted : colors.textSecondary;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 13, color: tone),
        const SizedBox(width: 5),
        ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 260),
          child: Text(
            label,
            style: TextStyle(fontSize: 12, color: tone),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );
  }
}
