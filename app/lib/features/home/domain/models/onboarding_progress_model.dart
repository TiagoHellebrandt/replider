import 'package:json_annotation/json_annotation.dart';

part 'onboarding_progress_model.g.dart';

/// Dashboard-side representation of /v1/me/onboarding-progress.
/// Fields mirror the Go domain.OnboardingProgress struct.
@JsonSerializable(fieldRename: FieldRename.snake)
class OnboardingProgressModel {
  final bool agentCreated;
  final bool previewChatTested;
  final bool channelConnected;
  final bool knowledgeAdded;

  /// The SALES milestones — what the checklist counts and shows. The three above
  /// belong to the chatbot side of the product; they still arrive because they
  /// are real facts and the `full` onboarding cohort renders them, but a
  /// checklist built from them nagged a new account to build a bot it will never
  /// use.
  ///
  /// Defaulted so a server that has not shipped them yet reads as "not done"
  /// rather than throwing — the same posture every other model here takes.
  final bool productsAdded;
  final bool customersAdded;
  final bool proposalMade;

  /// Somebody besides the owner has a seat (2026-08-18). The seller who
  /// approves the offers is usually not the person who signed up.
  final bool teamInvited;
  final bool firstInboundReceived;
  final bool onboardingCompleted;
  final DateTime? firstInboundAt;
  final DateTime? checklistDismissedAt;
  final int completionPercentage;

  const OnboardingProgressModel({
    required this.agentCreated,
    required this.previewChatTested,
    required this.channelConnected,
    required this.knowledgeAdded,
    this.productsAdded = false,
    this.customersAdded = false,
    this.proposalMade = false,
    this.teamInvited = false,
    required this.firstInboundReceived,
    required this.onboardingCompleted,
    this.firstInboundAt,
    this.checklistDismissedAt,
    required this.completionPercentage,
  });

  factory OnboardingProgressModel.fromJson(Map<String, dynamic> json) =>
      _$OnboardingProgressModelFromJson(json);

  /// True while the "Dispensar por enquanto" pause is still in effect
  /// (24h window). Keeps the card hidden without having to mutate
  /// anything else in the client.
  bool get isDismissActive {
    final at = checklistDismissedAt;
    if (at == null) return false;
    return DateTime.now().difference(at) < const Duration(hours: 24);
  }
}
