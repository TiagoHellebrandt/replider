import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/utils/account_currency.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/sales_import.dart';
import 'package:app/features/sales/presentation/controllers/portfolio_controller.dart';
import 'package:app/features/sales/presentation/controllers/radar_controller.dart';
import 'package:app/features/sales/presentation/controllers/sales_import_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Import the sales history the account already has.
///
/// After products and customers come in, the product knows WHO the account
/// sells to and WHAT it sells — and nothing about who bought what, which is
/// the only thing the Carteira, the reorder engine and the cross-sell read.
/// This is the door for that history ("vendas como fonte única").
///
/// A review, not an upload — same shape as the customer import: the file is
/// read, every line is shown with what the server made of it, and only then
/// does anything get recorded.
class SalesImportScreen extends ConsumerStatefulWidget {
  const SalesImportScreen({super.key});

  @override
  ConsumerState<SalesImportScreen> createState() => _SalesImportScreenState();
}

class _SalesImportScreenState extends ConsumerState<SalesImportScreen> {
  SalesImportPreview? _preview;
  SalesImportResult? _result;
  bool _busy = false;
  String? _filename;

  Future<void> _pick() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['csv', 'txt'],
      withData: true,
    );
    if (picked == null || picked.files.isEmpty) return;
    final file = picked.files.first;
    if (file.bytes == null) return;

    setState(() {
      _busy = true;
      // A new file invalidates the old reading. Leaving the previous preview
      // on screen while a new one loads is how somebody confirms the wrong
      // history.
      _preview = null;
      _result = null;
      _filename = file.name;
    });

    try {
      final preview = await ref.read(salesImportActionsProvider).preview(
            bytes: file.bytes!,
            filename: file.name,
          );
      if (!mounted) return;
      setState(() => _preview = preview);
    } catch (e) {
      if (!mounted) return;
      _showError(e);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _confirm() async {
    final rows = _preview?.rows.where((r) => r.importable).toList() ?? const [];
    if (rows.isEmpty) return;

    setState(() => _busy = true);
    try {
      final result = await ref.read(salesImportActionsProvider).confirm(rows);
      if (!mounted) return;
      setState(() {
        _result = result;
        // The reading is spent: those sales are recorded now, and showing
        // them as importable again would invite a second, duplicate import.
        _preview = null;
      });
      // The history just became the portfolio's fuel: the Carteira's reorder
      // dates and the Radar's cross-sell both read from `sales`.
      ref.invalidate(portfolioProvider);
      ref.invalidate(radarProvider);
    } catch (e) {
      if (!mounted) return;
      _showError(e);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _showError(Object e) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        key: const Key('sales_import_error'),
        content: Text(parseApiError(e)),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final preview = _preview;

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundPrimary,
        leading: IconButton(
          key: const Key('sales_import_back'),
          tooltip: t.sales.back_tooltip,
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () => context.canPop()
              ? context.pop()
              : context.go('/funnel?view=portfolio'),
        ),
        title: Text(t.sales_import.title),
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          // What the file needs to look like, before she goes looking for it.
          Text(
            t.sales_import.hint,
            key: const Key('sales_import_help'),
            style: TextStyle(color: colors.textMuted, fontSize: 13),
          ),
          const SizedBox(height: 12),
          OutlinedButton.icon(
            key: const Key('sales_import_pick'),
            onPressed: _busy ? null : _pick,
            icon: const Icon(Icons.upload_file),
            label: Text(_filename == null
                ? t.sales_import.pick_file
                : t.sales_import.pick_another),
          ),
          if (_filename != null) ...[
            const SizedBox(height: 6),
            Text(_filename!,
                style: TextStyle(color: colors.textMuted, fontSize: 12)),
          ],
          if (_busy) ...[
            const SizedBox(height: 16),
            const Center(child: CircularProgressIndicator()),
          ],
          if (_result != null) ...[
            const SizedBox(height: 20),
            SalesImportResultNotice(
              result: _result!,
              onViewPortfolio: () => context.go('/funnel?view=portfolio'),
            ),
          ],
          if (preview != null)
            SalesImportReview(
              preview: preview,
              busy: _busy,
              onConfirm: _confirm,
            ),
        ],
      ),
    );
  }
}

/// The reviewable half of the import: what the file said, and the one button
/// that acts on it.
///
/// Split out of the screen because the file picker above it is platform code a
/// widget test cannot drive, and the review is the part a person actually has
/// to be able to check — same split as [CustomerImportReview].
class SalesImportReview extends ConsumerWidget {
  const SalesImportReview({
    super.key,
    required this.preview,
    required this.onConfirm,
    this.busy = false,
  });

