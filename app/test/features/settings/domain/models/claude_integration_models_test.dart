import 'package:app/features/settings/domain/models/claude_integration_models.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('PatModel', () {
    test('fromJson parses full payload (create response with token)', () {
      final model = PatModel.fromJson(const {
        'id': '01TOK',
        'name': 'Claude Code',
        'token_prefix': 'rpl_mcp_AB12',
        'created_at': '2026-07-11T10:00:00Z',
        'last_used_at': '2026-07-11T12:30:00Z',
        'token': 'rpl_mcp_plaintext-once',
      });
      expect(model.id, '01TOK');
      expect(model.name, 'Claude Code');
      expect(model.tokenPrefix, 'rpl_mcp_AB12');
      expect(model.token, 'rpl_mcp_plaintext-once');
      expect(model.createdAt, isNotNull);
      expect(model.lastUsedAt, isNotNull);
      expect(model.expiresAt, isNull);
    });

    test('fromJson tolerates list payload without token', () {
      final model = PatModel.fromJson(const {
        'id': '01TOK',
        'name': 'x',
        'token_prefix': 'rpl_mcp_CD34',
      });
      expect(model.token, isNull);
      expect(model.lastUsedAt, isNull);
    });
  });

  group('ClaudeConnectionModel', () {
    test('fromJson parses snake_case payload', () {
      final model = ClaudeConnectionModel.fromJson(const {
        'client_id': '01CLIENT',
        'client_name': 'Claude',
        'first_granted_at': '2026-07-01T00:00:00Z',
        'last_used_at': '2026-07-10T00:00:00Z',
      });
      expect(model.clientId, '01CLIENT');
      expect(model.clientName, 'Claude');
      expect(model.firstGrantedAt, isNotNull);
      expect(model.lastUsedAt, isNotNull);
    });

    test('fromJson tolerates missing fields', () {
      final model = ClaudeConnectionModel.fromJson(const {});
      expect(model.clientId, '');
      expect(model.clientName, '');
      expect(model.lastUsedAt, isNull);
    });
  });
}
