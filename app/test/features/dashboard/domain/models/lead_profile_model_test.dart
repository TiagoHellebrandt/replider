import 'package:flutter_test/flutter_test.dart';

import 'package:app/features/dashboard/domain/models/lead_profile_model.dart';

void main() {
  group('LeadProfileModel.fromJson', () {
    test('in_funnel false yields empty profile', () {
      final p = LeadProfileModel.fromJson({'in_funnel': false});
      expect(p.inFunnel, false);
      expect(p.fields, isEmpty);
      expect(p.hasBuyIntent, false);
    });

    test('missing in_funnel treated as not in funnel', () {
      final p = LeadProfileModel.fromJson(const {});
      expect(p.inFunnel, false);
    });

    test('parses full payload with fields, status, score, buy intent', () {
      final p = LeadProfileModel.fromJson({
        'in_funnel': true,
        'lead_id': 'lead-1',
        'funnel_id': 'funnel-1',
        'funnel_name': 'Vendas',
        'status': 'qualified',
        'score': 42,
        'buy_intent_at': '2026-01-02T10:00:00Z',
        'fields': [
          {
            'key': 'name',
            'label': 'Nome',
            'type': 'text',
            'required': true,
            'value': 'Jane',
            'source': 'ai',
          },
          {
            'key': 'email',
            'type': 'text',
            'value': 'jane@x.com',
            'source': 'human',
          },
          {
            'key': 'budget',
            'label': 'Orçamento',
            'type': 'number',
            'required': true,
          },
        ],
      });

      expect(p.inFunnel, true);
      expect(p.leadId, 'lead-1');
      expect(p.funnelName, 'Vendas');
      expect(p.status, 'qualified');
      expect(p.score, 42);
      expect(p.hasBuyIntent, true);
      expect(p.fields, hasLength(3));

      final name = p.fields[0];
      expect(name.label, 'Nome');
      expect(name.value, 'Jane');
      expect(name.source, LeadFieldSource.ai);
      expect(name.hasValue, true);
      expect(name.isPending, false);

      // No label falls back to the key.
      expect(p.fields[1].label, 'email');
      expect(p.fields[1].source, LeadFieldSource.human);

      // Required + empty value = pending.
      final budget = p.fields[2];
      expect(budget.required, true);
      expect(budget.hasValue, false);
      expect(budget.isPending, true);
      expect(budget.source, LeadFieldSource.none);
    });

    test('non-string value is coerced to display string', () {
      final p = LeadProfileModel.fromJson({
        'in_funnel': true,
        'fields': [
          {'key': 'qty', 'type': 'number', 'value': 7, 'source': 'ai'},
        ],
      });
      expect(p.fields.single.value, '7');
    });

    test('unknown source defaults to none', () {
      final f = LeadFieldModel.fromJson({'key': 'k', 'source': 'bogus'});
      expect(f.source, LeadFieldSource.none);
    });
  });
}
