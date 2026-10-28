/// One line of an uploaded customer list, as the server parsed it.
///
/// A row with a [problem] is still a row. A line dropped silently is a customer
/// the person believes they imported, so problems travel to the screen attached
/// to the line number they came from — the line in EXCEL, header included, so it
/// points at the row she still has open.
class CustomerImportRow {
  const CustomerImportRow({
    required this.line,
    this.company = '',
    this.name = '',
    this.phone = '',
    this.email = '',
    this.problem = '',
    this.duplicate = false,
  });

  final int line;

  /// The organization and the person at it. Both travel even though only one is
  /// needed, because the review should say which column it read rather than
  /// merging them into a label she cannot check.
  final String company;
  final String name;

  /// E.164 when it could be read at all. A number the parser refused arrives
  /// empty with a [problem] set — never silently dropped, because a contact
  /// imported without the number she typed is a card claiming the customer has
  /// no phone.
  final String phone;
  final String email;

  /// Empty when the row may create a contact. Written for the person who wrote
  /// the spreadsheet, not for a log.
  final String problem;

  /// "já está na sua base", flagged structurally by the server: a fact, not a
  /// fault — the existing customer stays. Rendered neutrally.
  final bool duplicate;

  bool get importable =>
      problem.isEmpty && (phone.isNotEmpty || email.isNotEmpty);

  /// What the contact will be called: the person when there is one, the
  /// organization otherwise. Mirrors the server, which decides the same way.
  String get displayName =>
      name.trim().isNotEmpty ? name.trim() : company.trim();

  /// Where a message to this customer would go. Shown next to the name because
  /// a list of names cannot catch the mistake it exists to catch.
  String get address => phone.isNotEmpty ? phone : email;

  factory CustomerImportRow.fromJson(Map<String, dynamic> json) =>
      CustomerImportRow(
        line: (json['line'] as num?)?.toInt() ?? 0,
        company: (json['company'] as String? ?? '').trim(),
        name: (json['name'] as String? ?? '').trim(),
        phone: json['phone'] as String? ?? '',
        email: json['email'] as String? ?? '',
        problem: json['problem'] as String? ?? '',
        duplicate: json['duplicate'] == true,
      );

  /// Sent back on confirmation. The server re-derives every field from this, so
  /// it is what she approved rather than a set of instructions it trusts.
  Map<String, dynamic> toJson() => <String, dynamic>{
        'line': line,
        'company': company,
        'name': name,
        'phone': phone,
        'email': email,
        'problem': problem,
      };
}

/// What one upload found, counted server-side.
class CustomerImportPreview {
  const CustomerImportPreview({
    required this.rows,
    required this.importable,
    required this.problems,
    required this.alreadyInBase,
  });

  final List<CustomerImportRow> rows;

  /// Counted by the server so the screen states the same numbers the import will
  /// act on. Deriving them from [rows] would let the two disagree.
  final int importable;
  final int problems;

  /// Rows whose customer the account already has. Its own number because it is a
  /// fact rather than a mistake: re-importing an updated list is normal, and what
  /// she needs to know is that the existing contact is the one that stays.
  final int alreadyInBase;

  factory CustomerImportPreview.fromJson(Map<String, dynamic> json) =>
      CustomerImportPreview(
        rows: (json['rows'] as List<dynamic>? ?? const <dynamic>[])
            .whereType<Map<String, dynamic>>()
            .map(CustomerImportRow.fromJson)
            .toList(),
        importable: (json['importable'] as num?)?.toInt() ?? 0,
        problems: (json['problems'] as num?)?.toInt() ?? 0,
        alreadyInBase: (json['already_in_base'] as num?)?.toInt() ?? 0,
      );
}

/// What a confirmed import produced.
class CustomerImportResult {
  const CustomerImportResult({required this.created, required this.skipped});

  final int created;

  /// Rows the server refused on the second pass. Reported so a count that does
  /// not match the confirmation has an explanation.
  final int skipped;

  factory CustomerImportResult.fromJson(Map<String, dynamic> json) =>
      CustomerImportResult(
        created: (json['created'] as num?)?.toInt() ?? 0,
        skipped: (json['skipped'] as num?)?.toInt() ?? 0,
      );
}
