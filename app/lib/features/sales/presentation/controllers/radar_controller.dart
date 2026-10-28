import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/sales/presentation/controllers/lead_detail_controller.dart'
    show ParkMatch;

/// The opportunity radar (Ofertas phase 2): one feed over everything the
/// system detected — tenders, CNES park hits, reorders due, relationship
/// dates. Items are derived server-side; a failed collector arrives as a
/// named warning instead of an empty feed.
enum RadarItemType {
  licitacao,
  parque,
  recompra,
  data,
  crossSell,
  // The two forward-looking kinds. Everything above reacts to something that
  // already happened; these are the only rows that exist BEFORE the purchase.
  contrato,
  ata,
}

RadarItemType? _typeFrom(String? raw) {
  if (raw == 'cross_sell') return RadarItemType.crossSell;
  for (final t in RadarItemType.values) {
    if (t.name == raw) return t;
  }
  return null;
}

/// What this account IS to a buyer — graded by what was actually DONE.
///
/// Two versions of this shipped wrong, in production, one after the other. The
/// first read a contact row as a sale; the second read the same row as an
/// approach ("você já abordou") on an account with no channel connected and
/// not one message ever sent. Both were the same mistake: this product creates
/// a contact and a lead the moment it spots an opportunity, so neither is
/// evidence that anybody was contacted.
///
/// Every value below is backed by a record of an event, never by the existence
/// of a row we made ourselves.
enum BuyerRelationship {
  /// Not in the funnel at all.
  cold,

  /// A card exists and nothing is written. Says "don't open a second card",
  /// not "you talked to them".
  listed,

  /// An offer is written and waiting on a human. Still nothing sent.
  queued,

  /// An offer actually went out.
  contacted,

  /// A sale is on record.
  sold,
}

/// What this account already knows about a buyer, and what the published
/// record says they have contracted.
///
/// Two questions decide how a seller opens: do these people know my name, and
/// who is already selling to them. Both were answerable from data already in
/// the database and no screen had ever asked.
class BuyerContext {
  const BuyerContext({
    this.relationship = BuyerRelationship.cold,
    this.contactId = '',
    this.name = '',
    this.contracts = 0,
    this.lastSupplier = '',
    this.lastObject = '',
    this.lastContractAt,
    this.scanned = false,
    this.leadId = '',
    this.stage = '',
    this.historyVia = '',
  });

  final BuyerRelationship relationship;
  final String contactId;
  final String name;

  /// Published contracts for this buyer THAT ARE IN OUR TRADE. Unfiltered it
  /// named a marquee-rental firm as the incumbent to beat; wrong competitive
  /// intelligence is worse than none, because the seller repeats it.
  final int contracts;

  /// Who won the most recent one in our trade. The only place a competitor's
  /// name may come from is this published record — never a guess off the
  /// installed park, which publishes counts and never a brand.
  final String lastSupplier;
  final String lastObject;
  final DateTime? lastContractAt;

  /// The buyer's contracts were actually read. Without it, "no contract in
  /// your trade" and "we never looked" render identically, and the screen
  /// turns a gap in collection into a claim about a hospital.
  final bool scanned;

  /// The card this buyer already has on the board. Without it "já está no seu
  /// quadro" is a dead end — the seller then hunts a name across a board of
  /// seventy-three cards.
  final String leadId;

  /// The Kanban column that card sits in. "In the funnel" is not a position:
  /// a card in Ganhos and one nobody has touched are both in the funnel and
  /// call for opposite next moves. Empty renders as nothing, never a guess.
  final String stage;

  /// Whose published record the history below actually is, when it is NOT
  /// this hospital's.
  ///
  /// A public hospital rarely signs anything — its secretaria does, and 11 of
  /// Mato Grosso's 20 biggest parks publish only the secretaria's CNPJ.
  /// Reading that record is the only way to know who already supplies the
  /// place; calling it "este comprador já contratou" would be false, and
  /// would read identically on the seven hospitals that share one secretaria.
  final String historyVia;

