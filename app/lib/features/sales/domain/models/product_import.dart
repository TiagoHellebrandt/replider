/// One line of an uploaded price list, as the server parsed it.
///
/// A row with a [problem] is still a row. A line dropped silently is a product
/// the person believes they imported, so problems travel to the screen attached
/// to the line number they came from — the line in EXCEL, header included, so it
/// points at the row she is looking at.
class ProductImportRow {
  const ProductImportRow({
    required this.line,
    required this.name,
    this.description = '',
    this.priceCents = 0,
    this.currency = '',
    this.sku = '',
    this.brand = '',
    this.model = '',
    this.unit = '',
    this.sourceId = '',
    this.page = 0,
    this.problem = '',
    this.duplicate = false,
    this.source = '',
  });

  final int line;
  final String name;
  final String description;
  final int priceCents;
  final String currency;

  /// Brochure-only fields (PDF import). Empty on CSV rows — the spreadsheet
  /// has no columns for them. They travel back on confirmation so the server
  /// can store them on the product's attributes.
  final String sku;
  final String brand;
  final String model;
  final String unit;

  /// Which archived brochure this row came from, as the server named it.
  ///
  /// Server-minted and carried back untouched. It travelled to the preview and
  /// was dropped here, so every product imported from a PDF landed with a page
  /// number and no file: thirteen products in production point at "page 1" of
  /// nothing, and the "open the brochure" button — built, routed and wired —
  /// has never rendered for anybody.
  final String sourceId;

  /// 1-based brochure page this row was read from; 0 on CSV rows. It is how
  /// the person checks the model's reading against the document they have.
  final int page;

  /// Empty when the row may create a product. Written for the person who wrote
  /// the spreadsheet, not for a log.
  final String problem;

  /// The "já está no catálogo" case, flagged structurally by the server. A
  /// fact, not a mistake: the existing product stays and nothing is
  /// overwritten — so the tile renders it neutrally instead of with the
  /// error icon.
  final bool duplicate;

  /// Which uploaded file this row was read from. Set client-side when several
  /// files are read in one go — with five brochures merged into one review,
  /// "where did this line come from?" is the first question. Never travels back
  /// on confirmation: the server re-reads nothing from it.
  final String source;

  ProductImportRow withSource(String filename) => ProductImportRow(
        line: line,
        name: name,
        description: description,
        priceCents: priceCents,
        currency: currency,
        sku: sku,
        brand: brand,
        model: model,
        unit: unit,
        sourceId: sourceId,
        page: page,
        problem: problem,
        duplicate: duplicate,
        source: filename,
      );

  bool get importable => problem.isEmpty && name.trim().isNotEmpty;

  /// A brochure line whose price the model could not read. The row is still
  /// importable — the product is created for the person to price later — but
  /// rendering it as a green "\$0.00" claimed a price nobody stated.
  bool get priceMissing => importable && page > 0 && priceCents <= 0;

  /// "Marca · Modelo · SKU", for the third line of the review tile. The
  /// review promises "you check every line before anything is created", and
  /// these fields used to skip it entirely on their way into attributes.
  String get brochureDetails => [
        if (brand.trim().isNotEmpty) brand.trim(),
        if (model.trim().isNotEmpty) model.trim(),
        if (sku.trim().isNotEmpty) sku.trim(),
      ].join(' · ');

  factory ProductImportRow.fromJson(Map<String, dynamic> json) =>
      ProductImportRow(
        line: (json['line'] as num?)?.toInt() ?? 0,
        name: (json['name'] as String? ?? '').trim(),
        description: json['description'] as String? ?? '',
        priceCents: (json['price_cents'] as num?)?.toInt() ?? 0,
        currency: json['currency'] as String? ?? '',
        sku: json['sku'] as String? ?? '',
        brand: json['brand'] as String? ?? '',
        model: json['model'] as String? ?? '',
        unit: json['unit'] as String? ?? '',
        sourceId: json['source_id'] as String? ?? '',
        page: (json['page'] as num?)?.toInt() ?? 0,
        problem: json['problem'] as String? ?? '',
        duplicate: json['duplicate'] == true,
      );

