/// One line of an uploaded sales history, as the server parsed it.
///
/// Mirrors `usecases.SaleImportRow` on the API. A row with a [problem] is
/// still a row — a line dropped silently is a sale the person believes they
/// imported — so problems travel to the screen attached to the line number
/// they came from: the line in EXCEL, header included.
class SalesImportRow {
  const SalesImportRow({
    required this.line,
    this.customer = '',
    this.product = '',
    this.date = '',
    this.soldAt = '',
    this.quantity = 0,
    this.grossCents = 0,
    this.contactId = '',
    this.contactName = '',
    this.productId = '',
    this.problem = '',
    this.duplicate = false,
  });

  final int line;

  /// The customer cell as typed: an e-mail, a phone or a name. The server
  /// decides which at resolution time.
  final String customer;

  /// The product cell as typed. Matched against the catalogue by name; when
  /// nothing matches it becomes the sale item's free description.
  final String product;

  /// The date cell as typed; [soldAt] is what the server parsed it to
  /// (RFC3339, the Go zero value when it could not). Both travel so the
  /// preview shows the text she wrote next to the date the import will use.
  final String date;
  final String soldAt;

  final int quantity;
  final int grossCents;

  /// The resolution: who in the base this row is. Empty with a [problem]
  /// when nobody matched.
  final String contactId;
  final String contactName;

  /// Set when [product] matched the catalogue.
  final String productId;

  /// Empty when the row may record a sale. Written by the server for the
  /// person who wrote the spreadsheet, not for a log.
  final String problem;

  /// "venda já registrada", flagged structurally: a fact, not a fault — the
  /// recorded sale stays. Rendered neutrally.
  final bool duplicate;

  /// Whether the server parsed a real date — Go marshals its zero time as
  /// year 1, which is "no date" wearing a timestamp.
  bool get hasSoldAt => soldAt.isNotEmpty && !soldAt.startsWith('0001-01-01');

  /// Mirrors `SaleImportRow.Importable()` server-side.
  bool get importable =>
      problem.isEmpty && contactId.isNotEmpty && hasSoldAt && grossCents > 0;

  /// What the row is called in the review: the resolved contact when there is
  /// one, the cell as typed otherwise — so a wrong resolution is visible.
  String get displayName =>
      contactName.trim().isNotEmpty ? contactName.trim() : customer.trim();

  factory SalesImportRow.fromJson(Map<String, dynamic> json) => SalesImportRow(
        line: (json['line'] as num?)?.toInt() ?? 0,
        customer: (json['customer'] as String? ?? '').trim(),
        product: (json['product'] as String? ?? '').trim(),
        date: (json['date'] as String? ?? '').trim(),
        soldAt: json['sold_at'] as String? ?? '',
        quantity: (json['quantity'] as num?)?.toInt() ?? 0,
        grossCents: (json['gross_cents'] as num?)?.toInt() ?? 0,
        contactId: json['contact_id'] as String? ?? '',
        contactName: json['contact_name'] as String? ?? '',
        productId: json['product_id'] as String? ?? '',
        problem: json['problem'] as String? ?? '',
        duplicate: json['duplicate'] == true,
      );

  /// Sent back on confirmation. The server re-derives everything from this
  /// (`cleanSaleImportRow`), so it is what she approved rather than a set of
  /// instructions it trusts. `sold_at` travels because the server accepts the
  /// parsed date back when the typed cell alone is unreadable.
  Map<String, dynamic> toJson() => <String, dynamic>{
        'line': line,
        'customer': customer,
        'product': product,
        'date': date,
        'sold_at': soldAt,
        'quantity': quantity,
        'gross_cents': grossCents,
        'problem': problem,
      };
}

/// What one upload found, counted server-side (`usecases.SaleImportPreview`).
class SalesImportPreview {
  const SalesImportPreview({
    required this.rows,
    required this.importable,
    required this.problems,
    required this.alreadyRecorded,
    required this.unknownCustomers,
  });

  final List<SalesImportRow> rows;

  /// Counted by the server so the screen states the numbers the import will
  /// act on. Deriving them from [rows] would let the two disagree.
  final int importable;
  final int problems;

  /// Rows whose sale the account already has: a fact, not a mistake — a
  /// re-export of the same ERP report is normal.
  final int alreadyRecorded;

  /// Rows naming customers the base does not have — the number that tells the
  /// person to import the customer list first.
  final int unknownCustomers;

  factory SalesImportPreview.fromJson(Map<String, dynamic> json) =>
      SalesImportPreview(
        rows: (json['rows'] as List<dynamic>? ?? const <dynamic>[])
            .whereType<Map<String, dynamic>>()
            .map(SalesImportRow.fromJson)
            .toList(),
        importable: (json['importable'] as num?)?.toInt() ?? 0,
        problems: (json['problems'] as num?)?.toInt() ?? 0,
        alreadyRecorded: (json['already_recorded'] as num?)?.toInt() ?? 0,
        unknownCustomers: (json['unknown_customers'] as num?)?.toInt() ?? 0,
      );
}

/// What a confirmed import recorded (`usecases.ImportSalesResult`).
class SalesImportResult {
  const SalesImportResult({required this.created, required this.skipped});

  final int created;

  /// Rows the server refused on the second pass. Reported so a count that
  /// does not match the confirmation has an explanation.
  final int skipped;

  factory SalesImportResult.fromJson(Map<String, dynamic> json) =>
      SalesImportResult(
        created: (json['created'] as num?)?.toInt() ?? 0,
        skipped: (json['skipped'] as num?)?.toInt() ?? 0,
      );
}
