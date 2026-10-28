// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'tag_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(tagDataSource)
final tagDataSourceProvider = TagDataSourceProvider._();

final class TagDataSourceProvider extends $FunctionalProvider<
    TagRemoteDataSource,
    TagRemoteDataSource,
    TagRemoteDataSource> with $Provider<TagRemoteDataSource> {
  TagDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'tagDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$tagDataSourceHash();

  @$internal
  @override
  $ProviderElement<TagRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  TagRemoteDataSource create(Ref ref) {
    return tagDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(TagRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<TagRemoteDataSource>(value),
    );
  }
}

String _$tagDataSourceHash() => r'9dc5e725681b50b4693a0b87ab7866f72e8f8016';

@ProviderFor(TagController)
final tagControllerProvider = TagControllerProvider._();

final class TagControllerProvider
    extends $AsyncNotifierProvider<TagController, List<TagModel>> {
  TagControllerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'tagControllerProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$tagControllerHash();

  @$internal
  @override
  TagController create() => TagController();
}

String _$tagControllerHash() => r'dcb3825abfc836c067106ae9396a8e1224f65e66';

abstract class _$TagController extends $AsyncNotifier<List<TagModel>> {
  FutureOr<List<TagModel>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<List<TagModel>>, List<TagModel>>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<List<TagModel>>, List<TagModel>>,
        AsyncValue<List<TagModel>>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
