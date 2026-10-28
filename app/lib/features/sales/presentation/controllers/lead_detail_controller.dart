import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/sales/presentation/controllers/approval_queue_controller.dart'
    show MatchedProduct;
import 'package:app/features/sales/domain/models/approach_review.dart';
import 'package:app/i18n/strings.g.dart';

/// Lead detail + actions (US-M019/M020).
///
/// The detail is a read of the SAME lead the board already knows about, but
/// fetched fresh: the board payload is trimmed for scanning, and a panel
/// that acts on a lead must not act on a stale draft.

/// One fact about the lead, with a label a person reads.
///
/// Structured rather than a pre-joined string because the raw storage keys
/// were leaking into the UI: a salesperson opening a lead was shown
/// "cnae_label: ..." and "decision_maker_role: ...", which is a database
/// schema, not a profile.
class LeadFact {
  const LeadFact({
    required this.label,
    required this.value,
    this.fromResearch = false,
  });

  final String label;
  final String value;

  /// Whether this fact came from the RESEARCH, rather than from the
  /// prospecting seed or the conversation. It separates "we discovered this"
  /// from "we already knew" — without the distinction, a freshly imported lead
  /// presents itself as researched.
  final bool fromResearch;
}

class LeadDetail {
  const LeadDetail({
    required this.id,
    required this.status,
    required this.approachDraft,
    required this.recipient,
    required this.recipientAddress,
    required this.facts,
    this.collectedValues = const {},
    this.ownerUserId = '',
    this.visibility = 'team',
    this.sources = const [],
    required this.timeline,
    this.conversationId,
    this.city,
    this.website,
    this.linkedinUrl,
    this.instagramUrl,
    this.researchedAt,
    this.researchBlockedReason,
    this.findings = const [],
    this.matchedProducts = const [],
  });

  final String id;
  final String status;
  final String approachDraft;

  /// Who the message would go to, as a human recognises them: a name when
  /// there is one.
  final String recipient;

  /// WHERE the message would go — the phone or the email, never a name.
  ///
  /// Rendered on the panel now, not only in the send confirmation. Someone
  /// deciding whether to call has to be able to see the number without
  /// starting a send they might cancel.
  final String recipientAddress;

  final List<LeadFact> facts;

  /// The collection fields already answered, by key — the raw value, for the
  /// form to edit. `facts` shows the same content read-only; this map is what
  /// the form needs.
  final Map<String, String> collectedValues;

  /// Who answers for this lead. Empty means nobody.
  final String ownerUserId;

  /// `team` or `private`. Private disappears from the pipeline of anyone who
  /// is not the owner.
  final String visibility;

  /// Where the AI got the profile from. The panel showed the facts without
  /// their provenance: a partner's name taken from a LinkedIn page and one the
  /// model inferred looked like the same thing.
  final List<String> sources;
  final List<String> timeline;

  /// The thread this contact is in, when there is one. Absent for a freshly
  /// prospected lead nobody has written to yet.
  final String? conversationId;

  final String? city;
  final String? website;

  /// When the grounded search last ran. Present even when it came back with
  /// nothing — that is the whole point: "we searched and found nothing" is a
  /// different sentence from "nobody has looked yet".
  final String? researchedAt;

  /// What the automatic check found in the draft — the same one the approval
  /// queue shows. An empty list means "no pattern found", which differs from
  /// "reviewed and approved", and the screen says so in those words.
  final List<ApproachFinding> findings;

  /// Which of the account's products the matcher picked for this lead, and
  /// why. The same list the approval queue shows above the draft — read here
  /// so a priced offer can OPEN with the picks already in it instead of a
  /// blank form the seller has to fill from a hundred-item catalogue.
  final List<MatchedProduct> matchedProducts;

  /// Why the last attempt did no work, when it did none. Today the only
  /// reason is a lead carrying neither a company name nor a website, which is
  /// something the person can fix.
  final String? researchBlockedReason;

