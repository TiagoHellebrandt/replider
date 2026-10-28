// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'inbox_assignment_filter_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Resolves the current scope. Defaults by role, gated to the platform
/// account (US-020): only a platform-account attendant lands on "Minhas" —
/// customer-account teams keep the historical full view ("Todas") unless
/// the user explicitly picks otherwise. An explicit choice is persisted
/// per user via [FlutterSecureStorage] (the app's KV-pref mechanism; see
/// locale_bootstrap.dart) so it survives reloads and never leaks between
/// users sharing a browser.

@ProviderFor(InboxAssignmentFilter)
final inboxAssignmentFilterProvider = InboxAssignmentFilterProvider._();

/// Resolves the current scope. Defaults by role, gated to the platform
/// account (US-020): only a platform-account attendant lands on "Minhas" —
/// customer-account teams keep the historical full view ("Todas") unless
/// the user explicitly picks otherwise. An explicit choice is persisted
/// per user via [FlutterSecureStorage] (the app's KV-pref mechanism; see
/// locale_bootstrap.dart) so it survives reloads and never leaks between
/// users sharing a browser.
final class InboxAssignmentFilterProvider extends $AsyncNotifierProvider<
    InboxAssignmentFilter, InboxAssignmentScope> {
  /// Resolves the current scope. Defaults by role, gated to the platform
  /// account (US-020): only a platform-account attendant lands on "Minhas" —
  /// customer-account teams keep the historical full view ("Todas") unless
  /// the user explicitly picks otherwise. An explicit choice is persisted
  /// per user via [FlutterSecureStorage] (the app's KV-pref mechanism; see
  /// locale_bootstrap.dart) so it survives reloads and never leaks between
  /// users sharing a browser.
  InboxAssignmentFilterProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'inboxAssignmentFilterProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$inboxAssignmentFilterHash();

  @$internal
  @override
  InboxAssignmentFilter create() => InboxAssignmentFilter();
}

String _$inboxAssignmentFilterHash() =>
    r'6ca65cc993d07af2c7df956d03aacfe2c1b31247';

/// Resolves the current scope. Defaults by role, gated to the platform
/// account (US-020): only a platform-account attendant lands on "Minhas" —
/// customer-account teams keep the historical full view ("Todas") unless
/// the user explicitly picks otherwise. An explicit choice is persisted
/// per user via [FlutterSecureStorage] (the app's KV-pref mechanism; see
/// locale_bootstrap.dart) so it survives reloads and never leaks between
/// users sharing a browser.

abstract class _$InboxAssignmentFilter
    extends $AsyncNotifier<InboxAssignmentScope> {
  FutureOr<InboxAssignmentScope> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref
        as $Ref<AsyncValue<InboxAssignmentScope>, InboxAssignmentScope>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<InboxAssignmentScope>, InboxAssignmentScope>,
        AsyncValue<InboxAssignmentScope>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
