import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/features/settings/data/datasources/two_factor_remote_datasource.dart';
import 'package:app/features/settings/domain/models/two_factor_models.dart';
import 'package:app/features/settings/presentation/controllers/two_factor_controller.dart';

/// Fake datasource so the actions notifier runs without a real Dio.
class _FakeTwoFactorDataSource implements TwoFactorRemoteDataSource {
  @override
  Future<TwoFactorStatus> status() async => TwoFactorStatus(enabled: false);

  @override
  Future<TwoFactorSetupResult> setup() async =>
      TwoFactorSetupResult(secret: 'SECRET', otpauthUrl: 'otpauth://totp/x');

  @override
  Future<TwoFactorVerifyResult> verify(Map<String, dynamic> body) async =>
      TwoFactorVerifyResult(recoveryCodes: const ['a', 'b']);

  @override
  Future<void> disable(Map<String, dynamic> body) async {}
}

void main() {
  // Regression: the actions notifier is autoDispose and read via
  // `.notifier` (no listener), so it disposes during the network await.
  // It MUST NOT touch `ref` after the await — an earlier
  // `ref.invalidate(...)` there threw StateError on the dead ref and
  // surfaced as a bogus "unexpected error" right after a 200 on setup.
  // These calls run with no keep-alive listener, mirroring the widget.
  ProviderContainer makeContainer() {
    final c = ProviderContainer(overrides: [
      twoFactorDataSourceProvider.overrideWithValue(_FakeTwoFactorDataSource()),
    ]);
    addTearDown(c.dispose);
    return c;
  }

  test('setup returns the parsed result without throwing', () async {
    final c = makeContainer();
    final out = await c.read(twoFactorActionsProvider.notifier).setup();
    expect(out.secret, 'SECRET');
    expect(out.otpauthUrl, startsWith('otpauth://'));
  });

  test('verify returns recovery codes without throwing', () async {
    final c = makeContainer();
    final out =
        await c.read(twoFactorActionsProvider.notifier).verify('123456');
    expect(out.recoveryCodes, ['a', 'b']);
  });

  test('disable completes without throwing', () async {
    final c = makeContainer();
    await expectLater(
      c.read(twoFactorActionsProvider.notifier).disable('123456'),
      completes,
    );
  });
}
