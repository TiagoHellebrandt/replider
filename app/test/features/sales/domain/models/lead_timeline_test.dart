import 'package:app/features/sales/domain/models/lead_timeline.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('a failed send parses its stamp and reason', () {
    final m = TimelineMessage.fromJson(const {
      'id': 'm1',
      'author': 'team',
      'text': 'Segue a proposta.',
      'sent_at': '2026-08-24T12:00:00Z',
      'failed_at': '2026-08-24T12:00:05Z',
      'failed_reason': 'mailbox unavailable',
    });
    expect(m.failed, isTrue);
    expect(m.failedReason, 'mailbox unavailable');
  });

  test('a delivered message claims no failure', () {
    final m = TimelineMessage.fromJson(const {
      'id': 'm2',
      'author': 'customer',
      'text': 'Recebi, obrigado.',
      'sent_at': '2026-08-24T12:01:00Z',
    });
    expect(m.failed, isFalse);
    expect(m.failedReason, isEmpty);
  });
}
