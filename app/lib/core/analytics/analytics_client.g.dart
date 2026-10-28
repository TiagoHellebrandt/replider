// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'analytics_client.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(analyticsClient)
final analyticsClientProvider = AnalyticsClientProvider._();

final class AnalyticsClientProvider extends $FunctionalProvider<AnalyticsClient,
    AnalyticsClient, AnalyticsClient> with $Provider<AnalyticsClient> {
  AnalyticsClientProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'analyticsClientProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$analyticsClientHash();

  @$internal
  @override
  $ProviderElement<AnalyticsClient> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AnalyticsClient create(Ref ref) {
    return analyticsClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AnalyticsClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AnalyticsClient>(value),
    );
  }
}

String _$analyticsClientHash() => r'e65be987ee02875545dfbd4f17b680113ea4676c';
