// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'owner_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(ownerDataSource)
final ownerDataSourceProvider = OwnerDataSourceProvider._();

final class OwnerDataSourceProvider extends $FunctionalProvider<
    OwnerRemoteDataSource,
    OwnerRemoteDataSource,
    OwnerRemoteDataSource> with $Provider<OwnerRemoteDataSource> {
  OwnerDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'ownerDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$ownerDataSourceHash();

  @$internal
  @override
  $ProviderElement<OwnerRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  OwnerRemoteDataSource create(Ref ref) {
    return ownerDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(OwnerRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<OwnerRemoteDataSource>(value),
    );
  }
}

String _$ownerDataSourceHash() => r'1f28a4ca7bcf290105375a43c7e720d8c1915013';

@ProviderFor(impersonationStorage)
final impersonationStorageProvider = ImpersonationStorageProvider._();

final class ImpersonationStorageProvider extends $FunctionalProvider<
    ImpersonationStorage,
    ImpersonationStorage,
    ImpersonationStorage> with $Provider<ImpersonationStorage> {
  ImpersonationStorageProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'impersonationStorageProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$impersonationStorageHash();

  @$internal
  @override
  $ProviderElement<ImpersonationStorage> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  ImpersonationStorage create(Ref ref) {
    return impersonationStorage(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ImpersonationStorage value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ImpersonationStorage>(value),
    );
  }
}

String _$impersonationStorageHash() =>
    r'4f076a71502ae1bd33ff18c6b8ed385aa2303f7c';

/// Active impersonation session (or null). Watched by the global
/// banner so a write/clear in any screen surfaces immediately.

@ProviderFor(ImpersonationController)
final impersonationControllerProvider = ImpersonationControllerProvider._();

/// Active impersonation session (or null). Watched by the global
/// banner so a write/clear in any screen surfaces immediately.
final class ImpersonationControllerProvider extends $AsyncNotifierProvider<
    ImpersonationController, ImpersonationSession?> {
  /// Active impersonation session (or null). Watched by the global
  /// banner so a write/clear in any screen surfaces immediately.
  ImpersonationControllerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'impersonationControllerProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$impersonationControllerHash();

  @$internal
  @override
  ImpersonationController create() => ImpersonationController();
}

String _$impersonationControllerHash() =>
    r'eecd65f022a97577fcd6fa1752a42212ac8e3dcf';

/// Active impersonation session (or null). Watched by the global
/// banner so a write/clear in any screen surfaces immediately.

abstract class _$ImpersonationController
    extends $AsyncNotifier<ImpersonationSession?> {
  FutureOr<ImpersonationSession?> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref
        as $Ref<AsyncValue<ImpersonationSession?>, ImpersonationSession?>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<ImpersonationSession?>, ImpersonationSession?>,
        AsyncValue<ImpersonationSession?>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}

/// Probe the /owner namespace once and resolve the operator's access
/// level. The sidebar shows the entry for any non-forbidden status so
/// the user can complete their setup; the owner screen itself handles
/// the routing to settings / re-login when the status is not allowed.

@ProviderFor(ownerAccessStatus)
final ownerAccessStatusProvider = OwnerAccessStatusProvider._();

/// Probe the /owner namespace once and resolve the operator's access
/// level. The sidebar shows the entry for any non-forbidden status so
/// the user can complete their setup; the owner screen itself handles
/// the routing to settings / re-login when the status is not allowed.

final class OwnerAccessStatusProvider extends $FunctionalProvider<
        AsyncValue<OwnerAccessStatus>,
        OwnerAccessStatus,
        FutureOr<OwnerAccessStatus>>
    with
        $FutureModifier<OwnerAccessStatus>,
        $FutureProvider<OwnerAccessStatus> {
  /// Probe the /owner namespace once and resolve the operator's access
  /// level. The sidebar shows the entry for any non-forbidden status so
  /// the user can complete their setup; the owner screen itself handles
  /// the routing to settings / re-login when the status is not allowed.
  OwnerAccessStatusProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'ownerAccessStatusProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$ownerAccessStatusHash();

  @$internal
  @override
  $FutureProviderElement<OwnerAccessStatus> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<OwnerAccessStatus> create(Ref ref) {
    return ownerAccessStatus(ref);
  }
}

