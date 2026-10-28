import 'package:app/core/auth/session_expiry.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

// The 24h token has no refresh, so every daily user is thrown out once a day.
// What these pin: the person is TOLD it happened, a deliberate sign-out is not
// dressed up as an expiry, and the "take me back" path can't be steered
// somewhere else by whoever edits the address bar.

void main() {
  group('sanitizeReturnPath', () {
    test('lets an in-app path through', () {
      expect(sanitizeReturnPath('/funnel'), '/funnel');
      expect(sanitizeReturnPath('/settings/team?tab=metrics'),
          '/settings/team?tab=metrics');
    });

    test('refuses anything that could leave the app', () {
      // This value survives a round trip through the address bar, where anybody
      // can edit it. Following it would hand a logged-in person to a stranger.
      expect(sanitizeReturnPath('//evil.com'), isNull);
      expect(sanitizeReturnPath('https://evil.com'), isNull);
      expect(sanitizeReturnPath(r'/\evil.com'), isNull);
      expect(sanitizeReturnPath('javascript:alert(1)'), isNull);
      expect(sanitizeReturnPath('funnel'), isNull);
    });

    test('refuses the login screen itself', () {
      // Returning to '/' would bounce the person straight back to the form
      // they just filled in.
      expect(sanitizeReturnPath('/'), isNull);
    });

    test('treats empty and missing the same way', () {
      expect(sanitizeReturnPath(''), isNull);
      expect(sanitizeReturnPath(null), isNull);
    });
  });

  group('SessionExpiry', () {
    test('starts quiet', () {
      final c = ProviderContainer();
      addTearDown(c.dispose);
      expect(c.read(sessionExpiryProvider), isFalse);
    });

    test('raises on a 401 and clears once the person signs back in', () {
      final c = ProviderContainer();
      addTearDown(c.dispose);

      c.read(sessionExpiryProvider.notifier).markExpired();
      expect(c.read(sessionExpiryProvider), isTrue);

      c.read(sessionExpiryProvider.notifier).clear();
      // Left raised, the next deliberate "Sair" would claim the session
      // expired — a lie, and it costs the trust the honest message buys.
      expect(c.read(sessionExpiryProvider), isFalse);
    });
  });
}
