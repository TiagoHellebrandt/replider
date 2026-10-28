import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/utils/currency_format.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/product_import.dart';
import 'package:app/features/sales/presentation/controllers/product_import_controller.dart';
import 'package:app/features/sales/presentation/controllers/products_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Import the price list she already has (pillar 2).
///
/// A review, not an upload. The file is read, every line is shown with what the
/// server made of it, and only then does anything get created — because the one
/// thing this parser will not do is guess a price it cannot read, and the person
/// who wrote the spreadsheet is the only one who knows what "1,234" meant.
class ProductImportScreen extends ConsumerStatefulWidget {
  const ProductImportScreen({super.key});

  @override
  ConsumerState<ProductImportScreen> createState() =>
      _ProductImportScreenState();
}

class _ProductImportScreenState extends ConsumerState<ProductImportScreen> {
  ProductImportPreview? _preview;
  ProductImportResult? _result;
  bool _busy = false;
  // The PDF path goes through a model reading every page, so its wait is a
  // different promise than a CSV parse — the screen says so while it runs.
  bool _busyIsPdf = false;
  String? _filename;

  /// (read, total) while a multi-file batch goes through.
  (int, int)? _progress;

  Future<void> _pick() => _pickAndPreview(
        extensions: const ['csv', 'txt'],
        isPdf: false,
      );

  Future<void> _pickPdf() => _pickAndPreview(
        extensions: const ['pdf'],
        isPdf: true,
      );

