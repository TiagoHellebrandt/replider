// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'provider_cost_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(providerCostDataSource)
final providerCostDataSourceProvider = ProviderCostDataSourceProvider._();

final class ProviderCostDataSourceProvider extends $FunctionalProvider<
    ProviderCostRemoteDataSource,
    ProviderCostRemoteDataSource,
    ProviderCostRemoteDataSource> with $Provider<ProviderCostRemoteDataSource> {
  ProviderCostDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'providerCostDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$providerCostDataSourceHash();

  @$internal
  @override
  $ProviderElement<ProviderCostRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  ProviderCostRemoteDataSource create(Ref ref) {
    return providerCostDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ProviderCostRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ProviderCostRemoteDataSource>(value),
    );
  }
}

String _$providerCostDataSourceHash() =>
    r'5734cfb1673de4dfb85c31ffe434dac52f0911bf';

@ProviderFor(ownerProviderCosts)
final ownerProviderCostsProvider = OwnerProviderCostsProvider._();

final class OwnerProviderCostsProvider extends $FunctionalProvider<
        AsyncValue<OwnerProviderCostsResponse>,
        OwnerProviderCostsResponse,
        FutureOr<OwnerProviderCostsResponse>>
    with
        $FutureModifier<OwnerProviderCostsResponse>,
        $FutureProvider<OwnerProviderCostsResponse> {
  OwnerProviderCostsProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'ownerProviderCostsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$ownerProviderCostsHash();

  @$internal
  @override
  $FutureProviderElement<OwnerProviderCostsResponse> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<OwnerProviderCostsResponse> create(Ref ref) {
    return ownerProviderCosts(ref);
  }
}

String _$ownerProviderCostsHash() =>
    r'c234efba78bccc0c7e5519a30a7a83ad2f42622d';

@ProviderFor(ProviderCostActions)
final providerCostActionsProvider = ProviderCostActionsProvider._();

final class ProviderCostActionsProvider
    extends $NotifierProvider<ProviderCostActions, void> {
  ProviderCostActionsProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'providerCostActionsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$providerCostActionsHash();

  @$internal
  @override
  ProviderCostActions create() => ProviderCostActions();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$providerCostActionsHash() =>
    r'b4c8e01c26370a93a224a5c55cc568a2d795e5c1';

abstract class _$ProviderCostActions extends $Notifier<void> {
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
