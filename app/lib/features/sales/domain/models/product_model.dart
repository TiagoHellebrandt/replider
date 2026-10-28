import 'package:json_annotation/json_annotation.dart';

part 'product_model.g.dart';

/// Catalog product owned by an account (US-001/US-004). Sales funnels
/// reference products so the AI knows what it is selling. The API serialises
/// the Go `domain.Product` with snake_case JSON tags, so a plain
/// `FieldRename.snake` mapping is enough — no per-field `@JsonKey` overrides.
///
/// `attributes` is a free-form JSONB bag (sku, tags, warranty, ...) kept as a
/// passthrough map here; the basic product form does not edit it yet, but
/// round-tripping it preserves anything set elsewhere.
@JsonSerializable(fieldRename: FieldRename.snake)
class ProductModel {
  final String id;
  final String accountId;
  final String name;
  final String description;
  final int priceCents;
  final String currency;
  final Map<String, dynamic>? attributes;

  /// How many days a customer takes to go through one order (§6.5 F3). Zero
  /// means unknown, and a product with no cycle is one the reorder engine
  /// never resurfaces. Written through `PUT /products/{id}/cycles`, not the
  /// product update.
  final int consumptionCycleDays;

  /// Shelf life, a DIFFERENT number: gauze expires whether or not anyone
  /// uses it.
  final int shelfLifeDays;
  final bool active;

  /// The brochure this product was read from, and the page it was on. Empty
  /// for CSV imports and hand-typed products — a spreadsheet has no page to
  /// open. The file itself is fetched on demand (`GET /products/{id}/source`),
  /// never carried in the list payload.
  final String? sourceId;
  final int sourcePage;
  final String? createdAt;
  final String? updatedAt;

  const ProductModel({
    required this.id,
    required this.accountId,
    required this.name,
    this.description = '',
    this.priceCents = 0,
    this.currency = 'BRL',
    this.attributes,
    this.consumptionCycleDays = 0,
    this.shelfLifeDays = 0,
    this.active = true,
    this.sourceId,
    this.sourcePage = 0,
    this.createdAt,
    this.updatedAt,
  });

  /// Price in major currency units (e.g. 1990 cents -> 19.90).
  double get price => priceCents / 100.0;

  /// Zero is UNKNOWN, not free. In this market the seller quotes at the
  /// moment — a catalogue full of "R$ 0,00" reads as a broken import, and a
  /// proposal built from it would be a promise nobody made.
  bool get hasPrice => priceCents > 0;

  /// The brochure fields, read out of the free-form attributes bag. They were
  /// being SAVED and never SHOWN: the edit screen offered name, description,
  /// price and cycle, so a person looking for the model number of the thing
  /// they just imported found nothing and concluded the import lost it.
  String get brand => _attr('brand');
  String get model => _attr('model');
  String get sku => _attr('sku');
  String get unit => _attr('unit');

  /// The manufacturer's technical block, transcribed rather than summarised.
  String get specs => _attr('specs');

  List<String> get accessories => _attrList('accessories');
  List<String> get standards => _attrList('standards');

  /// True when there is anything from the brochure worth rendering.
  bool get hasSheet =>
      brand.isNotEmpty ||
      model.isNotEmpty ||
      sku.isNotEmpty ||
      specs.isNotEmpty ||
      accessories.isNotEmpty ||
      standards.isNotEmpty;

  String _attr(String key) {
    final v = attributes?[key];
    return v is String ? v.trim() : '';
  }

  List<String> _attrList(String key) {
    final v = attributes?[key];
    if (v is! List) return const [];
    return [
      for (final item in v)
        if (item is String && item.trim().isNotEmpty) item.trim(),
    ];
  }

  factory ProductModel.fromJson(Map<String, dynamic> json) =>
      _$ProductModelFromJson(json);
  Map<String, dynamic> toJson() => _$ProductModelToJson(this);
}
