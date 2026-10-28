import 'dart:async';

import 'package:app/core/i18n/locale_bootstrap.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

// `main()` awaits LocaleBootstrap.apply() BEFORE runApp, so anything that
// can hang in here does not degrade the app — it leaves a permanently BLANK
// page. That is exactly what a fresh visitor hit: FlutterSecureStorage.read
// never completed on an empty web store, and the app never mounted.

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  const channel = MethodChannel('plugins.it_nomads.com/flutter_secure_storage');

  void mockStorage(Future<Object?> Function(MethodCall call) handler) {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, handler);
  }

  setUp(() => LocaleSettings.setLocale(AppLocale.en));
  tearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, null);
  });

  test('startup completes even when storage never answers', () async {
    // A read that hangs forever — the failure mode that blanked the app.
    mockStorage((call) => Completer<Object?>().future);

    // Real time on purpose: the point is that the bounded wait actually
    // elapses and startup proceeds, not that a fake clock can be advanced.
    await LocaleBootstrap.apply().timeout(
      const Duration(seconds: 10),
      onTimeout: () => fail('startup hung on storage — the app would be blank'),
    );

    // Fell back to the device locale rather than blocking forever.
    expect(LocaleSettings.currentLocale, isNotNull);
  }, timeout: const Timeout(Duration(seconds: 20)));

  test('a stored preference is still applied when storage answers', () async {
    mockStorage((call) async => call.method == 'read' ? 'pt' : null);

    await LocaleBootstrap.apply();

    expect(LocaleSettings.currentLocale, AppLocale.pt);
  });
}
