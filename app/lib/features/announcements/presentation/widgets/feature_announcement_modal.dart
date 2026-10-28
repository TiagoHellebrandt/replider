import 'package:flutter/material.dart';

import 'package:app/core/ui/one_shot_prompt.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/announcements/presentation/controllers/feature_announcement_controller.dart';
import 'package:app/features/home/presentation/controllers/onboarding_progress_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// A single "What's new" entry. Kept private — the copy lives in i18n and the
/// list is built in [_items] so the order/contents are a single source.
class _AnnouncementItem {
  const _AnnouncementItem({
    required this.icon,
    required this.title,
    required this.body,
  });

  final IconData icon;
  final String title;
  final String body;
}

List<_AnnouncementItem> _items() => [
      _AnnouncementItem(
        icon: Icons.credit_card_outlined,
        title: t.feature_announcement.item1_title,
        body: t.feature_announcement.item1_body,
      ),
      _AnnouncementItem(
        icon: Icons.savings_outlined,
        title: t.feature_announcement.item2_title,
        body: t.feature_announcement.item2_body,
      ),
      _AnnouncementItem(
        icon: Icons.notifications_active_outlined,
        title: t.feature_announcement.item3_title,
        body: t.feature_announcement.item3_body,
      ),
    ];

/// "What's new" feature-announcement modal (US-184). A lightweight, one-shot
/// per-release dialog that surfaces the latest user-facing changes to RETURNING
/// users after an upgrade. The content (release + items) is i18n-driven; the
/// release identity lives in [FeatureAnnouncementController.currentRelease], so
/// shipping a new "What's new" is a copy + constant bump — no backend CMS.
///
/// Built as a simple dialog reusing the app's modal + secure-storage one-shot
/// pattern (same as the product tour / NPS prompt) so it stays widget-testable.
class _FeatureAnnouncementModal extends ConsumerWidget {
  const _FeatureAnnouncementModal();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final items = _items();

    Future<void> dismiss() async {
      await ref.read(featureAnnouncementControllerProvider.notifier).markSeen();
      if (context.mounted) Navigator.of(context).pop();
    }

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
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          t.feature_announcement.title,
                          style: TextStyle(
                            color: colors.textPrimary,
                            fontSize: 20,
                            fontWeight: FontWeight.w800,
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          t.feature_announcement.release_label,
                          style: TextStyle(
                            color: colors.textMuted,
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    onPressed: dismiss,
                    icon: Icon(Icons.close, color: colors.textMuted, size: 20),
                    tooltip: t.common.close,
                  ),
                ],
              ),
              const SizedBox(height: 12),
              Flexible(
                child: SingleChildScrollView(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      for (final item in items)
                        Padding(
                          padding: const EdgeInsets.only(bottom: 16),
                          child: _ItemView(colors: colors, item: item),
                        ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 4),
              SizedBox(
                height: 48,
                child: ElevatedButton(
                  onPressed: dismiss,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: colors.bluePrimary,
                    foregroundColor: Colors.white,
                    elevation: 0,
                    shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(10)),
                  ),
                  child: Text(
                    t.feature_announcement.cta,
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

class _ItemView extends StatelessWidget {
  const _ItemView({required this.colors, required this.item});

  final AppColorsExtension colors;
  final _AnnouncementItem item;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: 40,
          height: 40,
          alignment: Alignment.center,
          decoration: BoxDecoration(
            color: colors.bluePrimary.withValues(alpha: 0.14),
            borderRadius: BorderRadius.circular(12),
          ),
          child: Icon(item.icon, color: colors.bluePrimary, size: 20),
        ),
        const SizedBox(width: 14),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                item.title,
                style: TextStyle(
                  color: colors.textPrimary,
                  fontSize: 15,
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                item.body,
                style: TextStyle(
                    color: colors.textSecondary, fontSize: 13, height: 1.4),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

/// Opens the feature-announcement modal. Caller decides *when* (see
/// [maybeShowFeatureAnnouncement]).
Future<void> showFeatureAnnouncementModal(BuildContext context) {
  return showDialog<void>(
    context: context,
    barrierDismissible: false,
    builder: (_) => const _FeatureAnnouncementModal(),
  );
}

/// Session-scoped dedupe so the modal shows at most once per app launch even if
/// the dashboard rebuilds. Mirrors the product-tour / NPS launcher pattern.
bool _announcementShownThisSession = false;

@visibleForTesting
void resetFeatureAnnouncementShownForTest() {
  _announcementShownThisSession = false;
}

/// Fire-and-forget helper: shows the "What's new" modal iff (a) onboarding is
/// complete (returning, fully-set-up users only), (b) the announcement for the
/// current release hasn't been seen (per [featureAnnouncementControllerProvider]),
/// and (c) we haven't already shown it this session. Safe to call from `build`
/// — defers to a post-frame callback.
void maybeShowFeatureAnnouncement(BuildContext context, WidgetRef ref) {
  if (_announcementShownThisSession) return;
  // Only surface to established accounts — a brand-new account is mid-setup and
  // gets the product tour, not a "what's new" recap.
  final completed =
      ref.watch(onboardingProgressControllerProvider).asData?.value;
  if (completed == null || !completed.onboardingCompleted) return;
  final seen = ref.watch(featureAnnouncementControllerProvider).asData?.value;
  // `null` = still loading the stored flag; act only once it resolves to false.
  if (seen == null || seen) return;
  _announcementShownThisSession = true;
  WidgetsBinding.instance.addPostFrameCallback((_) {
    if (!context.mounted) return;
    showAsOneShotPrompt(
      () => showFeatureAnnouncementModal(context),
      onDeclined: () => _announcementShownThisSession = false,
    );
  });
}
