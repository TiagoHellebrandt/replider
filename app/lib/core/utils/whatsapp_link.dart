/// A `wa.me` link that opens the seller's OWN WhatsApp with the text ready.
///
/// Deliberately not an integration. The WhatsApp Business API bills per
/// conversation, needs templates approved in advance, and puts the platform
/// between a seller and a customer who are already talking. A link costs
/// nothing, needs no approval, works on the phone in their pocket — and it is
/// what makes a draft with no e-mail address still sendable by a human.
///
/// Because nothing leaves this product when the link is followed, whoever
/// follows it has to say so afterwards. A message the platform believes it
/// sent and did not is worse than one nobody sent.
library;

/// Returns null when the number cannot be dialled internationally with
/// confidence — which is the honest answer, and the reason the caller must
/// hide the affordance rather than render a button that goes nowhere.
String? whatsappLink(String rawPhone, String message) {
  final digits = rawPhone.replaceAll(RegExp(r'\D'), '');
  if (digits.isEmpty) return null;

  // wa.me wants a country code and nothing else: no +, no spaces, no dashes.
  // A stored E.164 number already carries one. Ten or eleven digits is how a
  // Brazilian writes their own number (DDD + line), and every contact this
  // product creates comes from a Brazilian public registry or a Brazilian
  // seller's own spreadsheet — so 55 is a reading, not a coin toss between
  // two countries. Anything shorter or longer is neither, and guessing at it
  // would open a chat with a stranger.
  final withCountry = switch (digits.length) {
    10 || 11 => '55$digits',
    final n when n >= 12 && n <= 15 => digits,
    _ => null,
  };
  if (withCountry == null) return null;

  final text = message.trim();
  if (text.isEmpty) return 'https://wa.me/$withCountry';
  return 'https://wa.me/$withCountry?text=${Uri.encodeComponent(text)}';
}