  /// The decision-maker's profile, when the research found one.
  ///
  /// Enrichment writes these two URLs (`enrich_lead.go`) and no funnel screen
  /// read them — only the old lead list did. The search found the decider's
  /// LinkedIn and the panel made it disappear.
  final String? linkedinUrl;
  final String? instagramUrl;

  /// Whether anything is known about the company beyond its contact details.
  ///
  /// Drives the difference between "nothing found" and "not looked up yet",
  /// which is what the old panel got wrong: an unenriched lead simply lost
  /// the whole profile section with no explanation and no way to fill it.
  bool get isEnriched => facts.isNotEmpty;

  /// The search is running right now (the API answers 202 and works in the
  /// background).
  bool get isResearching => status == 'enriching';

  /// Whether the RESEARCH discovered anything — not whether any fact is on
  /// screen.
  ///
  /// `company_name` is the prospecting SEED: `withCompanyName` injects it into
  /// the enrichment before the search runs. Measuring "researched" by `facts`
  /// made every prospected lead start out enriched, so `researchedNothing` was
  /// never true and the sentence "we searched and found nothing" — which
  /// exists — never appeared. The lead changed column and the screen said
  /// nothing more.
  bool get researchFoundSomething =>
      facts.any((f) => f.fromResearch) ||
      (linkedinUrl?.isNotEmpty ?? false) ||
      (instagramUrl?.isNotEmpty ?? false);

  /// A search ran and came back empty-handed. Not the same as untouched.
  bool get researchedNothing =>
      !researchFoundSomething && !isResearching && researchedAt != null;

  /// Nothing was searched, and the panel knows why.
  bool get researchBlocked =>
      !isResearching && (researchBlockedReason?.isNotEmpty ?? false);

  /// Whether the address looks like a phone, so the panel can offer to call
  /// rather than to write.
  bool get canCall {
    final a = recipientAddress.replaceAll(RegExp(r'[^0-9+]'), '');
    return !recipientAddress.contains('@') && a.length >= 10;
  }