  bool get hasHistory => contracts > 0;

  /// There is a card to open.
  bool get isOnBoard => leadId.isNotEmpty;

  static BuyerContext? fromJson(dynamic raw) {
    if (raw is! Map<String, dynamic>) return null;
    return BuyerContext(
      relationship: switch (raw['relationship']) {
        'sold' => BuyerRelationship.sold,
        'contacted' => BuyerRelationship.contacted,
        'queued' => BuyerRelationship.queued,
        'listed' => BuyerRelationship.listed,
        _ => BuyerRelationship.cold,
      },
      contactId: raw['contact_id'] as String? ?? '',
      name: raw['name'] as String? ?? '',
      contracts: (raw['contracts'] as num?)?.toInt() ?? 0,
      lastSupplier: raw['last_supplier'] as String? ?? '',
      lastObject: raw['last_object'] as String? ?? '',
      lastContractAt: raw['last_contract_at'] is String
          ? DateTime.tryParse(raw['last_contract_at'] as String)?.toLocal()
          : null,
      scanned: raw['scanned'] == true,
      leadId: raw['lead_id'] as String? ?? '',
      stage: raw['stage'] as String? ?? '',
      historyVia: raw['history_via'] as String? ?? '',
    );
  }
}

/// Why a row can be reached ahead of the market — and the two values are
/// different claims, not degrees of one. A contract is reached before an
/// edital exists; an ata needs no edital at all. One badge for both would
/// print "chegue antes do edital" on a row where no edital is coming.
enum RadarEdge { beforeTender, noTender }

RadarEdge? _edgeFrom(String? raw) => switch (raw) {
      'antes_do_edital' => RadarEdge.beforeTender,
      'sem_licitacao' => RadarEdge.noTender,
      _ => null,
    };

class RadarItem {
  const RadarItem({
    required this.type,
    required this.id,
    required this.title,
    required this.context,
    this.valueCents = 0,
    this.deadline,
    this.leadId,
    this.contactId,
    this.cnes,
    this.url,
    this.competencia,
    this.matches = const [],
    this.city = '',
    this.uf = '',
    this.incumbent = '',
    this.edge,
    this.ready = false,
    this.detail = '',
    this.cnpj = '',
    this.reference = '',
    this.publishedAt,
    this.buyer,
    this.phone = '',
    this.email = '',
    this.address = '',
    this.buyerEntityName = '',
    this.beds = const [],
    this.habilitations = const [],
    this.services = const [],
  });

  final RadarItemType type;
  final String id;
  final String title;
  final String context;

  /// Zero means "not disclosed", never "zero reais".
  final int valueCents;
  final DateTime? deadline;

  /// Set when the auto-offer already drafted this tender — the CTA is then
  /// "open the draft in the queue", not "prepare".
  final String? leadId;
  final String? contactId;
  final String? cnes;
  final String? url;

  /// What of the catalogue fits what this establishment declares. The card
  /// answers "what do I offer them" without opening anything.
  final List<ParkMatch> matches;

  /// The DATASUS month a park item's counts come from ("2026-07"). In
  /// September the data is July — the card says so instead of implying
  /// "today".
  final String? competencia;

  final String city;
  final String uf;

  /// Who holds the expiring commitment today, copied verbatim from the
  /// published record. Empty when the record names nobody, which happens and
  /// which the card says out loud — silence would read as "nobody supplies
  /// them", a far more encouraging claim than the record makes.
  final String incumbent;

  /// Set only on the two forward-looking kinds.
  final RadarEdge? edge;

  /// The offer is already written and waiting on a human, so the card's verb
  /// is "read it and send" rather than "prepare an offer". This is what let
  /// the approval queue stop being a tab of its own.
  final bool ready;

  /// The source's own text, whole. [context] is the composed one-liner the
  /// card shows and truncates — this is what the panel reads.
  ///
  /// The truncation is not cosmetic: a tender for "PEÇAS DE REPOSIÇÃO
  /// ORIGINAIS PARA ATENDER ÀS NECESSIDADES DO LABORATÓRIO MUNICIPAL" showed
  /// up to the word "NECESSIDADES", and on that evidence it read as out of
  /// trade.
  final String detail;

