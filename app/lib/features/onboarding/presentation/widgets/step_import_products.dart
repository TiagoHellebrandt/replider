import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/product_import.dart';
import 'package:app/features/sales/presentation/controllers/product_import_controller.dart';
import 'package:app/features/sales/presentation/controllers/products_controller.dart';
import 'package:app/features/sales/presentation/screens/product_import_screen.dart'
    show ProductImportReview;
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/i18n/strings.g.dart';

/// "What do you sell?" — the second step of the shipped flow.
///
/// This replaced "create an agent". Creating a chatbot was the first thing the
/// product asked for, which told every new account that Replider is a chatbot;
/// drafting an approach needs no agent at all. What the product does need before
/// it is worth anything is knowing what she sells, so this is where the catalogue
/// arrives — as a file, because somebody with four hundred SKUs has them in a
/// spreadsheet and typing them one by one is the reason they would give up.
///
/// Skippable, and honestly so: not everybody has the file open when they sign up,
/// and the catalogue is reachable afterwards from the products list.
/// True while this step holds a REVIEWED but unconfirmed file. The wizard's
/// Next lives on the parent screen, which cannot see this state — without
/// it, choosing a file, reading the preview and tapping Próximo silently
/// discarded the import the person believes they just did.
bool onboardingPendingProductImport = false;

class StepImportProducts extends ConsumerStatefulWidget {
  const StepImportProducts({super.key});

  @override
  ConsumerState<StepImportProducts> createState() => _StepImportProductsState();
}

class _StepImportProductsState extends ConsumerState<StepImportProducts> {
  @override
  void dispose() {
    // Global flag reset: a stale true from an abandoned pass fired a
    // phantom "arquivo não importado" dialog on a later, empty visit.
    onboardingPendingProductImport = false;
    super.dispose();
  }

  ProductImportPreview? _preview;
  ProductImportResult? _result;
  bool _busy = false;

  /// (read, total) while several files are going through, so a five-brochure
  /// batch shows movement instead of one long spinner.
  (int, int)? _progress;

  /// Files the server refused, and why — held on the screen, not only shown in
  /// a snack. A snack is gone in four seconds and what stays behind is "0
  /// linhas lidas", which reads as "your brochures are empty" when the truth
  /// was a broken service. That mistranslation cost two rounds of debugging.
  List<ProductImportFailure> _failed = const [];
  List<String> _failReasons = const [];

  void _setPreview(ProductImportPreview? preview) {
    setState(() => _preview = preview);
    onboardingPendingProductImport = preview != null;
  }

  /// A contract account with no key cannot read a brochure: the extraction
  /// runs on ITS model. Offering the PDF anyway produced a refusal AFTER the
  /// upload, so the option is simply not offered — and the CSV path, which
  /// uses no model at all, is named as the way through.
  static bool _needsKeyFor(UserModel? u) {
    if (u == null || !u.isContractPlan) return false;
    return u.geminiKeyMasked.isEmpty && u.openAIKeyMasked.isEmpty;
  }

