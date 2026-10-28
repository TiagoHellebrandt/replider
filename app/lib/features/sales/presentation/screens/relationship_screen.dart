import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/components/empty_state_card.dart';
import 'package:app/design_system/components/glass_card.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/relationship_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Standalone wrapper for the deep link `/sales/relationship`.
class RelationshipScreen extends StatelessWidget {
  const RelationshipScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundPrimary,
        // Without this the screen is a dead end: these three are reached by
        // deep link and render outside the shell, so there was no back button
        // and no bottom nav — you got in and could not get out except by
        // editing the URL.
        leading: IconButton(
          tooltip: t.sales.back_tooltip,
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () =>
              context.canPop() ? context.pop() : context.go('/funnel'),
        ),
        title: Text(t.relationship.title),
      ),
      body: const RelationshipBody(),
    );
  }
}

/// Relationship routines (§6.5, F4).
///
/// Nothing on this screen sends anything. It records what is worth
/// remembering; on the day, the draft lands in the approval queue like every
/// other outbound message. The subtitle says so out loud, because a screen
/// that quietly schedules messages to customers is one nobody should have to
/// discover by watching their outbox.
class RelationshipBody extends ConsumerWidget {
  const RelationshipBody({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final state = ref.watch(relationshipDatesProvider);

    return Scaffold(
      backgroundColor: Colors.transparent,
      floatingActionButton: FloatingActionButton.extended(
        key: const Key('relationship_add'),
        onPressed: () => _showAddSheet(context, ref),
        icon: const Icon(Icons.event_outlined),
        label: Text(t.relationship.add),
      ),
      body: state.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => RetryableError(
          error: err,
          onRetry: () => ref.invalidate(relationshipDatesProvider),
        ),
        data: (dates) => RefreshIndicator(
          onRefresh: () async => ref.invalidate(relationshipDatesProvider),
          child: ListView(
            padding: const EdgeInsets.fromLTRB(16, 16, 16, 96),
            children: [
              Text(
                t.relationship.subtitle,
                style: typography.body.copyWith(
                    color: colors.textSecondary, fontSize: 13, height: 1.4),
              ),
              const SizedBox(height: 8),
              // The safety rule, on the screen rather than in a doc.
              Text(
                t.relationship.greeting_only,
                style: TextStyle(color: colors.textSecondary, fontSize: 11),
              ),
              const SizedBox(height: 16),
              if (dates.isEmpty)
                EmptyStateCard(
                  key: const Key('relationship_empty'),
                  icon: Icons.cake_outlined,
                  title: t.relationship.empty,
                  subtitle: t.relationship.empty_hint,
                )
              else
                for (final date in dates)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: _DateRow(date: date),
                  ),
            ],
          ),
        ),
      ),
    );
  }
}

class _DateRow extends ConsumerWidget {
  const _DateRow({required this.date});

  final RelationshipDate date;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return GlassCard(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    date.label.isEmpty ? _kindLabel(date.kind) : date.label,
                    style: const TextStyle(
                        color: Colors.white,
                        fontSize: 14,
                        fontWeight: FontWeight.w600),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    _when(date),
                    key: Key('relationship_when_${date.id}'),
                    style: TextStyle(color: colors.textSecondary, fontSize: 12),
                  ),
                  if (date.accountWide) ...[
                    const SizedBox(height: 4),
                    // Says plainly that this one never becomes a message on
                    // its own — otherwise somebody adds Dia do Médico and
                    // waits for six hundred greetings that never come.
                    Text(
                      t.relationship.account_wide,
                      key: Key('relationship_account_wide_${date.id}'),
                      style:
                          TextStyle(color: colors.textSecondary, fontSize: 11),
                    ),
                  ],
                ],
              ),
            ),
            IconButton(
              key: Key('relationship_delete_${date.id}'),
              icon: Icon(Icons.delete_outline,
                  size: 18, color: colors.textSecondary),
              onPressed: () => _confirmDelete(context, ref, date),
            ),
          ],
        ),
      ),
    );
  }
}

String _kindLabel(String kind) => switch (kind) {
      'birthday' => t.relationship.kind_birthday,
      'founded' => t.relationship.kind_founded,
      'sector' => t.relationship.kind_sector,
      'maintenance' => t.relationship.kind_maintenance,
      _ => t.relationship.kind_custom,
    };

/// When it comes round. A recurring date shows dd/MM with no year, because a
/// year on a birthday is the wrong fact: what matters is that it returns.
String _when(RelationshipDate date) {
  if (date.dueAt case final due?) {
    return '${due.day.toString().padLeft(2, '0')}/'
        '${due.month.toString().padLeft(2, '0')}/${due.year}';
  }
  return '${date.day.toString().padLeft(2, '0')}/'
      '${date.month.toString().padLeft(2, '0')}';
}

Future<void> _confirmDelete(
    BuildContext context, WidgetRef ref, RelationshipDate date) async {
  final confirmed = await showConfirmDialog(
    context,
    title: t.relationship.delete_confirm,
    message: date.label.isEmpty ? _kindLabel(date.kind) : date.label,
    confirmText: t.common.delete,
    destructive: true,
  );
  if (!confirmed) return;

  try {
    await ref.read(relationshipActionsProvider).delete(date.id);
    ref.invalidate(relationshipDatesProvider);
  } catch (e) {
    if (context.mounted) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(parseApiError(e))));
    }
  }
}

