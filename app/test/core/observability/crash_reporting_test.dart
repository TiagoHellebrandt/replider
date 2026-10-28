import 'package:app/core/observability/crash_reporting.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('CrashReporting', () {
    test('is disabled when no SENTRY_DSN is provided at build time', () {
      // The default test/build environment defines no SENTRY_DSN dart-define,
      // so reporting must stay off (no network, no init).
      expect(CrashReporting.isEnabled, isFalse);
    });

    test('run() executes the appRunner exactly once when disabled', () async {
      var calls = 0;
      await CrashReporting.run(() async => calls++);
      expect(calls, 1);
    });

    test('capture() is a no-op (does not throw) when disabled', () async {
      await expectLater(
        CrashReporting.capture(Exception('boom'), StackTrace.current),
        completes,
      );
    });
  });
}