String _$ownerAccessStatusHash() => r'0878a39cd939aaf0af5e5d491a237948e81ce8f4';

/// Backwards-compat shim — true whenever the operator has any path
/// to the panel (allowed OR a setup/refresh step away). Existing
/// sidebar code reads this to decide whether to render the entry.

@ProviderFor(ownerIsSuperAdmin)
final ownerIsSuperAdminProvider = OwnerIsSuperAdminProvider._();

/// Backwards-compat shim — true whenever the operator has any path
/// to the panel (allowed OR a setup/refresh step away). Existing
/// sidebar code reads this to decide whether to render the entry.

final class OwnerIsSuperAdminProvider
    extends $FunctionalProvider<AsyncValue<bool>, bool, FutureOr<bool>>
    with $FutureModifier<bool>, $FutureProvider<bool> {
  /// Backwards-compat shim — true whenever the operator has any path
  /// to the panel (allowed OR a setup/refresh step away). Existing
  /// sidebar code reads this to decide whether to render the entry.
  OwnerIsSuperAdminProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'ownerIsSuperAdminProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$ownerIsSuperAdminHash();

  @$internal
  @override
  $FutureProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<bool> create(Ref ref) {
    return ownerIsSuperAdmin(ref);
  }
}

String _$ownerIsSuperAdminHash() => r'e12c5b6f9123aa3aeb67c5a92556bb7aa23a38b3';

/// Paginated owner-panel listing. Search/status debouncing happens
/// in the screen — the controller just refetches whenever the args
/// change.

@ProviderFor(OwnerAccounts)
final ownerAccountsProvider = OwnerAccountsFamily._();

