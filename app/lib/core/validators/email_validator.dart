/// Permissive email check. The old strict regex rejected perfectly valid
/// real-world addresses like `user+tag@company.co.uk` and
/// `first.last@sub.domain.com`, which cost us signup conversion.
///
/// RFC 5321 is 400 lines of grammar; we don't need to replicate it
/// client-side. The backend is the source of truth. Here we only catch
/// the two mistakes that are worth blocking before a round-trip:
///   1. No `@` at all
///   2. Nothing after the `@` (missing host)
/// Everything else goes through and the server rejects it with a
/// localized error if it's truly malformed.
bool isValidEmail(String? value) {
  if (value == null) return false;
  final v = value.trim();
  if (v.isEmpty) return false;
  final at = v.indexOf('@');
  // Require one '@' not at the ends and at least one char after it.
  if (at <= 0 || at == v.length - 1) return false;
  if (v.indexOf('@', at + 1) != -1) return false; // multiple @
  // Very loose host check — at least one dot or it's a bare token
  // like "localhost" that some internal setups use. We don't forbid it.
  return true;
}
