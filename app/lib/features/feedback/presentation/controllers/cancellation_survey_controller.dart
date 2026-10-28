import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';

part 'cancellation_survey_controller.g.dart';

/// The closed set of churn reasons offered by the cancellation survey
/// (US-180). Keys MUST match the API whitelist in
/// `domain/cancellation_survey.go`; labels are resolved via i18n
/// (`feedback.cancel_reasons.<key>`).
const cancellationReasonKeys = <String>[
  'too_expensive',
  'missing_features',
  'not_using',
  'switched_competitor',
  'technical_issues',
  'temporary_pause',
  'other',
];

/// Submits the cancellation-reason survey (US-180). Action-only controller —
/// the survey is triggered by the Stripe Portal cancel return URL
/// (`/billing?cancel_survey=1`), so there is no persisted "already handled"
/// state to track.
@riverpod
class CancellationSurveyController extends _$CancellationSurveyController {
  @override
  FutureOr<void> build() {}

  /// POSTs the structured reason plus an optional comment to the API. Returns
  /// `null` on success, or a localised error string the modal can surface
  /// while staying open.
  Future<String?> submit(String reason, String comment) async {
    try {
      final dio = ref.read(dioProvider);
      await dio.post<dynamic>('/me/cancellation-survey', data: {
        'reason': reason,
        'comment': comment,
      });
    } catch (e) {
      return parseApiError(e);
    }
    return null;
  }
}
