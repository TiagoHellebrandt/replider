// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'canned_response_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(cannedResponseDataSource)
final cannedResponseDataSourceProvider = CannedResponseDataSourceProvider._();

final class CannedResponseDataSourceProvider extends $FunctionalProvider<
        CannedResponseRemoteDataSource,
        CannedResponseRemoteDataSource,
        CannedResponseRemoteDataSource>
    with $Provider<CannedResponseRemoteDataSource> {
  CannedResponseDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'cannedResponseDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$cannedResponseDataSourceHash();

  @$internal
  @override
  $ProviderElement<CannedResponseRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  CannedResponseRemoteDataSource create(Ref ref) {
    return cannedResponseDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(CannedResponseRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride:
          $SyncValueProvider<CannedResponseRemoteDataSource>(value),
    );
  }
}

String _$cannedResponseDataSourceHash() =>
    r'663f67fff2a667bf78b4e8b990d53421a3987bdb';

@ProviderFor(CannedResponseController)
final cannedResponseControllerProvider = CannedResponseControllerProvider._();

final class CannedResponseControllerProvider extends $AsyncNotifierProvider<
    CannedResponseController, List<CannedResponseModel>> {
  CannedResponseControllerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'cannedResponseControllerProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$cannedResponseControllerHash();

  @$internal
  @override
  CannedResponseController create() => CannedResponseController();
}

String _$cannedResponseControllerHash() =>
    r'8e8f12718b45f65ca372ff06852c0bc119f2ed9f';

abstract class _$CannedResponseController
    extends $AsyncNotifier<List<CannedResponseModel>> {
  FutureOr<List<CannedResponseModel>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<List<CannedResponseModel>>,
        List<CannedResponseModel>>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<List<CannedResponseModel>>,
            List<CannedResponseModel>>,
        AsyncValue<List<CannedResponseModel>>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
