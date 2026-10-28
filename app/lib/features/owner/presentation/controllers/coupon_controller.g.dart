// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'coupon_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(couponDataSource)
final couponDataSourceProvider = CouponDataSourceProvider._();

final class CouponDataSourceProvider extends $FunctionalProvider<
    CouponRemoteDataSource,
    CouponRemoteDataSource,
    CouponRemoteDataSource> with $Provider<CouponRemoteDataSource> {
  CouponDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'couponDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$couponDataSourceHash();

  @$internal
  @override
  $ProviderElement<CouponRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  CouponRemoteDataSource create(Ref ref) {
    return couponDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(CouponRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<CouponRemoteDataSource>(value),
    );
  }
}

String _$couponDataSourceHash() => r'9a512345f110b84daae5f3d25a851ee95a160a43';

@ProviderFor(ownerCoupons)
final ownerCouponsProvider = OwnerCouponsProvider._();

final class OwnerCouponsProvider extends $FunctionalProvider<
        AsyncValue<OwnerCouponsResponse>,
        OwnerCouponsResponse,
        FutureOr<OwnerCouponsResponse>>
    with
        $FutureModifier<OwnerCouponsResponse>,
        $FutureProvider<OwnerCouponsResponse> {
  OwnerCouponsProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'ownerCouponsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$ownerCouponsHash();

  @$internal
  @override
  $FutureProviderElement<OwnerCouponsResponse> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<OwnerCouponsResponse> create(Ref ref) {
    return ownerCoupons(ref);
  }
}

String _$ownerCouponsHash() => r'839d66d705646d26206e78c287aec97dfa07829c';

/// Public lookup keyed by code. Used by the register / billing page
/// to render the "X% off applied" banner before the user is signed in.

@ProviderFor(couponPublicLookup)
final couponPublicLookupProvider = CouponPublicLookupFamily._();

/// Public lookup keyed by code. Used by the register / billing page
/// to render the "X% off applied" banner before the user is signed in.

final class CouponPublicLookupProvider extends $FunctionalProvider<
        AsyncValue<CouponPublicLookup?>,
        CouponPublicLookup?,
        FutureOr<CouponPublicLookup?>>
    with
        $FutureModifier<CouponPublicLookup?>,
        $FutureProvider<CouponPublicLookup?> {
  /// Public lookup keyed by code. Used by the register / billing page
  /// to render the "X% off applied" banner before the user is signed in.
  CouponPublicLookupProvider._(
      {required CouponPublicLookupFamily super.from,
      required String? super.argument})
      : super(
          retry: null,
          name: r'couponPublicLookupProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$couponPublicLookupHash();

  @override
  String toString() {
    return r'couponPublicLookupProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<CouponPublicLookup?> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<CouponPublicLookup?> create(Ref ref) {
    final argument = this.argument as String?;
    return couponPublicLookup(
      ref,
      argument,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is CouponPublicLookupProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$couponPublicLookupHash() =>
    r'69e262859dbce52f4e672e126bca1341b1f81fe9';

/// Public lookup keyed by code. Used by the register / billing page
/// to render the "X% off applied" banner before the user is signed in.

final class CouponPublicLookupFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<CouponPublicLookup?>, String?> {
  CouponPublicLookupFamily._()
      : super(
          retry: null,
          name: r'couponPublicLookupProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  /// Public lookup keyed by code. Used by the register / billing page
  /// to render the "X% off applied" banner before the user is signed in.

  CouponPublicLookupProvider call(
    String? code,
  ) =>
      CouponPublicLookupProvider._(argument: code, from: this);

  @override
  String toString() => r'couponPublicLookupProvider';
}

@ProviderFor(CouponActions)
final couponActionsProvider = CouponActionsProvider._();

final class CouponActionsProvider
    extends $NotifierProvider<CouponActions, void> {
  CouponActionsProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'couponActionsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$couponActionsHash();

  @$internal
  @override
  CouponActions create() => CouponActions();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$couponActionsHash() => r'8614f87cbc1416b67030981224bcdd377a0c8d67';

abstract class _$CouponActions extends $Notifier<void> {
  void build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<void, void>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<void, void>, void, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}
