import 'package:app/core/utils/whatsapp_link.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('an E.164 number keeps its own country code', () {
    expect(whatsappLink('+55 65 3322-4100', 'oi'),
        'https://wa.me/556533224100?text=oi');
  });

  test('a Brazilian number typed without +55 gets it', () {
    expect(whatsappLink('(65) 99000-1234', 'oi'),
        'https://wa.me/5565990001234?text=oi');
    expect(whatsappLink('65 3322-4100', 'oi'),
        'https://wa.me/556533224100?text=oi');
  });

  // A number we cannot dial must hide the button, not open a chat with a
  // stranger. Null is the signal the caller branches on.
  test('a number that is neither is refused', () {
    for (final bad in ['', '  ', '123', '99811', '1234567890123456', 'abc']) {
      expect(whatsappLink(bad, 'oi'), isNull, reason: 'for "$bad"');
    }
  });

  test('the message is percent-encoded, accents and newlines included', () {
    final url = whatsappLink('+5565999999999', 'Bom dia!\nVocê tem interesse?');

    expect(url, startsWith('https://wa.me/5565999999999?text='));
    // Decoding must give back exactly what the seller read on screen — a
    // draft that arrives mangled is worse than no draft.
    final text = Uri.decodeComponent(url!.split('?text=').last);
    expect(text, 'Bom dia!\nVocê tem interesse?');
  });

  test('an empty message still opens the chat', () {
    expect(
        whatsappLink('+5565999999999', '   '), 'https://wa.me/5565999999999');
  });
}
