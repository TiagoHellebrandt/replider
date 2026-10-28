import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/features/auth/presentation/controllers/auth_controller.dart';

part 'onboarding_variant.g.dart';

/// The activation flow (US-114).
///
/// - [reduced] is the SHIPPED flow (since 2026-08-18): company → price
///   list → customers → channel → first offer. It is the only variant left:
///   the historical 5-step `full` flow (company → agent → knowledge →
///   preview → channel) was deleted on 2026-09-01 together with the agent
///   and knowledge steps it rendered (branch `archive/v1-full-product`
///   keeps them).
///
///   The first offer is the terminal step and the reason the flow exists:
///   the person leaves the wizard having read one real, approvable message
///   about one of her customers and one of her products, which is the
///   product's whole promise. Every step before it is there so that
///   message can be written.
///
///   "Create an agent" used to be step two, and it was the wrong first thing to
///   ask for: it told every new account that Replider is a chatbot, while
///   drafting an approach needs no agent at all. What the product does need
///   before it is worth anything is what she SELLS and a way to send — so those
///   are the two steps after the company.
enum OnboardingVariant { reduced }

extension OnboardingVariantSteps on OnboardingVariant {
  /// Canonical step indices that make up this variant, in display order.
  /// The indices map onto the fixed PageView children in `OnboardingScreen`:
  /// 0=company, 1=AI key, 4=channel, 5=price list, 6=customers,
  /// 7=first offer. Indices 2–3 (knowledge, preview) belonged to the deleted
  /// full flow and are kept as holes so a step persisted under it still
  /// resolves on resume (see `OnboardingController._hydrate`).
  ///
  /// The AI key took over index 1 (the deleted "create agent" step). It comes
  /// SECOND because the step after it — the price list — reads brochures with
  /// the account's own model: a contract account with no key met that wall on
  /// its first day, in a step it had never been given a chance to prepare.
  ///
  /// The LAST index is the terminal step — completing (or skipping) it
  /// completes onboarding.
  List<int> get stepIndices => switch (this) {
        OnboardingVariant.reduced => const [0, aiKeyStepIndex, 5, 6, 4, 7],
      };

  /// The highest canonical index the wizard renders. `OnboardingScreen`
  /// builds this many + 1 pages, and the persisted step is clamped to it.
  static const int lastStepIndex = 7;
}

/// The active onboarding flow. Only [OnboardingVariant.reduced] exists now;
/// the provider stays so tests and the screen keep a single seam.
@Riverpod(keepAlive: true)
OnboardingVariant onboardingVariant(Ref ref) => OnboardingVariant.reduced;

/// The BYOK step's canonical index.
const aiKeyStepIndex = 1;

/// The steps this ACCOUNT walks, in order.
///
/// A self-serve account runs on the platform's key and must never be asked for
/// one, so the BYOK step is dropped for it. The filter is deliberately keyed on
/// `isContractPlan` alone — a stable fact — and NOT on whether a key is already
/// set: that flips the moment the person saves one, and a list that loses the
/// step somebody is standing on strands them there.
final onboardingStepIndicesProvider = Provider<List<int>>((ref) {
  final variant = ref.watch(onboardingVariantProvider);
  final isContract =
      ref.watch(authControllerProvider).asData?.value?.isContractPlan ?? false;
  final all = variant.stepIndices;
  if (isContract) return all;
  return all.where((i) => i != aiKeyStepIndex).toList(growable: false);
});
