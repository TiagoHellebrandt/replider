// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'funnels_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Account-scoped list of sales funnels (US-006/US-009), kept alive so the list
/// survives navigation between the list screen and the config screen. Reuses the
/// shared [salesRemoteDataSourceProvider] from [ProductsList].

@ProviderFor(FunnelsList)
final funnelsListProvider = FunnelsListProvider._();

/// Account-scoped list of sales funnels (US-006/US-009), kept alive so the list
/// survives navigation between the list screen and the config screen. Reuses the
/// shared [salesRemoteDataSourceProvider] from [ProductsList].
final class FunnelsListProvider
    extends $AsyncNotifierProvider<FunnelsList, List<FunnelModel>> {
  /// Account-scoped list of sales funnels (US-006/US-009), kept alive so the list
  /// survives navigation between the list screen and the config screen. Reuses the
  /// shared [salesRemoteDataSourceProvider] from [ProductsList].
  FunnelsListProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'funnelsListProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$funnelsListHash();

  @$internal
  @override
  FunnelsList create() => FunnelsList();
}

String _$funnelsListHash() => r'cb1068da16da43113b912dd16e05a366665bfb56';

/// Account-scoped list of sales funnels (US-006/US-009), kept alive so the list
/// survives navigation between the list screen and the config screen. Reuses the
/// shared [salesRemoteDataSourceProvider] from [ProductsList].

abstract class _$FunnelsList extends $AsyncNotifier<List<FunnelModel>> {
  FutureOr<List<FunnelModel>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref as $Ref<AsyncValue<List<FunnelModel>>, List<FunnelModel>>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<List<FunnelModel>>, List<FunnelModel>>,
        AsyncValue<List<FunnelModel>>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}

/// Single funnel by id (US-024). Used by the lead approach card to resolve the
/// funnel's channel so it can list that channel's approved WhatsApp templates
/// for the human-in-the-loop send. Cheap GET; autoDispose.

@ProviderFor(funnel)
final funnelProvider = FunnelFamily._();

/// Single funnel by id (US-024). Used by the lead approach card to resolve the
/// funnel's channel so it can list that channel's approved WhatsApp templates
/// for the human-in-the-loop send. Cheap GET; autoDispose.

final class FunnelProvider extends $FunctionalProvider<AsyncValue<FunnelModel>,
        FunnelModel, FutureOr<FunnelModel>>
    with $FutureModifier<FunnelModel>, $FutureProvider<FunnelModel> {
  /// Single funnel by id (US-024). Used by the lead approach card to resolve the
  /// funnel's channel so it can list that channel's approved WhatsApp templates
  /// for the human-in-the-loop send. Cheap GET; autoDispose.
  FunnelProvider._(
      {required FunnelFamily super.from, required String super.argument})
      : super(
          retry: null,
          name: r'funnelProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$funnelHash();

  @override
  String toString() {
    return r'funnelProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<FunnelModel> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<FunnelModel> create(Ref ref) {
    final argument = this.argument as String;
    return funnel(
      ref,
      argument,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is FunnelProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$funnelHash() => r'1860dc5f8e460523d7098ec5d4c4d9dde36799b5';

/// Single funnel by id (US-024). Used by the lead approach card to resolve the
/// funnel's channel so it can list that channel's approved WhatsApp templates
/// for the human-in-the-loop send. Cheap GET; autoDispose.

final class FunnelFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<FunnelModel>, String> {
  FunnelFamily._()
      : super(
          retry: null,
          name: r'funnelProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  /// Single funnel by id (US-024). Used by the lead approach card to resolve the
  /// funnel's channel so it can list that channel's approved WhatsApp templates
  /// for the human-in-the-loop send. Cheap GET; autoDispose.

  FunnelProvider call(
    String id,
  ) =>
      FunnelProvider._(argument: id, from: this);

  @override
  String toString() => r'funnelProvider';
}
