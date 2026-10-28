import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/settings/data/datasources/two_factor_remote_datasource.dart';
import 'package:app/features/settings/domain/models/two_factor_models.dart';

part 'two_factor_controller.g.dart';

@riverpod
TwoFactorRemoteDataSource twoFactorDataSource(Ref ref) {
  return TwoFactorRemoteDataSource(ref.watch(dioProvider));
}

@riverpod
Future<TwoFactorStatus> twoFactorStatus(Ref ref) {
  return ref.watch(twoFactorDataSourceProvider).status();
}

/// Imperative wrapper for the enrolment flow. The screen drives
/// `setup()` → `verify()` → `disable()`.
///
/// These methods MUST NOT touch `ref` after their `await`. This is an
/// autoDispose provider read via `.notifier` (no listener keeps it
/// alive), so it disposes during the network round-trip; a
/// `ref.invalidate(...)` after the await then throws StateError on the
/// dead ref — which surfaced as a bogus "unexpected error" right after
/// a successful 200 on Ativar 2FA. The caller (the card, whose
/// ConsumerState `ref` outlives the call) owns invalidating
/// [twoFactorStatusProvider] once the enabled state actually flips.
@riverpod
class TwoFactorActions extends _$TwoFactorActions {
  @override
  void build() {}

  Future<TwoFactorSetupResult> setup() {
    return ref.read(twoFactorDataSourceProvider).setup();
  }

  Future<TwoFactorVerifyResult> verify(String code) {
    return ref.read(twoFactorDataSourceProvider).verify({'code': code});
  }

  Future<void> disable(String code) {
    return ref.read(twoFactorDataSourceProvider).disable({'code': code});
  }
}
