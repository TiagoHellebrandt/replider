import 'package:flutter/material.dart';

import 'package:app/core/ui/one_shot_prompt.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/home/presentation/controllers/onboarding_progress_controller.dart';
import 'package:app/features/onboarding/presentation/controllers/product_tour_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// A single walkthrough step. Kept private — the copy lives in i18n and the
/// step list is built in [_steps] so the order/contents are a single source.
class _TourStep {
  const _TourStep({
    required this.icon,
    required this.title,
    required this.body,
  });

  final IconData icon;
  final String title;
  final String body;
}

List<_TourStep> _steps() => [
      _TourStep(
        icon: Icons.celebration_outlined,
        title: t.product_tour.step1_title,
        body: t.product_tour.step1_body,
      ),
      _TourStep(
        icon: Icons.inbox_outlined,
        title: t.product_tour.step2_title,
        body: t.product_tour.step2_body,
      ),
      _TourStep(
        icon: Icons.school_outlined,
        title: t.product_tour.step3_title,
        body: t.product_tour.step3_body,
      ),
      _TourStep(
        icon: Icons.speed_outlined,
        title: t.product_tour.step4_title,
        body: t.product_tour.step4_body,
      ),
    ];

/// In-app product tour for brand-new accounts (US-183). A lightweight stepped
/// walkthrough shown once, post-onboarding, on the first dashboard load. Always
/// skippable; completion (step-through or skip) persists via
/// [productTourControllerProvider] so it never replays.
///
/// Built as a paged dialog rather than pulling in a coach-mark package — it
/// reuses the app's existing modal + secure-storage one-shot pattern (same as
/// the NPS prompt) and stays fully widget-testable.
class _ProductTourModal extends ConsumerStatefulWidget {
  const _ProductTourModal();

  @override
  ConsumerState<_ProductTourModal> createState() => _ProductTourModalState();
}

class _ProductTourModalState extends ConsumerState<_ProductTourModal> {
  final _pageController = PageController();
  int _index = 0;

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  Future<void> _finish() async {
    await ref.read(productTourControllerProvider.notifier).complete();
    if (mounted) Navigator.of(context).pop();
  }

  void _next(int total) {
    if (_index >= total - 1) {
      _finish();
      return;
    }
    _pageController.nextPage(
      duration: const Duration(milliseconds: 250),
      curve: Curves.easeOut,
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final steps = _steps();
    final total = steps.length;
    final isLast = _index >= total - 1;

    return Dialog(
      backgroundColor: colors.backgroundPrimary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      insetPadding: const EdgeInsets.all(24),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 460),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(24, 20, 24, 16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  Text(
                    t.product_tour.step_counter(
                      current: (_index + 1).toString(),
                      total: total.toString(),
                    ),
                    style: TextStyle(
                        color: colors.textMuted,
                        fontSize: 12,
                        fontWeight: FontWeight.w600),
                  ),
                  const Spacer(),
                  // Skip is always available — a tour must never trap the user.
                  TextButton(
                    onPressed: _finish,
                    child: Text(
                      t.product_tour.skip,
                      style: TextStyle(
                          color: colors.textMuted,
                          fontSize: 13,
                          fontWeight: FontWeight.w600),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 4),
              SizedBox(
                height: 220,
                child: PageView.builder(
                  controller: _pageController,
                  itemCount: total,
                  onPageChanged: (i) => setState(() => _index = i),
                  itemBuilder: (context, i) => _StepView(
                    colors: colors,
                    step: steps[i],
                  ),
                ),
              ),
              const SizedBox(height: 16),
              _Dots(colors: colors, count: total, active: _index),
              const SizedBox(height: 16),
              SizedBox(
                height: 48,
                child: ElevatedButton(
                  onPressed: () => _next(total),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: colors.bluePrimary,
                    foregroundColor: Colors.white,
                    elevation: 0,
                    shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(10)),
                  ),
                  child: Text(
                    isLast ? t.product_tour.done : t.product_tour.next,
                    style: const TextStyle(
                        fontWeight: FontWeight.w700, fontSize: 14),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _StepView extends StatelessWidget {
  const _StepView({required this.colors, required this.step});

  final AppColorsExtension colors;
  final _TourStep step;

  @override
  Widget build(BuildContext context) {
    // Centered when the copy fits, scrollable when it doesn't — keeps the
    // fixed-height paged region overflow-proof across font/locale lengths.
    return SingleChildScrollView(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Container(
            width: 56,
            height: 56,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: colors.bluePrimary.withValues(alpha: 0.14),
              borderRadius: BorderRadius.circular(14),
            ),
            child: Icon(step.icon, color: colors.bluePrimary, size: 28),
          ),
          const SizedBox(height: 18),
          Text(
            step.title,
            style: TextStyle(
              color: colors.textPrimary,
              fontSize: 18,
              fontWeight: FontWeight.w800,
            ),
          ),
          const SizedBox(height: 10),
          Text(
            step.body,
            style: TextStyle(
                color: colors.textSecondary, fontSize: 14, height: 1.45),
          ),
        ],
      ),
    );
  }
}

class _Dots extends StatelessWidget {
  const _Dots(
      {required this.colors, required this.count, required this.active});

  final AppColorsExtension colors;
  final int count;
  final int active;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: List.generate(count, (i) {
        final selected = i == active;
        return AnimatedContainer(
          duration: const Duration(milliseconds: 200),
          margin: const EdgeInsets.symmetric(horizontal: 4),
          width: selected ? 20 : 8,
          height: 8,
          decoration: BoxDecoration(
            color: selected ? colors.bluePrimary : colors.borderLight,
            borderRadius: BorderRadius.circular(4),
          ),
        );
      }),
    );
  }
}

/// Opens the product tour. Caller decides *when* (see [maybeShowProductTour]).
Future<void> showProductTourModal(BuildContext context) {
  return showDialog<void>(
    context: context,
    barrierDismissible: false,
    builder: (_) => const _ProductTourModal(),
  );
}

/// Session-scoped dedupe so the tour shows at most once per app launch even if
/// the dashboard rebuilds. Mirrors the NPS/quota-wall launcher pattern.
bool _tourShownThisSession = false;

@visibleForTesting
void resetProductTourShownForTest() {
  _tourShownThisSession = false;
}

/// Fire-and-forget helper: shows the product tour iff (a) onboarding is
/// complete, (b) the tour hasn't been completed/skipped before (per
/// [productTourControllerProvider]), and (c) we haven't already shown it this
/// session. Safe to call from `build` — defers to a post-frame callback.
void maybeShowProductTour(BuildContext context, WidgetRef ref) {
  if (_tourShownThisSession) return;
  // Only run the tour once the account has finished onboarding — the
  // walkthrough is the hand-off from the guided setup to the live dashboard.
  final completed =
      ref.watch(onboardingProgressControllerProvider).asData?.value;
  if (completed == null || !completed.onboardingCompleted) return;
  final shown = ref.watch(productTourControllerProvider).asData?.value;
  // `null` = still loading the stored flag; act only once it resolves to false.
  if (shown == null || shown) return;
  _tourShownThisSession = true;
  WidgetsBinding.instance.addPostFrameCallback((_) {
    if (!context.mounted) return;
    showAsOneShotPrompt(
      () => showProductTourModal(context),
      onDeclined: () => _tourShownThisSession = false,
    );
  });
}
