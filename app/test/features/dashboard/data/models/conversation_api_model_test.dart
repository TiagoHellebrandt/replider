import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/dashboard/data/models/conversation_api_model.dart';

void main() {
  group('ConversationApiModel', () {
    const fullJson = {
      'id': 'conv-abc',
      'account_id': 'acc-123',
      'agent_id': 'agent-456',
      'contact_id': 'contact-789',
      'contact_name': 'Maria Silva',
      'contact_username': '@mariasilva',
      'channel_id': 'ch-001',
      'channel_type': 'whatsapp',
      'status': 'open',
      'is_bot_paused': false,
      'waiting_since': '2026-03-31T10:00:00Z',
      'last_customer_message_at': '2026-03-31T10:05:00Z',
      'last_message': 'Oi, preciso de ajuda',
      'created_at': '2026-03-30T08:00:00Z',
      'updated_at': '2026-03-31T10:05:00Z',
    };

    test('fromJson should parse all fields correctly', () {
      final model = ConversationApiModel.fromJson(fullJson);

      expect(model.id, 'conv-abc');
      expect(model.accountId, 'acc-123');
      expect(model.agentId, 'agent-456');
      expect(model.contactId, 'contact-789');
      expect(model.contactName, 'Maria Silva');
      expect(model.contactUsername, '@mariasilva');
      expect(model.channelId, 'ch-001');
      expect(model.channelType, 'whatsapp');
      expect(model.status, 'open');
      expect(model.isBotPaused, false);
      expect(model.waitingSince, '2026-03-31T10:00:00Z');
      expect(model.lastCustomerMessageAt, '2026-03-31T10:05:00Z');
      expect(model.lastMessage, 'Oi, preciso de ajuda');
      expect(model.createdAt, '2026-03-30T08:00:00Z');
      expect(model.updatedAt, '2026-03-31T10:05:00Z');
    });

    test('fromJson should handle missing optional fields', () {
      const minimalJson = {
        'id': 'conv-min',
        'account_id': 'acc-1',
        'contact_id': 'contact-1',
        'channel_id': 'ch-1',
        'status': 'pending_human',
        'is_bot_paused': true,
        'created_at': '2026-03-30T08:00:00Z',
        'updated_at': '2026-03-31T10:00:00Z',
      };

      final model = ConversationApiModel.fromJson(minimalJson);

      expect(model.id, 'conv-min');
      expect(model.agentId, isNull);
      expect(model.contactName, isNull);
      expect(model.contactUsername, isNull);
      expect(model.channelType, isNull);
      expect(model.waitingSince, isNull);
      expect(model.lastCustomerMessageAt, isNull);
      expect(model.lastMessage, isNull);
      expect(model.isBotPaused, true);
      expect(model.status, 'pending_human');
    });

    test('toJson should produce correct snake_case keys', () {
      final model = ConversationApiModel.fromJson(fullJson);
      final json = model.toJson();

      expect(json['id'], 'conv-abc');
      expect(json['account_id'], 'acc-123');
      expect(json['agent_id'], 'agent-456');
      expect(json['contact_id'], 'contact-789');
      expect(json['contact_name'], 'Maria Silva');
      expect(json['contact_username'], '@mariasilva');
      expect(json['channel_id'], 'ch-001');
      expect(json['channel_type'], 'whatsapp');
      expect(json['status'], 'open');
      expect(json['is_bot_paused'], false);
      expect(json['last_message'], 'Oi, preciso de ajuda');
    });

    test('toJson/fromJson round-trip should preserve data', () {
      final original = ConversationApiModel.fromJson(fullJson);
      final json = original.toJson();
      final restored = ConversationApiModel.fromJson(json);

      expect(restored.id, original.id);
      expect(restored.accountId, original.accountId);
      expect(restored.agentId, original.agentId);
      expect(restored.contactId, original.contactId);
      expect(restored.contactName, original.contactName);
      expect(restored.contactUsername, original.contactUsername);
      expect(restored.channelId, original.channelId);
      expect(restored.channelType, original.channelType);
      expect(restored.status, original.status);
      expect(restored.isBotPaused, original.isBotPaused);
      expect(restored.waitingSince, original.waitingSince);
      expect(restored.lastCustomerMessageAt, original.lastCustomerMessageAt);
      expect(restored.lastMessage, original.lastMessage);
      expect(restored.createdAt, original.createdAt);
      expect(restored.updatedAt, original.updatedAt);
    });
  });
}
