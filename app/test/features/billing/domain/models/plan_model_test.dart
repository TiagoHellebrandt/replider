import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/billing/domain/models/plan_model.dart';

void main() {
  group('PlanModel', () {
    final fullJson = <String, dynamic>{
      'id': 'plan-1',
      'name': 'Pro',
      'price': 4990,
      'currency': 'BRL',
      'ai_provider': 'gemini',
      'ai_model': 'gemini-2.0-flash',
      'input_multiplier': 1.5,
      'output_multiplier': 2.0,
      'max_agents': 5,
      'max_channels': 10,
      'max_seats': 5,
      'max_conversations_per_month': 1000,
      'max_ai_tokens_per_month': 500000,
      'max_storage_mb': 1024,
      'max_broadcast_messages_per_month': 25000,
      'max_recipients_per_campaign': 10000,
      'overage_conversation_cents': 50,
      'overage_cap_multiplier': 2.0,
      'stripe_price_id': null,
      'stripe_price_id_annual': null,
      'created_at': '2026-01-01T00:00:00Z',
      'updated_at': '2026-01-02T00:00:00Z',
    };

    test('fromJson should parse all fields correctly', () {
      final plan = PlanModel.fromJson(fullJson);

      expect(plan.id, 'plan-1');
      expect(plan.name, 'Pro');
      expect(plan.price, 4990);
      expect(plan.currency, 'BRL');
      expect(plan.aiProvider, 'gemini');
      expect(plan.aiModel, 'gemini-2.0-flash');
      expect(plan.inputMultiplier, 1.5);
      expect(plan.outputMultiplier, 2.0);
      expect(plan.maxAgents, 5);
      expect(plan.maxChannels, 10);
      expect(plan.maxSeats, 5);
      expect(plan.maxConversationsPerMonth, 1000);
      expect(plan.maxAiTokensPerMonth, 500000);
      expect(plan.maxStorageMb, 1024);
      expect(plan.maxBroadcastMessagesPerMonth, 25000);
      expect(plan.maxRecipientsPerCampaign, 10000);
      expect(plan.overageConversationCents, 50);
      expect(plan.overageCapMultiplier, 2.0);
      expect(plan.createdAt, '2026-01-01T00:00:00Z');
      expect(plan.updatedAt, '2026-01-02T00:00:00Z');
    });

    test('fromJson defaults missing optional fields to 0', () {
      final minimalJson = Map<String, dynamic>.from(fullJson)
        ..remove('max_seats')
        ..remove('max_broadcast_messages_per_month')
        ..remove('max_recipients_per_campaign')
        ..remove('overage_conversation_cents')
        ..remove('overage_cap_multiplier');
      final plan = PlanModel.fromJson(minimalJson);
      expect(plan.maxSeats, 0);
      expect(plan.maxBroadcastMessagesPerMonth, 0);
      expect(plan.maxRecipientsPerCampaign, 0);
      expect(plan.overageConversationCents, 0);
      expect(plan.overageCapMultiplier, 0.0);
    });

    test('toJson should produce correct snake_case map', () {
      final plan = PlanModel.fromJson(fullJson);
      final json = plan.toJson();

      expect(json['id'], 'plan-1');
      expect(json['name'], 'Pro');
      expect(json['price'], 4990);
      expect(json['currency'], 'BRL');
      expect(json['ai_provider'], 'gemini');
      expect(json['ai_model'], 'gemini-2.0-flash');
      expect(json['input_multiplier'], 1.5);
      expect(json['output_multiplier'], 2.0);
      expect(json['max_agents'], 5);
      expect(json['max_channels'], 10);
      expect(json['max_seats'], 5);
      expect(json['max_conversations_per_month'], 1000);
      expect(json['max_ai_tokens_per_month'], 500000);
      expect(json['max_storage_mb'], 1024);
      expect(json['max_broadcast_messages_per_month'], 25000);
      expect(json['max_recipients_per_campaign'], 10000);
      expect(json['overage_conversation_cents'], 50);
      expect(json['overage_cap_multiplier'], 2.0);
      expect(json['created_at'], '2026-01-01T00:00:00Z');
      expect(json['updated_at'], '2026-01-02T00:00:00Z');
    });

    test('fromJson and toJson should be symmetric', () {
      final plan = PlanModel.fromJson(fullJson);
      final roundTripped = plan.toJson();

      expect(roundTripped, fullJson);
    });

    test('formattedPrice should format BRL correctly', () {
      const plan = PlanModel(
        id: 'plan-brl',
        name: 'Basic',
        price: 4990,
        currency: 'BRL',
        aiProvider: 'gemini',
        aiModel: 'gemini-2.0-flash',
        inputMultiplier: 1.0,
        outputMultiplier: 1.0,
        maxAgents: 1,
        maxChannels: 1,
        maxConversationsPerMonth: 100,
        maxAiTokensPerMonth: 10000,
        maxStorageMb: 256,
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
      );

      expect(plan.formattedPrice, 'R\$ 49,90');
    });

    test('formattedPrice should format USD correctly', () {
      const plan = PlanModel(
        id: 'plan-usd',
        name: 'Basic',
        price: 4990,
        currency: 'USD',
        aiProvider: 'openai',
        aiModel: 'gpt-4o',
        inputMultiplier: 1.0,
        outputMultiplier: 1.0,
        maxAgents: 1,
        maxChannels: 1,
        maxConversationsPerMonth: 100,
        maxAiTokensPerMonth: 10000,
        maxStorageMb: 256,
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
      );

      expect(plan.formattedPrice, '\$49.90');
    });

    // US-169: annual billing shows a 20%-off monthly-equivalent price.
    test('formattedAnnualMonthlyPrice applies 20% discount (BRL)', () {
      const plan = PlanModel(
        id: 'plan-annual',
        name: 'Pro',
        price: 4990, // R$ 49,90/mo → 20% off = R$ 39,92/mo
        currency: 'BRL',
        aiProvider: 'gemini',
        aiModel: 'gemini-2.0-flash',
        inputMultiplier: 1.0,
        outputMultiplier: 1.0,
        maxAgents: 1,
        maxChannels: 1,
        maxConversationsPerMonth: 100,
        maxAiTokensPerMonth: 10000,
        maxStorageMb: 256,
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
      );

      expect(plan.formattedAnnualMonthlyPrice, 'R\$ 39,92');
    });

    test('formattedPrice should handle zero cents', () {
      const plan = PlanModel(
        id: 'plan-round',
        name: 'Round',
        price: 10000,
        currency: 'BRL',
        aiProvider: 'gemini',
        aiModel: 'gemini-2.0-flash',
        inputMultiplier: 1.0,
        outputMultiplier: 1.0,
        maxAgents: 1,
        maxChannels: 1,
        maxConversationsPerMonth: 100,
        maxAiTokensPerMonth: 10000,
        maxStorageMb: 256,
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
      );

      expect(plan.formattedPrice, 'R\$ 100,00');
    });

    // US-004: USD tiers ship monthly-only (migration 000109) — the app
    // must not offer the annual toggle when no annual Stripe price exists,
    // because the backend silently bills the full monthly price instead.
    test('hasAnnualBilling requires a non-empty annual Stripe price', () {
      PlanModel plan(String? annualId) => PlanModel(
            id: 'plan-x',
            name: 'Solo',
            price: 3900,
            currency: 'USD',
            aiProvider: 'google',
            aiModel: 'gemini-3.1-flash-lite',
            inputMultiplier: 1.0,
            outputMultiplier: 1.0,
            maxAgents: 1,
            maxChannels: 1,
            maxConversationsPerMonth: 200,
            maxAiTokensPerMonth: 7000000,
            maxStorageMb: 256,
            stripePriceId: 'price_monthly',
            stripePriceIdAnnual: annualId,
            createdAt: '2026-01-01T00:00:00Z',
            updatedAt: '2026-01-01T00:00:00Z',
          );

      expect(plan(null).hasAnnualBilling, isFalse);
      expect(plan('').hasAnnualBilling, isFalse);
      expect(plan('price_annual').hasAnnualBilling, isTrue);
    });

    // US-018 review fixes: 999 is the catalogue's "practically
    // unlimited" sentinel (USD Pro max_channels, migration 000109) —
    // plan cards must render "Unlimited", never the raw 999.
  });
}
