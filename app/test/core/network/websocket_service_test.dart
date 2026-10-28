import 'package:flutter_test/flutter_test.dart';
import 'package:app/core/network/websocket_service.dart';

void main() {
  group('WSEvent.fromJson', () {
    test('should parse all fields correctly', () {
      final json = {
        'event': 'new_message',
        'conversation_id': 'conv-123',
        'message': {
          'id': 'msg-1',
          'content': 'Hello',
          'sender': 'bot',
        },
      };

      final event = WSEvent.fromJson(json);

      expect(event.event, 'new_message');
      expect(event.conversationId, 'conv-123');
      expect(event.message, isNotNull);
      expect(event.message!['id'], 'msg-1');
      expect(event.message!['content'], 'Hello');
      expect(event.message!['sender'], 'bot');
      expect(event.raw, json);
    });

    test('should handle missing optional fields', () {
      final json = {
        'event': 'typing',
      };

      final event = WSEvent.fromJson(json);

      expect(event.event, 'typing');
      expect(event.conversationId, isNull);
      expect(event.message, isNull);
      expect(event.raw, json);
    });

    test('should default event to unknown when missing', () {
      final json = <String, dynamic>{};

      final event = WSEvent.fromJson(json);

      expect(event.event, 'unknown');
      expect(event.conversationId, isNull);
      expect(event.message, isNull);
    });

    test('should preserve raw json payload', () {
      final json = {
        'event': 'typing_stopped',
        'conversation_id': 'conv-456',
        'extra_field': 'some_value',
      };

      final event = WSEvent.fromJson(json);

      expect(event.raw, json);
      expect(event.raw['extra_field'], 'some_value');
    });

    test('should parse event with conversation_id but no message', () {
      final json = {
        'event': 'typing',
        'conversation_id': 'conv-789',
      };

      final event = WSEvent.fromJson(json);

      expect(event.event, 'typing');
      expect(event.conversationId, 'conv-789');
      expect(event.message, isNull);
    });
  });

  group('WebSocketService lifecycle (US-098)', () {
    test('dispose tears down without an open channel and closes streams',
        () async {
      final service = WebSocketService();

      // dispose() must be safe before any connect() — the new
      // _channelSub?.cancel() guard handles a null subscription.
      expect(service.dispose, returnsNormally);

      // Both broadcast streams must complete after dispose so no
      // listener stays subscribed.
      await expectLater(service.events, emitsDone);
      await expectLater(service.connectionStatus, emitsDone);
    });

    test('isConnected is false before connect', () {
      final service = WebSocketService();
      expect(service.isConnected, isFalse);
      service.dispose();
    });
  });

  // The server refuses a vendedor's auth frame by policy (the socket mirrors
  // the inbox's role gate) and closes. The client used to reconnect every 3s
  // against that refusal for the whole session.
  group('WebSocketService reconnect policy', () {
    test('a refused auth frame ends the reconnect cycle', () {
      final service = WebSocketService();
      addTearDown(service.dispose);

      expect(service.authRefused, isFalse);
      // The refusal frame is consumed, never surfaced as an event.
      expect(service.handleFrame({'error': 'unauthorized'}), isTrue);
      expect(service.authRefused, isTrue);
    });

    test('an ordinary frame is not a refusal and resets the backoff', () {
      final service = WebSocketService();
      addTearDown(service.dispose);

      expect(service.handleFrame({'event': 'new_message'}), isFalse);
      // A per-conversation stream error carries `error` WITH an event and
      // must keep flowing to listeners.
      expect(service.handleFrame({'event': 'error', 'error': 'unauthorized'}),
          isFalse);
      expect(service.authRefused, isFalse);
      expect(service.reconnectAttempts, 0);
    });

    test('the delay doubles from 3s and caps at five minutes', () {
      expect(WebSocketService.reconnectDelay(0), const Duration(seconds: 3));
      expect(WebSocketService.reconnectDelay(1), const Duration(seconds: 6));
      expect(WebSocketService.reconnectDelay(2), const Duration(seconds: 12));
      expect(WebSocketService.reconnectDelay(6), const Duration(seconds: 192));
      expect(WebSocketService.reconnectDelay(7), const Duration(minutes: 5));
      expect(WebSocketService.reconnectDelay(40), const Duration(minutes: 5));
    });
  });
}
