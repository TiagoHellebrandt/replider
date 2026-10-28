import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';

void main() {
  group('ConversationModel', () {
    test('should construct with all fields', () {
      final now = DateTime(2026, 3, 31, 10, 30);
      final fullModel = ConversationModel(
        id: 'conv-1',
        channelId: 'ch-test',
        contactExternalId: '5511999999999',
        contactName: 'John Doe',
        contactUsername: '@johndoe',
        contactAvatarUrl: 'https://example.com/avatar.png',
        lastMessage: 'Hello there!',
        lastMessageAt: now,
        source: ConversationSource.whatsapp,
        unreadCount: 5,
        status: ConversationStatus.pendingHuman,
      );

      expect(fullModel.id, 'conv-1');
      expect(fullModel.contactName, 'John Doe');
      expect(fullModel.contactUsername, '@johndoe');
      expect(fullModel.contactAvatarUrl, 'https://example.com/avatar.png');
      expect(fullModel.lastMessage, 'Hello there!');
      expect(fullModel.lastMessageAt, now);
      expect(fullModel.source, ConversationSource.whatsapp);
      expect(fullModel.unreadCount, 5);
      expect(fullModel.status, ConversationStatus.pendingHuman);
    });

    test('should default unreadCount to 0', () {
      final model = ConversationModel(
        id: 'conv-2',
        channelId: 'ch-test',
        contactExternalId: '5511999999999',
        contactName: 'Jane',
        lastMessage: 'Hi',
        lastMessageAt: DateTime.now(),
        source: ConversationSource.telegram,
      );

      expect(model.unreadCount, 0);
    });

    test('should default status to open', () {
      final model = ConversationModel(
        id: 'conv-3',
        channelId: 'ch-test',
        contactExternalId: '5511999999999',
        contactName: 'Alice',
        lastMessage: 'Hey',
        lastMessageAt: DateTime.now(),
        source: ConversationSource.instagram,
      );

      expect(model.status, ConversationStatus.open);
    });

    test('should allow null contactUsername', () {
      final model = ConversationModel(
        id: 'conv-4',
        channelId: 'ch-test',
        contactExternalId: '5511999999999',
        contactName: 'Bob',
        lastMessage: 'Test',
        lastMessageAt: DateTime.now(),
        source: ConversationSource.whatsapp,
      );

      expect(model.contactUsername, isNull);
    });

    test('should allow null contactAvatarUrl', () {
      final model = ConversationModel(
        id: 'conv-5',
        channelId: 'ch-test',
        contactExternalId: '5511999999999',
        contactName: 'Carol',
        lastMessage: 'Test',
        lastMessageAt: DateTime.now(),
        source: ConversationSource.whatsapp,
      );

      expect(model.contactAvatarUrl, isNull);
    });
  });

  group('ConversationSource', () {
    test('should have all expected values', () {
      expect(ConversationSource.values, hasLength(6));
      expect(ConversationSource.values, contains(ConversationSource.whatsapp));
      expect(ConversationSource.values, contains(ConversationSource.email));
      expect(ConversationSource.values, contains(ConversationSource.instagram));
      expect(ConversationSource.values, contains(ConversationSource.telegram));
      expect(ConversationSource.values, contains(ConversationSource.sms));
      expect(ConversationSource.values, contains(ConversationSource.internal));
    });
  });

  group('ConversationStatus', () {
    test('should have all expected values', () {
      expect(ConversationStatus.values, hasLength(3));
      expect(ConversationStatus.values, contains(ConversationStatus.open));
      expect(
          ConversationStatus.values, contains(ConversationStatus.pendingHuman));
      expect(ConversationStatus.values, contains(ConversationStatus.closed));
    });
  });
}
