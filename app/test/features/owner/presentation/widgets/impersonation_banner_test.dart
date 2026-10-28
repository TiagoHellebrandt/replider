import 'package:app/features/owner/presentation/widgets/impersonation_banner.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('formatBannerRemaining', () {
    final now = DateTime(2026, 5, 6, 12, 0, 0);

    test('renders 0:00 once the JWT has already expired', () {
      expect(
        formatBannerRemaining(
          now.subtract(const Duration(seconds: 5)),
          now: now,
        ),
        '0:00',
      );
    });

    test(
        'uses mm:ss precision under 5 minutes so the operator can watch it tick',
        () {
      expect(
        formatBannerRemaining(
          now.add(const Duration(minutes: 4, seconds: 7)),
          now: now,
        ),
        '4:07',
      );
      expect(
        formatBannerRemaining(now.add(const Duration(seconds: 30)), now: now),
        '0:30',
      );
    });

    test('renders Xm shorthand between 5 and 60 minutes', () {
      expect(
        formatBannerRemaining(
          now.add(const Duration(minutes: 14, seconds: 45)),
          now: now,
        ),
        '14m',
      );
    });

    test('renders Hh / HhMm above one hour for long impersonation windows', () {
      expect(
        formatBannerRemaining(
          now.add(const Duration(hours: 2, minutes: 0)),
          now: now,
        ),
        '2h',
      );
      expect(
        formatBannerRemaining(
          now.add(const Duration(hours: 1, minutes: 23)),
          now: now,
        ),
        '1h23m',
      );
    });
  });

  group('isBannerUrgent', () {
    final now = DateTime(2026, 5, 6, 12, 0, 0);

    test(
        'flags the last 60 seconds so the banner can switch to the brighter style',
        () {
      expect(
        isBannerUrgent(now.add(const Duration(seconds: 30)), now: now),
        isTrue,
      );
      expect(
        isBannerUrgent(now.add(const Duration(seconds: 60)), now: now),
        isTrue,
      );
    });

    test('stays calm above the 60s threshold', () {
      expect(
        isBannerUrgent(now.add(const Duration(seconds: 61)), now: now),
        isFalse,
      );
      expect(
        isBannerUrgent(now.add(const Duration(minutes: 5)), now: now),
        isFalse,
      );
    });

    test('treats expired sessions as urgent', () {
      expect(
        isBannerUrgent(now.subtract(const Duration(seconds: 1)), now: now),
        isTrue,
      );
    });
  });
}
