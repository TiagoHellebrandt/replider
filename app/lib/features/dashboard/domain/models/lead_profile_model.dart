/// The sales-funnel memory for the contact behind a conversation.
///
/// Aggregated server-side from the conversation's active funnel
/// (metadata schema) joined with the lead's collected values. Surfaces
/// the "Perfil de vendas" section on the contact detail sheet.
///
/// Fetched via `GET /v1/conversations/{id}/sales-profile`. When the
/// conversation is not bound to any funnel (no channel funnel or no lead
/// yet) the endpoint returns `in_funnel: false` (or 404), rendered as the
/// empty state.
class LeadProfileModel {
  /// False when the conversation's channel has no active sales funnel, or
  /// no lead exists for the contact yet.
  final bool inFunnel;

  /// Lead identifier — needed for manual metadata edits (US-014:
  /// PATCH /v1/leads/{id}/metadata). Null when [inFunnel] is false.
  final String? leadId;
  final String? funnelId;
  final String? funnelName;

  /// Pipeline status of the lead (e.g. `responded`, `qualified`). Null
  /// when [inFunnel] is false.
  final String? status;
  final int score;

  /// Stamped when the AI detected purchase intent (US-012). Null until
  /// the lead signals intent.
  final DateTime? buyIntentAt;

  /// Schema fields merged with their collected values, in schema order.
  final List<LeadFieldModel> fields;

  const LeadProfileModel({
    required this.inFunnel,
    this.leadId,
    this.funnelId,
    this.funnelName,
    this.status,
    this.score = 0,
    this.buyIntentAt,
    this.fields = const [],
  });

  /// The conversation is not part of any funnel — render empty state.
  const LeadProfileModel.empty() : this(inFunnel: false);

  bool get hasBuyIntent => buyIntentAt != null;

  factory LeadProfileModel.fromJson(Map<String, dynamic> json) {
    final inFunnel = json['in_funnel'] == true;
    if (!inFunnel) return const LeadProfileModel.empty();
    final rawFields = json['fields'];
    final fields = rawFields is List
        ? rawFields
            .whereType<Map<String, dynamic>>()
            .map(LeadFieldModel.fromJson)
            .toList()
        : <LeadFieldModel>[];
    return LeadProfileModel(
      inFunnel: true,
      leadId: json['lead_id'] as String?,
      funnelId: json['funnel_id'] as String?,
      funnelName: json['funnel_name'] as String?,
      status: json['status'] as String?,
      score: (json['score'] as num?)?.toInt() ?? 0,
      buyIntentAt: _parseDate(json['buy_intent_at']),
      fields: fields,
    );
  }

  static DateTime? _parseDate(dynamic raw) {
    if (raw is! String || raw.isEmpty) return null;
    return DateTime.tryParse(raw);
  }
}

/// Origin of a collected metadata value.
enum LeadFieldSource { ai, human, none }

/// A single metadata field merged from the funnel schema (key/label/type/
/// required) with the lead's collected value (value/source).
class LeadFieldModel {
  final String key;
  final String label;
  final String type;
  final bool required;

  /// Collected value as a display string. Empty when not collected yet.
  final String value;

  final LeadFieldSource source;

  const LeadFieldModel({
    required this.key,
    required this.label,
    required this.type,
    this.required = false,
    this.value = '',
    this.source = LeadFieldSource.none,
  });

  /// A required field with no collected value yet — highlighted as pending.
  bool get isPending => required && value.isEmpty;

  bool get hasValue => value.isNotEmpty;

  factory LeadFieldModel.fromJson(Map<String, dynamic> json) {
    final rawValue = json['value'];
    return LeadFieldModel(
      key: json['key'] as String? ?? '',
      label: (json['label'] as String?)?.isNotEmpty == true
          ? json['label'] as String
          : json['key'] as String? ?? '',
      type: json['type'] as String? ?? 'text',
      required: json['required'] == true,
      value: rawValue == null ? '' : rawValue.toString(),
      source: _parseSource(json['source']),
    );
  }

  static LeadFieldSource _parseSource(dynamic raw) => switch (raw) {
        'ai' => LeadFieldSource.ai,
        'human' => LeadFieldSource.human,
        _ => LeadFieldSource.none,
      };
}