  /// The buyer's CNPJ. A public notice publishes no contact, so this is the
  /// thread a seller actually pulls: the registry, the phone, who already
  /// sells to them.
  final String cnpj;

  /// The source's own identifier, shown verbatim so any claim built on it can
  /// be checked — and, for the quarter of tenders that carry no link at all,
  /// the only way to find the notice.
  final String reference;

  /// How the seller actually reaches this buyer. A published notice carries
  /// none of the three, which is why the panel used to end at the control
  /// number; the health registry carries them for most of the territory, and
  /// the radar was reading that record already and dropping all of it.
  final String phone;
  final String email;
  final String address;

  /// Who signs the contracts for this establishment, when it has no CNPJ of
  /// its own. Deliberately not the hospital: seven of MT's twenty biggest
  /// parks are maintained by the same secretaria.
  final String buyerEntityName;

  /// What this hospital is BUILT to treat, beside what it owns.
  ///
  /// The park says which machines are in the building; the bed census says how
  /// much care the place is set up to deliver, and that is what sizes an
  /// order. Twenty neonatal cots means twenty incubators and the monitors to
  /// match — six incubators in the park does not contradict that, it measures
  /// the gap.
  final List<BedCount> beds;

  /// Neonatal beds, the segment this account's catalogue is built for.
  int get neonatalBeds =>
      beds.where((b) => b.isNeonatal).fold(0, (a, b) => a + b.qtExist);

  /// Intensive-care beds of any age group.
  int get icuBeds =>
      beds.where((b) => b.isIcu).fold(0, (a, b) => a + b.qtExist);

  /// Capability the MINISTRY FUNDS — a harder signal than a declared bed. A
  /// unit habilitated for "UTI II Neonatal" is paid against an ordinance and
  /// is required to hold the equipment that entitles it.
  final List<Habilitation> habilitations;

  /// What the establishment says it PERFORMS — the qualifier that separates
  /// a hospital running a neonatal service from one that has a cot.
  final List<CnesService> services;

  /// When the source published it. The other half of the clock: a deadline in
  /// five days reads differently if the notice went up yesterday or three
  /// weeks ago.
  final DateTime? publishedAt;

  /// What we already know about whoever is buying. Null when the source names
  /// no CNPJ, or when there is genuinely nothing to say — an empty section
  /// reads as a failure, an absent one reads as nothing to report.
  final BuyerContext? buyer;

  /// "Cuiabá/MT", or whichever half the record actually carries.
  String get location {
    if (city.isEmpty) return uf;
    if (uf.isEmpty) return city;
    return '$city/$uf';
  }

  /// The PNCP control number behind a contract/ata row, for the action that
  /// turns it into a card in the funnel. Empty on every other kind.
  String get pncpId =>
      id.startsWith('contract-') ? id.substring('contract-'.length) : '';

