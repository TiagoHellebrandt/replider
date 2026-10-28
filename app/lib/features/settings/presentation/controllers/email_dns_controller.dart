import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';

/// Email authentication setup (US-M006 / US-M022).
///
/// A sending domain without SPF, DKIM and DMARC does not deliver "a bit
/// worse": Gmail and Outlook drop it, and the SMTP handoff still succeeds —
/// so the customer's only signal that their outreach is going nowhere is
/// this screen. It therefore shows each record's own status, not a single
/// green/red verdict.

enum DnsRecordStatus { pending, ok, mismatch }

DnsRecordStatus _statusFrom(String? raw) => switch (raw) {
      'ok' => DnsRecordStatus.ok,
      'mismatch' => DnsRecordStatus.mismatch,
      // Unknown or absent reads as pending: DNS that has not propagated
      // looks exactly like DNS that was never published, and calling a
      // correct record "wrong" during that window creates support tickets.
      _ => DnsRecordStatus.pending,
    };

class EmailDnsRecord {
  const EmailDnsRecord({
    required this.kind,
    required this.type,
    required this.host,
    required this.hostShort,
    required this.value,
    required this.required_,
    required this.purpose,
    required this.status,
    this.found = '',
  });

  final String kind;
  final String type;

  /// Registrars disagree on whether they want the FQDN or just the
  /// subdomain — the single most common setup mistake — so both are shown.
  final String host;
  final String hostShort;

  final String value;
  final bool required_;
  final String purpose;
  final DnsRecordStatus status;

  /// What DNS actually returned, so a mismatch is debuggable without
  /// leaving the screen.
  final String found;

  factory EmailDnsRecord.fromJson(Map<String, dynamic> json) => EmailDnsRecord(
        kind: json['kind'] as String? ?? '',
        type: json['type'] as String? ?? 'TXT',
        host: json['host'] as String? ?? '',
        hostShort: json['host_short'] as String? ?? '',
        value: json['value'] as String? ?? '',
        required_: json['required'] as bool? ?? false,
        purpose: json['purpose'] as String? ?? '',
        status: _statusFrom(json['status'] as String?),
        found: json['found'] as String? ?? '',
      );
}

class EmailDnsSetup {
  const EmailDnsSetup({
    required this.domain,
    required this.records,
    required this.verified,
    this.nameserverHint = '',
    this.providerManaged = false,
  });

  final String domain;
  final List<EmailDnsRecord> records;

  /// True only when every REQUIRED record checks out — the same flag the
  /// send caps read server-side.
  final bool verified;

  final String nameserverHint;

  /// The mailbox provider authenticates the domain itself (a personal
  /// @gmail.com): nothing for the customer to publish, and the card must say
  /// so instead of demanding edits to a zone they could never touch.
  final bool providerManaged;

  factory EmailDnsSetup.fromJson(Map<String, dynamic> json) => EmailDnsSetup(
        domain: json['domain'] as String? ?? '',
        records: (json['records'] as List<dynamic>? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(EmailDnsRecord.fromJson)
            .toList(),
        verified: json['verified'] as bool? ?? false,
        nameserverHint: json['nameserver_hint'] as String? ?? '',
        providerManaged: json['provider_managed'] as bool? ?? false,
      );
}

/// Live check. Each read hits DNS server-side, so the screen calls it on
/// open and on an explicit "check again" — never on a timer the user did
/// not ask for.
final emailDnsProvider =
    FutureProvider.autoDispose.family<EmailDnsSetup, String>(
  (ref, channelId) async {
    final dio = ref.watch(dioProvider);
    final res =
        await dio.get<Map<String, dynamic>>('/channels/$channelId/email-dns');
    return EmailDnsSetup.fromJson(res.data ?? const <String, dynamic>{});
  },
);

/// Where a customer actually goes to publish these records.
///
/// The nameserver hint the server computes is worth nothing on its own — a
/// slug does not tell anyone which menu to open. This maps it to the exact
/// path through that provider's panel, because "add a TXT record" means a
/// different sequence of clicks in Registro.br than in Cloudflare, and the
/// gap between those is where people give up.
class DnsProviderGuide {
  const DnsProviderGuide({
    required this.slug,
    required this.name,
    required this.url,
    this.canAutoConfigure = false,
  });

  final String slug;
  final String name;

  /// The provider's own DNS documentation. Ours would go stale the first
  /// time they redesign the panel; theirs does not.
  final String url;

