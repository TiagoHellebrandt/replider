import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/core/analytics/analytics_client.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/presentation/widgets/user_account_chip.dart';
import 'package:app/features/billing/presentation/widgets/billing_pill.dart';
import 'package:app/features/onboarding/domain/onboarding_variant.dart';
import 'package:app/features/onboarding/presentation/controllers/onboarding_controller.dart';
import 'package:app/features/onboarding/presentation/widgets/onboarding_progress_indicator.dart';
import 'package:app/features/onboarding/presentation/widgets/step_ai_key.dart';
import 'package:app/features/onboarding/presentation/widgets/step_company_name.dart';
import 'package:app/features/onboarding/presentation/widgets/step_connect_channel.dart';
import 'package:app/features/onboarding/presentation/widgets/step_import_products.dart';
import 'package:app/features/onboarding/presentation/widgets/step_import_customers.dart';
import 'package:app/features/onboarding/presentation/widgets/step_first_offer.dart';
import 'package:app/i18n/strings.g.dart';

class OnboardingScreen extends ConsumerStatefulWidget {
  const OnboardingScreen({super.key});

  @override
  ConsumerState<OnboardingScreen> createState() => _OnboardingScreenState();
}

class _OnboardingScreenState extends ConsumerState<OnboardingScreen> {
  late final PageController _pageController;
  final _companyNameController = TextEditingController();
  final _aboutController = TextEditingController();

  /// Wall-clock of the last step_viewed, so we can report duration_ms
  /// on step_completed / step_skipped.
  DateTime? _stepEnteredAt;

  /// Caps the wizard column on wide screens (web/tablet) so the fields
  /// and the bottom nav buttons don't stretch edge-to-edge on desktop.
  /// On narrow/mobile widths the ConstrainedBox is a no-op (content is
  /// already < 500), so this only kicks in past the phone breakpoint.
  static const double _maxContentWidth = 500;

  /// Canonical step names indexed 0..7. Indices 1–3 belonged to the deleted
  /// full flow (agent, knowledge, preview) and are kept as holes so
  /// `currentStep` keeps carrying the canonical index — analytics stays
  /// comparable with the cohorts that ran before, and a step persisted under
  /// the old flow still resolves on resume. We only emit the first
  /// step_viewed after build completes (initState is too early for ref.read
  /// analytics).
  static const _stepNames = <String>[
    'company_name',
    'ai_key',
    'add_knowledge',
    'preview_chat',
    'connect_channel',
    'import_products',
    'import_customers',
    'first_offer',
  ];

  void _trackStepViewed(int step) {
    if (step < 0 || step >= _stepNames.length) return;
    _stepEnteredAt = DateTime.now();
    ref.read(analyticsClientProvider).track(
      OnboardingEvents.stepViewed,
      {
        'step_index': step,
        'step_name': _stepNames[step],
        'variant': ref.read(onboardingVariantProvider).name,
      },
    );
  }

  void _trackStepExit(int step, {required bool skipped}) {
    if (step < 0 || step >= _stepNames.length) return;
    final entered = _stepEnteredAt;
    final durationMs = entered == null
        ? null
        : DateTime.now().difference(entered).inMilliseconds;
    ref.read(analyticsClientProvider).track(
      skipped ? OnboardingEvents.stepSkipped : OnboardingEvents.stepCompleted,
      {
        'step_index': step,
        'step_name': _stepNames[step],
        'variant': ref.read(onboardingVariantProvider).name,
        if (durationMs != null) 'duration_ms': durationMs,
      },
    );
  }