  factory LeadDetail.fromJson(Map<String, dynamic> json) {
    final enrichment = _asMap(json['enrichment']);
    final collected = _asMap(json['collected_metadata']);

    // `company_name` and `address` are the prospecting SEED; the rest comes
    // from the research. The mark travels on the fact because it is what
    // answers "what did we discover?" — without it, a freshly imported lead
    // presents itself as researched and the research button looks like it did
    // nothing.
    const seeded = {'company_name', 'address'};
    // `cnae_label` reads a key nothing currently writes. Its only producer is
    // `services.CompanyLookup` (the Brazilian CNPJ provider), which main.go
    // builds and then discards — `_ = companyLookup // wired by the prospecting
    // slice`. The read stays so that wiring the lookup surfaces the trade
    // without a client change; it costs one line in a list and renders nothing
    // until then. Delete it only together with that service.
    final facts = <LeadFact>[
      for (final key in const [
        'company_name',
        'decision_maker_name',
        'decision_maker_role',
        'cnae_label',
        'address',
      ])
        if (_str(enrichment[key]) != null)
          LeadFact(
            label: _label(key),
            value: _str(enrichment[key])!,
            fromResearch: !seeded.contains(key),
          ),
      // What the AI collected during the conversation is usually the most
      // current thing known. The KEY is humanised: a field the customer
      // configured as `orcamento_previsto` reads as "Orçamento previsto",
      // not as a column name.
      for (final entry in collected.entries)
        if (_collectedValue(entry.value) != null)
          LeadFact(
            label: _humanise(entry.key),
            value: _collectedValue(entry.value)!,
          ),
    ];

    // Through slang, not PT literals — the same rule _label() below was
    // written for; an EN user was reading a Portuguese timeline.
    final timeline = <String>[
      if (_str(json['created_at']) != null)
        t.lead_detail.timeline_created(date: _fmtDate(json['created_at'])),
      if (_str(json['qualified_at']) != null)
        t.lead_detail.timeline_qualified(date: _fmtDate(json['qualified_at'])),
      if (_str(json['buy_intent_at']) != null)
        t.lead_detail
            .timeline_buy_intent(date: _fmtDate(json['buy_intent_at'])),
      // The research is the one thing the panel DOES on its own. A timeline
      // that omits it makes a lead that came back empty look untouched.
      if (_str(enrichment['enriched_at']) != null)
        t.lead_detail
            .timeline_researched(date: _fmtDate(enrichment['enriched_at'])),
      if (_str(json['updated_at']) != null)
        t.lead_detail.timeline_updated(date: _fmtDate(json['updated_at'])),
    ];

    return LeadDetail(
      id: json['id'] as String? ?? '',
      status: json['status'] as String? ?? '',
      approachDraft: json['approach_draft'] as String? ?? '',
      // Name → phone → email, resolved server-side; the id only survives
      // when the contact row itself is gone.
      recipient: json['contact_label'] as String? ??
          json['contact_id'] as String? ??
          '',
      recipientAddress: json['contact_address'] as String? ?? '',
      conversationId: _str(json['conversation_id']),
      city: _cityFrom(enrichment),
      website: _str(enrichment['website']),
      linkedinUrl: _str(enrichment['linkedin_url']),
      instagramUrl: _str(enrichment['instagram_url']),
      facts: facts,
      ownerUserId: json['owner_user_id'] as String? ?? '',
      visibility: json['visibility'] as String? ?? 'team',
      sources: ((enrichment['sources'] as List<dynamic>?) ?? const [])
          .whereType<String>()
          .where((u) => u.startsWith('http'))
          .toList(),
      collectedValues: {
        for (final entry in collected.entries)
          if (_collectedValue(entry.value) != null)
            entry.key: _collectedValue(entry.value)!,
      },
      timeline: timeline,
      researchedAt: _str(enrichment['enriched_at']),
      researchBlockedReason: _str(enrichment['enrich_blocked']),
      findings:
          ((json['approach_findings'] as List<dynamic>?) ?? const <dynamic>[])
              .whereType<Map<String, dynamic>>()
              .map(ApproachFinding.fromJson)
              .where((f) => f.quote.isNotEmpty)
              .toList(),
      matchedProducts: MatchedProduct.listFrom(json['matched_products']),
    );
  }

  /// The city, best effort. Prospecting writes a full address; enrichment may
  /// write a city. Null rather than a guess: a wrong city on a lead sends
  /// someone to the wrong town.
  static String? _cityFrom(Map<String, dynamic> enrichment) {
    for (final key in ['city', 'cidade', 'municipio']) {
      final v = _str(enrichment[key]);
      if (v != null) return v;
    }
    final address = enrichment['address'];
    if (address is String) {
      final parts =
          address.split(',').map((p) => p.trim()).where((p) => p.isNotEmpty);
      if (parts.length >= 2) return parts.elementAt(parts.length - 2);
    }
    return null;
  }

  static Map<String, dynamic> _asMap(dynamic v) {
    if (v is Map<String, dynamic>) return v;
    if (v is String && v.trim().isNotEmpty) {
      try {
        final decoded = jsonDecode(v);
        if (decoded is Map<String, dynamic>) return decoded;
      } catch (_) {}
    }
    return const {};
  }

  static String? _str(dynamic v) {
    if (v is String && v.trim().isNotEmpty) return v.trim();
    return null;
  }

  /// Collected fields are stored as {value, source, at} — the raw map would
  /// render as JSON noise in a profile list.
  static String? _collectedValue(dynamic v) {
    if (v is Map && v['value'] is String) return _str(v['value']);
    return _str(v);
  }

