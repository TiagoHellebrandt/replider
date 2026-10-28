import 'package:flutter_test/flutter_test.dart';

import 'package:app/features/sales/domain/models/lead_model.dart';
import 'package:app/i18n/strings.g.dart';

void main() {
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  group('LeadPipelineStatus', () {
    test('fromWire maps every known status and defaults to new', () {
      expect(LeadPipelineStatus.fromWire('prospected'),
          LeadPipelineStatus.prospected);
      expect(LeadPipelineStatus.fromWire('pending_approval'),
          LeadPipelineStatus.pendingApproval);
      expect(LeadPipelineStatus.fromWire('won'), LeadPipelineStatus.won);
      expect(LeadPipelineStatus.fromWire(null), LeadPipelineStatus.newLead);
      expect(
          LeadPipelineStatus.fromWire('garbage'), LeadPipelineStatus.newLead);
    });

    test('wire round-trips and label resolves', () {
      for (final s in LeadPipelineStatus.values) {
        expect(LeadPipelineStatus.fromWire(s.wire), s);
        expect(s.label, isNotEmpty);
      }
    });

    test('isInConversation only for contacted/responded/qualified/won', () {
      expect(LeadPipelineStatus.contacted.isInConversation, isTrue);
      expect(LeadPipelineStatus.responded.isInConversation, isTrue);
      expect(LeadPipelineStatus.qualified.isInConversation, isTrue);
      expect(LeadPipelineStatus.won.isInConversation, isTrue);
      expect(LeadPipelineStatus.prospected.isInConversation, isFalse);
      expect(LeadPipelineStatus.pendingApproval.isInConversation, isFalse);
    });
  });

  group('LeadModel.fromJson', () {
    test('parses enrichment fields and buy-intent flag', () {
      final lead = LeadModel.fromJson({
        'id': 'lead_1',
        'funnel_id': 'fn_1',
        'contact_id': '+5511999999999',
        'status': 'enriched',
        'source': 'google_places',
        'score': 7,
        'approach_draft': 'Olá!',
        'buy_intent_at': '2026-06-09T10:00:00Z',
        'enrichment': {
          'company_name': 'Acme Ltda',
          'decision_maker_name': 'Ana Silva',
          'decision_maker_role': 'CEO',
          'linkedin_url': 'https://linkedin.com/in/ana',
          'instagram_url': 'https://instagram.com/acme',
          'website': 'https://acme.com',
          'address': 'Rua X, 100',
        },
      });

      expect(lead.id, 'lead_1');
      expect(lead.status, LeadPipelineStatus.enriched);
      expect(lead.score, 7);
      expect(lead.approachDraft, 'Olá!');
      expect(lead.hasBuyIntent, isTrue);
      expect(lead.displayName, 'Acme Ltda');
      expect(lead.decisionMakerName, 'Ana Silva');
      expect(lead.decisionMakerRole, 'CEO');
      expect(lead.linkedinUrl, 'https://linkedin.com/in/ana');
      expect(lead.instagramUrl, 'https://instagram.com/acme');
      expect(lead.website, 'https://acme.com');
      expect(lead.address, 'Rua X, 100');
    });

    test('falls back to contact id when no company name; tolerates empty', () {
      final lead = LeadModel.fromJson({
        'id': 'lead_2',
        'contact_id': '+5511888888888',
        'status': 'prospected',
      });
      expect(lead.displayName, '+5511888888888');
      expect(lead.decisionMakerName, isNull);
      expect(lead.linkedinUrl, isNull);
      expect(lead.hasBuyIntent, isFalse);
      expect(lead.enrichment, isEmpty);
    });

    test('blank enrichment strings are treated as absent', () {
      final lead = LeadModel.fromJson({
        'id': 'lead_3',
        'contact_id': 'place_123',
        'enrichment': {'company_name': '   ', 'linkedin_url': ''},
      });
      expect(lead.displayName, 'place_123');
      expect(lead.linkedinUrl, isNull);
    });
  });

  group('ProspectingResultModel.fromJson', () {
    test('parses counts with defaults', () {
      final r = ProspectingResultModel.fromJson({
        'results_count': 3,
        'leads_created': 2,
        'skipped_duplicate': 1,
      });
      expect(r.resultsCount, 3);
      expect(r.leadsCreated, 2);
      expect(r.skippedDuplicate, 1);
      expect(r.skippedNoContact, 0);
    });
  });
}
