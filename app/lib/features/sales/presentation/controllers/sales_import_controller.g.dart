// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'sales_import_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(salesImportDataSource)
final salesImportDataSourceProvider = SalesImportDataSourceProvider._();

final class SalesImportDataSourceProvider extends $FunctionalProvider<
    SalesImportDataSource,
    SalesImportDataSource,
    SalesImportDataSource> with $Provider<SalesImportDataSource> {
  SalesImportDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'salesImportDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$salesImportDataSourceHash();

  @$internal
  @override
  $ProviderElement<SalesImportDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  SalesImportDataSource create(Ref ref) {
    return salesImportDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SalesImportDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SalesImportDataSource>(value),
    );
  }
}

String _$salesImportDataSourceHash() =>
    r'04d01594938e12a8c58963218851e5f0ad2d7adf';

@ProviderFor(salesImportActions)
final salesImportActionsProvider = SalesImportActionsProvider._();

final class SalesImportActionsProvider extends $FunctionalProvider<
    SalesImportActions,
    SalesImportActions,
    SalesImportActions> with $Provider<SalesImportActions> {
  SalesImportActionsProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'salesImportActionsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$salesImportActionsHash();

  @$internal
  @override
  $ProviderElement<SalesImportActions> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  SalesImportActions create(Ref ref) {
    return salesImportActions(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SalesImportActions value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SalesImportActions>(value),
    );
  }
}

String _$salesImportActionsHash() =>
    r'cd5dc3342e9fda16d556206f3496c76eea51826a';