  /// dd/MM/yyyy HH:mm by hand: `intl` is only a transitive dependency here,
  /// and one date format is not worth promoting it to a direct one.
  static String _fmtDate(dynamic raw) {
    final parsed = DateTime.tryParse(raw as String? ?? '');
    if (parsed == null) return '';
    final d = parsed.toLocal();
    String two(int v) => v.toString().padLeft(2, '0');
    return '${two(d.day)}/${two(d.month)}/${d.year} ${two(d.hour)}:${two(d.minute)}';
  }

  // Through slang, not a PT literal map: an EN-locale user used to read a
  // lead panel that mixed English chrome with Portuguese field names.
  static String _label(String key) => switch (key) {
        'company_name' => t.lead_detail.field_company,
        'website' => t.lead_detail.field_website,
        'decision_maker_name' => t.lead_detail.field_decision_maker,
        'decision_maker_role' => t.lead_detail.field_role,
        'address' => t.lead_detail.field_address,
        'cnae_label' => t.lead_detail.field_activity,
        _ => _humanise(key),
      };

  /// Turns a storage key into something a person reads: `orcamento_previsto`
  /// becomes "Orçamento previsto". Imperfect by nature — but a humanised key
  /// is always better than the raw one, which is what was on screen before.
  static String _humanise(String key) {
    final words = key.replaceAll(RegExp(r'[_\-]+'), ' ').trim();
    if (words.isEmpty) return key;
    return words[0].toUpperCase() + words.substring(1);
  }
}

final leadDetailProvider =
    FutureProvider.family<LeadDetail, String>((ref, leadId) async {
  final dio = ref.watch(dioProvider);
  final res = await dio.get<Map<String, dynamic>>('/leads/$leadId');
  return LeadDetail.fromJson(res.data ?? const <String, dynamic>{});
});

final leadActionsProvider = Provider<LeadActions>(
  (ref) => LeadActions(ref.watch(dioProvider)),
);

class LeadActions {
  const LeadActions(this._dio);

  final Dio _dio;

  /// Asks the AI for a fresh approach. Returns the new draft so the editor
  /// can show it without a second round-trip.
  Future<String?> draft({required String leadId}) async {
    final res = await _dio.post<Map<String, dynamic>>('/leads/$leadId/draft');
    return res.data?['approach_draft'] as String?;
  }

  /// Stores what the person wrote, without sending and without queuing.
  ///
  /// The only writers of the draft were the AI and the send: whoever typed
  /// their own text and closed the panel lost all of it, because the words
  /// lived in a screen field and nowhere else. Losing what somebody wrote is
  /// the bug that makes them stop writing in the tool.
  Future<void> saveDraft({
    required String leadId,
    required String text,
  }) async {
    await _dio.patch<Map<String, dynamic>>(
      '/leads/$leadId/approach',
      data: <String, dynamic>{'text': text},
    );
  }

