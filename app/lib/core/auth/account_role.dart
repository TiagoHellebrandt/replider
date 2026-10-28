/// Per-account roles, mirroring `api/internal/core/domain/rbac.go`.
///
/// The SDR pivot added owner/gestor/vendedor alongside the original
/// admin/attendant pair. The API answers "may this user do X?" through a
/// role-satisfaction table; the app was still comparing `role == 'admin'`
/// as a string, so an **owner** — the highest role, and the one the account
/// creator gets — read as not-an-admin and lost every admin surface in the
/// UI while the API happily served those same endpoints.
///
/// Keep this table identical to the Go one. It is an explicit map rather
/// than a numeric rank because the hierarchy is not a line: a vendedor
/// outranks nobody, and must NOT inherit the attendant's full-inbox access.
enum AccountRole {
  owner('owner'),
  admin('admin'),
  gestor('gestor'),
  vendedor('vendedor'),
  attendant('attendant');

  const AccountRole(this.wire);

  /// The value stored on the user row and carried in the JWT claim.
  final String wire;

  /// Maps a stored/JWT value onto a known role. An empty or unrecognized
  /// value degrades to [attendant] — the least-privileged role — so a
  /// corrupt claim can never escalate. Same rule as `NormalizeRole` in Go.
  static AccountRole from(String? raw) {
    for (final role in AccountRole.values) {
      if (role.wire == raw) return role;
    }
    return AccountRole.attendant;
  }

  static const Map<AccountRole, Set<AccountRole>> _satisfies = {
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
    // A gestor runs the sales floor and can work the inbox, but is not an
    // admin: no billing, no integrations, no commission rules.
    AccountRole.gestor: {
      AccountRole.gestor,
      AccountRole.vendedor,
      AccountRole.attendant,
    },
    // A vendedor is deliberately NOT an attendant: opening every
    // conversation in the account is not part of the job.
    AccountRole.vendedor: {AccountRole.vendedor},
    AccountRole.attendant: {AccountRole.attendant},
  };

  /// Whether this role can act in the place of [required].
  bool satisfies(AccountRole required) =>
      _satisfies[this]?.contains(required) ?? false;
}

/// Convenience for the most common question in the UI: may this user reach
/// the account-level surfaces (team, channels, billing, compliance)?
bool roleIsAdmin(String? raw) =>
    AccountRole.from(raw).satisfies(AccountRole.admin);
