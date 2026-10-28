import 'package:app/i18n/strings.g.dart';

/// Stage of a lead in the outbound sales pipeline (mirrors the Go
/// `domain.LeadStatus`, US-003). Ordered as the pipeline flows so the leads
/// board can render columns left-to-right.
enum LeadPipelineStatus {
  newLead('new'),
  prospected('prospected'),
  enriching('enriching'),
  enriched('enriched'),
  pendingApproval('pending_approval'),
  contacted('contacted'),
  responded('responded'),
  qualified('qualified'),
  won('won'),
  lost('lost');

  const LeadPipelineStatus(this.wire);

  /// The snake_case value the API serialises/accepts.
  final String wire;

  static LeadPipelineStatus fromWire(String? raw) {
    for (final s in LeadPipelineStatus.values) {
      if (s.wire == raw) return s;
    }
    return LeadPipelineStatus.newLead;
  }

  /// Localised column/badge label. Single source of truth — the UI never
  /// re-implements this switch.
  String get label {
    switch (this) {
      case LeadPipelineStatus.newLead:
        return t.sales.leads.status_new;
      case LeadPipelineStatus.prospected:
        return t.sales.leads.status_prospected;
      case LeadPipelineStatus.enriching:
        return t.sales.leads.status_enriching;
      case LeadPipelineStatus.enriched:
        return t.sales.leads.status_enriched;
      case LeadPipelineStatus.pendingApproval:
        return t.sales.leads.status_pending_approval;
      case LeadPipelineStatus.contacted:
        return t.sales.leads.status_contacted;
      case LeadPipelineStatus.responded:
        return t.sales.leads.status_responded;
      case LeadPipelineStatus.qualified:
        return t.sales.leads.status_qualified;
      case LeadPipelineStatus.won:
        return t.sales.leads.status_won;
      case LeadPipelineStatus.lost:
        return t.sales.leads.status_lost;
    }
  }

  /// A lead already contacted/responded lives in a live conversation — the
  /// card links to the inbox instead of offering an approach draft.
  bool get isInConversation =>
      this == LeadPipelineStatus.contacted ||
      this == LeadPipelineStatus.responded ||
      this == LeadPipelineStatus.qualified ||
      this == LeadPipelineStatus.won;
}

/// One outbound-sales lead (mirrors the Go `domain.Lead`, US-003/US-015). The
/// `enrichment` blob is the Gemini-grounding result (US-023) merged over the
/// prospecting seed; helper getters surface the fields the approach card shows.
class LeadModel {
  final String id;
  final String funnelId;
  final String contactId;
  final LeadPipelineStatus status;
  final String source;
  final int score;
  final Map<String, dynamic> enrichment;
  final String approachDraft;
  final bool hasBuyIntent;

  const LeadModel({
    required this.id,
    this.funnelId = '',
    this.contactId = '',
    this.status = LeadPipelineStatus.newLead,
    this.source = '',
    this.score = 0,
    this.enrichment = const {},
    this.approachDraft = '',
    this.hasBuyIntent = false,
  });

  String? _str(String key) {
    final v = enrichment[key];
    if (v is String && v.trim().isNotEmpty) return v;
    return null;
  }

  /// Best label for the lead: the discovered company name, falling back to the
  /// raw contact id (phone / place id) so a not-yet-enriched lead still renders.
  String get displayName => _str('company_name') ?? contactId;
  String? get decisionMakerName => _str('decision_maker_name');
  String? get decisionMakerRole => _str('decision_maker_role');
  String? get linkedinUrl => _str('linkedin_url');
  String? get instagramUrl => _str('instagram_url');
  String? get website => _str('website');
  String? get address => _str('address');

  factory LeadModel.fromJson(Map<String, dynamic> json) {
    Map<String, dynamic> decodeMap(dynamic raw) {
      if (raw is Map<String, dynamic>) return raw;
      return const {};
    }

    return LeadModel(
      id: json['id'] as String? ?? '',
      funnelId: json['funnel_id'] as String? ?? '',
      contactId: json['contact_id'] as String? ?? '',
      status: LeadPipelineStatus.fromWire(json['status'] as String?),
      source: json['source'] as String? ?? '',
      score: (json['score'] as num?)?.toInt() ?? 0,
      enrichment: decodeMap(json['enrichment']),
      approachDraft: json['approach_draft'] as String? ?? '',
      hasBuyIntent: json['buy_intent_at'] != null,
    );
  }
}

/// Outcome of a prospecting search (US-022 backend `ProspectingResult`).
class ProspectingResultModel {
  final int resultsCount;
  final int leadsCreated;
  final int skippedDuplicate;
  final int skippedNoContact;

  const ProspectingResultModel({
    this.resultsCount = 0,
    this.leadsCreated = 0,
    this.skippedDuplicate = 0,
    this.skippedNoContact = 0,
  });

  factory ProspectingResultModel.fromJson(Map<String, dynamic> json) =>
      ProspectingResultModel(
        resultsCount: (json['results_count'] as num?)?.toInt() ?? 0,
        leadsCreated: (json['leads_created'] as num?)?.toInt() ?? 0,
        skippedDuplicate: (json['skipped_duplicate'] as num?)?.toInt() ?? 0,
        skippedNoContact: (json['skipped_no_contact'] as num?)?.toInt() ?? 0,
      );
}
