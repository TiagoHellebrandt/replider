import 'package:flutter/foundation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'analytics_client.g.dart';

/// Thin telemetry surface for product events. Stays transport-agnostic
/// on purpose — no SDKs in this repo yet — so the call sites can land
/// before we pick PostHog / Mixpanel / Amplitude.
///
/// To plug a real provider in:
///   1. Implement [AnalyticsClient] with whatever SDK you picked.
///   2. Override the provider below (e.g. in main.dart) with the real
///      instance; nothing else changes, call sites keep using
///      `ref.read(analyticsClientProvider).track(...)`.
abstract class AnalyticsClient {
  /// Emit an event with optional properties. Must never throw — the
  /// UI is not allowed to degrade on analytics failure.
  void track(String event, [Map<String, Object?> props = const {}]);
}

/// Default implementation: debugPrint in dev builds, no-op otherwise.
/// Safe to ship with — doesn't leak PII anywhere, doesn't hit the
/// network, doesn't affect users.
class DebugPrintAnalyticsClient implements AnalyticsClient {
  const DebugPrintAnalyticsClient();

  @override
  void track(String event, [Map<String, Object?> props = const {}]) {
    if (!kDebugMode) return;
    if (props.isEmpty) {
      debugPrint('[analytics] $event');
      return;
    }
    debugPrint('[analytics] $event ${props.toString()}');
  }
}

@Riverpod(keepAlive: true)
AnalyticsClient analyticsClient(Ref ref) {
  return const DebugPrintAnalyticsClient();
}

/// Constants so call sites don't sprinkle raw strings. Keeping them in
/// one place also makes it easy to grep "onboarding_*" when we wire a
/// real provider and need to register the schema upstream.
class OnboardingEvents {
  OnboardingEvents._();

  static const stepViewed = 'onboarding_step_viewed';
  static const stepCompleted = 'onboarding_step_completed';
  static const stepSkipped = 'onboarding_step_skipped';
  static const previewChatMessageSent = 'onboarding_preview_chat_message_sent';
  static const previewChatFirstResponse =
      'onboarding_preview_chat_received_first_response';
  static const channelSelected = 'onboarding_channel_selected';
  static const channelConnected = 'onboarding_channel_connected';
  static const knowledgeAdded = 'onboarding_knowledge_added';
  static const checklistItemClicked = 'onboarding_checklist_item_clicked';
  static const checklistDismissed = 'onboarding_checklist_dismissed';
}
