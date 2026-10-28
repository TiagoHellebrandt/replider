import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/glass_card.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/components/whatsapp_text.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/onboarding/presentation/controllers/first_offer_controller.dart';
import 'package:app/features/onboarding/presentation/controllers/onboarding_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// "Your first offer" — the last step (2026-08-18), and the reason the wizard
/// exists.
///
/// The person leaves onboarding having READ one real, approvable message:
/// about one of the customers she just imported (or one hospital in her
/// territory), offering one of the products she just listed, written the
/// way every morning's offers will be written. Every step before this one
/// was there so this message could exist. Describing the product does not
/// do what showing this does.
///
/// It sends nothing. The draft is already in the approval queue; finishing
/// lands her there. The Next button of the wizard is the way out, so there
/// is no button here that duplicates it.
class StepFirstOffer extends ConsumerWidget {
  const StepFirstOffer({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final offer = ref.watch(firstOfferControllerProvider);

    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 560),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: 24),
              Center(
                child: Container(
                  width: 64,
                  height: 64,
                  decoration: BoxDecoration(
                    color: colors.bluePrimary.withValues(alpha: 0.12),
                    shape: BoxShape.circle,
                  ),
                  child: Icon(Icons.local_offer_outlined,
                      size: 32, color: colors.bluePrimary),
                ),
              ),
              const SizedBox(height: 20),
              Text(
                t.onboarding.first_offer_title,
                style: typography.display.copyWith(
                  fontSize: 24,
                  color: Colors.white,
                  fontWeight: FontWeight.w700,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 24),
              offer.when(
                loading: () => _Writing(colors: colors),
                error: (e, _) => _Failed(error: e, colors: colors),
                data: (o) => o.isReady
                    ? _Offer(offer: o, colors: colors, typography: typography)
                    : _Nothing(reason: o.reason, colors: colors),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Writing extends StatelessWidget {
  const _Writing({required this.colors});
  final AppColorsExtension colors;

  @override
  Widget build(BuildContext context) {
    return Column(
      key: const Key('first_offer_writing'),
      children: [
        const CircularProgressIndicator(),
        const SizedBox(height: 16),
        Text(
          t.onboarding.first_offer_writing,
          style: TextStyle(color: colors.textSecondary, fontSize: 14),
          textAlign: TextAlign.center,
        ),
      ],
    );
  }
}

class _Offer extends StatelessWidget {
  const _Offer({
    required this.offer,
    required this.colors,
    required this.typography,
  });
  final FirstOffer offer;
  final AppColorsExtension colors;
  final AppTypographyExtension typography;

  @override
  Widget build(BuildContext context) {
    final who = offer.lead?.displayName ?? '';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          offer.source == 'prospect'
              ? t.onboarding.first_offer_for_prospect(name: who)
              : t.onboarding.first_offer_for_customer(name: who),
          key: const Key('first_offer_recipient'),
          style: TextStyle(color: colors.textSecondary, fontSize: 14),
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: 12),
        GlassCard(
          key: const Key('first_offer_card'),
          padding: const EdgeInsets.all(20),
          child: WhatsAppText(
            offer.draft,
            baseStyle: typography.body.copyWith(
              color: colors.textPrimary,
              fontSize: 15,
              height: 1.5,
            ),
          ),
        ),
        const SizedBox(height: 16),
        Text(
          t.onboarding.first_offer_in_queue,
          style: TextStyle(color: colors.textSecondary, fontSize: 13),
          textAlign: TextAlign.center,
        ),
      ],
    );
  }
}

/// The server refused to write — and the card says what to do about it,
/// not just "try again". The refusal codes the wizard can act on each get
/// the door that fixes them: an empty catalogue goes back to Products, a
/// customer with no address goes back to Customers, a missing AI key
/// names Settings. Everything else keeps the plain Retry.
class _Failed extends ConsumerWidget {
  const _Failed({required this.error, required this.colors});
  final Object error;
  final AppColorsExtension colors;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final code = parseApiErrorCode(error);
    // Read on tap, not at build: the async error state rebuilds this card
    // and a ref captured by an earlier build is unmounted by the time the
    // person presses the button.
    void goTo(int step) =>
        ref.read(onboardingControllerProvider.notifier).goToStep(step);
    final Widget? cta = switch (code) {
      'APPROACH_NEEDS_PRODUCTS' => OutlinedButton.icon(
          key: const Key('first_offer_cta_products'),
          onPressed: () => goTo(kOnboardingProductsStep),
          icon: const Icon(Icons.inventory_2_outlined, size: 16),
          label: Text(t.onboarding.first_offer_cta_products),
        ),
      'LEAD_CONTACT_NOT_MESSAGEABLE' => OutlinedButton.icon(
          key: const Key('first_offer_cta_customers'),
          onPressed: () => goTo(kOnboardingCustomersStep),
          icon: const Icon(Icons.people_outline, size: 16),
          label: Text(t.onboarding.first_offer_cta_customers),
        ),
      'BYOK_NOT_CONFIGURED' => OutlinedButton.icon(
          key: const Key('first_offer_cta_settings'),
          onPressed: () => context.go('/settings/account'),
          icon: const Icon(Icons.settings_outlined, size: 16),
          label: Text(t.onboarding.first_offer_cta_settings),
        ),
      _ => null,
    };
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        RetryableError(
          key: const Key('first_offer_error'),
          error: error,
          onRetry: () => ref.invalidate(firstOfferControllerProvider),
        ),
        if (cta != null) ...[
          const SizedBox(height: 12),
          Center(child: cta),
        ],
      ],
    );
  }
}

/// Canonical page indices of the wizard steps the first-offer card can send
/// the person back to (see `OnboardingVariantSteps.stepIndices`).
const kOnboardingProductsStep = 5;
const kOnboardingCustomersStep = 6;

/// Nothing to write yet — and why. Honest, not apologetic: the product
/// still works, it just needs one of two things to start.
class _Nothing extends ConsumerWidget {
  const _Nothing({required this.reason, required this.colors});
  final String reason;
  final AppColorsExtension colors;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final body = switch (reason) {
      'no_business_found' => t.onboarding.first_offer_no_business,
      // A platform gap, not the customer's: the old copy asked her to
      // "diga em que estado você vende" — a field the wizard already made
      // mandatory two steps ago.
      'no_customers_no_territory' => t.onboarding.first_offer_unavailable,
      // No prospecting provider on the platform at all — used to be
      // reported as "no business found", which sent her looking for a
      // hospital nobody had searched for.
      'prospecting_not_configured' =>
        t.onboarding.first_offer_prospecting_not_configured,
      _ => t.onboarding.first_offer_nothing,
    };
    return GlassCard(
      key: const Key('first_offer_nothing'),
      padding: const EdgeInsets.all(20),
      child: Column(
        children: [
          Icon(Icons.inbox_outlined, size: 32, color: colors.textMuted),
          const SizedBox(height: 12),
          Text(
            body,
            style: TextStyle(color: colors.textSecondary, fontSize: 14),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 16),
          // The text says "import customers" — this is the door. Without it
          // the only button on the page was "finish", which landed on an
          // empty queue.
          OutlinedButton.icon(
            key: const Key('first_offer_back_to_customers'),
            onPressed: () => ref
                .read(onboardingControllerProvider.notifier)
                .goToStep(kOnboardingCustomersStep),
            icon: const Icon(Icons.upload_file, size: 16),
            label: Text(t.onboarding.first_offer_import_cta),
          ),
        ],
      ),
    );
  }
}
