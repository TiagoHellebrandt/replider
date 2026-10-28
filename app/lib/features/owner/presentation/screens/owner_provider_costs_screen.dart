import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/components/empty_state_card.dart';
import 'package:app/design_system/components/glass_card.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/owner/domain/models/provider_cost_models.dart';
import 'package:app/features/owner/presentation/controllers/provider_cost_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Owner-panel management of the manual API price table (US-005,
/// vendedores v3) + the USD->BRL rate used by the profit computation.
/// Costs are cents of USD per 1M tokens; a price change should be a
/// NEW row with a later effective_from, editing is for typo fixes.
class OwnerProviderCostsScreen extends ConsumerWidget {
  const OwnerProviderCostsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final async = ref.watch(ownerProviderCostsProvider);
    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        title: Text(
          t.owner.provider_costs_title,
          style: TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w700,
            color: colors.textPrimary,
          ),
        ),
        leading: IconButton(
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () =>
              context.canPop() ? context.pop() : context.go('/owner/accounts'),
        ),
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => _openSheet(context, ref),
        icon: const Icon(Icons.add),
        label: Text(t.owner.provider_cost_add_cta),
      ),
      body: async.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => RetryableError(
          error: e,
          onRetry: () => ref.invalidate(ownerProviderCostsProvider),
        ),
        data: (data) => ListView(
          padding: const EdgeInsets.all(16),
          children: [
            _ExchangeRateCard(
              rateCents: data.usdBrlRateCents,
              colors: colors,
              typography: typography,
            ),
            const SizedBox(height: 16),
            if (data.items.isEmpty)
              EmptyStateCard(
                icon: Icons.request_quote_outlined,
                title: t.owner.provider_costs_empty_title,
                subtitle: t.owner.provider_costs_empty_desc,
              )
            else
              ...data.items.map(
                (c) => _ProviderCostRow(
                  cost: c,
                  colors: colors,
                  typography: typography,
                ),
              ),
            const SizedBox(height: 72),
          ],
        ),
      ),
    );
  }

  Future<void> _openSheet(BuildContext context, WidgetRef ref) async {
    final saved = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => const _ProviderCostSheet(),
    );
    if (saved == true && context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(t.owner.provider_cost_saved_toast)),
      );
    }
  }
}

class _ExchangeRateCard extends ConsumerWidget {
  final int rateCents;
  final AppColorsExtension colors;
  final AppTypographyExtension typography;

  const _ExchangeRateCard({
    required this.rateCents,
    required this.colors,
    required this.typography,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final rate = (rateCents / 100).toStringAsFixed(2);
    return GlassCard(
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Row(
          children: [
            Icon(Icons.currency_exchange, color: colors.bluePrimary, size: 20),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    t.owner.exchange_rate_title,
                    style: typography.body.copyWith(
                      color: colors.textSecondary,
                      fontSize: 12,
                    ),
                  ),
                  Text(
                    t.owner.exchange_rate_value(rate: rate),
                    style: typography.body.copyWith(
                      color: colors.textPrimary,
                      fontSize: 16,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ],
              ),
            ),
            TextButton(
              onPressed: () => _edit(context, ref),
              child: Text(t.owner.exchange_rate_edit_cta),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _edit(BuildContext context, WidgetRef ref) async {
    final ctrl =
        TextEditingController(text: (rateCents / 100).toStringAsFixed(2));
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(t.owner.exchange_rate_title),
        content: TextField(
          controller: ctrl,
          keyboardType: const TextInputType.numberWithOptions(decimal: true),
          decoration: InputDecoration(
            labelText: t.owner.exchange_rate_label,
            prefixText: r'R$ ',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(t.common.cancel),
          ),
          ElevatedButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(t.common.save),
          ),
        ],
      ),
    );
    if (ok != true || !context.mounted) return;
    final parsed = double.tryParse(ctrl.text.trim().replaceAll(',', '.'));
    if (parsed == null || parsed <= 0) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(t.owner.provider_cost_form_invalid)),
      );
      return;
    }
    try {
      await ref
          .read(providerCostActionsProvider.notifier)
          .setExchangeRate((parsed * 100).round());
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(t.owner.exchange_rate_saved)),
        );
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(parseApiError(e))),
        );
      }
    }
  }
}

class _ProviderCostRow extends ConsumerWidget {
  final OwnerProviderCost cost;
  final AppColorsExtension colors;
  final AppTypographyExtension typography;

  const _ProviderCostRow({
    required this.cost,
    required this.colors,
    required this.typography,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: GlassCard(
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '${cost.provider} · ${cost.model}',
                      style: typography.body.copyWith(
                        color: colors.textPrimary,
                        fontSize: 14,
                        fontWeight: FontWeight.w700,
                        fontFamily: 'monospace',
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      t.owner.provider_cost_costs_line(
                        input: '${cost.inputCostPer1mCentsUsd}',
                        output: '${cost.outputCostPer1mCentsUsd}',
                      ),
                      style: typography.body.copyWith(
                        color: colors.bluePrimary,
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      t.owner.provider_cost_effective_from(
                        date: cost.effectiveFrom
                            .toIso8601String()
                            .substring(0, 10),
                      ),
                      style: typography.body.copyWith(
                        color: colors.textSecondary,
                        fontSize: 11,
                      ),
                    ),
                  ],
                ),
              ),
              PopupMenuButton<String>(
                icon: Icon(Icons.more_vert, color: colors.textSecondary),
                onSelected: (v) => v == 'edit'
                    ? _edit(context, ref)
                    : _confirmDelete(context, ref),
                itemBuilder: (_) => [
                  PopupMenuItem(value: 'edit', child: Text(t.common.edit)),
                  PopupMenuItem(value: 'delete', child: Text(t.common.delete)),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _edit(BuildContext context, WidgetRef ref) async {
    final saved = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => _ProviderCostSheet(existing: cost),
    );
    if (saved == true && context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(t.owner.provider_cost_saved_toast)),
      );
    }
  }

