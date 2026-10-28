import 'package:app/features/billing/domain/models/effective_plan_limits_model.dart';
import 'package:app/features/billing/domain/models/usage_model.dart';
import 'package:app/features/settings/presentation/screens/settings_screen.dart';
import 'package:flutter_test/flutter_test.dart';

// The plan line in Settings said "Tokens: 0% | Conversas: 0%" on a live
// Enterprise account that had spent 5.5K tokens across 13 conversations.
//
// Contract plans store 0 in the limit columns and the API resolves that as "no
// ceiling"; an enterprise ceiling can also arrive as the int32 sentinel.
// Dividing by either produces a confident 0% — on the one line a customer is
// most likely to glance at to see whether they are close to their limit.

UsageModel _usage({required int tokenLimit, required int convLimit}) =>
    UsageModel(
      aiTokensUsed: 5500,
      conversationsCount: 13,
      maxAiTokensPerMonth: tokenLimit,
      maxConversationsPerMonth: convLimit,
    );

void main() {
  test('a contract plan storing 0 has no ceiling to divide by', () {
    expect(planUsageHasCeiling(_usage(tokenLimit: 0, convLimit: 0)), isFalse);
  });

  test('the int32 sentinel is not a ceiling either', () {
    const sentinel = EffectivePlanLimitsModel.int32Sentinel;
    expect(
      planUsageHasCeiling(_usage(tokenLimit: sentinel, convLimit: sentinel)),
      isFalse,
    );
  });

  test('one unbounded side is enough to drop the percentage', () {
    // Reporting "Tokens: 55% | Conversas: 0%" would be half-true, which on a
    // usage readout is worse than declining to state a share.
    expect(
      planUsageHasCeiling(_usage(tokenLimit: 10000, convLimit: 0)),
      isFalse,
    );
    expect(
      planUsageHasCeiling(_usage(tokenLimit: 0, convLimit: 100)),
      isFalse,
    );
  });

  test('a real paid plan still reports percentages', () {
    expect(
      planUsageHasCeiling(_usage(tokenLimit: 10000, convLimit: 100)),
      isTrue,
    );
  });
}