  /// Sends the HUMAN-EDITED text. There is deliberately no path that sends
  /// the AI's draft unreviewed — that gate is the product's whole posture on
  /// cold outbound.
  /// Returns null when the message went out NOW, or the moment the server
  /// scheduled it for — approving past the mailbox's daily cap no longer
  /// refuses, it books the send for the next morning (phase 1b).
  Future<DateTime?> approveSend({
    required String leadId,
    required String text,
    String? subject,
    String? templateName,
    String? templateLanguage,
    String? templateCategory,
    List<Map<String, dynamic>>? templateButtons,
    String? emailTemplateId,
    int emailTemplateVersion = 0,
    bool viaBulk = false,
  }) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/leads/$leadId/approve-send',
      data: <String, dynamic>{
        'text': text,
        // Hybrid sending: batch approvals go through the platform's bulk
        // provider (refused with BULK_SENDING_NOT_READY until the domain is
        // configured for volume); one-by-one approvals never set this.
        if (viaBulk) 'via_bulk': true,
        if (subject != null && subject.isNotEmpty) 'subject': subject,
        if (templateName != null && templateName.isNotEmpty) ...{
          'template_name': templateName,
          // Language/category/buttons ride along so the API's anti-ban guard
          // can see the opt-out quick-reply — a MARKETING template sent
          // without its declared buttons is refused before any network call.
          if (templateLanguage != null && templateLanguage.isNotEmpty)
            'template_language': templateLanguage,
          if (templateCategory != null && templateCategory.isNotEmpty)
            'template_category': templateCategory,
          if (templateButtons != null) 'template_buttons': templateButtons,
        },
        // Attribution for the email path: the send, and any reply it earns,
        // count against the exact template version the text came from.
        if (emailTemplateId != null && emailTemplateId.isNotEmpty) ...{
          'email_template_id': emailTemplateId,
          'email_template_version': emailTemplateVersion,
        },
      },
    );
    final body = res.data;
    if (body != null && body['scheduled'] == true) {
      final raw = body['scheduled_for'] as String?;
      return raw == null ? null : DateTime.tryParse(raw)?.toLocal();
    }
    return null;
  }

  /// Asks a SECOND model to read the text about to be sent, against the facts
  /// the company supplied about itself and the lead.
  ///
  /// Reviews the final text, not the generated draft: whoever edits can
  /// introduce a promise the model never wrote, and a draft-time check would
  /// have passed it. Never rewrites and never blocks — it reports, the human
  /// decides.
  ///
  /// On demand rather than automatic because this one costs a model call,
  /// unlike the pattern pass that already rides along with the queue.
  Future<ApproachReview> review({
    required String leadId,
    required String text,
  }) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/leads/$leadId/review-approach',
      data: <String, dynamic>{'text': text},
    );
    return ApproachReview.fromJson(res.data ?? const <String, dynamic>{});
  }

  /// Grava um campo de coleta preenchido à mão.
  ///
  /// The route has existed for as long as the funnel schema has, and no screen
  /// called it: the required fields are what makes a lead `qualified`, and only
  /// the AI could fill them in during a conversation. A lead that arrived by
  /// referral could be qualified by nobody.
  ///
  /// One field per call, as the server accepts — and as we want: saving the
  /// whole form would make editing one field rewrite the others with whatever
  /// was on screen.
  Future<void> saveMetadataField({
    required String leadId,
    required String fieldKey,
    required String value,
  }) async {
    await _dio.patch<Map<String, dynamic>>(
      '/leads/$leadId/metadata',
      data: <String, dynamic>{'field_key': fieldKey, 'value': value},
    );
  }

  /// Hands the lead to somebody — or takes it from everybody with null.
  ///
  /// The action existed in the use case from the start and had no route: the
  /// board marked every lead "no owner" and nothing could change that.
  Future<void> assignOwner({
    required String leadId,
    required String? ownerUserId,
  }) async {
    await _dio.patch<Map<String, dynamic>>(
      '/leads/$leadId/owner',
      data: <String, dynamic>{'owner_user_id': ownerUserId},
    );
  }

  /// Hides the lead from the rest of the team, or gives it back to them.
  Future<void> setVisibility({
    required String leadId,
    required String visibility,
  }) async {
    await _dio.patch<Map<String, dynamic>>(
      '/leads/$leadId/visibility',
      data: <String, dynamic>{'visibility': visibility},
    );
  }

  /// Discards a queued approach: the lead leaves the approval queue without
  /// anything being sent.
  Future<void> discard({required String leadId}) async {
    await _dio.patch<Map<String, dynamic>>(
      '/leads/$leadId/status',
      data: <String, dynamic>{'status': 'enriched'},
    );
  }
}

/// The lead's CNES installed park (Clientes redesign): what the public
/// registry says this establishment declares — counts, never age. Null when
/// the bridge found nothing; an absent card is the honest render.
/// One product of yours against one thing they own.
class ParkMatch {
  const ParkMatch({
    required this.productName,
    required this.equipment,
    required this.qtExist,
    this.productId = '',
  });

