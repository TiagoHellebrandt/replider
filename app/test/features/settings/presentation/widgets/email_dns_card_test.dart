import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/settings/presentation/controllers/email_dns_controller.dart';
import 'package:app/features/settings/presentation/widgets/email_dns_card.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// US-M006/US-M022. Unauthenticated mail fails INVISIBLY — the SMTP handoff
// succeeds and Gmail drops the message — so this card is the customer's only
// signal. It has to be per-record (which one is missing?) and show what is
// actually published when a record is wrong.

EmailDnsRecord _record({
  required String kind,
  DnsRecordStatus status = DnsRecordStatus.pending,
  bool required_ = true,
  String found = '',
}) =>
    EmailDnsRecord(
      kind: kind,
      type: kind == 'dkim' ? 'CNAME' : 'TXT',
      host: kind == 'spf' ? 'clinica.com.br' : '_$kind.clinica.com.br',
      hostShort: kind == 'spf' ? '@' : '_$kind',
      value: 'v=${kind}1 something',
      required_: required_,
      purpose: 'Autoriza o envio em nome do domínio.',
      status: status,
      found: found,
    );

Future<void> _pump(WidgetTester tester, EmailDnsSetup setup) async {
  tester.view.physicalSize = const Size(900, 1600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(ProviderScope(
    overrides: [
      emailDnsProvider('chan-1').overrideWith((ref) async => setup),
      dioProvider.overrideWithValue(Dio()),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: const Scaffold(body: EmailDnsCard(channelId: 'chan-1')),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('an unauthenticated domain says mail is being dropped',
      (tester) async {
    await _pump(
      tester,
      EmailDnsSetup(
        domain: 'clinica.com.br',
        verified: false,
        records: [
          _record(kind: 'spf', status: DnsRecordStatus.ok),
          _record(kind: 'dkim'),
          _record(kind: 'dmarc'),
        ],
      ),
    );

    expect(
      tester.widget<Text>(find.byKey(const Key('email_dns_headline'))).data,
      t.channels.email_dns_pending,
    );
    // Per record, not one verdict: the customer has to know WHICH one.
    expect(find.byKey(const Key('email_dns_record_spf')), findsOneWidget);
    expect(find.byKey(const Key('email_dns_record_dkim')), findsOneWidget);
    expect(find.text('clinica.com.br'), findsWidgets);
  });

  testWidgets('every required record live flips the headline', (tester) async {
    await _pump(
      tester,
      EmailDnsSetup(
        domain: 'clinica.com.br',
        verified: true,
        records: [
          _record(kind: 'spf', status: DnsRecordStatus.ok),
          _record(kind: 'dkim', status: DnsRecordStatus.ok),
          _record(kind: 'dmarc', status: DnsRecordStatus.ok),
        ],
      ),
    );

    expect(
      tester.widget<Text>(find.byKey(const Key('email_dns_headline'))).data,
      t.channels.email_dns_verified,
    );
  });

  // A record that is published but wrong is only fixable if you can see what
  // is out there — otherwise the user re-pastes the same correct value.
  testWidgets('a mismatch shows what DNS actually returns', (tester) async {
    await _pump(
      tester,
      EmailDnsSetup(
        domain: 'clinica.com.br',
        verified: false,
        records: [
          _record(
            kind: 'spf',
            status: DnsRecordStatus.mismatch,
            found: 'v=spf1 include:outro-provedor.com ~all',
          ),
        ],
      ),
    );

    expect(find.byKey(const Key('email_dns_found_spf')), findsOneWidget);
    expect(find.textContaining('outro-provedor.com'), findsOneWidget);
  });

  testWidgets('optional records are marked so, required ones are not',
      (tester) async {
    await _pump(
      tester,
      EmailDnsSetup(
        domain: 'clinica.com.br',
        verified: false,
        records: [
          _record(kind: 'spf'),
          _record(kind: 'tracking', required_: false),
        ],
      ),
    );

    expect(find.text(t.channels.email_dns_optional), findsOneWidget);
  });

  testWidgets('a failed check offers a retry instead of a blank card',
      (tester) async {
    await tester.pumpWidget(ProviderScope(
      overrides: [
        emailDnsProvider('chan-1')
            .overrideWith((ref) async => throw Exception('boom')),
        dioProvider.overrideWithValue(Dio()),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: EmailDnsCard(channelId: 'chan-1')),
      ),
    ));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('email_dns_error')), findsOneWidget);
    expect(find.text(t.common.retry), findsOneWidget);
  });

  // ── The provider guide and the one-click publish ─────────────────────────
  //
  // Four records pasted by hand into a registrar panel is where email
  // onboarding dies. These pin the two things that stop it.

  // A slug is worth nothing on its own — it does not tell anyone which menu
  // to open.
  testWidgets('names the detected DNS provider and links its guide',
      (tester) async {
    await _pump(
        tester,
        EmailDnsSetup(
          domain: 'clinica.com.br',
          records: [_record(kind: 'spf')],
          verified: false,
          nameserverHint: 'cloudflare',
        ));

    expect(find.byKey(const Key('email_dns_provider')), findsOneWidget);
    expect(find.byKey(const Key('email_dns_guide')), findsOneWidget);
  });

  // Showing the WRONG provider's instructions would be worse than showing
  // none: the records and copy buttons are already everything needed.
  // An unrecognised provider is a NORMAL case for this audience —
  // professional domains live wherever the web agency put them. It must get
  // the generic walkthrough (which panel to open, what to do with the
  // records), never a silent nothing, and never a wrong provider's guide.
  testWidgets('an unknown provider gets the generic walkthrough',
      (tester) async {
    await _pump(
        tester,
        EmailDnsSetup(
          domain: 'clinica.com.br',
          records: [_record(kind: 'spf')],
          verified: false,
          nameserverHint: 'algum-registrador-obscuro',
        ));

    expect(find.byKey(const Key('email_dns_provider')), findsNothing);
    expect(find.byKey(const Key('email_dns_auto')), findsNothing);
    expect(find.byKey(const Key('email_dns_generic_help')), findsOneWidget);
    expect(find.text(t.channels.email_dns_generic_steps), findsOneWidget);
    // The what-to-do line renders for everyone, known provider or not.
    expect(find.byKey(const Key('email_dns_how_to')), findsOneWidget);
  });

  testWidgets('offers one-click where we can write the zone', (tester) async {
    await _pump(
        tester,
        EmailDnsSetup(
          domain: 'clinica.com.br',
          records: [_record(kind: 'spf')],
          verified: false,
          nameserverHint: 'cloudflare',
        ));

    expect(find.byKey(const Key('email_dns_auto')), findsOneWidget);
  });

  // Registro.br has no API we can write through. Offering a button that
  // cannot work is worse than offering the guide alone.
  testWidgets('offers only the guide where we cannot', (tester) async {
    await _pump(
        tester,
        EmailDnsSetup(
          domain: 'clinica.com.br',
          records: [_record(kind: 'spf')],
          verified: false,
          nameserverHint: 'registrobr',
        ));

    expect(find.byKey(const Key('email_dns_guide')), findsOneWidget);
    expect(find.byKey(const Key('email_dns_auto')), findsNothing);
  });

  // Nothing left to do, nothing to offer.
  testWidgets('a verified domain shows no setup help', (tester) async {
    await _pump(
        tester,
        EmailDnsSetup(
          domain: 'clinica.com.br',
          records: [_record(kind: 'spf', status: DnsRecordStatus.ok)],
          verified: true,
          nameserverHint: 'cloudflare',
        ));

    expect(find.byKey(const Key('email_dns_provider')), findsNothing);
    expect(find.byKey(const Key('email_dns_auto')), findsNothing);
  });

  // Asking someone to paste a credential that can redirect their company's
  // mail without saying what happens to it is not a request anyone should
  // grant — so the dialog says it is used once and never stored.
  testWidgets('the token prompt says the token is not kept', (tester) async {
    await _pump(
        tester,
        EmailDnsSetup(
          domain: 'clinica.com.br',
          records: [_record(kind: 'spf')],
          verified: false,
          nameserverHint: 'cloudflare',
        ));

    await tester.tap(find.byKey(const Key('email_dns_auto')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('email_dns_token_field')), findsOneWidget);
    expect(find.text(t.channels.email_dns_token_help(provider: 'Cloudflare')),
        findsOneWidget);
  });

  // The guide maps a slug to that provider's OWN documentation — ours would
  // go stale the first time they redesign the panel.
  test('every known provider maps to a guide', () {
    for (final slug in ['cloudflare', 'registrobr', 'godaddy', 'hostinger']) {
      final guide = dnsGuideFor(slug);
      expect(guide, isNotNull, reason: slug);
      expect(guide!.url, startsWith('https://'));
      expect(guide.name, isNotEmpty);
    }
    expect(dnsGuideFor('nada'), isNull);
  });

  // Only Cloudflare can be written to today, and claiming otherwise would
  // put a button in front of a customer that cannot work.
  test('only providers with a real API offer auto-configuration', () {
    expect(dnsGuideFor('cloudflare')!.canAutoConfigure, isTrue);
    expect(dnsGuideFor('registrobr')!.canAutoConfigure, isFalse);
    expect(dnsGuideFor('godaddy')!.canAutoConfigure, isFalse);
  });

  // ── Self-checking while the user waits ───────────────────────────────────

  // DNS propagation is minutes, and the user has just come back from their
  // registrar. Making them hunt for a refresh button while they wait is how
  // a correct setup gets abandoned as broken.
  testWidgets('re-checks itself while the domain is still pending',
      (tester) async {
    var reads = 0;
    await tester.pumpWidget(ProviderScope(
      overrides: [
        emailDnsProvider('chan-1').overrideWith((ref) async {
          reads++;
          return EmailDnsSetup(
            domain: 'clinica.com.br',
            records: [_record(kind: 'spf')],
            verified: false,
          );
        }),
        dioProvider.overrideWithValue(Dio()),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: EmailDnsCard(channelId: 'chan-1')),
      ),
    ));
    await tester.pump();
    expect(reads, 1);

    await tester.pump(const Duration(seconds: 21));
    await tester.pump();

    expect(reads, greaterThan(1));
  });

  // And stops the moment it goes green — a poll that never ends keeps a DNS
  // lookup running server-side for every tab someone left open.
  testWidgets('stops re-checking once verified', (tester) async {
    var reads = 0;
    await tester.pumpWidget(ProviderScope(
      overrides: [
        emailDnsProvider('chan-1').overrideWith((ref) async {
          reads++;
          return EmailDnsSetup(
            domain: 'clinica.com.br',
            records: [_record(kind: 'spf', status: DnsRecordStatus.ok)],
            verified: true,
          );
        }),
        dioProvider.overrideWithValue(Dio()),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: EmailDnsCard(channelId: 'chan-1')),
      ),
    ));
    await tester.pump();
    final afterFirst = reads;

    await tester.pump(const Duration(seconds: 60));
    await tester.pump();

    expect(reads, afterFirst);
  });

  // A support thread that starts with "meu email não funciona" takes three
  // round trips to get going. The card knows the domain, the provider and
  // exactly which records are missing.
  testWidgets('offers help with the problem already described', (tester) async {
    await _pump(
        tester,
        EmailDnsSetup(
          domain: 'clinica.com.br',
          records: [_record(kind: 'spf'), _record(kind: 'dkim')],
          verified: false,
          nameserverHint: 'registrobr',
        ));

    expect(find.byKey(const Key('email_dns_help')), findsOneWidget);
  });

  // Nothing to ask about.
  testWidgets('a verified domain does not offer help', (tester) async {
    await _pump(
        tester,
        EmailDnsSetup(
          domain: 'clinica.com.br',
          records: [_record(kind: 'spf', status: DnsRecordStatus.ok)],
          verified: true,
        ));

    expect(find.byKey(const Key('email_dns_help')), findsNothing);
  });
  // A personal @gmail.com has nothing to publish — Google authenticates its
  // own domain. The card used to demand DNS edits on a zone the customer
  // could never touch.
  testWidgets('a provider-managed setup says Google handles it',
      (tester) async {
    await _pump(
      tester,
      const EmailDnsSetup(
        domain: 'gmail.com',
        verified: true,
        providerManaged: true,
        records: [],
      ),
    );

    expect(
      tester.widget<Text>(find.byKey(const Key('email_dns_headline'))).data,
      t.channels.email_dns_provider_managed,
    );
  });

  // The DMARC-with-a-check confusion: a green row still worded as an
  // instruction reads as "not done yet". A done row must say it is done.
  testWidgets('a green record is labeled already published', (tester) async {
    await _pump(
      tester,
      EmailDnsSetup(
        domain: 'clinica.com.br',
        verified: false,
        records: [
          _record(kind: 'spf', status: DnsRecordStatus.ok),
          _record(kind: 'dmarc'),
        ],
      ),
    );

    expect(find.byKey(const Key('email_dns_done_spf')), findsOneWidget);
    expect(find.byKey(const Key('email_dns_done_dmarc')), findsNothing);
    expect(find.text(t.channels.email_dns_already_published), findsOneWidget);
  });
}
