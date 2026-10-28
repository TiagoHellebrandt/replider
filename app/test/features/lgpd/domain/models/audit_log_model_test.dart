import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/lgpd/domain/models/audit_log_model.dart';
import 'package:app/i18n/strings.g.dart';

void main() {
  setUp(() {
    LocaleSettings.setLocale(AppLocale.pt);
  });

  group('AuditLogEntry.fromJson', () {
    test('parses all fields with a valid timestamp', () {
      final json = <String, dynamic>{
        'id': 'a1',
        'account_id': 'acc',
        'actor_id': 'user-1',
        'action': 'lgpd.export',
        'entity_type': 'contact',
        'entity_id': '5511',
        'payload': {'conversations': 1, 'messages': 10},
        'created_at': '2026-04-14T12:00:00Z',
      };
      final e = AuditLogEntry.fromJson(json);
      expect(e.id, 'a1');
      expect(e.actorId, 'user-1');
      expect(e.action, 'lgpd.export');
      expect(e.entityId, '5511');
      expect(e.payload['messages'], 10);
      expect(e.createdAt.year, 2026);
    });

    test('gracefully handles missing fields', () {
      final e = AuditLogEntry.fromJson({});
      expect(e.id, '');
      expect(e.action, '');
      expect(e.payload, isEmpty);
      // created_at falls back to now when missing — not zero
      expect(e.createdAt, isNotNull);
    });

    test('gracefully handles malformed timestamp', () {
      final e = AuditLogEntry.fromJson({'created_at': 'not-a-date'});
      expect(e.createdAt, isNotNull);
    });

    test('payload defaults to empty when server returns list (base64 jsonb)',
        () {
      final e = AuditLogEntry.fromJson({
        'action': 'x',
        'payload': <dynamic>[],
      });
      expect(e.payload, isEmpty);
    });
  });

  group('AuditLogEntry.actionLabel', () {
    test('maps known codes to localized labels', () {
      final exp = AuditLogEntry.fromJson({'action': 'lgpd.export'});
      final anon = AuditLogEntry.fromJson({'action': 'lgpd.anonymize'});
      expect(exp.actionLabel, t.lgpd.audit_action_export);
      expect(anon.actionLabel, t.lgpd.audit_action_anonymize);
    });

    test('unknown codes pass through', () {
      final e = AuditLogEntry.fromJson({'action': 'custom.action'});
      expect(e.actionLabel, 'custom.action');
    });
  });

  group('AuditLogPage.fromJson', () {
    test('parses total + entries', () {
      final page = AuditLogPage.fromJson({
        'total': 42,
        'entries': [
          {'id': 'a1', 'action': 'lgpd.export'},
          {'id': 'a2', 'action': 'lgpd.anonymize'},
        ],
      });
      expect(page.total, 42);
      expect(page.entries, hasLength(2));
      expect(page.entries[0].id, 'a1');
    });

    test('defaults to empty page when missing', () {
      final page = AuditLogPage.fromJson({});
      expect(page.total, 0);
      expect(page.entries, isEmpty);
    });

    test('skips non-Map entries', () {
      final page = AuditLogPage.fromJson({
        'total': 2,
        'entries': [
          {'id': 'a1'},
          'not a map',
          {'id': 'a2'},
        ],
      });
      expect(page.entries, hasLength(2));
    });
  });
}
