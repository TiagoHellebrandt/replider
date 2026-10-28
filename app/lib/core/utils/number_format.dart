/// Compact human-readable integer: `999`, `12.0K`, `1.5M`. Mirrors the
/// abbreviation the billing/analytics usage cards already use so token
/// and conversation counts read consistently across the app instead of
/// dumping raw digits like `1500000`.
String formatCompactNumber(int n) {
  if (n < 0) return '-${formatCompactNumber(-n)}';
  if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
  if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}K';
  return n.toString();
}