  Future<void> _confirmDelete(BuildContext context, WidgetRef ref) async {
    final ok = await showConfirmDialog(
      context,
      title: t.owner.provider_cost_delete_title,
      message: t.owner
          .provider_cost_delete_body(model: '${cost.provider}/${cost.model}'),
      confirmText: t.common.delete,
      destructive: true,
    );
    if (!ok) return;
    try {
      await ref.read(providerCostActionsProvider.notifier).delete(cost.id);
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(parseApiError(e))),
        );
      }
    }
  }
}

class _ProviderCostSheet extends ConsumerStatefulWidget {
  final OwnerProviderCost? existing;

  const _ProviderCostSheet({this.existing});

  @override
  ConsumerState<_ProviderCostSheet> createState() => _ProviderCostSheetState();
}

class _ProviderCostSheetState extends ConsumerState<_ProviderCostSheet> {
  late final TextEditingController _providerCtrl;
  late final TextEditingController _modelCtrl;
  late final TextEditingController _inputCtrl;
  late final TextEditingController _outputCtrl;
  late DateTime _effectiveFrom;
  bool _busy = false;
  String? _error;

  bool get _isEdit => widget.existing != null;

  @override
  void initState() {
    super.initState();
    final e = widget.existing;
    _providerCtrl = TextEditingController(text: e?.provider ?? '');
    _modelCtrl = TextEditingController(text: e?.model ?? '');
    _inputCtrl =
        TextEditingController(text: e?.inputCostPer1mCentsUsd.toString() ?? '');
    _outputCtrl = TextEditingController(
        text: e?.outputCostPer1mCentsUsd.toString() ?? '');
    _effectiveFrom = e?.effectiveFrom ?? DateTime.now();
  }

  @override
  void dispose() {
    _providerCtrl.dispose();
    _modelCtrl.dispose();
    _inputCtrl.dispose();
    _outputCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final provider = _providerCtrl.text.trim().toLowerCase();
    final model = _modelCtrl.text.trim();
    final input = int.tryParse(_inputCtrl.text.trim());
    final output = int.tryParse(_outputCtrl.text.trim());
    if ((!_isEdit && (provider.isEmpty || model.isEmpty)) ||
        input == null ||
        output == null ||
        input < 0 ||
        output < 0) {
      setState(() => _error = t.owner.provider_cost_form_invalid);
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final actions = ref.read(providerCostActionsProvider.notifier);
      if (_isEdit) {
        await actions.update(
          id: widget.existing!.id,
          inputCents: input,
          outputCents: output,
          effectiveFrom: _effectiveFrom,
        );
      } else {
        await actions.create(
          provider: provider,
          model: model,
          inputCents: input,
          outputCents: output,
          effectiveFrom: _effectiveFrom,
        );
      }
      if (mounted) Navigator.of(context).pop(true);
    } catch (e) {
      setState(() {
        _busy = false;
        _error = parseApiError(e);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Padding(
      padding: EdgeInsets.only(
        bottom: MediaQuery.of(context).viewInsets.bottom,
      ),
      child: Container(
        decoration: BoxDecoration(
          color: colors.backgroundSecondary,
          borderRadius: const BorderRadius.vertical(top: Radius.circular(16)),
        ),
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              _isEdit
                  ? t.owner.provider_cost_edit_title
                  : t.owner.provider_cost_add_title,
              style: TextStyle(
                color: colors.textPrimary,
                fontSize: 16,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _providerCtrl,
              enabled: !_isEdit,
              decoration: InputDecoration(
                labelText: t.owner.provider_cost_provider_label,
                hintText: 'google',
              ),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _modelCtrl,
              enabled: !_isEdit,
              decoration: InputDecoration(
                labelText: t.owner.provider_cost_model_label,
                hintText: 'gemini-2.5-flash',
              ),
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _inputCtrl,
                    keyboardType: TextInputType.number,
                    decoration: InputDecoration(
                      labelText: t.owner.provider_cost_input_label,
                      suffixText: '¢',
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: TextField(
                    controller: _outputCtrl,
                    keyboardType: TextInputType.number,
                    decoration: InputDecoration(
                      labelText: t.owner.provider_cost_output_label,
                      suffixText: '¢',
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: Text(
                    t.owner.provider_cost_effective_from(
                      date: _effectiveFrom.toIso8601String().substring(0, 10),
                    ),
                    style: TextStyle(color: colors.textSecondary, fontSize: 13),
                  ),
                ),
                TextButton(
                  onPressed: _busy
                      ? null
                      : () async {
                          final picked = await showDatePicker(
                            context: context,
                            firstDate: DateTime(2024),
                            lastDate:
                                DateTime.now().add(const Duration(days: 365)),
                            initialDate: _effectiveFrom,
                          );
                          if (picked != null) {
                            setState(() => _effectiveFrom = picked);
                          }
                        },
                  child: Text(t.owner.provider_cost_pick_date),
                ),
              ],
            ),
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(
                _error!,
                style: TextStyle(color: colors.redAlert, fontSize: 12),
              ),
            ],
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: _busy ? null : _submit,
              style: ElevatedButton.styleFrom(
                backgroundColor: colors.bluePrimary,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 14),
              ),
              child: _busy
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(t.common.save),
            ),
          ],
        ),
      ),
    );
  }
}
