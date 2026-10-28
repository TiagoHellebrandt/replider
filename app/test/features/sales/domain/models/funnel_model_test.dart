import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';

void main() {
  group('MetadataFieldModel', () {
    test('fromJson maps required + ai_hint + enum_options', () {
      final f = MetadataFieldModel.fromJson(<String, dynamic>{
        'key': 'plan_size',
        'label': 'Plan size',
        'type': 'enum',
        'required': true,
        'ai_hint': 'ask casually',
        'enum_options': ['small', 'large'],
      });
      expect(f.key, 'plan_size');
      expect(f.label, 'Plan size');
      expect(f.type, 'enum');
      expect(f.isRequired, true);
      expect(f.aiHint, 'ask casually');
      expect(f.enumOptions, ['small', 'large']);
    });

    test('toJson emits required key (not is_required)', () {
      const f = MetadataFieldModel(key: 'cpf', type: 'cpf', isRequired: true);
      final json = f.toJson();
      expect(json['required'], true);
      expect(json.containsKey('is_required'), false);
      expect(json['key'], 'cpf');
    });

    test('tolerates minimal payload with defaults', () {
      final f = MetadataFieldModel.fromJson(<String, dynamic>{'key': 'name'});
      expect(f.label, '');
      expect(f.type, 'text');
      expect(f.isRequired, false);
      expect(f.aiHint, '');
      expect(f.enumOptions, isNull);
    });
  });

  group('FunnelModel', () {
    final fullJson = <String, dynamic>{
      'id': 'fun-1',
      'account_id': 'acc-1',
      'name': 'Premium funnel',
      'product_id': 'prod-1',
      'agent_id': null,
      'channel_id': null,
      'persona_prompt': 'be consultative',
      'metadata_schema': [
        {'key': 'cpf', 'label': 'CPF', 'type': 'cpf', 'required': true},
      ],
      'webhook_url': 'https://example.com/hook',
      'webhook_secret': 's3cr3t',
      'trigger_on_required_complete': true,
      'trigger_on_buy_intent': false,
      'active': true,
      'created_at': '2026-01-01T00:00:00Z',
      'updated_at': '2026-01-02T00:00:00Z',
    };

    test('fromJson decodes the nested metadata schema array', () {
      final f = FunnelModel.fromJson(fullJson);
      expect(f.id, 'fun-1');
      expect(f.name, 'Premium funnel');
      expect(f.productId, 'prod-1');
      expect(f.metadataSchema, hasLength(1));
      expect(f.metadataSchema.first.key, 'cpf');
      expect(f.metadataSchema.first.isRequired, true);
      expect(f.webhookUrl, 'https://example.com/hook');
      expect(f.triggerOnRequiredComplete, true);
    });

    test('isNew is true only for an empty id', () {
      expect(const FunnelModel().isNew, true);
      expect(FunnelModel.fromJson(fullJson).isNew, false);
    });

    test('tolerates missing metadata_schema with empty default', () {
      final f = FunnelModel.fromJson(<String, dynamic>{
        'id': 'fun-2',
        'account_id': 'acc-1',
        'name': 'Bare',
      });
      expect(f.metadataSchema, isEmpty);
      expect(f.active, true);
      expect(f.webhookUrl, '');
    });

    test('toRequestBody serialises schema and omits read-only fields', () {
      final f = FunnelModel.fromJson(fullJson);
      final body = f.toRequestBody();
      expect(body['name'], 'Premium funnel');
      expect(body['product_id'], 'prod-1');
      expect(body['trigger_on_required_complete'], true);
      expect(body.containsKey('id'), false);
      expect(body.containsKey('created_at'), false);
      final schema = body['metadata_schema'] as List<dynamic>;
      expect(schema, hasLength(1));
      expect((schema.first as Map)['required'], true);
    });

    test(
        'toRequestBody omits channel_id when unset so the server keeps the link',
        () {
      final unset = FunnelModel.fromJson(fullJson);
      expect(unset.toRequestBody().containsKey('channel_id'), false);

      final bound = unset.copyWith(channelId: 'ch-1');
      expect(bound.toRequestBody()['channel_id'], 'ch-1');
    });

    test('copyWith overrides only the given fields', () {
      final f = FunnelModel.fromJson(fullJson);
      final updated = f.copyWith(name: 'Renamed', metadataSchema: const []);
      expect(updated.name, 'Renamed');
      expect(updated.metadataSchema, isEmpty);
      // untouched fields preserved
      expect(updated.id, 'fun-1');
      expect(updated.webhookUrl, 'https://example.com/hook');
      expect(updated.productId, 'prod-1');
    });
  });
}
