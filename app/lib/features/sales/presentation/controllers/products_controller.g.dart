// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'products_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(salesRemoteDataSource)
final salesRemoteDataSourceProvider = SalesRemoteDataSourceProvider._();

final class SalesRemoteDataSourceProvider extends $FunctionalProvider<
    SalesRemoteDataSource,
    SalesRemoteDataSource,
    SalesRemoteDataSource> with $Provider<SalesRemoteDataSource> {
  SalesRemoteDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'salesRemoteDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$salesRemoteDataSourceHash();

  @$internal
  @override
  $ProviderElement<SalesRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  SalesRemoteDataSource create(Ref ref) {
    return salesRemoteDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SalesRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SalesRemoteDataSource>(value),
    );
  }
}

String _$salesRemoteDataSourceHash() =>
    r'63e04214effe474f2407437ca2e8a1a79ce2bb06';

/// Account-scoped catalog of products, kept alive so the list survives
/// navigation between the list screen and the create/edit form.

@ProviderFor(ProductsList)
final productsListProvider = ProductsListProvider._();

/// Account-scoped catalog of products, kept alive so the list survives
/// navigation between the list screen and the create/edit form.
final class ProductsListProvider
    extends $AsyncNotifierProvider<ProductsList, List<ProductModel>> {
  /// Account-scoped catalog of products, kept alive so the list survives
  /// navigation between the list screen and the create/edit form.
  ProductsListProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'productsListProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$productsListHash();

  @$internal
  @override
  ProductsList create() => ProductsList();
}

String _$productsListHash() => r'cf10170d0433ffbc1e96153c11125118eb8d78be';

/// Account-scoped catalog of products, kept alive so the list survives
/// navigation between the list screen and the create/edit form.

abstract class _$ProductsList extends $AsyncNotifier<List<ProductModel>> {
  FutureOr<List<ProductModel>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref as $Ref<AsyncValue<List<ProductModel>>, List<ProductModel>>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<List<ProductModel>>, List<ProductModel>>,
        AsyncValue<List<ProductModel>>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