  Future<void> _pick() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      // PDF as well: whoever sells hospital equipment has the manufacturer's
      // brochure, not a spreadsheet. And SEVERAL of them — a catalogue arrives
      // as one folder per line, so taking a single file made the person import
      // five times or give up.
      allowedExtensions:
          _needsKeyFor(ref.read(authControllerProvider).asData?.value)
              ? const ['csv', 'txt']
              : const ['csv', 'txt', 'pdf'],
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
      _result = null;
      _failed = const [];
      _failReasons = const [];
      _progress = files.length > 1 ? (0, files.length) : null;
    });
    _setPreview(null);
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
      _setPreview(batch.preview);
      // A file that failed must not hide the ones that worked.
      if (batch.failed.isNotEmpty) {
        setState(() {
          _failed = batch.failed;
          _failReasons = batch.reasons;
        });
        // Reason first: eighteen file names with no cause read like eighteen
        // broken PDFs instead of one broken service.
        _snack(t.product_import.files_failed(
          count: '${batch.failed.length}',
          reason: batch.reasons.join(' · '),
        ));
      }
    } catch (e) {
      if (!mounted) return;
      _error(e);
    } finally {
      if (mounted) {
        setState(() {
          _busy = false;
          _progress = null;
        });
      }
    }
  }

  void _snack(String msg) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(msg), behavior: SnackBarBehavior.floating),
    );
  }

  Future<void> _confirm() async {
    final rows = _preview?.rows.where((r) => r.importable).toList() ?? const [];
    if (rows.isEmpty) return;

    setState(() => _busy = true);
    try {
      final result = await ref.read(productImportActionsProvider).confirm(rows);
      if (!mounted) return;
      setState(() => _result = result);
      _setPreview(null);
      ref.invalidate(productsListProvider);
    } catch (e) {
      if (!mounted) return;
      _error(e);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _error(Object e) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        key: const Key('onboarding_import_error'),
        content: Text(parseApiError(e)),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final preview = _preview;

    return SingleChildScrollView(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.onboarding.products_title,
            style: typography.display.copyWith(
              color: colors.textPrimary,
              fontSize: 24,
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            t.onboarding.products_subtitle,
            style: TextStyle(color: colors.textSecondary, fontSize: 14),
          ),
          const SizedBox(height: 20),
          OutlinedButton.icon(
            key: const Key('onboarding_import_pick'),
            onPressed: _busy ? null : _pick,
            icon: const Icon(Icons.upload_file),
            label: Text(t.product_import.pick_file),
          ),
          const SizedBox(height: 8),
          Text(
            t.product_import.what_we_read,
            style: TextStyle(color: colors.textMuted, fontSize: 12),
          ),
          if (_needsKeyFor(
              ref.watch(authControllerProvider).asData?.value)) ...[
            const SizedBox(height: 6),
            Text(
              t.onboarding.products_needs_key,
              key: const Key('onboarding_products_needs_key'),
              style: TextStyle(color: colors.orangeAlert, fontSize: 12),
            ),
          ],
          if (_busy) ...[
            const SizedBox(height: 16),
            const Center(child: CircularProgressIndicator()),
            if (_progress != null) ...[
              const SizedBox(height: 8),
              Center(
                child: Text(
                  t.product_import.reading_progress(
                    done: '${_progress!.$1}',
                    total: '${_progress!.$2}',
                  ),
                  key: const Key('onboarding_import_progress'),
                  style: TextStyle(color: colors.textMuted, fontSize: 12),
                ),
              ),
            ],
          ],
          if (_result != null) ...[
            const SizedBox(height: 16),
            Text(
              _result!.skipped == 0
                  ? t.product_import.imported(count: '${_result!.created}')
                  : t.product_import.imported_partial(
                      count: '${_result!.created}',
                      skipped: '${_result!.skipped}'),
              key: const Key('onboarding_import_result'),
              style: TextStyle(
                  color: colors.whatsappGreen, fontWeight: FontWeight.w600),
            ),
          ],
          // Why the file did not come in, on the screen and staying there.
          if (_failed.isNotEmpty) ...[
            const SizedBox(height: 16),
            Container(
              key: const Key('onboarding_import_failures'),
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: colors.redAlert.withValues(alpha: 0.08),
                borderRadius: BorderRadius.circular(12),
                border:
                    Border.all(color: colors.redAlert.withValues(alpha: 0.4)),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    t.product_import.files_failed(
                      count: '${_failed.length}',
                      reason: _failReasons.join(' · '),
                    ),
                    style: TextStyle(
                        color: colors.redAlert, fontWeight: FontWeight.w600),
                  ),
                  const SizedBox(height: 6),
                  Text(
                    _failed.map((f) => f.filename).join(', '),
                    style: TextStyle(color: colors.textMuted, fontSize: 12),
                  ),
                ],
              ),
            ),
          ],
          // The same review the products screen shows. One renderer, so the
          // wizard cannot quietly become more permissive than the screen.
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
