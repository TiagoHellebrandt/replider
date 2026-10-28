// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'cancellation_survey_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Submits the cancellation-reason survey (US-180). Action-only controller —
/// the survey is triggered by the Stripe Portal cancel return URL
/// (`/billing?cancel_survey=1`), so there is no persisted "already handled"
/// state to track.

@ProviderFor(CancellationSurveyController)
final cancellationSurveyControllerProvider =
    CancellationSurveyControllerProvider._();

/// Submits the cancellation-reason survey (US-180). Action-only controller —
/// the survey is triggered by the Stripe Portal cancel return URL
/// (`/billing?cancel_survey=1`), so there is no persisted "already handled"
/// state to track.
final class CancellationSurveyControllerProvider
    extends $AsyncNotifierProvider<CancellationSurveyController, void> {
  /// Submits the cancellation-reason survey (US-180). Action-only controller —
  /// the survey is triggered by the Stripe Portal cancel return URL
  /// (`/billing?cancel_survey=1`), so there is no persisted "already handled"
  /// state to track.
  CancellationSurveyControllerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'cancellationSurveyControllerProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$cancellationSurveyControllerHash();

  @$internal
  @override
  CancellationSurveyController create() => CancellationSurveyController();
}

String _$cancellationSurveyControllerHash() =>
    r'b3d9162094e19c42e84b7aa70d7bf67ef6174780';

/// Submits the cancellation-reason survey (US-180). Action-only controller —
/// the survey is triggered by the Stripe Portal cancel return URL
/// (`/billing?cancel_survey=1`), so there is no persisted "already handled"
/// state to track.

abstract class _$CancellationSurveyController extends $AsyncNotifier<void> {
  FutureOr<void> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<void>, void>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<void>, void>,
        AsyncValue<void>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
