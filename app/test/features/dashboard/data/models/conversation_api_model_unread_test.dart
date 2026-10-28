import 'package:flutter_test/flutter_test.dart';

import 'package:app/features/dashboard/data/models/conversation_api_model.dart';

// unread_count is counted server-side from messages.read_at. The tile used
// to fake it from status == pending_human — "waiting on a human", which is a
// different fact and stayed lit after everything had been read.

Map<String, dynamic> _row({int? unread, String status = 'open'}) => {
      'id': 'c1',
      'account_id': 'a1',
      'contact_id': '5511999999999',
      'channel_id': 'ch1',
      'status': status,
      'is_bot_paused': false,
      'created_at': '2026-09-01T10:00:00Z',
      'updated_at': '2026-09-01T10:00:00Z',
      if (unread != null) 'unread_count': unread,
    };

void main() {
  test('parses unread_count from the list row', () {
    final model = ConversationApiModel.fromJson(_row(unread: 3));
    expect(model.unreadCount, 3);
  });

  test('a row without the field is not unread, whatever its status', () {
    final model = ConversationApiModel.fromJson(_row(status: 'pending_human'));
    expect(model.unreadCount, isNull);
  });

  test('round-trips through toJson', () {
    final model = ConversationApiModel.fromJson(_row(unread: 2));
    expect(model.toJson()['unread_count'], 2);
  });
}
