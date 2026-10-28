// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'usage_warning_dismissal_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Tracks whether the 80% token-usage warning banner was dismissed for the
/// current calendar day. Persisted locally via [FlutterSecureStorage] — the
/// app's KV-pref mechanism (see locale_bootstrap.dart; the Drift db carries
/// no tables) — so a dismissal survives reloads but the banner re-surfaces
/// the next day, keeping the upgrade nudge alive without nagging within a day.
///
/// Only the soft 80% banner is dismissible; the 100% "exhausted" banner is a
/// hard block (the bot stops replying) and is never hidden.

@ProviderFor(UsageWarningDismissal)
final usageWarningDismissalProvider = UsageWarningDismissalProvider._();

/// Tracks whether the 80% token-usage warning banner was dismissed for the
/// current calendar day. Persisted locally via [FlutterSecureStorage] — the
/// app's KV-pref mechanism (see locale_bootstrap.dart; the Drift db carries
/// no tables) — so a dismissal survives reloads but the banner re-surfaces
/// the next day, keeping the upgrade nudge alive without nagging within a day.
///
/// Only the soft 80% banner is dismissible; the 100% "exhausted" banner is a
/// hard block (the bot stops replying) and is never hidden.
final class UsageWarningDismissalProvider
    extends $AsyncNotifierProvider<UsageWarningDismissal, bool> {
  /// Tracks whether the 80% token-usage warning banner was dismissed for the
  /// current calendar day. Persisted locally via [FlutterSecureStorage] — the
  /// app's KV-pref mechanism (see locale_bootstrap.dart; the Drift db carries
  /// no tables) — so a dismissal survives reloads but the banner re-surfaces
  /// the next day, keeping the upgrade nudge alive without nagging within a day.
  ///
  /// Only the soft 80% banner is dismissible; the 100% "exhausted" banner is a
  /// hard block (the bot stops replying) and is never hidden.
  UsageWarningDismissalProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'usageWarningDismissalProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$usageWarningDismissalHash();

  @$internal
  @override
  UsageWarningDismissal create() => UsageWarningDismissal();
}

String _$usageWarningDismissalHash() =>
    r'4bb0b2c06dbf2d74c999220c1b02df6eadfb0e45';

/// Tracks whether the 80% token-usage warning banner was dismissed for the
/// current calendar day. Persisted locally via [FlutterSecureStorage] — the
/// app's KV-pref mechanism (see locale_bootstrap.dart; the Drift db carries
/// no tables) — so a dismissal survives reloads but the banner re-surfaces
/// the next day, keeping the upgrade nudge alive without nagging within a day.
///
/// Only the soft 80% banner is dismissible; the 100% "exhausted" banner is a
/// hard block (the bot stops replying) and is never hidden.

abstract class _$UsageWarningDismissal extends $AsyncNotifier<bool> {
  FutureOr<bool> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<bool>, bool>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<bool>, bool>,
        AsyncValue<bool>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