/// Paginated owner-panel listing. Search/status debouncing happens
/// in the screen — the controller just refetches whenever the args
/// change.
final class OwnerAccountsProvider
    extends $AsyncNotifierProvider<OwnerAccounts, OwnerAccountsListResponse> {
  /// Paginated owner-panel listing. Search/status debouncing happens
  /// in the screen — the controller just refetches whenever the args
  /// change.
  OwnerAccountsProvider._(
      {required OwnerAccountsFamily super.from,
      required ({
        String search,
        String status,
        int limit,
        int offset,
      })
          super.argument})
      : super(
          retry: null,
          name: r'ownerAccountsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$ownerAccountsHash();

  @override
  String toString() {
    return r'ownerAccountsProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  OwnerAccounts create() => OwnerAccounts();

  @override
  bool operator ==(Object other) {
    return other is OwnerAccountsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$ownerAccountsHash() => r'9509b3bd04af1404f91c14543e964c2418b32657';

/// Paginated owner-panel listing. Search/status debouncing happens
/// in the screen — the controller just refetches whenever the args
/// change.

final class OwnerAccountsFamily extends $Family
    with
        $ClassFamilyOverride<
            OwnerAccounts,
            AsyncValue<OwnerAccountsListResponse>,
            OwnerAccountsListResponse,
            FutureOr<OwnerAccountsListResponse>,
            ({
              String search,
              String status,
              int limit,
              int offset,
            })> {
  OwnerAccountsFamily._()
      : super(
          retry: null,
          name: r'ownerAccountsProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  /// Paginated owner-panel listing. Search/status debouncing happens
  /// in the screen — the controller just refetches whenever the args
  /// change.

  OwnerAccountsProvider call({
    String search = '',
    String status = '',
    int limit = 50,
    int offset = 0,
  }) =>
      OwnerAccountsProvider._(argument: (
        search: search,
        status: status,
        limit: limit,
        offset: offset,
      ), from: this);

  @override
  String toString() => r'ownerAccountsProvider';
}

/// Paginated owner-panel listing. Search/status debouncing happens
/// in the screen — the controller just refetches whenever the args
/// change.

abstract class _$OwnerAccounts
    extends $AsyncNotifier<OwnerAccountsListResponse> {
  late final _$args = ref.$arg as ({
    String search,
    String status,
    int limit,
    int offset,
  });
  String get search => _$args.search;
  String get status => _$args.status;
  int get limit => _$args.limit;
  int get offset => _$args.offset;

  FutureOr<OwnerAccountsListResponse> build({
    String search = '',
    String status = '',
    int limit = 50,
    int offset = 0,
  });
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<OwnerAccountsListResponse>,
        OwnerAccountsListResponse>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<OwnerAccountsListResponse>,
            OwnerAccountsListResponse>,
        AsyncValue<OwnerAccountsListResponse>,
        Object?,
        Object?>;
    element.handleCreate(
        ref,
        () => build(
              search: _$args.search,
              status: _$args.status,
              limit: _$args.limit,
              offset: _$args.offset,
            ));
  }
}

@ProviderFor(ownerAccountDetail)
final ownerAccountDetailProvider = OwnerAccountDetailFamily._();

final class OwnerAccountDetailProvider extends $FunctionalProvider<
        AsyncValue<OwnerAccountDetail>,
        OwnerAccountDetail,
        FutureOr<OwnerAccountDetail>>
    with
        $FutureModifier<OwnerAccountDetail>,
        $FutureProvider<OwnerAccountDetail> {
  OwnerAccountDetailProvider._(
      {required OwnerAccountDetailFamily super.from,
      required String super.argument})
      : super(
          retry: null,
          name: r'ownerAccountDetailProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$ownerAccountDetailHash();

  @override
  String toString() {
    return r'ownerAccountDetailProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<OwnerAccountDetail> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<OwnerAccountDetail> create(Ref ref) {
    final argument = this.argument as String;
    return ownerAccountDetail(
      ref,
      argument,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is OwnerAccountDetailProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$ownerAccountDetailHash() =>
    r'0e9394bc2f0b0cb84a3208be0c041854cc3c95c8';

final class OwnerAccountDetailFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<OwnerAccountDetail>, String> {
  OwnerAccountDetailFamily._()
      : super(
          retry: null,
          name: r'ownerAccountDetailProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  OwnerAccountDetailProvider call(
    String id,
  ) =>
      OwnerAccountDetailProvider._(argument: id, from: this);

  @override
  String toString() => r'ownerAccountDetailProvider';
}

/// Top spenders this period — drives the "biggest accounts" widget on
/// the owner-panel home.

@ProviderFor(ownerTopAccounts)
final ownerTopAccountsProvider = OwnerTopAccountsFamily._();

/// Top spenders this period — drives the "biggest accounts" widget on
/// the owner-panel home.

final class OwnerTopAccountsProvider extends $FunctionalProvider<
        AsyncValue<TopAccountsResponse>,
        TopAccountsResponse,
        FutureOr<TopAccountsResponse>>
    with
        $FutureModifier<TopAccountsResponse>,
        $FutureProvider<TopAccountsResponse> {
  /// Top spenders this period — drives the "biggest accounts" widget on
  /// the owner-panel home.
  OwnerTopAccountsProvider._(
      {required OwnerTopAccountsFamily super.from, required int super.argument})
      : super(
          retry: null,
          name: r'ownerTopAccountsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$ownerTopAccountsHash();

  @override
  String toString() {
    return r'ownerTopAccountsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<TopAccountsResponse> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<TopAccountsResponse> create(Ref ref) {
    final argument = this.argument as int;
    return ownerTopAccounts(
      ref,
      limit: argument,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is OwnerTopAccountsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$ownerTopAccountsHash() => r'3bbde2f53685cbb5417bc9536565c44c43766221';

/// Top spenders this period — drives the "biggest accounts" widget on
/// the owner-panel home.

final class OwnerTopAccountsFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<TopAccountsResponse>, int> {
  OwnerTopAccountsFamily._()
      : super(
          retry: null,
          name: r'ownerTopAccountsProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  /// Top spenders this period — drives the "biggest accounts" widget on
  /// the owner-panel home.

  OwnerTopAccountsProvider call({
    int limit = 10,
  }) =>
      OwnerTopAccountsProvider._(argument: limit, from: this);

  @override
  String toString() => r'ownerTopAccountsProvider';
}

/// Platform-wide aggregate counts for the owner-panel "system" widget.

@ProviderFor(ownerSystemHealth)
final ownerSystemHealthProvider = OwnerSystemHealthProvider._();

/// Platform-wide aggregate counts for the owner-panel "system" widget.

final class OwnerSystemHealthProvider extends $FunctionalProvider<
        AsyncValue<OwnerSystemHealth>,
        OwnerSystemHealth,
        FutureOr<OwnerSystemHealth>>
    with
        $FutureModifier<OwnerSystemHealth>,
        $FutureProvider<OwnerSystemHealth> {
  /// Platform-wide aggregate counts for the owner-panel "system" widget.
  OwnerSystemHealthProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'ownerSystemHealthProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$ownerSystemHealthHash();

  @$internal
  @override
  $FutureProviderElement<OwnerSystemHealth> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<OwnerSystemHealth> create(Ref ref) {
    return ownerSystemHealth(ref);
  }
}

String _$ownerSystemHealthHash() => r'b88af2bb2d4829bd8b4626f01582bf012db652ea';

/// 30-day daily token + conversation rollup for a target account —
/// drives the mini bar chart on the detail screen.

@ProviderFor(ownerUsageTrend)
final ownerUsageTrendProvider = OwnerUsageTrendFamily._();

/// 30-day daily token + conversation rollup for a target account —
/// drives the mini bar chart on the detail screen.

final class OwnerUsageTrendProvider extends $FunctionalProvider<
        AsyncValue<OwnerUsageTrendResponse>,
        OwnerUsageTrendResponse,
        FutureOr<OwnerUsageTrendResponse>>
    with
        $FutureModifier<OwnerUsageTrendResponse>,
        $FutureProvider<OwnerUsageTrendResponse> {
  /// 30-day daily token + conversation rollup for a target account —
  /// drives the mini bar chart on the detail screen.
  OwnerUsageTrendProvider._(
      {required OwnerUsageTrendFamily super.from,
      required String super.argument})
      : super(
          retry: null,
          name: r'ownerUsageTrendProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$ownerUsageTrendHash();

  @override
  String toString() {
    return r'ownerUsageTrendProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<OwnerUsageTrendResponse> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<OwnerUsageTrendResponse> create(Ref ref) {
    final argument = this.argument as String;
    return ownerUsageTrend(
      ref,
      argument,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is OwnerUsageTrendProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$ownerUsageTrendHash() => r'908f7606053495733273545a8464de5b59e9cabd';

/// 30-day daily token + conversation rollup for a target account —
/// drives the mini bar chart on the detail screen.

final class OwnerUsageTrendFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<OwnerUsageTrendResponse>, String> {
  OwnerUsageTrendFamily._()
      : super(
          retry: null,
          name: r'ownerUsageTrendProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  /// 30-day daily token + conversation rollup for a target account —
  /// drives the mini bar chart on the detail screen.

  OwnerUsageTrendProvider call(
    String accountId,
  ) =>
      OwnerUsageTrendProvider._(argument: accountId, from: this);

  @override
  String toString() => r'ownerUsageTrendProvider';
}

/// Last 10 Stripe invoices for a target account — feeds the read-only
/// invoices section on the owner-panel detail screen. Empty list when
/// the account has no Stripe customer (contract / pre-checkout).

@ProviderFor(ownerInvoices)
final ownerInvoicesProvider = OwnerInvoicesFamily._();

/// Last 10 Stripe invoices for a target account — feeds the read-only
/// invoices section on the owner-panel detail screen. Empty list when
/// the account has no Stripe customer (contract / pre-checkout).

final class OwnerInvoicesProvider extends $FunctionalProvider<
        AsyncValue<OwnerInvoicesResponse>,
        OwnerInvoicesResponse,
        FutureOr<OwnerInvoicesResponse>>
    with
        $FutureModifier<OwnerInvoicesResponse>,
        $FutureProvider<OwnerInvoicesResponse> {
  /// Last 10 Stripe invoices for a target account — feeds the read-only
  /// invoices section on the owner-panel detail screen. Empty list when
  /// the account has no Stripe customer (contract / pre-checkout).
  OwnerInvoicesProvider._(
      {required OwnerInvoicesFamily super.from, required String super.argument})
      : super(
          retry: null,
          name: r'ownerInvoicesProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$ownerInvoicesHash();

  @override
  String toString() {
    return r'ownerInvoicesProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<OwnerInvoicesResponse> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<OwnerInvoicesResponse> create(Ref ref) {
    final argument = this.argument as String;
    return ownerInvoices(
      ref,
      argument,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is OwnerInvoicesProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$ownerInvoicesHash() => r'9c071d38fc8dcedbaac888fa861ab0f708407c66';

/// Last 10 Stripe invoices for a target account — feeds the read-only
/// invoices section on the owner-panel detail screen. Empty list when
/// the account has no Stripe customer (contract / pre-checkout).

final class OwnerInvoicesFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<OwnerInvoicesResponse>, String> {
  OwnerInvoicesFamily._()
      : super(
          retry: null,
          name: r'ownerInvoicesProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  /// Last 10 Stripe invoices for a target account — feeds the read-only
  /// invoices section on the owner-panel detail screen. Empty list when
  /// the account has no Stripe customer (contract / pre-checkout).

  OwnerInvoicesProvider call(
    String accountId,
  ) =>
      OwnerInvoicesProvider._(argument: accountId, from: this);

  @override
  String toString() => r'ownerInvoicesProvider';
}

/// Latest conversations for a target account (read-only triage list
/// surfaced on the owner-panel detail screen). The detail screen can
/// then nudge operators into impersonation when they need to read the
/// full thread.

@ProviderFor(ownerConversations)
final ownerConversationsProvider = OwnerConversationsFamily._();

/// Latest conversations for a target account (read-only triage list
/// surfaced on the owner-panel detail screen). The detail screen can
/// then nudge operators into impersonation when they need to read the
/// full thread.

final class OwnerConversationsProvider extends $FunctionalProvider<
        AsyncValue<OwnerConversationsResponse>,
        OwnerConversationsResponse,
        FutureOr<OwnerConversationsResponse>>
    with
        $FutureModifier<OwnerConversationsResponse>,
        $FutureProvider<OwnerConversationsResponse> {
  /// Latest conversations for a target account (read-only triage list
  /// surfaced on the owner-panel detail screen). The detail screen can
  /// then nudge operators into impersonation when they need to read the
  /// full thread.
  OwnerConversationsProvider._(
      {required OwnerConversationsFamily super.from,
      required (
        String, {
        int limit,
      })
          super.argument})
      : super(
          retry: null,
          name: r'ownerConversationsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$ownerConversationsHash();

  @override
  String toString() {
    return r'ownerConversationsProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  $FutureProviderElement<OwnerConversationsResponse> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<OwnerConversationsResponse> create(Ref ref) {
    final argument = this.argument as (
      String, {
      int limit,
    });
    return ownerConversations(
      ref,
      argument.$1,
      limit: argument.limit,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is OwnerConversationsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$ownerConversationsHash() =>
    r'c7583ecab0016c69e7edaa330e79cf9cc0eb1fab';

/// Latest conversations for a target account (read-only triage list
/// surfaced on the owner-panel detail screen). The detail screen can
/// then nudge operators into impersonation when they need to read the
/// full thread.

final class OwnerConversationsFamily extends $Family
    with
        $FunctionalFamilyOverride<
            FutureOr<OwnerConversationsResponse>,
            (
              String, {
              int limit,
            })> {
  OwnerConversationsFamily._()
      : super(
          retry: null,
          name: r'ownerConversationsProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  /// Latest conversations for a target account (read-only triage list
  /// surfaced on the owner-panel detail screen). The detail screen can
  /// then nudge operators into impersonation when they need to read the
  /// full thread.

  OwnerConversationsProvider call(
    String accountId, {
    int limit = 50,
  }) =>
      OwnerConversationsProvider._(argument: (
        accountId,
        limit: limit,
      ), from: this);

  @override
  String toString() => r'ownerConversationsProvider';
}

/// Imperative actions exposed as a plain controller. UI calls these
/// from button handlers and invalidates the relevant providers on
/// success.

@ProviderFor(OwnerActions)
final ownerActionsProvider = OwnerActionsProvider._();

/// Imperative actions exposed as a plain controller. UI calls these
/// from button handlers and invalidates the relevant providers on
/// success.
final class OwnerActionsProvider extends $NotifierProvider<OwnerActions, void> {
  /// Imperative actions exposed as a plain controller. UI calls these
  /// from button handlers and invalidates the relevant providers on
  /// success.
  OwnerActionsProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'ownerActionsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$ownerActionsHash();

  @$internal
  @override
  OwnerActions create() => OwnerActions();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$ownerActionsHash() => r'70d4a7fbd69bb9708865c6c0b06ca4a519125c4d';

/// Imperative actions exposed as a plain controller. UI calls these
/// from button handlers and invalidates the relevant providers on
/// success.

abstract class _$OwnerActions extends $Notifier<void> {
  void build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<void, void>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<void, void>, void, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}
