import 'package:flutter_test/flutter_test.dart';
import 'package:app/core/utils/number_format.dart';

void main() {
  group('formatCompactNumber', () {
    test('passes through values below 1000', () {
      expect(formatCompactNumber(0), '0');
      expect(formatCompactNumber(42), '42');
      expect(formatCompactNumber(999), '999');
    });

    test('abbreviates thousands with one decimal', () {
      expect(formatCompactNumber(1000), '1.0K');
      expect(formatCompactNumber(1312), '1.3K');
      expect(formatCompactNumber(12500), '12.5K');
    });

    test('abbreviates millions with one decimal', () {
      expect(formatCompactNumber(1000000), '1.0M');
      expect(formatCompactNumber(1500000), '1.5M');
    });

    test('handles negatives symmetrically', () {
      expect(formatCompactNumber(-1500), '-1.5K');
      expect(formatCompactNumber(-2000000), '-2.0M');
    });
  });
}