  final SalesImportPreview preview;
  final VoidCallback onConfirm;
  final bool busy;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final importable = preview.importable;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(height: 20),
        _Summary(preview: preview),
        const SizedBox(height: 12),
        // Every line, including the ones that cannot be imported. A line
        // dropped from this list is a sale she believes she recorded.
        for (final row in preview.rows) _RowTile(row: row),
        const SizedBox(height: 20),
        FilledButton.icon(
          key: const Key('sales_import_confirm'),
          onPressed: (busy || importable == 0) ? null : onConfirm,
          icon: const Icon(Icons.receipt_long_outlined),
          // The button states HOW MANY it will record. "Importar" over a list
          // where half the rows have problems is a promise it will not keep.
          label: Text(t.sales_import.confirm(count: '$importable')),
        ),
        if (importable == 0) ...[
          const SizedBox(height: 6),
          Text(
            t.sales_import.nothing_importable,
            key: const Key('sales_import_nothing'),
            style: TextStyle(color: colors.textMuted, fontSize: 12),
          ),
        ],
      ],
    );
  }
}

/// The counts, as the server reported them.
class _Summary extends StatelessWidget {
  const _Summary({required this.preview});

  final SalesImportPreview preview;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Container(
      key: const Key('sales_import_summary'),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.glassMedium,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.sales_import.found(
              total: '${preview.rows.length}',
              importable: '${preview.importable}',
            ),
            style: TextStyle(
                color: colors.textPrimary, fontWeight: FontWeight.w600),
          ),
          if (preview.problems > 0) ...[
            const SizedBox(height: 4),
            Text(
              t.sales_import.problems_title(count: '${preview.problems}'),
              key: const Key('sales_import_problems'),
              style: TextStyle(color: colors.orangeAlert, fontSize: 13),
            ),
          ],
          // Two separate numbers because they are two different situations: a
          // recorded sale needs nothing, an unknown customer needs the OTHER
          // import first.
          if (preview.alreadyRecorded > 0) ...[
            const SizedBox(height: 4),
            Text(
              t.sales_import
                  .already_recorded(count: '${preview.alreadyRecorded}'),
              key: const Key('sales_import_already'),
              style: TextStyle(color: colors.textMuted, fontSize: 13),
            ),
          ],
          if (preview.unknownCustomers > 0) ...[
            const SizedBox(height: 4),
            Text(
              t.sales_import
                  .unknown_customers(count: '${preview.unknownCustomers}'),
              key: const Key('sales_import_unknown'),
              style: TextStyle(color: colors.orangeAlert, fontSize: 13),
            ),
          ],
        ],
      ),
    );
  }
}

/// One line of the file.
class _RowTile extends ConsumerWidget {
  const _RowTile({required this.row});

  final SalesImportRow row;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final ok = row.importable;

    // A duplicate is a fact (the recorded sale stays), not a fault — it must
    // not wear the same error icon as an unreadable date.
    final (IconData icon, Color color) = row.duplicate
        ? (Icons.info_outline, colors.textMuted)
        : ok
            ? (Icons.check_circle_outline, colors.whatsappGreen)
            : (Icons.error_outline, colors.orangeAlert);

    // The date and the money — never just the name. A list of names cannot
    // catch the mistake this review exists to catch: a value parsed 100×
    // wrong, or a sale filed on the wrong day. The line number is always
    // there, because it is how she finds the row in the spreadsheet she
    // still has open.
    final detail = [
      row.date,
      formatAccountMoney(ref, row.grossCents),
      if (row.product.isNotEmpty) row.product,
    ].join(' · ');

    return ListTile(
      key: Key('sales_import_row_${row.line}'),
      contentPadding: EdgeInsets.zero,
      dense: true,
      leading: Icon(icon, color: color, size: 20),
      title: Text(
        row.displayName.isEmpty
            ? t.sales_import.blank_customer
            : row.displayName,
        style:
            TextStyle(color: colors.textPrimary, fontWeight: FontWeight.w600),
      ),
      subtitle: Text(
        row.problem.isNotEmpty
            ? t.sales_import
                .line_problem(line: '${row.line}', problem: row.problem)
            : t.sales_import.line_detail(line: '${row.line}', detail: detail),
        style: TextStyle(
          color: row.problem.isEmpty || row.duplicate
              ? colors.textMuted
              : colors.orangeAlert,
          fontSize: 12,
        ),
      ),
    );
  }
}

/// What a confirmed import recorded, and the door to where it landed.
class SalesImportResultNotice extends StatelessWidget {
  const SalesImportResultNotice({
    super.key,
    required this.result,
    required this.onViewPortfolio,
  });

  final SalesImportResult result;
  final VoidCallback onViewPortfolio;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Container(
      key: const Key('sales_import_result'),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.whatsappGreen.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            // Both numbers, always. A count that does not match what she
            // confirmed is a question, and answering it here saves a support
            // message.
            result.skipped == 0
                ? t.sales_import.result(count: '${result.created}')
                : t.sales_import.skipped(
                    count: '${result.created}', skipped: '${result.skipped}'),
            style: TextStyle(
                color: colors.textPrimary, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 8),
          // Where the history went: the Carteira now knows who bought what,
          // which is what this import exists to feed.
          FilledButton.icon(
            key: const Key('sales_import_view_portfolio'),
            onPressed: onViewPortfolio,
            icon: const Icon(Icons.people_outline, size: 18),
            label: Text(t.sales_import.view_portfolio),
          ),
        ],
      ),
    );
  }
}
