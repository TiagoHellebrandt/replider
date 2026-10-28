import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/utils/phone_format.dart';

void main() {
  group('parsePhone', () {
    test('formats Brazilian mobile (13 digits) with DDD + 9-digit cell', () {
      final p = parsePhone('5511987654321');
      expect(p.formatted, '+55 (11) 98765-4321');
      expect(p.countryCode, 'BR');
      expect(p.flagEmoji, '🇧🇷');
    });

    test('formats Brazilian landline (12 digits) without 9-prefix', () {
      final p = parsePhone('551133334444');
      expect(p.formatted, '+55 (11) 3333-4444');
      expect(p.countryCode, 'BR');
    });

    test('detects Argentina', () {
      final p = parsePhone('5491123456789');
      expect(p.countryCode, 'AR');
      expect(p.flagEmoji, '🇦🇷');
      expect(p.formatted.startsWith('+54 '), isTrue);
    });

    test('detects US', () {
      final p = parsePhone('14155551234');
      expect(p.countryCode, 'US');
      expect(p.flagEmoji, '🇺🇸');
    });

    test('strips non-digit characters before parsing', () {
      final p = parsePhone('+55 (11) 98765-4321');
      expect(p.countryCode, 'BR');
      expect(p.formatted, '+55 (11) 98765-4321');
    });

    test('falls back to globe + raw digits for unknown country', () {
      final p = parsePhone('99988877766');
      expect(p.countryCode, isNull);
      expect(p.flagEmoji, '🌐');
      expect(p.formatted, '+99988877766');
    });

    test('returns empty bundle for empty input', () {
      final p = parsePhone('');
      expect(p.formatted, '');
      expect(p.flagEmoji, '🌐');
    });
  });
}
