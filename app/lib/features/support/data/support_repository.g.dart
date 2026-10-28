// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'support_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(supportRepository)
final supportRepositoryProvider = SupportRepositoryProvider._();

final class SupportRepositoryProvider extends $FunctionalProvider<
    SupportRepository,
    SupportRepository,
    SupportRepository> with $Provider<SupportRepository> {
  SupportRepositoryProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'supportRepositoryProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$supportRepositoryHash();

  @$internal
  @override
  $ProviderElement<SupportRepository> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  SupportRepository create(Ref ref) {
    return supportRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SupportRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SupportRepository>(value),
    );
  }
}

String _$supportRepositoryHash() => r'ff26efd5973e0d881433d4d9a2cb0f7b16129087';