  @override
  void initState() {
    super.initState();
    _pageController = PageController();
    // Seed the local TextEditingControllers from whatever state the
    // onboarding controller is already holding. The controllers live on
    // this screen (shared across all step widgets), but they were
    // starting empty even when the user was returning to a step they
    // had already filled — so going back showed a blank field even
    // though the state still had the value.
    // WidgetsBinding + ref.read runs post-first-frame so it's safe.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final state = ref.read(onboardingControllerProvider);
      if (_companyNameController.text.isEmpty && state.accountName.isNotEmpty) {
        _companyNameController.text = state.accountName;
      }
      // The register form already asked for the company name — asking again
      // here is a question the product already knows the answer to. Prefill
      // from the ACCOUNT's name, but only when it differs from the user's
      // own name: social signups inherit the person's name as the account
      // name, and "Tiago" is not a clinic. Still editable either way.
      if (_companyNameController.text.isEmpty) {
        // Await the auth FUTURE, not the snapshot: on a hard refresh this
        // screen can mount before the session rehydrates, and reading the
        // still-loading snapshot silently skips the prefill.
        // Future.microtask, NOT Future(fn): the latter schedules via a
        // zero-duration Timer, which widget tests flag as pending.
        Future<void>.microtask(() async {
          final user = await ref
              .read(authControllerProvider.future)
              .catchError((_) => null);
          if (!mounted || user == null) return;
          if (_companyNameController.text.isNotEmpty) return;
          final accountName = user.accountName.trim();
          final personName = user.name.trim();
          if (accountName.isNotEmpty &&
              accountName.toLowerCase() != personName.toLowerCase()) {
            _companyNameController.text = accountName;
            ref
                .read(onboardingControllerProvider.notifier)
                .updateField('accountName', accountName);
          }
        });
      }
      if (_aboutController.text.isEmpty && state.about.isNotEmpty) {
        _aboutController.text = state.about;
      }
      _trackStepViewed(state.currentStep);
    });
  }

  @override
  void dispose() {
    _pageController.dispose();
    _companyNameController.dispose();
    _aboutController.dispose();
    super.dispose();
  }

  void _animateToPage(int page) {
    // The flow's canonical order is not monotonic ([0,5,6,4,7]):
    // "Próximo" from customers (6) to channel (4) is FORWARD in the flow but
    // BACKWARD in canonical page numbers, so the slide animation moved
    // against the button. When the two directions disagree, jump — no
    // animation beats one that contradicts what the person pressed.
    if (!_pageController.hasClients) {
      _pageController.jumpToPage(page);
      return;
    }
    final current = _pageController.page?.round() ?? 0;
    final active = ref.read(onboardingStepIndicesProvider);
    final forwardInFlow = active.indexOf(page) >= active.indexOf(current);
    final forwardCanonical = page >= current;
    if (forwardInFlow == forwardCanonical) {
      _pageController.animateToPage(
        page,
        duration: const Duration(milliseconds: 350),
        curve: Curves.easeInOut,
      );
    } else {
      _pageController.jumpToPage(page);
    }
  }

  /// The step after [step] in the flow, or null on the last one.
  int? _nextActiveStep(int step) {
    final active = ref.read(onboardingStepIndicesProvider);
    final pos = active.indexOf(step);
    if (pos < 0 || pos + 1 >= active.length) return null;
    return active[pos + 1];
  }

  Future<void> _handleNext() async {
    final currentStep = ref.read(onboardingControllerProvider).currentStep;
    final notifier = ref.read(onboardingControllerProvider.notifier);
    final next = _nextActiveStep(currentStep);

    switch (currentStep) {
      case 0:
        await notifier.submitStepAccountName();
      case aiKeyStepIndex:
        // The key saves itself inside the step (like the two imports), so
        // Next only moves on — and skipping is a real option: the price list
        // still comes in as CSV, which uses no model.
        if (next != null) notifier.goToStep(next);
      case 4:
        // The channel is never terminal — the first offer follows it — so
        // a null `next` here is a hole in stepIndices, not a finish.
        if (next != null) await notifier.submitChannelAndAdvance(next);
      case 5:
      case 6:
        // The price list and the customer list import themselves, inside
        // the step: the file is reviewed and confirmed there, so "next" only
        // moves on. (Until 2026-08-18 the price list's Next did nothing at
        // all — the only way forward was Skip.)
        //
        // But a file that was PICKED and REVIEWED and never confirmed is
        // work the person believes they did. Advancing used to discard it
        // in silence; now it asks.
        final pending = currentStep == 5
            ? onboardingPendingProductImport
            : onboardingPendingCustomerImport;
        if (pending) {
          final proceed = await showConfirmDialog(
            context,
            title: t.onboarding.unconfirmed_import_title,
            message: t.onboarding.unconfirmed_import_body,
            confirmText: t.onboarding.unconfirmed_import_confirm,
          );
          if (!proceed) return;
        }
        if (next != null) notifier.goToStep(next);
      case 7:
        // The first offer is terminal: Next finishes onboarding and lands on
        // the queue, where the offer she just read is one tap from approved.
        if (!mounted) return;
        await notifier.completeOnboarding(context,
            destination: '/prospect?tab=queue');
    }
  }

  Future<void> _handleSkip() async {
    final currentStep = ref.read(onboardingControllerProvider).currentStep;
    _trackStepExit(currentStep, skipped: true);
    final activeIndices = ref.read(onboardingStepIndicesProvider);
    if (currentStep == activeIndices.last) {
      // Terminal step: skipping completes onboarding rather than
      // advancing past the end of the wizard.
      if (!mounted) return;
      await ref
          .read(onboardingControllerProvider.notifier)
          .completeOnboarding(context);
      return;
    }
    // Advance to the next step the wizard actually renders — the active
    // indices aren't contiguous.
    final pos = activeIndices.indexOf(currentStep);
    final next = activeIndices[pos + 1];
    ref.read(onboardingControllerProvider.notifier).goToStep(next);
  }

  void _handleBack() {
    final currentStep = ref.read(onboardingControllerProvider).currentStep;
    // The previous step the wizard actually renders — the canonical
    // indices are not contiguous (0, 5, 6, 4, 7), so
    // "minus one" would land on a page the wizard never shows.
    final active = ref.read(onboardingStepIndicesProvider);
    final pos = active.indexOf(currentStep);
    if (pos > 0) {
      ref.read(onboardingControllerProvider.notifier).goToStep(active[pos - 1]);
    }
  }

  bool _isNextEnabled(OnboardingState state) {
    switch (state.currentStep) {
      case 0:
        // Name, one line about the business, at least one way it earns, and
        // the state it sells in. The last two are what every search and
        // every offer downstream keys on; the first two are what make the
        // AI drafts specific to this business. None is optional, because
        // optional here means skipped, and the customer would meet the
        // generic version of the product first.
        return ref
            .read(onboardingControllerProvider.notifier)
            .stepCompanyComplete;
      case aiKeyStepIndex:
        // Advancing is always allowed: the key saves itself inside the step,
        // and moving on without one is a real choice — the price list still
        // imports as CSV, which uses no model. (Leaving this step out of the
        // switch is what made Next dead after a key was saved.)
        return true;
      case 4:
        return true; // Channel: always skippable
      case 5:
      case 6:
        // The price list and the customers are imported inside the step.
        // Advancing is always allowed: gating it would strand somebody who
        // does not have the file open, and both are reachable afterwards.
        return true;
      case 7:
        // The first offer: finishing is always allowed — approving is the
        // point, but a person who wants to read it in the queue tomorrow
        // must not be held hostage on the last page.
        return true;
      default:
        return false;
    }
  }

  bool _isSkippable(int step) {
    // Only the channel allows skip. Company does not — the company name and
    // description are what make every draft afterwards specific to this
    // business. The first offer has no Skip: its Next already means
    // "finish". The two IMPORT steps have no Skip either: their Next already
    // just advances, and two buttons doing the same thing taught people the
    // primary button does nothing.
    return step == 4;
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final onboardingState = ref.watch(onboardingControllerProvider);
    final currentStep = onboardingState.currentStep;

    // The flow decides which canonical steps render. Map the canonical
    // `currentStep` onto a position within the active list so the progress
    // dots, "step X of Y" label and terminal detection all reflect the
    // five steps the person actually sees.
    final activeIndices = ref.watch(onboardingStepIndicesProvider);
    final rawPosition = activeIndices.indexOf(currentStep);
    final position = rawPosition < 0 ? 0 : rawPosition;
    final totalSteps = activeIndices.length;
    final isTerminalStep = currentStep == activeIndices.last;

    // Sync page controller with state
    ref.listen(onboardingControllerProvider, (prev, next) {
      // Hydration (GET /me/onboarding-progress) lands asynchronously
      // after build — sync the locally-owned TextEditingControllers
      // whenever the canonical state carries different values, so the
      // user sees their saved drafts when returning to the flow.
      if (next.accountName != _companyNameController.text) {
        _companyNameController.text = next.accountName;
      }
      if (next.about != _aboutController.text) {
        _aboutController.text = next.about;
      }
      if (prev?.currentStep != next.currentStep) {
        _animateToPage(next.currentStep);
        // Emit step_completed only when advancing (not on back). Skips
        // are already tracked explicitly by _handleSkip; when we land
        // here without going through the skip button it's because the
        // user completed the step normally.
        final prevStep = prev?.currentStep ?? 0;
        if (next.currentStep > prevStep) {
          _trackStepExit(prevStep, skipped: false);
        }
        _trackStepViewed(next.currentStep);
      }
      if (next.error != null) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(next.error!),
            backgroundColor: colors.redAlert,
            behavior: SnackBarBehavior.floating,
            margin: const EdgeInsets.all(16),
            shape:
                RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          ),
        );
      }
    });

    return Scaffold(
      backgroundColor: const Color(0xFF1E1E1E),
      body: SafeArea(
        child: Column(
          children: [
            // Top bar: logo left-of-center, user chip flush right.
            // Using Expanded spacers on both sides keeps the logo roughly
            // centered while the chip takes its natural width on the right
            // without overlapping the text.
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
              child: Row(
                children: [
                  // Left spacer — balances the chip on the right so the
                  // logo stays visually centered.
                  const Expanded(child: SizedBox.shrink()),
                  Image.asset(
                    'assets/logo_white.webp',
                    height: 20,
                    errorBuilder: (context, error, stackTrace) => const Icon(
                        Icons.layers,
                        size: 20,
                        color: Color(0xFF4A89F3)),
                  ),
                  const SizedBox(width: 8),
                  Text(
                    'Replider',
                    style: typography.display.copyWith(
                      fontSize: 18,
                      color: Colors.white,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  // Right cluster: billing pill (upgrade CTA inline) +
                  // user chip. Pill lives here so the upgrade path is
                  // visible from step 1 onwards — the user doesn't need
                  // to finish onboarding to discover how to pay.
                  Expanded(
                    child: Align(
                      alignment: Alignment.centerRight,
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: const [
                          BillingPill(),
                          SizedBox(width: 8),
                          UserAccountChip(),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),

            // Page content
            Expanded(
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: _maxContentWidth),
                  child: PageView(
                    controller: _pageController,
                    physics: const NeverScrollableScrollPhysics(),
                    children: [
                      StepCompanyName(
                        companyNameController: _companyNameController,
                        aboutController: _aboutController,
                      ),
                      // Holes 1–3: the deleted full flow's agent, knowledge
                      // and preview pages. Never shown (they are not in
                      // stepIndices); they only keep the canonical indices
                      // of the pages after them stable.
                      const StepAIKey(),
                      const SizedBox.shrink(),
                      const SizedBox.shrink(),
                      const StepConnectChannel(),
                      const StepImportProducts(),
                      const StepImportCustomers(),
                      const StepFirstOffer(),
                    ],
                  ),
                ),
              ),
            ),

            // Bottom navigation — always visible; the terminal step
            // (first offer) owns the finishing action instead of a
            // separate Done page.
            if (rawPosition >= 0)
              Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
                decoration: BoxDecoration(
                  color: colors.backgroundSecondary,
                  border: Border(
                    top: BorderSide(color: colors.borderLight),
                  ),
                ),
                child: SafeArea(
                  top: false,
                  child: Center(
                    child: ConstrainedBox(
                      constraints:
                          const BoxConstraints(maxWidth: _maxContentWidth),
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          // Progress indicator + step label
                          Row(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              OnboardingProgressIndicator(
                                currentStep: position,
                                totalSteps: totalSteps,
                              ),
                            ],
                          ),
                          const SizedBox(height: 4),
                          Text(
                            t.onboarding.step_of(
                                current: '${position + 1}',
                                total: '$totalSteps'),
                            style: TextStyle(
                              color: colors.textMuted,
                              fontSize: 11,
                            ),
                          ),
                          const SizedBox(height: 16),

                          // Action buttons
                          Row(
                            children: [
                              // Back button
                              if (currentStep > 0)
                                TextButton(
                                  onPressed: _handleBack,
                                  child: Text(
                                    t.onboarding.back,
                                    style:
                                        TextStyle(color: colors.textSecondary),
                                  ),
                                )
                              else
                                const SizedBox(width: 80),

                              const Spacer(),

                              // Skip button (channel step only)
                              if (_isSkippable(currentStep))
                                Padding(
                                  padding: const EdgeInsets.only(right: 12),
                                  child: TextButton(
                                    onPressed: _handleSkip,
                                    child: Text(
                                      t.onboarding.skip,
                                      style: TextStyle(color: colors.textMuted),
                                    ),
                                  ),
                                ),

                              // Next button
                              SizedBox(
                                width: 120,
                                height: 44,
                                child: ElevatedButton(
                                  onPressed: _isNextEnabled(onboardingState) &&
                                          !onboardingState.isLoading
                                      ? _handleNext
                                      : null,
                                  style: ElevatedButton.styleFrom(
                                    backgroundColor: colors.bluePrimary,
                                    foregroundColor: Colors.white,
                                    disabledBackgroundColor:
                                        colors.borderMedium,
                                    disabledForegroundColor:
                                        colors.textSecondary,
                                    elevation: 0,
                                    shape: RoundedRectangleBorder(
                                      borderRadius: BorderRadius.circular(100),
                                    ),
                                  ),
                                  child: onboardingState.isLoading
                                      ? const SizedBox(
                                          width: 20,
                                          height: 20,
                                          child: CircularProgressIndicator(
                                            color: Colors.white,
                                            strokeWidth: 2,
                                          ),
                                        )
                                      : Text(
                                          // The terminal button is labelled
                                          // by where it actually LANDS. The
                                          // first-offer step finishes on the
                                          // approval queue — "Ir para o
                                          // Dashboard" promised a different
                                          // place one line after the step
                                          // said the offer waits in Ofertas.
                                          isTerminalStep
                                              ? (currentStep == 7
                                                  ? t.onboarding.finish_to_queue
                                                  : t.onboarding
                                                      .go_to_dashboard)
                                              : t.onboarding.next,
                                          style: const TextStyle(
                                            fontWeight: FontWeight.w500,
                                            fontSize: 15,
                                          ),
                                        ),
                                ),
                              ),
                            ],
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
