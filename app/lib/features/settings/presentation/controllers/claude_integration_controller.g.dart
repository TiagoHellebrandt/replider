// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'claude_integration_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(claudeIntegrationRemoteDataSource)
final claudeIntegrationRemoteDataSourceProvider =
    ClaudeIntegrationRemoteDataSourceProvider._();

final class ClaudeIntegrationRemoteDataSourceProvider
    extends $FunctionalProvider<ClaudeIntegrationRemoteDataSource,
        ClaudeIntegrationRemoteDataSource, ClaudeIntegrationRemoteDataSource>
    with $Provider<ClaudeIntegrationRemoteDataSource> {
  ClaudeIntegrationRemoteDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'claudeIntegrationRemoteDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() =>
      _$claudeIntegrationRemoteDataSourceHash();

  @$internal
  @override
  $ProviderElement<ClaudeIntegrationRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  ClaudeIntegrationRemoteDataSource create(Ref ref) {
    return claudeIntegrationRemoteDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ClaudeIntegrationRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride:
          $SyncValueProvider<ClaudeIntegrationRemoteDataSource>(value),
    );
  }
}

String _$claudeIntegrationRemoteDataSourceHash() =>
    r'67e755551504dbd9ab9ca64ab45aed1805c706b1';

@ProviderFor(ClaudeIntegration)
final claudeIntegrationProvider = ClaudeIntegrationProvider._();

final class ClaudeIntegrationProvider
    extends $AsyncNotifierProvider<ClaudeIntegration, ClaudeIntegrationState> {
  ClaudeIntegrationProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'claudeIntegrationProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$claudeIntegrationHash();

  @$internal
  @override
  ClaudeIntegration create() => ClaudeIntegration();
}

String _$claudeIntegrationHash() => r'c56282bc222f5ae7b5ed237ca68da80dd0224132';

abstract class _$ClaudeIntegration
    extends $AsyncNotifier<ClaudeIntegrationState> {
  FutureOr<ClaudeIntegrationState> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref
        as $Ref<AsyncValue<ClaudeIntegrationState>, ClaudeIntegrationState>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<ClaudeIntegrationState>, ClaudeIntegrationState>,
        AsyncValue<ClaudeIntegrationState>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
