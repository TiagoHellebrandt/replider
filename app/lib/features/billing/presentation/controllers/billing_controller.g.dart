// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'billing_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(billingRemoteDataSource)
final billingRemoteDataSourceProvider = BillingRemoteDataSourceProvider._();

final class BillingRemoteDataSourceProvider extends $FunctionalProvider<
    BillingRemoteDataSource,
    BillingRemoteDataSource,
    BillingRemoteDataSource> with $Provider<BillingRemoteDataSource> {
  BillingRemoteDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'billingRemoteDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$billingRemoteDataSourceHash();

  @$internal
  @override
  $ProviderElement<BillingRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  BillingRemoteDataSource create(Ref ref) {
    return billingRemoteDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(BillingRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<BillingRemoteDataSource>(value),
    );
  }
}

String _$billingRemoteDataSourceHash() =>
    r'eed300c1534d83f5244d83c5f73bd5fabde5c074';

@ProviderFor(PlansList)
final plansListProvider = PlansListProvider._();

final class PlansListProvider
    extends $AsyncNotifierProvider<PlansList, List<PlanModel>> {
  PlansListProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'plansListProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$plansListHash();

  @$internal
  @override
  PlansList create() => PlansList();
}

String _$plansListHash() => r'9f11ee22d1b1bbe4f764742921229fbcb0bf4200';

abstract class _$PlansList extends $AsyncNotifier<List<PlanModel>> {
  FutureOr<List<PlanModel>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<List<PlanModel>>, List<PlanModel>>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<List<PlanModel>>, List<PlanModel>>,
        AsyncValue<List<PlanModel>>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}

@ProviderFor(AccountUsage)
final accountUsageProvider = AccountUsageProvider._();

final class AccountUsageProvider
    extends $AsyncNotifierProvider<AccountUsage, UsageModel> {
  AccountUsageProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'accountUsageProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$accountUsageHash();

  @$internal
  @override
  AccountUsage create() => AccountUsage();
}

String _$accountUsageHash() => r'252ae2df7eb4245deab409a389f5dd94b33a6261';

abstract class _$AccountUsage extends $AsyncNotifier<UsageModel> {
  FutureOr<UsageModel> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<UsageModel>, UsageModel>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<UsageModel>, UsageModel>,
        AsyncValue<UsageModel>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}

/// Resolves the calling account's effective limits via /me/plan-limits.
/// Distinct from [PlansList] (the public catalogue) — this surfaces the
/// per-account overrides that drive Partner / Enterprise contract caps.
/// keepAlive: read by the billing screen + (potentially) the upgrade
/// modal; logout invalidates from auth_controller.dart alongside the
/// other user-scoped providers.

@ProviderFor(AccountPlanLimits)
final accountPlanLimitsProvider = AccountPlanLimitsProvider._();

/// Resolves the calling account's effective limits via /me/plan-limits.
/// Distinct from [PlansList] (the public catalogue) — this surfaces the
/// per-account overrides that drive Partner / Enterprise contract caps.
/// keepAlive: read by the billing screen + (potentially) the upgrade
/// modal; logout invalidates from auth_controller.dart alongside the
/// other user-scoped providers.
final class AccountPlanLimitsProvider extends $AsyncNotifierProvider<
    AccountPlanLimits, EffectivePlanLimitsModel> {
  /// Resolves the calling account's effective limits via /me/plan-limits.
  /// Distinct from [PlansList] (the public catalogue) — this surfaces the
  /// per-account overrides that drive Partner / Enterprise contract caps.
  /// keepAlive: read by the billing screen + (potentially) the upgrade
  /// modal; logout invalidates from auth_controller.dart alongside the
  /// other user-scoped providers.
  AccountPlanLimitsProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'accountPlanLimitsProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$accountPlanLimitsHash();

  @$internal
  @override
  AccountPlanLimits create() => AccountPlanLimits();
}

String _$accountPlanLimitsHash() => r'f8ab7d943fee270e1f17d10ccda1a085bee250ce';

/// Resolves the calling account's effective limits via /me/plan-limits.
/// Distinct from [PlansList] (the public catalogue) — this surfaces the
/// per-account overrides that drive Partner / Enterprise contract caps.
/// keepAlive: read by the billing screen + (potentially) the upgrade
/// modal; logout invalidates from auth_controller.dart alongside the
/// other user-scoped providers.

abstract class _$AccountPlanLimits
    extends $AsyncNotifier<EffectivePlanLimitsModel> {
  FutureOr<EffectivePlanLimitsModel> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref
        as $Ref<AsyncValue<EffectivePlanLimitsModel>, EffectivePlanLimitsModel>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<EffectivePlanLimitsModel>,
            EffectivePlanLimitsModel>,
        AsyncValue<EffectivePlanLimitsModel>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}

@ProviderFor(BillingActions)
final billingActionsProvider = BillingActionsProvider._();

final class BillingActionsProvider
    extends $AsyncNotifierProvider<BillingActions, void> {
  BillingActionsProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'billingActionsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$billingActionsHash();

  @$internal
  @override
  BillingActions create() => BillingActions();
}

String _$billingActionsHash() => r'b8fcaebb9be89571f94c2e531735874f50e263f9';

abstract class _$BillingActions extends $AsyncNotifier<void> {
  FutureOr<void> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<void>, void>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<void>, void>,
        AsyncValue<void>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
