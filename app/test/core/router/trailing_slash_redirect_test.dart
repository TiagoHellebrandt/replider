import 'package:flutter_test/flutter_test.dart';
import 'package:app/core/router/app_router.dart';

void main() {
  group('canonicalTrailingSlashRedirect', () {
    test('strips a trailing slash and preserves the query', () {
      final r = canonicalTrailingSlashRedirect(
          Uri.parse('/reset-password/?token=abc123'));
      expect(r, '/reset-password?token=abc123');
    });

    test('strips a trailing slash with no query', () {
      expect(canonicalTrailingSlashRedirect(Uri.parse('/reset-password/')),
          '/reset-password');
    });

    test('no redirect when there is no trailing slash', () {
      expect(
          canonicalTrailingSlashRedirect(
              Uri.parse('/reset-password?token=abc')),
          isNull);
    });

    test('never strips the root path', () {
      expect(canonicalTrailingSlashRedirect(Uri.parse('/')), isNull);
    });

    test('preserves a multi-param query', () {
      expect(
          canonicalTrailingSlashRedirect(Uri.parse('/verify-email/?a=1&b=2')),
          '/verify-email?a=1&b=2');
    });
  });
}
