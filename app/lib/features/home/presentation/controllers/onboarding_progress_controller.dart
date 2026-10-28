import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/home/domain/models/onboarding_progress_model.dart';

part 'onboarding_progress_controller.g.dart';

/// Fetches and refreshes the onboarding checklist data for the logged-in
/// account. Kept alive so the card can survive widget re-mounts (tab
/// switches within the dashboard) without re-fetching.
///
/// Refreshes are triggered explicitly by:
///  * The "Dispensar por enquanto" action (invalidate after POST).
///  * The existing WebSocket `new_message` event path, which is already
///    the de-facto "something happened" signal — the dashboard invalidates
///    this provider alongside its conversation list so first_inbound and
///    channel_connected flip visibly without a full reload.
@riverpod
class OnboardingProgressController extends _$OnboardingProgressController {
  @override
  Future<OnboardingProgressModel?> build() async {
    return _fetch();
  }

  Future<OnboardingProgressModel?> _fetch() async {
    try {
      final dio = ref.read(dioProvider);
      final res = await dio.get<dynamic>('/me/onboarding-progress');
      final data = res.data;
      if (data is Map<String, dynamic>) {
        return OnboardingProgressModel.fromJson(data);
      }
      return null;
    } catch (_) {
      // Best-effort — a failing checklist must not break the dashboard.
      return null;
    }
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = AsyncValue.data(await _fetch());
  }

  /// Hide the checklist card for the next 24h. Clears the cached state
  /// optimistically so the user sees the card disappear immediately,
  /// then reconciles with the server.
  Future<void> dismissForNow() async {
    final current = state.asData?.value;
    try {
      final dio = ref.read(dioProvider);
      await dio.post<dynamic>('/me/onboarding-checklist/dismiss');
    } catch (_) {
      // Ignore — dismiss is not critical enough to surface an error to
      // the user. They can click again next session.
    }
    if (current != null) {
      state = AsyncValue.data(OnboardingProgressModel(
        agentCreated: current.agentCreated,
        previewChatTested: current.previewChatTested,
        channelConnected: current.channelConnected,
        knowledgeAdded: current.knowledgeAdded,
        firstInboundReceived: current.firstInboundReceived,
        onboardingCompleted: current.onboardingCompleted,
        firstInboundAt: current.firstInboundAt,
        checklistDismissedAt: DateTime.now(),
        completionPercentage: current.completionPercentage,
      ));
    }
  }
}