  static RadarItem? fromJson(Map<String, dynamic> json) {
    final type = _typeFrom(json['type'] as String?);
    // An unknown type from a newer server renders as nothing rather than a
    // broken card claiming to be something it is not.
    if (type == null) return null;
    return RadarItem(
      type: type,
      id: json['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      context: json['context'] as String? ?? '',
      valueCents: (json['value_cents'] as num?)?.toInt() ?? 0,
      deadline: json['deadline'] is String
          ? DateTime.tryParse(json['deadline'] as String)?.toLocal()
          : null,
      leadId: _str(json['lead_id']),
      contactId: _str(json['contact_id']),
      cnes: _str(json['cnes']),
      url: _str(json['url']),
      competencia: _str(json['competencia']),
      matches: (json['matches'] as List<dynamic>? ?? const <dynamic>[])
          .map(ParkMatch.fromJson)
          .whereType<ParkMatch>()
          .toList(),
      city: json['city'] as String? ?? '',
      uf: json['uf'] as String? ?? '',
      incumbent: json['incumbent'] as String? ?? '',
      edge: _edgeFrom(json['edge'] as String?),
      ready: json['ready'] == true,
      detail: json['detail'] as String? ?? '',
      cnpj: json['cnpj'] as String? ?? '',
      reference: json['reference'] as String? ?? '',
      publishedAt: json['published_at'] is String
          ? DateTime.tryParse(json['published_at'] as String)?.toLocal()
          : null,
      buyer: BuyerContext.fromJson(json['buyer']),
      phone: json['phone'] as String? ?? '',
      email: json['email'] as String? ?? '',
      address: json['address'] as String? ?? '',
      buyerEntityName: json['buyer_entity_name'] as String? ?? '',
      beds: (json['beds'] as List<dynamic>? ?? const <dynamic>[])
          .map(BedCount.fromJson)
          .whereType<BedCount>()
          .toList(),
      habilitations:
          (json['habilitations'] as List<dynamic>? ?? const <dynamic>[])
              .map(Habilitation.fromJson)
              .whereType<Habilitation>()
              .toList(),
      services: (json['services'] as List<dynamic>? ?? const <dynamic>[])
          .map(CnesService.fromJson)
          .whereType<CnesService>()
          .toList(),
    );
  }

  static String? _str(dynamic v) {
    if (v is String && v.trim().isNotEmpty) return v.trim();
    return null;
  }
}

/// One line of a hospital's bed census, as the ministry's own dictionary
/// names it.
///
/// `isIcu` and `isNeonatal` ride ON the row rather than being re-derived here,
/// because deriving them is what went wrong twice while this was built: bed
/// TYPE 4 reads as intensive care and means obstetric, and code 86 reads as
/// neonatal and is coronary intensive care. The dictionary ships with the data
/// and answers both; a constant in the client drifts from it.
class BedCount {
  const BedCount({
    required this.bedCode,
    required this.name,
    required this.qtExist,
    this.qtSus = 0,
    this.isIcu = false,
    this.isNeonatal = false,
  });

  final String bedCode;
  final String name;
  final int qtExist;
  final int qtSus;
  final bool isIcu;
  final bool isNeonatal;

  static BedCount? fromJson(dynamic raw) {
    if (raw is! Map<String, dynamic>) return null;
    final name = (raw['name'] as String? ?? '').trim();
    if (name.isEmpty) return null;
    return BedCount(
      bedCode: raw['bed_code'] as String? ?? '',
      name: name,
      qtExist: (raw['qt_exist'] as num?)?.toInt() ?? 0,
      qtSus: (raw['qt_sus'] as num?)?.toInt() ?? 0,
      isIcu: raw['is_icu'] == true,
      isNeonatal: raw['is_neonatal'] == true,
    );
  }
}

/// One capability the Ministry funds, against a named ordinance.
///
/// Zero beds is common and must never read as "no capacity": plenty of
/// habilitations are not bed-based at all.
class Habilitation {
  const Habilitation({
    required this.code,
    required this.name,
    this.beds = 0,
    this.isIcu = false,
    this.isNeonatal = false,
  });

  final String code;
  final String name;
  final int beds;
  final bool isIcu;
  final bool isNeonatal;

  static Habilitation? fromJson(dynamic raw) {
    if (raw is! Map<String, dynamic>) return null;
    final name = (raw['name'] as String? ?? '').trim();
    if (name.isEmpty) return null;
    return Habilitation(
      code: raw['code'] as String? ?? '',
      name: name,
      beds: (raw['beds'] as num?)?.toInt() ?? 0,
      isIcu: raw['is_icu'] == true,
      isNeonatal: raw['is_neonatal'] == true,
    );
  }
}

/// One specialized service the establishment declares it performs.
class CnesService {
  const CnesService({required this.code, required this.name, this.sus = false});

  final String code;
  final String name;

  /// Whether it is offered to SUS patients — which budget pays for it.
  final bool sus;

