import 'package:app/features/billing/domain/models/effective_plan_limits_model.dart';
import 'package:flutter_test/flutter_test.dart';

// A buyer read "5.5K / 2147.5M" on their own usage strip. 2147483647 is int32
// max, which the backend uses in places to mean "no ceiling" — printed
// literally it becomes a quantity nobody was ever sold, sitting next to a real
// number as though the two were comparable.

void main() {
  group('EffectivePlanLimitsModel.isUnlimited', () {
    test('recognises the explicit -1', () {
      expect(EffectivePlanLimitsModel.isUnlimited(-1), isTrue);
    });

    test('recognises the int32 sentinel and anything above it', () {
      expect(
          EffectivePlanLimitsModel.isUnlimited(
              EffectivePlanLimitsModel.int32Sentinel),
          isTrue);
      expect(EffectivePlanLimitsModel.isUnlimited(2147483647), isTrue);
      expect(EffectivePlanLimitsModel.isUnlimited(9999999999), isTrue);
    });

    test('leaves real ceilings alone', () {
      // No plan meaningfully caps anything at two billion, but 1000 and
      // 1000000 are numbers somebody was actually sold.
      expect(EffectivePlanLimitsModel.isUnlimited(0), isFalse);
      expect(EffectivePlanLimitsModel.isUnlimited(1), isFalse);
      expect(EffectivePlanLimitsModel.isUnlimited(1000), isFalse);
      expect(EffectivePlanLimitsModel.isUnlimited(1000000), isFalse);
      expect(
          EffectivePlanLimitsModel.isUnlimited(
              EffectivePlanLimitsModel.int32Sentinel - 1),
          isFalse);
    });
  });
}
