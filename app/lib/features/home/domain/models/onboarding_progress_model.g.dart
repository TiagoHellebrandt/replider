// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'onboarding_progress_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

OnboardingProgressModel _$OnboardingProgressModelFromJson(
        Map<String, dynamic> json) =>
    OnboardingProgressModel(
      agentCreated: json['agent_created'] as bool,
      previewChatTested: json['preview_chat_tested'] as bool,
      channelConnected: json['channel_connected'] as bool,
      knowledgeAdded: json['knowledge_added'] as bool,
      productsAdded: json['products_added'] as bool? ?? false,
      customersAdded: json['customers_added'] as bool? ?? false,
      proposalMade: json['proposal_made'] as bool? ?? false,
      teamInvited: json['team_invited'] as bool? ?? false,
      firstInboundReceived: json['first_inbound_received'] as bool,
      onboardingCompleted: json['onboarding_completed'] as bool,
      firstInboundAt: json['first_inbound_at'] == null
          ? null
          : DateTime.parse(json['first_inbound_at'] as String),
      checklistDismissedAt: json['checklist_dismissed_at'] == null
          ? null
          : DateTime.parse(json['checklist_dismissed_at'] as String),
      completionPercentage: (json['completion_percentage'] as num).toInt(),
    );

Map<String, dynamic> _$OnboardingProgressModelToJson(
        OnboardingProgressModel instance) =>
    <String, dynamic>{
      'agent_created': instance.agentCreated,
      'preview_chat_tested': instance.previewChatTested,
      'channel_connected': instance.channelConnected,
      'knowledge_added': instance.knowledgeAdded,
      'products_added': instance.productsAdded,
      'customers_added': instance.customersAdded,
      'proposal_made': instance.proposalMade,
      'team_invited': instance.teamInvited,
      'first_inbound_received': instance.firstInboundReceived,
      'onboarding_completed': instance.onboardingCompleted,
      'first_inbound_at': instance.firstInboundAt?.toIso8601String(),
      'checklist_dismissed_at':
          instance.checklistDismissedAt?.toIso8601String(),
      'completion_percentage': instance.completionPercentage,
    };
