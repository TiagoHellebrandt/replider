import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/portfolio_controller.dart';
import 'package:app/features/sales/domain/models/customer_import.dart';
import 'package:app/features/sales/presentation/controllers/customer_import_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Import the customer list she already has (pillar 2).
///
/// A review, not an upload. The file is read, every line is shown with what the
/// server made of it, and only then does anything get created — because this
/// file decides who the account will be writing to, and a number the parser
/// could not read is a row only the person who typed it can settle.
class CustomerImportScreen extends ConsumerStatefulWidget {
  const CustomerImportScreen({super.key});

  @override
  ConsumerState<CustomerImportScreen> createState() =>
      _CustomerImportScreenState();
}

class _CustomerImportScreenState extends ConsumerState<CustomerImportScreen> {
  CustomerImportPreview? _preview;
  CustomerImportResult? _result;
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
      // A new file invalidates the old reading. Leaving the previous preview on
      // screen while a new one loads is how somebody confirms the wrong list.
      _preview = null;
      _result = null;
      _filename = file.name;
    });

    try {
      final preview = await ref.read(customerImportActionsProvider).preview(
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
      final result =
          await ref.read(customerImportActionsProvider).confirm(rows);
      if (!mounted) return;
      setState(() {
        _result = result;
        // The reading is spent: those customers are in the base now, and showing
        // them as importable again would invite a second, empty import.
        _preview = null;
      });
      ref.invalidate(portfolioProvider);
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
        key: const Key('customer_import_error'),
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
          key: const Key('customer_import_back'),
          tooltip: t.sales.back_tooltip,
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () => context.canPop()
              ? context.pop()
              : context.go('/funnel?view=portfolio'),
        ),
        title: Text(t.customer_import.title),
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          // What the file needs to look like, before she goes looking for it.
          Text(
            t.customer_import.what_we_read,
            key: const Key('customer_import_help'),
            style: TextStyle(color: colors.textMuted, fontSize: 13),
          ),
          const SizedBox(height: 12),
          OutlinedButton.icon(
            key: const Key('customer_import_pick'),
            onPressed: _busy ? null : _pick,
            icon: const Icon(Icons.upload_file),
            label: Text(_filename == null
                ? t.customer_import.pick_file
                : t.customer_import.pick_another),
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
            _ResultNotice(result: _result!),
          ],
          if (preview != null)
            CustomerImportReview(
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
/// widget test cannot drive, and the review is the part a person actually has to
/// be able to check. A screen whose reviewable part cannot be tested is a screen
/// whose honesty rests on nobody having changed it.
class CustomerImportReview extends StatelessWidget {
  const CustomerImportReview({
    super.key,
    required this.preview,
    required this.onConfirm,
    this.busy = false,
  });

  final CustomerImportPreview preview;
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
        // from this list is a customer she believes she imported.
        for (final row in preview.rows) _RowTile(row: row),
        const SizedBox(height: 20),
        FilledButton.icon(
          key: const Key('customer_import_confirm'),
          onPressed: (busy || importable == 0) ? null : onConfirm,
          icon: const Icon(Icons.group_add_outlined),
          // The button states HOW MANY it will create. "Importar" over a list
          // where half the rows have problems is a promise it will not keep.
          label: Text(t.customer_import.confirm(count: '$importable')),
        ),
        if (importable == 0) ...[
          const SizedBox(height: 6),
          Text(
            t.customer_import.nothing_importable,
            key: const Key('customer_import_nothing'),
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

  final CustomerImportPreview preview;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Container(
      key: const Key('customer_import_summary'),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.glassMedium,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.customer_import.found(
              total: '${preview.rows.length}',
              importable: '${preview.importable}',
            ),
            style: TextStyle(
                color: colors.textPrimary, fontWeight: FontWeight.w600),
          ),
          // Two separate numbers because they are two different situations: a
          // problem needs fixing, an existing customer needs nothing.
          if (preview.problems > 0) ...[
            const SizedBox(height: 4),
            Text(
              t.customer_import.with_problems(count: '${preview.problems}'),
              key: const Key('customer_import_problems'),
              style: TextStyle(color: colors.orangeAlert, fontSize: 13),
            ),
          ],
          if (preview.alreadyInBase > 0) ...[
            const SizedBox(height: 4),
            Text(
              t.customer_import.already_here(count: '${preview.alreadyInBase}'),
              key: const Key('customer_import_already'),
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

  final CustomerImportRow row;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final ok = row.importable;

    // A duplicate is a fact (the existing customer stays), not a fault —
    // it must not wear the same error icon as an unreadable phone.
    final (IconData icon, Color color) = row.duplicate
        ? (Icons.info_outline, colors.textMuted)
        : ok
            ? (Icons.check_circle_outline, colors.whatsappGreen)
            : (Icons.error_outline, colors.orangeAlert);

    return ListTile(
      key: Key('customer_import_row_${row.line}'),
      contentPadding: EdgeInsets.zero,
      dense: true,
      leading: Icon(icon, color: color, size: 20),
      title: Text(
        row.displayName.isEmpty
            ? t.customer_import.blank_name
            : row.displayName,
        style:
            TextStyle(color: colors.textPrimary, fontWeight: FontWeight.w600),
      ),
      subtitle: Text(
        // The problem, or the ADDRESS — never just the name. A list of names
        // cannot catch the mistake this review exists to catch: two clinics with
        // similar names, or a number typed wrong. The line number is always
        // there, because it is how she finds the row in the spreadsheet she
        // still has open.
        row.problem.isNotEmpty
            ? t.customer_import
                .line_problem(line: '${row.line}', problem: row.problem)
            : t.customer_import
                .line_address(line: '${row.line}', address: row.address),
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

class _ResultNotice extends StatelessWidget {
  const _ResultNotice({required this.result});

  final CustomerImportResult result;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Container(
      key: const Key('customer_import_result'),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.whatsappGreen.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        // Both numbers, always. A count that does not match what she confirmed
        // is a question, and answering it here saves a support message.
        result.skipped == 0
            ? t.customer_import.imported(count: '${result.created}')
            : t.customer_import.imported_partial(
                count: '${result.created}', skipped: '${result.skipped}'),
        style:
            TextStyle(color: colors.textPrimary, fontWeight: FontWeight.w600),
      ),
    );
  }
}
