import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/settings/domain/models/team_member_model.dart';

void main() {
  TeamMemberModel member({
    required bool verified,
    String? inviteSentAt,
  }) =>
      TeamMemberModel(
        id: 'u',
        name: 'N',
        email: 'e@x.com',
        role: 'admin',
        isActive: true,
        isEmailVerified: verified,
        inviteSentAt: inviteSentAt,
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
      );

  group('TeamMemberModel.isPendingInvite', () {
    test('account owner (unverified, never invited) is NOT pending', () {
      // The owner is created unverified at signup but carries no
      // invite_sent_at — the regression this guards: the owner showed a
      // false "Pending" badge on their own team row.
      expect(
          member(verified: false, inviteSentAt: null).isPendingInvite, isFalse);
    });

    test('invited member (unverified, invite stamped) IS pending', () {
      expect(
        member(verified: false, inviteSentAt: '2026-06-19T12:00:00Z')
            .isPendingInvite,
        isTrue,
      );
    });

    test('verified member is never pending, even with an invite stamp', () {
      expect(
        member(verified: true, inviteSentAt: '2026-06-19T12:00:00Z')
            .isPendingInvite,
        isFalse,
      );
    });
  });

  group('TeamMemberModel JSON', () {
    test('fromJson parses invite_sent_at and is_email_verified', () {
      final m = TeamMemberModel.fromJson(<String, dynamic>{
        'id': 'u1',
        'name': 'Jane',
        'email': 'jane@x.com',
        'role': 'attendant',
        'is_active': true,
        'is_email_verified': false,
        'invite_sent_at': '2026-06-19T12:00:00Z',
        'created_at': '2026-01-01T00:00:00Z',
        'updated_at': '2026-01-01T00:00:00Z',
      });
      expect(m.isEmailVerified, isFalse);
      expect(m.inviteSentAt, '2026-06-19T12:00:00Z');
      expect(m.isPendingInvite, isTrue);
    });

    test('is_email_verified defaults to false when absent', () {
      final m = TeamMemberModel.fromJson(<String, dynamic>{
        'id': 'u2',
        'name': 'Bob',
        'email': 'bob@x.com',
        'role': 'admin',
        'is_active': true,
        'created_at': '2026-01-01T00:00:00Z',
        'updated_at': '2026-01-01T00:00:00Z',
      });
      expect(m.isEmailVerified, isFalse);
      expect(m.inviteSentAt, isNull);
      expect(m.isPendingInvite, isFalse);
    });
  });
}
