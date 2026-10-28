import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/billing/domain/models/usage_model.dart';

void main() {
  group('UsageModel', () {
    final fullJson = <String, dynamic>{
      'ai_tokens_used': 25000,
      'conversations_count': 150,
      'max_ai_tokens_per_month': 500000,
      'max_conversations_per_month': 1000,
    };

    test('fromJson should parse all fields correctly', () {
      final usage = UsageModel.fromJson(fullJson);

      expect(usage.aiTokensUsed, 25000);
      expect(usage.conversationsCount, 150);
      expect(usage.maxAiTokensPerMonth, 500000);
      expect(usage.maxConversationsPerMonth, 1000);
    });

    test('toJson should produce correct snake_case map', () {
      final usage = UsageModel.fromJson(fullJson);
      final json = usage.toJson();

      expect(json['ai_tokens_used'], 25000);
      expect(json['conversations_count'], 150);
      expect(json['max_ai_tokens_per_month'], 500000);
      expect(json['max_conversations_per_month'], 1000);
    });

    test('fromJson and toJson should be symmetric', () {
      final completeJson = <String, dynamic>{
        'ai_tokens_used': 25000,
        'ai_input_tokens_used': 0,
        'ai_output_tokens_used': 0,
        'addon_tokens': 0,
        'effective_limit': 0,
        'conversations_count': 150,
        'max_ai_tokens_per_month': 500000,
        'max_conversations_per_month': 1000,
        'tokens_usage_percent': 0.0,
        'conversations_usage_percent': 0.0,
        'plan_name': 'Trial',
        'is_trialing': false,
        'overage_conversations_count': 0,
        'overage_cents_charged': 0,
        'overage_conversation_cents': 0,
        'cap_reached': false,
        // O consumo do mês em unidades de SDR viaja junto. Null quando o
        // servidor não conseguiu contar — e null é o que precisa sobreviver
        // à ida e volta, porque é o que diz "não sei", não "zero".
        'sdr': null,
      };
      final usage = UsageModel.fromJson(completeJson);
      final roundTripped = usage.toJson();

      expect(roundTripped, completeJson);
    });

    group('overage', () {
      test('isInOverage is false within the plan franchise', () {
        const usage = UsageModel(
          aiTokensUsed: 100,
          conversationsCount: 650,
          maxAiTokensPerMonth: 500000,
          maxConversationsPerMonth: 700,
        );

        expect(usage.isInOverage, false);
      });

      test('isInOverage is true once overage conversations accrue', () {
        const usage = UsageModel(
          aiTokensUsed: 100,
          conversationsCount: 750,
          maxAiTokensPerMonth: 500000,
          maxConversationsPerMonth: 700,
          overageConversationsCount: 50,
        );

        expect(usage.isInOverage, true);
      });

      test('overage fields parse from JSON', () {
        final usage = UsageModel.fromJson(<String, dynamic>{
          'ai_tokens_used': 100,
          'conversations_count': 2000,
          'max_ai_tokens_per_month': 500000,
          'max_conversations_per_month': 700,
          'overage_conversations_count': 1300,
          'overage_cents_charged': 79800,
          'overage_conversation_cents': 70,
          'cap_reached': true,
        });

        expect(usage.isInOverage, true);
        expect(usage.capReached, true);
        expect(usage.overageConversationsCount, 1300);
      });
    });

    test('tokensUsagePercent returns backend value', () {
      const usage = UsageModel(
        aiTokensUsed: 250000,
        conversationsCount: 500,
        maxAiTokensPerMonth: 500000,
        maxConversationsPerMonth: 1000,
        tokensUsagePercent: 50.0,
      );

      expect(usage.tokensUsagePercent, 50.0);
    });

    test('tokensUsageFraction converts percentage to 0-1 range', () {
      const usage = UsageModel(
        aiTokensUsed: 250000,
        conversationsCount: 500,
        maxAiTokensPerMonth: 500000,
        maxConversationsPerMonth: 1000,
        tokensUsagePercent: 50.0,
      );

      expect(usage.tokensUsageFraction, 0.5);
    });

    test('conversationsUsageFraction converts percentage to 0-1 range', () {
      const usage = UsageModel(
        aiTokensUsed: 100000,
        conversationsCount: 750,
        maxAiTokensPerMonth: 500000,
        maxConversationsPerMonth: 1000,
        conversationsUsagePercent: 75.0,
      );

      expect(usage.conversationsUsageFraction, 0.75);
    });

    test('tokensUsagePercent defaults to 0 when not provided', () {
      const usage = UsageModel(
        aiTokensUsed: 100,
        conversationsCount: 10,
        maxAiTokensPerMonth: 0,
        maxConversationsPerMonth: 1000,
      );

      expect(usage.tokensUsagePercent, 0);
    });

    test('conversationsUsagePercent defaults to 0 when not provided', () {
      const usage = UsageModel(
        aiTokensUsed: 100,
        conversationsCount: 10,
        maxAiTokensPerMonth: 500000,
        maxConversationsPerMonth: 0,
      );

      expect(usage.conversationsUsagePercent, 0);
    });

    test('usage percentages can exceed 100 when over limit', () {
      const usage = UsageModel(
        aiTokensUsed: 600000,
        conversationsCount: 1500,
        maxAiTokensPerMonth: 500000,
        maxConversationsPerMonth: 1000,
        tokensUsagePercent: 120.0,
        conversationsUsagePercent: 150.0,
      );

      expect(usage.tokensUsagePercent, 120.0);
      expect(usage.conversationsUsagePercent, 150.0);
    });

    test('planName and isTrialing parsed from JSON', () {
      final json = <String, dynamic>{
        'ai_tokens_used': 100,
        'conversations_count': 5,
        'max_ai_tokens_per_month': 5000,
        'max_conversations_per_month': 50,
        'plan_name': 'Trial',
        'is_trialing': true,
        'tokens_usage_percent': 2.0,
        'conversations_usage_percent': 10.0,
      };
      final usage = UsageModel.fromJson(json);

      expect(usage.planName, 'Trial');
      expect(usage.isTrialing, true);
    });
  });
}
