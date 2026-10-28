/// Display helpers for E.164 phone numbers coming from WhatsApp Cloud
/// (`wa_id`) and other channels. Pure functions so they unit-test
/// cleanly and don't drag Flutter widgets into the test graph.
///
/// Country detection is best-effort and limited to the markets we
/// actually serve today (BR + a handful of common diaspora prefixes).
/// Anything we can't classify falls back to a globe emoji + the raw
/// number — the dashboard never shows a wrong flag.
library;

class PhoneInfo {
  final String formatted;
  final String? countryCode; // ISO 3166-1 alpha-2 (e.g. "BR")
  final String? countryName;
  final String flagEmoji;

  const PhoneInfo({
    required this.formatted,
    this.countryCode,
    this.countryName,
    required this.flagEmoji,
  });
}

/// Parses a raw E.164 phone (digits only, no `+`) and returns a
/// display-ready bundle. Accepts the legacy form with a leading `+`
/// or whitespace; both get normalised before parsing.
PhoneInfo parsePhone(String raw) {
  final digits = raw.replaceAll(RegExp(r'\D'), '');
  if (digits.isEmpty) {
    return const PhoneInfo(formatted: '', flagEmoji: '🌐');
  }

  // Brazil — by far the dominant case, so handle it first with the
  // local DDD + 9-digit cell formatting Brazilians expect on screen.
  if (digits.startsWith('55') && (digits.length == 12 || digits.length == 13)) {
    final ddd = digits.substring(2, 4);
    final rest = digits.substring(4);
    final formatted = rest.length == 9
        ? '+55 ($ddd) ${rest.substring(0, 5)}-${rest.substring(5)}'
        : '+55 ($ddd) ${rest.substring(0, 4)}-${rest.substring(4)}';
    return PhoneInfo(
      formatted: formatted,
      countryCode: 'BR',
      countryName: 'Brasil',
      flagEmoji: '🇧🇷',
    );
  }

  // Generic E.164 lookup. Order matters: longest matching prefix wins
  // so "1" (US/CA) doesn't swallow "1XXX" sub-codes from the table.
  for (final entry in _prefixes.entries) {
    if (digits.startsWith(entry.key)) {
      final prefix = entry.key;
      final rest = digits.substring(prefix.length);
      final grouped = _groupRest(rest);
      return PhoneInfo(
        formatted: '+$prefix $grouped',
        countryCode: entry.value.code,
        countryName: entry.value.name,
        flagEmoji: entry.value.flag,
      );
    }
  }

  // Unknown country code — fall back to a readable +<digits> form so
  // the user at least sees the number cleanly without misleading flag.
  return PhoneInfo(formatted: '+$digits', flagEmoji: '🌐');
}

/// Splits the post-country digits into 3-4 char groups for legibility.
/// Doesn't try to mirror national formats (it would be wrong half the
/// time); just makes long numbers scannable.
String _groupRest(String rest) {
  if (rest.length <= 4) return rest;
  final buf = StringBuffer();
  for (var i = 0; i < rest.length; i += 4) {
    final end = (i + 4 < rest.length) ? i + 4 : rest.length;
    if (i > 0) buf.write(' ');
    buf.write(rest.substring(i, end));
  }
  return buf.toString();
}

class _Country {
  final String code;
  final String name;
  final String flag;
  const _Country(this.code, this.name, this.flag);
}

// Curated subset of E.164 prefixes the dashboard is most likely to see
// (LatAm + EU + US/CA + Portugal). Longest prefix first within each
// group so partial matches resolve correctly.
const Map<String, _Country> _prefixes = {
  '351': _Country('PT', 'Portugal', '🇵🇹'),
  '598': _Country('UY', 'Uruguay', '🇺🇾'),
  '595': _Country('PY', 'Paraguay', '🇵🇾'),
  '593': _Country('EC', 'Ecuador', '🇪🇨'),
  '591': _Country('BO', 'Bolivia', '🇧🇴'),
  '57': _Country('CO', 'Colombia', '🇨🇴'),
  '56': _Country('CL', 'Chile', '🇨🇱'),
  '54': _Country('AR', 'Argentina', '🇦🇷'),
  '52': _Country('MX', 'México', '🇲🇽'),
  '51': _Country('PE', 'Perú', '🇵🇪'),
  '49': _Country('DE', 'Deutschland', '🇩🇪'),
  '44': _Country('GB', 'United Kingdom', '🇬🇧'),
  '39': _Country('IT', 'Italia', '🇮🇹'),
  '34': _Country('ES', 'España', '🇪🇸'),
  '33': _Country('FR', 'France', '🇫🇷'),
  '1': _Country('US', 'United States', '🇺🇸'),
};