  Future<void> _pickAndPreview({
    required List<String> extensions,
    required bool isPdf,
  }) async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: extensions,
      // A catalogue arrives as several files — one folder per product line.
      allowMultiple: true,
      withData: true,
    );
    if (picked == null || picked.files.isEmpty) return;
    final files = [
      for (final f in picked.files)
        if (f.bytes != null) (bytes: f.bytes!, filename: f.name),
    ];
    if (files.isEmpty) return;

    setState(() {
      _busy = true;
      _busyIsPdf = isPdf;
      // A new pick invalidates the old reading. Leaving the previous preview on
      // screen while a new one loads is how somebody confirms the wrong list.
      _preview = null;
      _result = null;
      _filename = files.length == 1
          ? files.first.filename
          : t.product_import.files_read(count: '${files.length}');
      _progress = files.length > 1 ? (0, files.length) : null;
    });

    try {
      final batch = await ref.read(productImportActionsProvider).previewMany(
        files,
        onProgress: (done, total) {
          if (mounted && total > 1) {
            setState(() => _progress = (done, total));
          }
        },
      );
      if (!mounted) return;
      setState(() => _preview = batch.preview);
      // A file that failed must not discard the ones that worked.
      if (batch.failed.isNotEmpty) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(t.product_import.files_failed(
            count: '${batch.failed.length}',
            reason: batch.reasons.join(' · '),
          )),
          behavior: SnackBarBehavior.floating,
        ));
      }
    } catch (e) {
      if (!mounted) return;
      _showError(e);
    } finally {
      if (mounted) {
        setState(() {
          _busy = false;
          _progress = null;
        });
      }
    }
  }

  Future<void> _confirm() async {
    final rows = _preview?.rows.where((r) => r.importable).toList() ?? const [];
    if (rows.isEmpty) return;

    setState(() => _busy = true);
    try {
      final result = await ref.read(productImportActionsProvider).confirm(rows);
      if (!mounted) return;
      setState(() {
        _result = result;
        // The reading is spent: those rows are in the catalogue now, and showing
        // them as importable again would invite a second, empty import.
        _preview = null;
      });
      ref.invalidate(productsListProvider);
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
        key: const Key('product_import_error'),
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
        // Rendered outside the shell: no rail, no bottom bar.
        leading: IconButton(
          key: const Key('product_import_back'),
          tooltip: t.sales.back_tooltip,
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () =>
              context.canPop() ? context.pop() : context.go('/sales/products'),
        ),
        title: Text(t.product_import.title),
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          // What the file needs to look like, before she goes looking for it.
          // Two sentences here save the support thread that starts with "não
          // funcionou".
          Text(
            t.product_import.what_we_read,
            key: const Key('product_import_help'),
            style: TextStyle(color: colors.textMuted, fontSize: 13),
          ),
          const SizedBox(height: 12),
          OutlinedButton.icon(
            key: const Key('product_import_pick'),
            onPressed: _busy ? null : _pick,
            icon: const Icon(Icons.upload_file),
            label: Text(_filename == null
                ? t.product_import.pick_file
                : t.product_import.pick_another),
          ),
          const SizedBox(height: 8),
          // The brochure door: brands ship PDFs, not spreadsheets. Same
          // preview, same confirmation — only the reading differs.
          Text(
            t.product_import.what_we_read_pdf,
            key: const Key('product_import_pdf_help'),
            style: TextStyle(color: colors.textMuted, fontSize: 13),
          ),
          const SizedBox(height: 8),
          OutlinedButton.icon(
            key: const Key('product_import_pick_pdf'),
            onPressed: _busy ? null : _pickPdf,
            icon: const Icon(Icons.picture_as_pdf_outlined),
            label: Text(t.product_import.pick_pdf),
          ),
          if (_filename != null) ...[
            const SizedBox(height: 6),
            Text(_filename!,
                style: TextStyle(color: colors.textMuted, fontSize: 12)),
          ],
          if (_busy) ...[
            const SizedBox(height: 16),
            const Center(child: CircularProgressIndicator()),
            if (_busyIsPdf) ...[
              const SizedBox(height: 8),
              Center(
                child: Text(
                  t.product_import.pdf_reading,
                  key: const Key('product_import_pdf_reading'),
                  style: TextStyle(color: colors.textMuted, fontSize: 13),
                ),
              ),
            ],
            // Several files read one at a time: show movement instead of one
            // long spinner over a five-brochure batch.
            if (_progress != null) ...[
              const SizedBox(height: 8),
              Center(
                child: Text(
                  t.product_import.reading_progress(
                    done: '${_progress!.$1}',
                    total: '${_progress!.$2}',
                  ),
                  key: const Key('product_import_progress'),
                  style: TextStyle(color: colors.textMuted, fontSize: 12),
                ),
              ),
            ],
          ],

          if (_result != null) ...[
            const SizedBox(height: 20),
            _ResultNotice(result: _result!),
          ],

          if (preview != null)
            ProductImportReview(
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
/// Split out of the screen because the file picker above it is platform code that
/// a widget test cannot drive, and the review is the part a person actually has to
/// be able to check. A screen whose reviewable part cannot be tested is a screen
/// whose honesty rests on nobody having changed it.
class ProductImportReview extends StatelessWidget {
  const ProductImportReview({
    super.key,
    required this.preview,
    required this.onConfirm,
    this.busy = false,
  });

  final ProductImportPreview preview;
  final VoidCallback onConfirm;
  final bool busy;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final importable = preview.importable;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(height: 20),
        _Summary(preview: preview),
        const SizedBox(height: 12),
        // Every line, including the ones that cannot be imported. A line dropped
        // from this list is a product she believes she imported.
        for (final row in preview.rows) _RowTile(row: row),
        const SizedBox(height: 20),
        FilledButton.icon(
          key: const Key('product_import_confirm'),
          onPressed: (busy || importable == 0) ? null : onConfirm,
          icon: const Icon(Icons.playlist_add_check),
          // The button states HOW MANY it will create. "Importar" over a list
          // where half the rows have problems is a promise it will not keep.
          label: Text(t.product_import.confirm(count: '$importable')),
        ),
        if (importable == 0) ...[
          const SizedBox(height: 6),
          Text(
            t.product_import.nothing_importable,
            key: const Key('product_import_nothing'),
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

  final ProductImportPreview preview;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Container(
      key: const Key('product_import_summary'),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.glassMedium,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.product_import.found(
              total: '${preview.rows.length}',
              importable: '${preview.importable}',
            ),
            style: TextStyle(
                color: colors.textPrimary, fontWeight: FontWeight.w600),
          ),
          // Two separate numbers because they are two different situations: a
          // problem needs fixing, an existing product needs nothing.
          if (preview.problems > 0) ...[
            const SizedBox(height: 4),
            Text(
              t.product_import.with_problems(count: '${preview.problems}'),
              key: const Key('product_import_problems'),
              style: TextStyle(color: colors.orangeAlert, fontSize: 13),
            ),
          ],
          if (preview.alreadyInCatalogue > 0) ...[
            const SizedBox(height: 4),
            Text(
              t.product_import
                  .already_here(count: '${preview.alreadyInCatalogue}'),
              key: const Key('product_import_already'),
              style: TextStyle(color: colors.textMuted, fontSize: 13),
            ),
          ],
        ],
      ),
    );
  }
}

/// One line of the file.
class _RowTile extends StatelessWidget {
  const _RowTile({required this.row});

  final ProductImportRow row;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    // Four situations, four looks. Two of them used to lie: a brochure line
    // whose price the model could not read rendered as a GREEN "$0.00"
    // (the exact opposite of "price ambiguous is never guessed"), and a
    // duplicate wore the same error icon as an unreadable line even though
    // it is a fact, not a mistake.
    final (IconData icon, Color color) = switch ((row)) {
      _ when row.duplicate => (Icons.info_outline, colors.textMuted),
      _ when row.problem.isNotEmpty => (
          Icons.error_outline,
          colors.orangeAlert
        ),
      _ when row.priceMissing => (
          Icons.warning_amber_outlined,
          colors.orangeAlert
        ),
      _ => (Icons.check_circle_outline, colors.whatsappGreen),
    };

    final String priceLabel = row.priceMissing
        ? t.product_import.price_missing
        : formatCurrencyCents(row.priceCents, row.currency);

    return ListTile(
      key: Key('product_import_row_${row.line}'),
      contentPadding: EdgeInsets.zero,
      dense: true,
      leading: Icon(icon, color: color, size: 20),
      title: Text(
        row.name.isEmpty ? t.product_import.blank_name : row.name,
        style:
            TextStyle(color: colors.textPrimary, fontWeight: FontWeight.w600),
      ),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            // The problem, or the price. The reference is always there: the LINE
            // for a spreadsheet she still has open, the PAGE for a brochure she
            // can leaf back to — it is how she checks the reading at the source.
            row.problem.isNotEmpty
                ? (row.page > 0
                    ? t.product_import
                        .page_problem(page: '${row.page}', problem: row.problem)
                    : t.product_import.line_problem(
                        line: '${row.line}', problem: row.problem))
                : (row.page > 0
                    ? t.product_import
                        .page_price(page: '${row.page}', price: priceLabel)
                    : t.product_import
                        .line_price(line: '${row.line}', price: priceLabel)),
            style: TextStyle(
              color: row.problem.isEmpty && !row.priceMissing
                  ? colors.textMuted
                  : (row.duplicate ? colors.textMuted : colors.orangeAlert),
              fontSize: 12,
            ),
          ),
          // Marca · Modelo · SKU: extracted from the brochure and headed for
          // the product's attributes. The review promises "you check every
          // line" — these fields must not skip it on the way through.
          if (row.brochureDetails.isNotEmpty)
            Text(
              row.brochureDetails,
              key: Key('product_import_details_${row.line}'),
              style: TextStyle(color: colors.textMuted, fontSize: 11),
            ),
        ],
      ),
    );
  }
}

class _ResultNotice extends StatelessWidget {
  const _ResultNotice({required this.result});

  final ProductImportResult result;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Container(
      key: const Key('product_import_result'),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.whatsappGreen.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        // Both numbers, always. A count that does not match what she confirmed
        // is a question, and answering it here saves a support message.
        result.skipped == 0
            ? t.product_import.imported(count: '${result.created}')
            : t.product_import.imported_partial(
                count: '${result.created}', skipped: '${result.skipped}'),
        style:
            TextStyle(color: colors.textPrimary, fontWeight: FontWeight.w600),
      ),
    );
  }
}
