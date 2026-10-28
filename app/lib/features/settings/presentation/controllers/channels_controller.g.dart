// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channels_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(channelsRemoteDataSource)
final channelsRemoteDataSourceProvider = ChannelsRemoteDataSourceProvider._();

final class ChannelsRemoteDataSourceProvider extends $FunctionalProvider<
    ChannelsRemoteDataSource,
    ChannelsRemoteDataSource,
    ChannelsRemoteDataSource> with $Provider<ChannelsRemoteDataSource> {
  ChannelsRemoteDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'channelsRemoteDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$channelsRemoteDataSourceHash();

  @$internal
  @override
  $ProviderElement<ChannelsRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  ChannelsRemoteDataSource create(Ref ref) {
    return channelsRemoteDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ChannelsRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ChannelsRemoteDataSource>(value),
    );
  }
}

String _$channelsRemoteDataSourceHash() =>
    r'edc790a89fce81c2d124771bd3ba0d60fb02c915';

@ProviderFor(ChannelsList)
final channelsListProvider = ChannelsListProvider._();

final class ChannelsListProvider
    extends $AsyncNotifierProvider<ChannelsList, List<ChannelModel>> {
  ChannelsListProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'channelsListProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$channelsListHash();

  @$internal
  @override
  ChannelsList create() => ChannelsList();
}

String _$channelsListHash() => r'b64c9de743e2bfef77457f13fa104955696185a5';

abstract class _$ChannelsList extends $AsyncNotifier<List<ChannelModel>> {
  FutureOr<List<ChannelModel>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref as $Ref<AsyncValue<List<ChannelModel>>, List<ChannelModel>>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<List<ChannelModel>>, List<ChannelModel>>,
        AsyncValue<List<ChannelModel>>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}

/// Thin wrapper around the existing getWebhookStatus call that exposes
/// just the is_healthy flag — enough to render a green/red badge on
/// the channel list without baking the full response shape into the
/// UI. AutoDispose so leaving the screen stops polling; the card
/// itself re-subscribes when re-rendered.

@ProviderFor(channelWebhookHealth)
final channelWebhookHealthProvider = ChannelWebhookHealthFamily._();

/// Thin wrapper around the existing getWebhookStatus call that exposes
/// just the is_healthy flag — enough to render a green/red badge on
/// the channel list without baking the full response shape into the
/// UI. AutoDispose so leaving the screen stops polling; the card
/// itself re-subscribes when re-rendered.

final class ChannelWebhookHealthProvider
    extends $FunctionalProvider<AsyncValue<bool?>, bool?, FutureOr<bool?>>
    with $FutureModifier<bool?>, $FutureProvider<bool?> {
  /// Thin wrapper around the existing getWebhookStatus call that exposes
  /// just the is_healthy flag — enough to render a green/red badge on
  /// the channel list without baking the full response shape into the
  /// UI. AutoDispose so leaving the screen stops polling; the card
  /// itself re-subscribes when re-rendered.
  ChannelWebhookHealthProvider._(
      {required ChannelWebhookHealthFamily super.from,
      required String super.argument})
      : super(
          retry: null,
          name: r'channelWebhookHealthProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$channelWebhookHealthHash();

  @override
  String toString() {
    return r'channelWebhookHealthProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<bool?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<bool?> create(Ref ref) {
    final argument = this.argument as String;
    return channelWebhookHealth(
      ref,
      argument,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ChannelWebhookHealthProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$channelWebhookHealthHash() =>
    r'aa738a9b45a3f3855c4f19a625b3b13b0fea31dc';

/// Thin wrapper around the existing getWebhookStatus call that exposes
/// just the is_healthy flag — enough to render a green/red badge on
/// the channel list without baking the full response shape into the
/// UI. AutoDispose so leaving the screen stops polling; the card
/// itself re-subscribes when re-rendered.

final class ChannelWebhookHealthFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<bool?>, String> {
  ChannelWebhookHealthFamily._()
      : super(
          retry: null,
          name: r'channelWebhookHealthProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  /// Thin wrapper around the existing getWebhookStatus call that exposes
  /// just the is_healthy flag — enough to render a green/red badge on
  /// the channel list without baking the full response shape into the
  /// UI. AutoDispose so leaving the screen stops polling; the card
  /// itself re-subscribes when re-rendered.

  ChannelWebhookHealthProvider call(
    String channelId,
  ) =>
      ChannelWebhookHealthProvider._(argument: channelId, from: this);

  @override
  String toString() => r'channelWebhookHealthProvider';
}