  static CnesService? fromJson(dynamic raw) {
    if (raw is! Map<String, dynamic>) return null;
    final name = (raw['name'] as String? ?? '').trim();
    if (name.isEmpty) return null;
    return CnesService(
      code: raw['code'] as String? ?? '',
      name: name,
      sus: raw['sus'] == true,
    );
  }
}

/// "2026-07" → "07/2026", the way a Brazilian reads a month. Anything
/// else comes back unchanged — a raw stamp beats a fabricated date.
String competenciaLabel(String competencia) {
  final parts = competencia.split('-');
  if (parts.length != 2 || parts[0].length != 4 || parts[1].length != 2) {
    return competencia;
  }
  return '${parts[1]}/${parts[0]}';
}

/// How big the park opportunity is behind the page of items.
///
/// The screen used to show five hospitals with nothing saying five of WHAT —
/// for a territory holding thousands, that reads as "you have five
/// opportunities". The number that matters is the one being missed.
class RadarParkUniverse {
  const RadarParkUniverse({
    required this.establishments,
    required this.units,
    required this.shown,
    required this.scope,
    this.competencia,
  });

  final int establishments;
  final int units;
  final int shown;
  final String scope;
  final String? competencia;

  /// Whether the page is a sample of something bigger.
  bool get hasMore => establishments > shown;

  static RadarParkUniverse? fromJson(dynamic raw) {
    if (raw is! Map<String, dynamic>) return null;
    final estabs = (raw['establishments'] as num?)?.toInt() ?? 0;
    if (estabs <= 0) return null;
    return RadarParkUniverse(
      establishments: estabs,
      units: (raw['units'] as num?)?.toInt() ?? 0,
      shown: (raw['shown'] as num?)?.toInt() ?? 0,
      scope: (raw['scope'] as String? ?? '').trim(),
      competencia: RadarItem._str(raw['competencia']),
    );
  }
}

class RadarFeed {
  const RadarFeed({
    required this.items,
    required this.warnings,
    this.park,
  });
  final List<RadarItem> items;

  /// The size of the park universe, when the server measured it.
  final RadarParkUniverse? park;

  /// Collector names that failed server-side — the screen says which slice
  /// is missing instead of silently showing less.
  final List<String> warnings;
}

final radarProvider = FutureProvider.autoDispose<RadarFeed>((ref) async {
  final dio = ref.watch(dioProvider);
  final res = await dio.get<Map<String, dynamic>>('/me/radar');
  final body = res.data ?? const <String, dynamic>{};
  return RadarFeed(
    items: (body['items'] as List<dynamic>? ?? const [])
        .whereType<Map<String, dynamic>>()
        .map(RadarItem.fromJson)
        .whereType<RadarItem>()
        .toList(),
    warnings: (body['warnings'] as List<dynamic>? ?? const [])
        .whereType<String>()
        .toList(),
    park: RadarParkUniverse.fromJson(body['park']),
  );
});

class RadarActions {
  RadarActions(this._ref);
  final Ref _ref;

  /// Turns a CNES park hit into a queued draft. Nothing is sent — the draft
  /// lands in the approval queue like every other approach.
  Future<void> parkOffer(String cnes) async {
    await _ref.read(dioProvider).post<Map<String, dynamic>>(
      '/me/radar/park-offer',
      data: {'cnes': cnes},
    );
  }

  /// "I will chase this one myself": an open tender becomes a card in the
  /// funnel, with no AI draft and nothing sent.
  ///
  /// The door a seller had no button for. Until now the only path from a
  /// tender to a card was the automatic offer, which always spends a draft on
  /// the account's own AI key — so somebody who wanted to work it by hand
  /// could read the notice (and leave the product), open a draft that might
  /// not exist, or dismiss it. An expiring CONTRACT could already be worked by
  /// hand, which made the asymmetry visible on the same screen.
  ///
  /// Idempotent: pressing twice is navigation, not a second card for the same
  /// hospital.
  Future<void> workSignal(String signalId) async {
    await _ref.read(dioProvider).post<Map<String, dynamic>>(
          '/signals/${Uri.encodeComponent(signalId)}/work',
        );
  }

