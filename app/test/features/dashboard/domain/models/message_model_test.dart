import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/dashboard/domain/models/message_model.dart';

void main() {
  group('MessageModel', () {
    test('should construct with all required fields', () {
      final now = DateTime(2026, 3, 31, 14, 0);
      final model = MessageModel(
        id: 'msg-1',
        conversationId: 'conv-1',
        content: 'Hello, how can I help?',
        senderType: SenderType.agent,
        senderLabel: 'Agent Smith',
        createdAt: now,
      );

      expect(model.id, 'msg-1');
      expect(model.conversationId, 'conv-1');
      expect(model.content, 'Hello, how can I help?');
      expect(model.senderType, SenderType.agent);
      expect(model.senderLabel, 'Agent Smith');
      expect(model.createdAt, now);
    });

    test('should default senderLabel to empty string', () {
      final model = MessageModel(
        id: 'msg-2',
        conversationId: 'conv-1',
        content: 'Hi',
        senderType: SenderType.customer,
        createdAt: DateTime.now(),
      );

      expect(model.senderLabel, '');
    });

    test('should construct with bot sender type', () {
      final model = MessageModel(
        id: 'msg-3',
        conversationId: 'conv-2',
        content: 'I am a bot response',
        senderType: SenderType.bot,
        senderLabel: 'bot',
        createdAt: DateTime.now(),
      );

      expect(model.senderType, SenderType.bot);
      expect(model.senderLabel, 'bot');
    });
  });

  group('MessageModel.audioTranscript', () {
    MessageModel audio({
      required String content,
      String? transcript,
      MessageType type = MessageType.audio,
    }) =>
        MessageModel(
          id: 'm',
          conversationId: 'c',
          content: content,
          senderType: SenderType.customer,
          createdAt: DateTime(2026, 1, 1),
          messageType: type,
          transcript: transcript,
        );

    test('prefers the cached transcript column', () {
      final m = audio(content: 'inline text', transcript: 'cached text');
      expect(m.audioTranscript, 'cached text');
    });

    test('falls back to content for inbound audio', () {
      final m = audio(content: 'Olá, gostaria de um orçamento');
      expect(m.audioTranscript, 'Olá, gostaria de um orçamento');
    });

    test('treats a bracketed placeholder as no transcript', () {
      final m = audio(content: '[áudio não transcrito]');
      expect(m.audioTranscript, isNull);
    });

    test('is null for empty audio content with no cached transcript', () {
      final m = audio(content: '   ');
      expect(m.audioTranscript, isNull);
    });

    test('is null for non-audio messages', () {
      final m = audio(content: 'just a text message', type: MessageType.text);
      expect(m.audioTranscript, isNull);
    });

    test('voice messages resolve the same way', () {
      final m = audio(content: 'transcrição da voz', type: MessageType.voice);
      expect(m.audioTranscript, 'transcrição da voz');
    });
  });

  group('SenderType', () {
    test('should have all expected values', () {
      expect(SenderType.values, hasLength(3));
      expect(SenderType.values, contains(SenderType.customer));
      expect(SenderType.values, contains(SenderType.agent));
      expect(SenderType.values, contains(SenderType.bot));
    });
  });
}
