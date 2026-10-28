import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';
import 'package:app/core/utils/signup_locale.dart';

void main() {
  group('signupLocale', () {
    test('sends the full device locale tag, region included', () {
      // The server only maps a real en-US to USD — the region must
      // survive the trip (US-017 review fixes).
      expect(signupLocale(const Locale('en', 'US')), 'en-US');
      expect(signupLocale(const Locale('pt', 'BR')), 'pt-BR');
      expect(signupLocale(const Locale('de', 'DE')), 'de-DE');
    });

    test('region-less device locale stays bare (ambiguous → BRL server-side)',
        () {
      expect(signupLocale(const Locale('en')), 'en');
      expect(signupLocale(const Locale('es')), 'es');
    });

    test('falls back to the platform dispatcher locale when not injected', () {
      // In the test binding the platform locale is a real Locale — the
      // important part is that the default path produces a non-empty tag
      // and never throws.
      expect(signupLocale(), isNotEmpty);
    });
  });
}
