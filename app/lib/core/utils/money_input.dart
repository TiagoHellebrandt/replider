/// Reads a typed money amount into cents, in either notation.
///
/// Five screens carried their own copy of this:
///
/// ```dart
/// raw.trim().replaceAll('.', '').replaceAll(',', '.')
/// ```
///
/// which is "drop the Brazilian thousands separator, turn the Brazilian decimal
/// comma into a point". Fed the US notation it destroys the number: `1,234.56`
/// loses its point, gains a decimal comma, and lands as **1.23456** — the
/// person records a sale of $1,234.56 and the database stores $1.23. A
/// thousandfold error, silently, on the figure commissions are calculated from.
///
/// The fix is to decide by POSITION rather than by character. Whichever
/// separator appears LAST is the decimal one; the other groups thousands. That
/// rule needs no locale, so it also survives a Brazilian typing `1234.56` and
/// an American typing `1.234,56` — people do both, and neither should silently
/// produce a different amount from the one on screen.
///
/// Returns 0 for anything unreadable: a form that refuses to parse is a form
/// whose Save button lies, and every caller already treats 0 as "nothing
/// entered".
library;

/// The separators a person might type. Non-breaking and thin spaces show up in
/// pasted amounts (`1 234,56`) and are grouping, never decimal.
const _groupingSpaces = [' ', ' ', ' ', ' '];

int parseMoneyToCents(String input) {
  var raw = input.trim();
  if (raw.isEmpty) return 0;

  // A leading minus survives; commission adjustments are legitimately negative.
  final negative = raw.startsWith('-');
  if (negative || raw.startsWith('+')) raw = raw.substring(1).trim();

  // Currency symbols and codes are noise here — the caller already knows the
  // currency, and a pasted "R$ 1.234,56" should not parse differently from what
  // the same person typed by hand.
  raw = raw.replaceAll(RegExp(r'[^0-9.,]'), '');
  for (final space in _groupingSpaces) {
    raw = raw.replaceAll(space, '');
  }
  if (raw.isEmpty) return 0;

  final lastComma = raw.lastIndexOf(',');
  final lastDot = raw.lastIndexOf('.');

  String digitsOnly;
  int decimals;

  if (lastComma == -1 && lastDot == -1) {
    digitsOnly = raw;
    decimals = 0;
  } else {
    // The LAST separator is the decimal one — unless it groups, which is what
    // the checks below decide.
    final decimalAt = lastComma > lastDot ? lastComma : lastDot;
    final tail = raw.substring(decimalAt + 1);
    final head = raw.substring(0, decimalAt);
    final onlySeparator = lastComma == -1 || lastDot == -1;
    final repeated = onlySeparator &&
        raw.split(raw[decimalAt]).length > 2; // appears more than once

    // `1.234` and `1,234` are one thousand two hundred and thirty-four in BOTH
    // notations — three trailing digits after a lone separator is grouping, not
    // a fractional part. Reading it as a decimal would turn 1,234 into 1.23 for
    // the one case where both locales already agree.
    //
    // The head has to be a plausible leading group for that to hold: at most
    // three digits, as in `1.234` / `12.345` / `123.456`. With four or more the
    // number cannot be grouped that way — real grouping would have written
    // `1.234.995` — so `1234.995` is three decimal places, not thousands.
    //
    // A bare `0` cannot lead a group either: nobody writes `0.005` meaning
    // five.
    final groupsThousands = onlySeparator &&
        head.isNotEmpty &&
        head != '0' &&
        (repeated || (tail.length == 3 && head.length <= 3));

    if (groupsThousands) {
      digitsOnly = raw.replaceAll(RegExp(r'[.,]'), '');
      decimals = 0;
    } else {
      digitsOnly = (head + tail).replaceAll(RegExp(r'[.,]'), '');
      decimals = tail.length;
    }
  }

  if (digitsOnly.isEmpty) return 0;
  final units = int.tryParse(digitsOnly);
  if (units == null) return 0;

  // Scale to cents from however many decimal places were written. More than two
  // rounds rather than truncates: 1234.995 is 1,235.00 to everyone reading it.
  final cents = switch (decimals) {
    0 => units * 100,
    1 => units * 10,
    2 => units,
    _ => (units / _pow10(decimals - 2)).round(),
  };
  return negative ? -cents : cents;
}

int _pow10(int exponent) {
  var out = 1;
  for (var i = 0; i < exponent; i++) {
    out *= 10;
  }
  return out;
}

/// Cents back into the text a field should show, in the notation of [currency].
///
/// The inverse of [parseMoneyToCents] for the round trip a form makes when it
/// loads a stored amount. BRL writes a decimal comma because that is what the
/// person will type over; everything else writes a point.
String formatCentsForInput(int cents, String currency) {
  final negative = cents < 0;
  final absolute = negative ? -cents : cents;
  final units = absolute ~/ 100;
  final sub = (absolute % 100).toString().padLeft(2, '0');
  final separator = currency.toUpperCase() == 'BRL' ? ',' : '.';
  return '${negative ? '-' : ''}$units$separator$sub';
}
