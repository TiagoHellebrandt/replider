import 'package:app/core/router/app_router.dart';
import 'package:flutter_test/flutter_test.dart';

// Post-login landing by role. /start (the command center) requires vendedor
// server-side and answers 403 to an attendant — sending every login there put
// every attendant on an error screen as their first sight of the day.
void main() {
  test('an attendant lands on the inbox, not the 403 command center', () {
    expect(landingFor('attendant'), '/home');
  });

  test('selling roles land on the command center', () {
    for (final role in ['owner', 'admin', 'gestor', 'vendedor']) {
      expect(landingFor(role), '/start', reason: role);
    }
  });

  test('an unknown or missing role degrades to the least-privileged landing',
      () {
    expect(landingFor(null), '/home');
    expect(landingFor('garbage'), '/home');
  });
}
