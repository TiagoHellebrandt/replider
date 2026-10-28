// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'first_offer_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Asks the server for the first offer once per wizard visit.
///
/// A POST, not a GET, because the server may create a lead and spend a
/// draft to answer — but the server is idempotent (a reload gets the same
/// offer), so retrying after a failure is safe and the step's Retry button
/// simply invalidates this.

@ProviderFor(FirstOfferController)
final firstOfferControllerProvider = FirstOfferControllerProvider._();

/// Asks the server for the first offer once per wizard visit.
///
/// A POST, not a GET, because the server may create a lead and spend a
/// draft to answer — but the server is idempotent (a reload gets the same
/// offer), so retrying after a failure is safe and the step's Retry button
/// simply invalidates this.
final class FirstOfferControllerProvider
    extends $AsyncNotifierProvider<FirstOfferController, FirstOffer> {
  /// Asks the server for the first offer once per wizard visit.
  ///
  /// A POST, not a GET, because the server may create a lead and spend a
  /// draft to answer — but the server is idempotent (a reload gets the same
  /// offer), so retrying after a failure is safe and the step's Retry button
  /// simply invalidates this.
  FirstOfferControllerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'firstOfferControllerProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$firstOfferControllerHash();

  @$internal
  @override
  FirstOfferController create() => FirstOfferController();
}

String _$firstOfferControllerHash() =>
    r'e99711a80be0e184a6afaabad2380b90eeb47a3d';

/// Asks the server for the first offer once per wizard visit.
///
/// A POST, not a GET, because the server may create a lead and spend a
/// draft to answer — but the server is idempotent (a reload gets the same
/// offer), so retrying after a failure is safe and the step's Retry button
/// simply invalidates this.

abstract class _$FirstOfferController extends $AsyncNotifier<FirstOffer> {
  FutureOr<FirstOffer> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<FirstOffer>, FirstOffer>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<FirstOffer>, FirstOffer>,
        AsyncValue<FirstOffer>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
