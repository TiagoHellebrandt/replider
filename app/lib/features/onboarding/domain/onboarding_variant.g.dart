// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'onboarding_variant.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The active onboarding flow. Only [OnboardingVariant.reduced] exists now;
/// the provider stays so tests and the screen keep a single seam.

@ProviderFor(onboardingVariant)
final onboardingVariantProvider = OnboardingVariantProvider._();

/// The active onboarding flow. Only [OnboardingVariant.reduced] exists now;
/// the provider stays so tests and the screen keep a single seam.

final class OnboardingVariantProvider extends $FunctionalProvider<
    OnboardingVariant,
    OnboardingVariant,
    OnboardingVariant> with $Provider<OnboardingVariant> {
  /// The active onboarding flow. Only [OnboardingVariant.reduced] exists now;
  /// the provider stays so tests and the screen keep a single seam.
  OnboardingVariantProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'onboardingVariantProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$onboardingVariantHash();

  @$internal
  @override
  $ProviderElement<OnboardingVariant> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  OnboardingVariant create(Ref ref) {
    return onboardingVariant(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(OnboardingVariant value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<OnboardingVariant>(value),
    );
  }
}

String _$onboardingVariantHash() => r'c10cfc754a2393b5452bedac4d2baa594df075f4';
