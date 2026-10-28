import 'package:app/features/sales/presentation/controllers/lead_detail_controller.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter_test/flutter_test.dart';

// The panel can only tell "we searched and found nothing" apart from "nobody
// has looked yet" if the parsing keeps the two facts the server sends for it:
// the stamp of the last research, and the reason the last attempt did no work.

void main() {
  Map<String, dynamic> lead(Map<String, dynamic> enrichment,
          {String status = 'enriched'}) =>
      <String, dynamic>{
        'id': 'lead1',
        'status': status,
        'contact_id': '+5511988770001',
        'contact_address': '+5511988770001',
        'enrichment': enrichment,
      };

  test('a lead nobody researched carries no stamp and no excuse', () {
    final d = LeadDetail.fromJson(lead(<String, dynamic>{}, status: 'new'));

    expect(d.researchedAt, isNull);
    expect(d.researchedNothing, isFalse);
    expect(d.researchBlocked, isFalse);
  });

  test('a research that found nothing is not an untouched lead', () {
    final d = LeadDetail.fromJson(
        lead(<String, dynamic>{'enriched_at': '2026-07-31T10:40:00Z'}));

    expect(d.isEnriched, isFalse, reason: 'no facts came back');
    expect(d.researchedNothing, isTrue);
    // And it shows up in the history, which used to omit the one thing the
    // panel does by itself.
    expect(
        d.timeline.any((e) =>
            e.startsWith(t.lead_detail.timeline_researched(date: '').trim())),
        isTrue);
  });

  test('a blocked attempt says why', () {
    final d = LeadDetail.fromJson(
        lead(<String, dynamic>{'enrich_blocked': 'no_company_or_website'}));

    expect(d.researchBlocked, isTrue);
    expect(d.researchBlockedReason, 'no_company_or_website');
  });

  test('a research in flight is neither blocked nor finished', () {
    final d = LeadDetail.fromJson(lead(
        <String, dynamic>{'enrich_blocked': 'no_company_or_website'},
        status: 'enriching'));

    expect(d.isResearching, isTrue);
    expect(d.researchBlocked, isFalse,
        reason: 'the old excuse is being retried');
    expect(d.researchedNothing, isFalse);
  });
}