  /// The hunt that needs no words: it reads the account's catalogue, resolves
  /// it against the CNES dictionary and fills the funnel with the biggest
  /// untouched parks in the territory.
  ///
  /// The same sweep the nightly worker runs. It exists as a button because the
  /// alternative was waiting until tomorrow or typing equipment names by hand,
  /// and typing names is the opposite of a product that goes looking.
  ///
  /// Creates leads and nothing else: no approach is written, nothing is sent.
  Future<AutoHuntResult> autoHunt() async {
    final res = await _ref.read(dioProvider).post<Map<String, dynamic>>(
          '/me/territory/auto-hunt',
        );
    return AutoHuntResult.fromJson(res.data ?? const {});
  }

  /// The interactive CNES hunt behind "Abastecer o território".
  Future<TerritoryHuntResult> hunt(String equipment) async {
    final res = await _ref.read(dioProvider).post<Map<String, dynamic>>(
      '/me/territory/hunt',
      data: {'equipment': equipment},
    );
    return TerritoryHuntResult.fromJson(res.data ?? const {});
  }
}

final radarActionsProvider = Provider.autoDispose<RadarActions>(
  RadarActions.new,
);

/// One hunt result: an establishment and how many units of the asked
/// equipment it declares to CNES. A count, never an age.
class TerritoryHolder {
  const TerritoryHolder({
    required this.cnes,
    required this.total,
    this.tradeName,
    this.address,
    this.phone,
    this.alreadyKnown = false,
  });

  final String cnes;
  final int total;
  final String? tradeName;
  final String? address;
  final String? phone;

  /// Already in this account's contacts via a previous park offer — the
  /// seller must not "discover" their own pipeline.
  final bool alreadyKnown;

  factory TerritoryHolder.fromJson(Map<String, dynamic> json) =>
      TerritoryHolder(
        cnes: json['cnes'] as String? ?? '',
        total: (json['total'] as num?)?.toInt() ?? 0,
        tradeName: RadarItem._str(json['trade_name']),
        address: RadarItem._str(json['address']),
        phone: RadarItem._str(json['phone']),
        alreadyKnown: json['already_known'] == true,
      );
}

class TerritoryHuntResult {
  const TerritoryHuntResult({
    required this.uf,
    required this.holders,
    this.note,
    this.cities = const [],
    this.competencia,
  });
  final String uf;
  final List<TerritoryHolder> holders;

  /// The territory cities the hunt was narrowed to; empty = the whole UF.
  final List<String> cities;

  /// The DATASUS month the counts come from ("2026-07").
  final String? competencia;

  /// The server's reason for an empty answer — an empty list with no reason
  /// reads as "nobody has it", a claim the data has not earned.
  final String? note;

  factory TerritoryHuntResult.fromJson(Map<String, dynamic> json) =>
      TerritoryHuntResult(
        uf: json['uf'] as String? ?? '',
        holders: (json['holders'] as List<dynamic>? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(TerritoryHolder.fromJson)
            .toList(),
        note: RadarItem._str(json['note']),
        cities: (json['cities'] as List<dynamic>? ?? const [])
            .whereType<String>()
            .toList(),
        competencia: RadarItem._str(json['competencia']),
      );
}

/// What one catalogue hunt produced.
class AutoHuntResult {
  const AutoHuntResult({this.found = 0, this.created = 0});

  /// Untouched establishments the territory offered.
  final int found;

  /// How many became leads. Lower than [found] once the per-run cap bites,
  /// and the difference is not a failure — it is the next run's batch.
  final int created;

  factory AutoHuntResult.fromJson(Map<String, dynamic> json) => AutoHuntResult(
        found: (json['found'] as num?)?.toInt() ?? 0,
        created: (json['created'] as num?)?.toInt() ?? 0,
      );
}