  /// Whether we can write the records ourselves given a scoped token.
  final bool canAutoConfigure;
}

const _guides = <String, DnsProviderGuide>{
  'cloudflare': DnsProviderGuide(
    slug: 'cloudflare',
    name: 'Cloudflare',
    url:
        'https://developers.cloudflare.com/dns/manage-dns-records/how-to/create-dns-records/',
    canAutoConfigure: true,
  ),
  'registrobr': DnsProviderGuide(
    slug: 'registrobr',
    name: 'Registro.br',
    url: 'https://registro.br/tecnologia/faq/dns/',
  ),
  'godaddy': DnsProviderGuide(
    slug: 'godaddy',
    name: 'GoDaddy',
    url: 'https://www.godaddy.com/help/manage-dns-records-680',
  ),
  'hostinger': DnsProviderGuide(
    slug: 'hostinger',
    name: 'Hostinger',
    url:
        'https://support.hostinger.com/en/articles/1583227-how-to-manage-dns-records-at-hostinger',
  ),
  'hostgator': DnsProviderGuide(
    slug: 'hostgator',
    name: 'HostGator',
    url: 'https://suporte.hostgator.com.br/hc/pt-br/articles/115000505334',
  ),
  'locaweb': DnsProviderGuide(
    slug: 'locaweb',
    name: 'Locaweb',
    url: 'https://www.locaweb.com.br/ajuda/wiki/como-configurar-registros-dns/',
  ),
  'uolhost': DnsProviderGuide(
    slug: 'uolhost',
    name: 'UOL Host',
    url: 'https://ajuda.uolhost.uol.com.br/',
  ),
  'kinghost': DnsProviderGuide(
    slug: 'kinghost',
    name: 'KingHost',
    url: 'https://king.host/wiki/artigo/gerenciamento-de-zona-dns/',
  ),
  'umbler': DnsProviderGuide(
    slug: 'umbler',
    name: 'Umbler',
    url: 'https://help.umbler.com/hc/pt-br/articles/218906143',
  ),
  'weblink': DnsProviderGuide(
    slug: 'weblink',
    name: 'Weblink',
    url: 'https://www.weblink.com.br/blog/ajuda/',
  ),
  'redehost': DnsProviderGuide(
    slug: 'redehost',
    name: 'RedeHost',
    url: 'https://www.redehost.com.br/central-de-ajuda',
  ),
  'hostnet': DnsProviderGuide(
    slug: 'hostnet',
    name: 'Hostnet',
    url: 'https://www.hostnet.com.br/info/',
  ),
  'wix': DnsProviderGuide(
    slug: 'wix',
    name: 'Wix',
    url:
        'https://support.wix.com/pt/article/adicionando-ou-atualizando-registros-txt-no-seu-painel-de-controle-wix',
  ),
  'route53': DnsProviderGuide(
    slug: 'route53',
    name: 'Amazon Route 53',
    url:
        'https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-creating.html',
  ),
  'googledomains': DnsProviderGuide(
    slug: 'googledomains',
    name: 'Google Domains / Squarespace',
    url: 'https://support.google.com/domains/answer/3290350',
  ),
  'azure': DnsProviderGuide(
    slug: 'azure',
    name: 'Azure DNS',
    url:
        'https://learn.microsoft.com/azure/dns/dns-operations-recordsets-portal',
  ),
};

/// Returns the guide for a detected provider, or null when we do not know.
///
/// Null is a normal outcome, not a failure: an unrecognised nameserver still
/// gets the records and the copy buttons, which is everything needed. Showing
/// a wrong provider's instructions would be worse than showing none.
DnsProviderGuide? dnsGuideFor(String slug) => _guides[slug];

/// One-click publish.
final dnsApplyActionsProvider = Provider<DnsApplyActions>(
  (ref) => DnsApplyActions(ref.watch(dioProvider)),
);

class DnsApplyActions {
  const DnsApplyActions(this._dio);

  final Dio _dio;

  /// Publishes the records into the customer's own zone.
  ///
  /// The token goes straight to the server and is never stored — not here,
  /// not there. It authorises editing a company's DNS, which is close enough
  /// to their identity that holding on to it would be a liability with no
  /// matching benefit.
  Future<DnsApplyReport> apply({
    required String channelId,
    required String provider,
    required String apiToken,
  }) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/channels/$channelId/email-dns/apply',
      data: <String, dynamic>{'provider': provider, 'api_token': apiToken},
    );
    return DnsApplyReport.fromJson(res.data ?? const <String, dynamic>{});
  }
}

class DnsApplyResult {
  const DnsApplyResult({
    required this.kind,
    required this.action,
    this.detail = '',
  });

  final String kind;

  /// `created`, `updated`, `merged`, `kept` or `failed`. The interesting
  /// ones are `merged` and `kept` — they mean we found something of the
  /// customer's and did not destroy it.
  final String action;
  final String detail;

  bool get failed => action == 'failed';

  factory DnsApplyResult.fromJson(Map<String, dynamic> json) => DnsApplyResult(
        kind: json['kind'] as String? ?? '',
        action: json['action'] as String? ?? '',
        detail: json['detail'] as String? ?? '',
      );
}

class DnsApplyReport {
  const DnsApplyReport({required this.zone, required this.results});

  final String zone;
  final List<DnsApplyResult> results;

  bool get anyFailed => results.any((r) => r.failed);

  factory DnsApplyReport.fromJson(Map<String, dynamic> json) => DnsApplyReport(
        zone: json['zone'] as String? ?? '',
        results: (json['results'] as List<dynamic>? ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(DnsApplyResult.fromJson)
            .toList(),
      );
}
