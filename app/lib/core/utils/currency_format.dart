/// Formats an integer amount of cents in the account's billing currency:
/// BRL reads `R$ 1.234,56` (dot thousands, decimal comma), anything else reads
/// `$1,234.56` (en-US style — `usd` is the only other currency the API issues,
/// US-001). Single source of truth for money rendering so plan cards, addon
/// packs, overage banners and commission statements never disagree.
///
/// Thousands are grouped because a commission statement is read by the person
/// being paid: `R$ 4230,00` invites a second look, and the API's own
/// explanation strings already group. A negative amount puts the sign before
/// the symbol (`-R$ 200,00`) — that is a deduction, not a currency called
/// "minus reais".
String formatCurrencyCents(int cents, String currency) {
  final negative = cents < 0;
  final absolute = negative ? -cents : cents;

  final units = absolute ~/ 100;
  final sub = (absolute % 100).toString().padLeft(2, '0');
  final brl = currency.toUpperCase() == 'BRL';
  final grouped = _group(units, brl ? '.' : ',');

  final sign = negative ? '-' : '';
  return brl ? '${sign}R\$ $grouped,$sub' : '$sign\$$grouped.$sub';
}

/// Inserts a thousands separator every three digits, right to left.
String _group(int units, String separator) {
  final digits = units.toString();
  final out = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) out.write(separator);
    out.write(digits[i]);
  }
  return out.toString();
}
