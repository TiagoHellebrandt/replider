import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/portfolio_controller.dart';
import 'package:app/features/sales/domain/models/customer_import.dart';
import 'package:app/features/sales/presentation/controllers/customer_import_controller.dart';
import 'package:app/features/sales/presentation/screens/customer_import_screen.dart'
    show CustomerImportReview;
import 'package:app/i18n/strings.g.dart';

/// "Who already buys from you?" — the third step (2026-08-18).
///
/// The product's whole thesis is "the customers she already has": the
/// portfolio, the repurchase dates, the cross-sell all read from this list,
/// and the first offer the wizard writes is for one of these people. It used
/// to arrive only through a screen behind the Clientes tab, after onboarding
/// was over — so most accounts reached the offer engine with nobody to write
/// to and met the empty version of the product first.
///
/// Skippable, honestly: not everybody has the spreadsheet open, and the
/// import is reachable afterwards from Clientes. But it is asked here, once,
/// where it changes what the next screen can show.
/// True while this step holds a REVIEWED but unconfirmed file — see the
/// products twin for why the parent needs to know.
bool onboardingPendingCustomerImport = false;

class StepImportCustomers extends ConsumerStatefulWidget {
  const StepImportCustomers({super.key});

  @override
  ConsumerState<StepImportCustomers> createState() =>
      _StepImportCustomersState();
}

class _StepImportCustomersState extends ConsumerState<StepImportCustomers> {
  @override
  void dispose() {
    // Global flag reset: a stale true from an abandoned pass fired a
    // phantom "arquivo não importado" dialog on a later, empty visit.
    onboardingPendingCustomerImport = false;
    super.dispose();
  }

  CustomerImportPreview? _preview;
  CustomerImportResult? _result;
  bool _busy = false;

  void _setPreview(CustomerImportPreview? preview) {
    setState(() => _preview = preview);
    onboardingPendingCustomerImport = preview != null;
  }

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
      _result = null;
    });
    _setPreview(null);
    try {
      final preview = await ref.read(customerImportActionsProvider).preview(
            bytes: file.bytes!,
            filename: file.name,
          );
      if (!mounted) return;
      _setPreview(preview);
    } catch (e) {
      if (!mounted) return;
      _error(e);
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
      setState(() => _result = result);
      _setPreview(null);
      ref.invalidate(portfolioProvider);
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
        key: const Key('onboarding_customers_error'),
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
            t.onboarding.customers_title,
            style: typography.display.copyWith(
              color: colors.textPrimary,
              fontSize: 24,
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            t.onboarding.customers_subtitle,
            style: TextStyle(color: colors.textSecondary, fontSize: 14),
          ),
          const SizedBox(height: 20),
          OutlinedButton.icon(
            key: const Key('onboarding_customers_pick'),
            onPressed: _busy ? null : _pick,
            icon: const Icon(Icons.upload_file),
            label: Text(t.customer_import.pick_file),
          ),
          const SizedBox(height: 8),
          Text(
            t.customer_import.what_we_read,
            style: TextStyle(color: colors.textMuted, fontSize: 12),
          ),
          if (_busy) ...[
            const SizedBox(height: 16),
            const Center(child: CircularProgressIndicator()),
          ],
          if (_result != null) ...[
            const SizedBox(height: 16),
            Text(
              _result!.skipped == 0
                  ? t.customer_import.imported(count: '${_result!.created}')
                  : t.customer_import.imported_partial(
                      count: '${_result!.created}',
                      skipped: '${_result!.skipped}'),
              key: const Key('onboarding_customers_result'),
              style: TextStyle(
                  color: colors.whatsappGreen, fontWeight: FontWeight.w600),
            ),
          ],
          // The same review the Clientes screen shows. One renderer, so the
          // wizard cannot quietly become more permissive than the screen.
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