  /// Sent back on confirmation. The server re-checks every one of these, so this
  /// is what she approved rather than a set of instructions it trusts.
  Map<String, dynamic> toJson() => <String, dynamic>{
        'line': line,
        'name': name,
        'description': description,
        'price_cents': priceCents,
        'currency': currency,
        if (sku.isNotEmpty) 'sku': sku,
        if (brand.isNotEmpty) 'brand': brand,
        if (model.isNotEmpty) 'model': model,
        if (unit.isNotEmpty) 'unit': unit,
        if (sourceId.isNotEmpty) 'source_id': sourceId,
        if (page > 0) 'page': page,
        'problem': problem,
      };
}

/// What one upload found, counted server-side.
class ProductImportPreview {
  const ProductImportPreview({
    required this.rows,
    required this.importable,
    required this.problems,
    required this.alreadyInCatalogue,
  });

  final List<ProductImportRow> rows;

  /// Counted by the server so the screen states the same numbers the import will
  /// act on. Deriving them from [rows] would let the two disagree.
  final int importable;
  final int problems;

  /// Rows whose product is already in the catalogue. Its own number because it is
  /// a fact rather than a mistake: re-importing an updated list is normal, and
  /// what she needs to know is that the existing row is the one that stays.
  final int alreadyInCatalogue;

  factory ProductImportPreview.fromJson(Map<String, dynamic> json) =>
      ProductImportPreview(
        rows: (json['rows'] as List<dynamic>? ?? const <dynamic>[])
            .whereType<Map<String, dynamic>>()
            .map(ProductImportRow.fromJson)
            .toList(),
        importable: (json['importable'] as num?)?.toInt() ?? 0,
        problems: (json['problems'] as num?)?.toInt() ?? 0,
        alreadyInCatalogue:
            (json['already_in_catalogue'] as num?)?.toInt() ?? 0,
      );
}

/// Several files read in one go.
///
/// The counts are the SUM of what the server counted per file, never derived
/// from [rows] — same reason a single preview trusts the server's numbers.
/// Cross-file repeats are deliberately NOT collapsed: a line dropped silently
/// is a product the person believes they imported, so both copies stay on the
/// review with their source file and the human decides.
class ProductImportFailure {
  const ProductImportFailure({required this.filename, required this.reason});

  final String filename;

  /// WHY it failed, already localized. A batch that lists eighteen file names
  /// and no cause tells the person nothing — the first version of this did
  /// exactly that when the antivirus was down, and eighteen names read like
  /// eighteen broken PDFs instead of one broken service.
  final String reason;
}

class ProductImportBatch {
  const ProductImportBatch({required this.preview, required this.failed});

  final ProductImportPreview preview;

  /// The files that could not be read, each with its reason. Their failure does
  /// not discard the ones that worked — the person retries just these.
  final List<ProductImportFailure> failed;

  /// The distinct reasons, in the order they first appeared. One entry means
  /// one cause for the whole batch (a service being down), which is the
  /// sentence worth putting on screen.
  List<String> get reasons {
    final seen = <String>[];
    for (final f in failed) {
      if (!seen.contains(f.reason)) seen.add(f.reason);
    }
    return seen;
  }

  static ProductImportBatch merge(
    List<({String filename, ProductImportPreview preview})> ok,
    List<ProductImportFailure> failed,
  ) {
    return ProductImportBatch(
      preview: ProductImportPreview(
        rows: [
          for (final e in ok)
            for (final r in e.preview.rows) r.withSource(e.filename),
        ],
        importable: ok.fold(0, (a, e) => a + e.preview.importable),
        problems: ok.fold(0, (a, e) => a + e.preview.problems),
        alreadyInCatalogue:
            ok.fold(0, (a, e) => a + e.preview.alreadyInCatalogue),
      ),
      failed: failed,
    );
  }
}

/// What a confirmed import produced.
class ProductImportResult {
  const ProductImportResult({required this.created, required this.skipped});

  final int created;

  /// Rows the server refused on the second pass. Reported so a count that does
  /// not match the confirmation has an explanation.
  final int skipped;

  factory ProductImportResult.fromJson(Map<String, dynamic> json) =>
      ProductImportResult(
        created: (json['created'] as num?)?.toInt() ?? 0,
        skipped: (json['skipped'] as num?)?.toInt() ?? 0,
      );
}