  /// Which product of the catalogue this is, so the line can open it.
  ///
  /// A salesperson does not know the whole catalogue by heart — that is the
  /// point of the crossing. Naming a product they have never sold and giving
  /// them nowhere to read it leaves them guessing at the specification in
  /// front of a buyer. Empty on an older server; the line then renders as
  /// plain text rather than a dead link.
  final String productId;

  final String productName;

  /// The registry's own label for what they have, and how many they declare —
  /// a count, never an age.
  final String equipment;
  final int qtExist;

  /// The same shape on the lead panel and on the radar card — one parser so
  /// the two surfaces cannot drift on what a pairing is.
  static ParkMatch? fromJson(dynamic raw) {
    if (raw is! Map<String, dynamic>) return null;
    final product = (raw['product_name'] as String? ?? '').trim();
    if (product.isEmpty) return null;
    return ParkMatch(
      productId: (raw['product_id'] as String? ?? '').trim(),
      productName: product,
      equipment: (raw['equipment'] as String? ?? '').trim(),
      qtExist: (raw['qt_exist'] as num?)?.toInt() ?? 0,
    );
  }
}

class LeadPark {
  const LeadPark(
      {required this.cnes,
      required this.tradeName,
      required this.matchedBy,
      required this.items,
      this.competencia,
      this.matches = const []});

  /// What of the catalogue fits this park.
  ///
  /// The answer to "what can I offer this hospital", available before spending
  /// anything: the crossing is a word match the server does for free. The AI
  /// was only ever needed to WRITE the message.
  final List<ParkMatch> matches;
  final String cnes;
  final String tradeName;

  /// "phone" (an identity) or "name" (a unique hit) — the UI qualifies the
  /// claim with it.
  final String matchedBy;
  final List<String> items;

  /// The DATASUS month the counts come from ("2026-07") — the box says
  /// "dados CNES de 07/2026", never "hoje".
  final String? competencia;
}

/// One park line, honestly: a code the official dictionary does not name
/// stays a CODE ("código CNES 0565", with its official group when one is
/// sourced) — never dropped, never a guessed name.
String leadParkItemLabel(Map<String, dynamic> p) {
  final name = (p['name'] as String? ?? '').trim();
  final code = (p['equip_code'] as String? ?? '').trim();
  final group = (p['group_name'] as String? ?? '').trim();
  final lower = name.toLowerCase();
  if (!lower.startsWith('código ') || code.isEmpty) return name;
  final label = t.board.panel_cnes_code(code: code);
  return group.isEmpty ? label : '$label ($group)';
}

final leadParkProvider =
    FutureProvider.autoDispose.family<LeadPark?, String>((ref, leadId) async {
  final dio = ref.watch(dioProvider);
  final res = await dio.get<Map<String, dynamic>>('/leads/$leadId/cnes-park');
  final body = res.data;
  if (res.statusCode == 204 || body == null) return null;
  final items = (body['park'] as List<dynamic>? ?? const [])
      .whereType<Map<String, dynamic>>()
      .map((p) =>
          '${(p['qt_exist'] as num?)?.toInt() ?? 0}x ${leadParkItemLabel(p)}')
      .where((s) => !s.startsWith('0x'))
      .toList();
  if (items.isEmpty) return null;
  final competencia = (body['competencia'] as String? ?? '').trim();
  return LeadPark(
    cnes: body['cnes'] as String? ?? '',
    tradeName: body['trade_name'] as String? ?? '',
    matchedBy: body['matched_by'] as String? ?? '',
    items: items,
    competencia: competencia.isEmpty ? null : competencia,
    matches: (body['matches'] as List<dynamic>? ?? const <dynamic>[])
        .map(ParkMatch.fromJson)
        .whereType<ParkMatch>()
        .toList(),
  );
});
