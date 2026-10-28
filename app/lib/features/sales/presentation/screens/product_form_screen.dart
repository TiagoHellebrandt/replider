import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/forms/dirty_form_mixin.dart';
import 'package:app/core/utils/account_currency.dart';
import 'package:app/core/utils/money_input.dart';
import 'package:app/design_system/components/custom_text_field.dart';
import 'package:app/design_system/components/primary_pill_button.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/product_model.dart';
import 'package:app/features/sales/presentation/widgets/product_brochure_sheet.dart';
import 'package:app/features/sales/presentation/controllers/products_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Create / edit form for a catalog product (US-008). A null [productId]
/// means create; otherwise the matching product is loaded from the
/// keep-alive list provider and edited in place. Reuses [CustomTextField],
/// [DirtyFormMixin] (unsaved-changes guard) and [PrimaryPillButton].
class ProductFormScreen extends ConsumerStatefulWidget {
  final String? productId;

  const ProductFormScreen({super.key, this.productId});

  bool get isEdit => productId != null;

  @override
  ConsumerState<ProductFormScreen> createState() => _ProductFormScreenState();
}

class _ProductFormScreenState extends ConsumerState<ProductFormScreen>
    with DirtyFormMixin<ProductFormScreen> {
  final _formKey = GlobalKey<FormState>();
  final _nameController = TextEditingController();
  final _descriptionController = TextEditingController();
  final _priceController = TextEditingController();

  /// Repurchase cycle in days (§6.5 F3). The one number the reorder engine
  /// runs on: a product with no cycle is one no customer is ever reminded
  /// of. Written through its own route, not the product update.
  final _cycleController = TextEditingController();

  /// A closed choice, not a text field. "reais", "R\$" and "brl" all used to
  /// pass through here and anything ≠ BRL rendered the product in US dollars
  /// AND tripped the proposal's mixed-currency block — a refusal the person
  /// could not trace back to a typo in this form.
  String _currency = 'BRL';

  bool _active = true;
  bool _initialized = false;
  bool _saving = false;

  /// The product being edited, kept so the brochure block can render what the
  /// file said. The controllers only carry the four editable fields.
  ProductModel? _editing;

  static const _currencies = ['BRL', 'USD'];

  @override
  Map<String, Object?> snapshotForm() => {
        'name': _nameController.text.trim(),
        'description': _descriptionController.text.trim(),
        'price': _priceController.text.trim(),
        'cycle': _cycleController.text.trim(),
        'currency': _currency,
        'active': _active,
      };

  @override
  void initState() {
    super.initState();
    _nameController.addListener(recomputeDirty);
    _descriptionController.addListener(recomputeDirty);
    _priceController.addListener(recomputeDirty);
    _cycleController.addListener(recomputeDirty);
    // A new product starts in the ACCOUNT's currency, not a hardcoded one.
    if (!widget.isEdit) {
      _currency = ref.read(accountCurrencyProvider).toUpperCase();
      if (!_currencies.contains(_currency)) _currency = 'BRL';
    }
  }

  @override
  void dispose() {
    _nameController.removeListener(recomputeDirty);
    _descriptionController.removeListener(recomputeDirty);
    _priceController.removeListener(recomputeDirty);
    _cycleController.removeListener(recomputeDirty);
    _nameController.dispose();
    _descriptionController.dispose();
    _priceController.dispose();
    _cycleController.dispose();
    super.dispose();
  }

  void _hydrate(ProductModel p) {
    if (_initialized) return;
    _initialized = true;
    _nameController.text = p.name;
    _descriptionController.text = p.description;
    _priceController.text = formatCentsForInput(p.priceCents, p.currency);
    _cycleController.text =
        p.consumptionCycleDays > 0 ? p.consumptionCycleDays.toString() : '';
    final stored = p.currency.trim().toUpperCase();
    _currency = _currencies.contains(stored) ? stored : 'BRL';
    _active = p.active;
    resetSnapshot();
  }

  Future<void> _save() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;
    setState(() => _saving = true);

    final notifier = ref.read(productsListProvider.notifier);
    final name = _nameController.text.trim();
    final description = _descriptionController.text.trim();
    final currency = _currency;
    // The position-based parser, shared with every money field: "1.234,56"
    // and "1,234.56" both land as 123456 cents. The inline
    // `double.tryParse ?? 0` this replaced turned the Brazilian notation
    // into ZERO and saved the product at R$ 0,00 with a success toast.
    final priceCents = parseMoneyToCents(_priceController.text);
    // Empty means "unknown", which the server stores as zero — and zero is
    // exactly "never resurface this", so the two readings agree.
    final cycleDays = int.tryParse(_cycleController.text.trim()) ?? 0;

    final String? err = widget.isEdit
        ? await notifier.updateProduct(
            id: widget.productId!,
            name: name,
            description: description,
            priceCents: priceCents,
            currency: currency,
            active: _active,
            consumptionCycleDays: cycleDays,
          )
        : await notifier.create(
            name: name,
            description: description,
            priceCents: priceCents,
            currency: currency,
            active: _active,
            consumptionCycleDays: cycleDays,
          );

    if (!mounted) return;
    setState(() => _saving = false);

    if (err != null) {
      _toast(err, false);
      return;
    }
    resetSnapshot();
    _toast(
      widget.isEdit
          ? t.sales.products.updated_success
          : t.sales.products.created_success,
      true,
    );
    context.go('/sales/products');
  }

  void _toast(String msg, bool success) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(msg),
      backgroundColor: success ? Colors.green.shade700 : Colors.red.shade700,
      behavior: SnackBarBehavior.floating,
    ));
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    // In edit mode, hydrate from the keep-alive list once it's available.
    // Three states, not two: loading, found and MISSING. Without the third, a
    // non-existent id opened "Edit product" with a blank form and an active Save
    // button — the screen claiming to edit something that is not there.
    var missing = false;
    if (widget.isEdit && !_initialized) {
      final list = ref.watch(productsListProvider).value;
      if (list != null) {
        final found = list.where((ProductModel p) => p.id == widget.productId);
        if (found.isNotEmpty) {
          _editing = found.first;
          _hydrate(found.first);
        } else {
          missing = true;
        }
      }
    }

    return buildDirtyGuard(
      onDiscard: () => context.go('/sales/products'),
      child: Scaffold(
        backgroundColor: colors.backgroundPrimary,
        appBar: AppBar(
          backgroundColor: colors.backgroundSecondary,
          elevation: 0,
          leading: IconButton(
            tooltip: t.sales.back_tooltip,
            icon: Icon(Icons.arrow_back, color: colors.textSecondary),
            onPressed: () => context.go('/sales/products'),
          ),
          title: Text(
            widget.isEdit
                ? t.sales.products.edit_product
                : t.sales.products.new_product,
            style: TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.w700,
              color: colors.textPrimary,
            ),
          ),
        ),
        body: missing
            ? Center(
                child: Padding(
                  padding: const EdgeInsets.all(32),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.inventory_2_outlined,
                          size: 44, color: colors.textMuted),
                      const SizedBox(height: 14),
                      Text(
                        t.sales.products.not_found,
                        key: const Key('product_not_found'),
                        style: TextStyle(
                            color: colors.textPrimary,
                            fontSize: 16,
                            fontWeight: FontWeight.w700),
                      ),
                      const SizedBox(height: 6),
                      Text(
                        t.sales.products.not_found_hint,
                        textAlign: TextAlign.center,
                        style: TextStyle(color: colors.textMuted, fontSize: 13),
                      ),
                      const SizedBox(height: 20),
                      FilledButton(
                        onPressed: () => context.go('/sales/products'),
                        child: Text(t.sales.products.title),
                      ),
                    ],
                  ),
                ),
              )
            : Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 640),
                  child: Form(
                    key: _formKey,
                    child: ListView(
                      padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
                      children: [
                        _label(colors, t.sales.products.field_name),
                        CustomTextField(
                          hint: t.sales.products.field_name_hint,
                          controller: _nameController,
                          autofocus: !widget.isEdit,
                          validator: (v) => (v == null || v.trim().isEmpty)
                              ? t.sales.products.name_required
                              : null,
                        ),
                        const SizedBox(height: 16),
                        _label(colors, t.sales.products.field_description),
                        CustomTextField(
                          hint: t.sales.products.field_description_hint,
                          controller: _descriptionController,
                        ),
                        const SizedBox(height: 16),
                        Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Expanded(
                              flex: 2,
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  _label(colors, t.sales.products.field_price),
                                  CustomTextField(
                                    hint: _currency == 'BRL'
                                        ? '1.234,56'
                                        : '1,234.56',
                                    controller: _priceController,
                                    keyboardType:
                                        const TextInputType.numberWithOptions(
                                            decimal: true),
                                    // Optional, deliberately. In this market
                                    // the rep quotes at the moment of the
                                    // deal, so a required price forced a
                                    // number nobody meant and filled the
                                    // catalogue with "R$ 0,00" — which reads
                                    // as a broken import. Empty is a legible
                                    // "unknown"; a TYPED price still has to
                                    // parse, because a price that silently
                                    // becomes zero is worse than no price.
                                    validator: (v) {
                                      if (v == null || v.trim().isEmpty) {
                                        return null;
                                      }
                                      if (parseMoneyToCents(v) <= 0) {
                                        return t.sales.products.price_invalid;
                                      }
                                      return null;
                                    },
                                  ),
                                  const SizedBox(height: 4),
                                  Text(
                                    t.sales.products.field_price_optional,
                                    key: const Key('product_price_optional'),
                                    style: TextStyle(
                                        color: colors.textMuted, fontSize: 11),
                                  ),
                                ],
                              ),
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  _label(
                                      colors, t.sales.products.field_currency),
                                  DropdownButtonFormField<String>(
                                    key: const Key('product_currency'),
                                    initialValue: _currency,
                                    dropdownColor: colors.backgroundSecondary,
                                    style: TextStyle(
                                        color: colors.textPrimary,
                                        fontSize: 14),
                                    items: [
                                      for (final c in _currencies)
                                        DropdownMenuItem(
                                            value: c, child: Text(c)),
                                    ],
                                    onChanged: (v) {
                                      setState(() => _currency = v ?? 'BRL');
                                      recomputeDirty();
                                    },
                                  ),
                                ],
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 16),
                        _label(colors, t.sales.products.field_cycle),
                        CustomTextField(
                          key: const Key('product_cycle_days'),
                          hint: t.sales.products.field_cycle_hint,
                          controller: _cycleController,
                          keyboardType: TextInputType.number,
                          // Ten years is the server's ceiling too: a cycle
                          // of 99999 days silently removes the product from
                          // the reorder engine forever.
                          validator: (v) {
                            if (v == null || v.trim().isEmpty) return null;
                            final days = int.tryParse(v.trim());
                            if (days == null || days < 0 || days > 3650) {
                              return t.sales.products.cycle_invalid;
                            }
                            return null;
                          },
                        ),
                        const SizedBox(height: 16),
                        SwitchListTile(
                          contentPadding: EdgeInsets.zero,
                          value: _active,
                          activeThumbColor: colors.whatsappGreen,
                          onChanged: (v) {
                            setState(() => _active = v);
                            recomputeDirty();
                          },
                          title: Text(
                            t.sales.products.field_active,
                            style: TextStyle(
                                color: colors.textPrimary,
                                fontSize: 14,
                                fontWeight: FontWeight.w600),
                          ),
                          subtitle: Text(
                            t.sales.products.field_active_hint,
                            style: TextStyle(
                                color: colors.textMuted, fontSize: 12),
                          ),
                        ),
                        const SizedBox(height: 24),
                        // What the manufacturer's brochure said. Below the
                        // editable fields, because it is evidence rather than
                        // a form: read it, quote it, open the original.
                        if (_editing != null)
                          ProductBrochureSheet(product: _editing!),
                        const SizedBox(height: 8),
                        PrimaryPillButton(
                          text: t.sales.products.save,
                          isLoading: _saving,
                          onPressed: _saving ? null : _save,
                        ),
                      ],
                    ),
                  ),
                ),
              ),
      ),
    );
  }

  Widget _label(AppColorsExtension colors, String text) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Text(
        text,
        style: TextStyle(
          color: colors.textSecondary,
          fontSize: 13,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}