Future<void> _showAddSheet(BuildContext context, WidgetRef ref) =>
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => const _AddDateSheet(),
    );

class _AddDateSheet extends ConsumerStatefulWidget {
  const _AddDateSheet();

  @override
  ConsumerState<_AddDateSheet> createState() => _AddDateSheetState();
}

class _AddDateSheetState extends ConsumerState<_AddDateSheet> {
  String _kind = 'birthday';
  String _sectorKey = '';
  final _label = TextEditingController();
  final _day = TextEditingController();
  final _month = TextEditingController();
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    // The save button reflects what is typed, so it can be dead while there
    // is no date to save.
    _day.addListener(_onFormChanged);
    _month.addListener(_onFormChanged);
  }

  void _onFormChanged() => setState(() {});

  /// Whether this form can produce a date at all.
  ///
  /// The server refuses an empty one with "provide a date" — correct, but
  /// the person only found out after pressing a button that looked ready,
  /// and the answer arrived in a snackbar half-hidden behind this sheet.
  bool get _canSave {
    if (_kind == 'sector') return _sectorKey.isNotEmpty;
    return _validDayMonth(int.tryParse(_day.text.trim()) ?? 0,
        int.tryParse(_month.text.trim()) ?? 0);
  }

  /// Mirrors `validMonthDay` on the API side, including the deliberate
  /// acceptance of 29 February: somebody born on it has a birthday, and the
  /// system saying otherwise is the system arguing with a fact.
  static bool _validDayMonth(int day, int month) {
    if (month < 1 || month > 12 || day < 1 || day > 31) return false;
    if (month == 2) return day <= 29;
    if (const [4, 6, 9, 11].contains(month)) return day <= 30;
    return true;
  }

  @override
  void dispose() {
    _day.removeListener(_onFormChanged);
    _month.removeListener(_onFormChanged);
    _label.dispose();
    _day.dispose();
    _month.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final sectors = ref.watch(sectorDatesProvider).asData?.value ?? const [];

    return Padding(
      padding: EdgeInsets.only(
          bottom: MediaQuery.of(context).viewInsets.bottom,
          left: 16,
          right: 16,
          top: 16),
      child: GlassCard(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(t.relationship.add,
                  style: const TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.w700)),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                key: const Key('relationship_kind'),
                initialValue: _kind,
                dropdownColor: colors.backgroundSecondary,
                style: const TextStyle(color: Colors.white, fontSize: 14),
                items: [
                  for (final kind in const [
                    'birthday',
                    'founded',
                    'sector',
                    'maintenance',
                    'custom'
                  ])
                    DropdownMenuItem(value: kind, child: Text(_kindLabel(kind)))
                ],
                onChanged: (v) => setState(() => _kind = v ?? 'birthday'),
              ),
              const SizedBox(height: 12),
              // The catalogue, so nobody has to research that Dia do
              // Fonoaudiólogo is 9 December.
              if (_kind == 'sector')
                DropdownButtonFormField<String>(
                  key: const Key('relationship_sector'),
                  initialValue: _sectorKey.isEmpty ? null : _sectorKey,
                  dropdownColor: colors.backgroundSecondary,
                  style: const TextStyle(color: Colors.white, fontSize: 14),
                  hint: Text(t.relationship.sector_hint,
                      style: TextStyle(color: colors.textSecondary)),
                  items: [
                    for (final sector in sectors)
                      DropdownMenuItem(
                        value: sector.key,
                        child: Text('${sector.label} '
                            '(${sector.day.toString().padLeft(2, '0')}/'
                            '${sector.month.toString().padLeft(2, '0')})'),
                      )
                  ],
                  onChanged: (v) => setState(() => _sectorKey = v ?? ''),
                )
              else
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        key: const Key('relationship_day'),
                        controller: _day,
                        keyboardType: TextInputType.number,
                        style: const TextStyle(color: Colors.white),
                        decoration:
                            InputDecoration(labelText: t.relationship.day),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: TextField(
                        key: const Key('relationship_month'),
                        controller: _month,
                        keyboardType: TextInputType.number,
                        style: const TextStyle(color: Colors.white),
                        decoration:
                            InputDecoration(labelText: t.relationship.month),
                      ),
                    ),
                  ],
                ),
              const SizedBox(height: 12),
              TextField(
                key: const Key('relationship_label'),
                controller: _label,
                style: const TextStyle(color: Colors.white),
                decoration: InputDecoration(labelText: t.relationship.label),
              ),
              const SizedBox(height: 16),
              Align(
                alignment: Alignment.centerRight,
                child: FilledButton(
                  key: const Key('relationship_save'),
                  onPressed: _saving || !_canSave ? null : _save,
                  child: Text(t.common.save),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _save() async {
    setState(() => _saving = true);
    try {
      await ref.read(relationshipActionsProvider).save(
            kind: _kind,
            sectorKey: _sectorKey,
            day: int.tryParse(_day.text) ?? 0,
            month: int.tryParse(_month.text) ?? 0,
            label: _label.text.trim(),
          );
      ref.invalidate(relationshipDatesProvider);
      if (mounted) Navigator.of(context).pop();
    } catch (e) {
      if (mounted) {
        setState(() => _saving = false);
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(parseApiError(e))));
      }
    }
  }
}
