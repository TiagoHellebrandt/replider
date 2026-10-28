import 'package:app/features/settings/presentation/screens/channels_screen.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter_test/flutter_test.dart';

// For a channel sending through the customer's own provider, the connection
// test IS the authentication — so its result has to say what it unlocked, not
// just that something worked. These pin the three answers apart, because
// collapsing them is how a screen promises sending that the next approach
// refuses.

void main() {
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  test('a proven mailbox reports where the probe went and the daily ceiling',
      () {
    final msg = emailTestMessage(<String, dynamic>{
      'ok': true,
      'sent_to': 'contato@clinica.com.br',
      'cold_send_unlocked': true,
      'daily_cap': 25,
    });

    expect(
        msg,
        t.settings.channel_test_email_success(
            to: 'contato@clinica.com.br', cap: '25'));
    // The address matters: the operator has to know which inbox to open, and
    // that no stranger received anything.
    expect(msg, contains('contato@clinica.com.br'));
    expect(msg, contains('25'));
  });

  test('a send that could not be recorded is not reported as unlocked', () {
    final msg = emailTestMessage(<String, dynamic>{
      'ok': true,
      'sent_to': 'contato@clinica.com.br',
      'cold_send_unlocked': false,
      'daily_cap': 0,
    });

    expect(
        msg,
        t.settings
            .channel_test_email_not_unlocked(to: 'contato@clinica.com.br'));
  });

  // Unlocked with a zero ceiling is a contradiction the server should never
  // send; if it does, the honest reading is "not released".
  test('a zero ceiling counts as not released', () {
    final msg = emailTestMessage(<String, dynamic>{
      'sent_to': 'contato@clinica.com.br',
      'cold_send_unlocked': true,
      'daily_cap': 0,
    });

    expect(
        msg,
        t.settings
            .channel_test_email_not_unlocked(to: 'contato@clinica.com.br'));
  });

  test('a response with no email fields falls back to the generic copy', () {
    // Telegram and WhatsApp answer with their own shapes; this helper must not
    // claim anything about them.
    expect(emailTestMessage(<String, dynamic>{'ok': true, 'bot_name': '@bot'}),
        isNull);
    expect(
        emailTestMessage(<String, dynamic>{'ok': true, 'sent_to': ''}), isNull);
    expect(emailTestMessage(null), isNull);
  });
}
