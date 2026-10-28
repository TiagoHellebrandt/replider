import 'package:app/core/auth/account_role.dart';
import 'package:flutter_test/flutter_test.dart';

// The app used to answer "is this user an admin?" with `role == 'admin'`.
// The SDR pivot added owner/gestor/vendedor, so the account CREATOR — an
// `owner`, the highest role there is — read as not-an-admin and lost Team,
// Channels, Products, Billing and Compliance from the account menu, while
// the API served those same endpoints without complaint.

void main() {
  test('an owner satisfies admin — the regression that hid the account menu',
      () {
    expect(AccountRole.owner.satisfies(AccountRole.admin), isTrue);
    expect(roleIsAdmin('owner'), isTrue);
  });

  test('an admin still satisfies admin', () {
    expect(roleIsAdmin('admin'), isTrue);
  });

  test('gestor, vendedor and attendant do not reach admin surfaces', () {
    expect(roleIsAdmin('gestor'), isFalse);
    expect(roleIsAdmin('vendedor'), isFalse);
    expect(roleIsAdmin('attendant'), isFalse);
  });

  // A vendedor is deliberately not an attendant: opening every conversation
  // in the account is not part of the job.
  test('a vendedor does not inherit the attendant inbox', () {
    expect(AccountRole.vendedor.satisfies(AccountRole.attendant), isFalse);
    expect(AccountRole.gestor.satisfies(AccountRole.attendant), isTrue);
  });

  // Deny by default: a corrupt or unknown claim must never escalate.
  test('an unknown or missing role degrades to attendant', () {
    expect(AccountRole.from('super_admin'), AccountRole.attendant);
    expect(AccountRole.from(''), AccountRole.attendant);
    expect(AccountRole.from(null), AccountRole.attendant);
    expect(roleIsAdmin('super_admin'), isFalse);
  });

  // This table is a copy of api/internal/core/domain/rbac.go. If the two
  // drift, the UI and the API disagree about who may do what.
  test('the satisfaction table matches the API, role for role', () {
    const expected = {
      AccountRole.owner: {
        AccountRole.owner,
        AccountRole.admin,
        AccountRole.gestor,
        AccountRole.vendedor,
        AccountRole.attendant,
      },
      AccountRole.admin: {
        AccountRole.admin,
        AccountRole.gestor,
        AccountRole.vendedor,
        AccountRole.attendant,
      },
      AccountRole.gestor: {
        AccountRole.gestor,
        AccountRole.vendedor,
        AccountRole.attendant,
      },
      AccountRole.vendedor: {AccountRole.vendedor},
      AccountRole.attendant: {AccountRole.attendant},
    };

    for (final actor in AccountRole.values) {
      for (final required in AccountRole.values) {
        expect(
          actor.satisfies(required),
          expected[actor]!.contains(required),
          reason: '${actor.wire} satisfies ${required.wire}',
        );
      }
    }
  });
}
