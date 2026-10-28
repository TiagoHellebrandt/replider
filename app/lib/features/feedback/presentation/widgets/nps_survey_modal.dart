import 'package:flutter/material.dart';

import 'package:app/core/ui/one_shot_prompt.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/components/save_feedback.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/feedback/presentation/controllers/nps_prompt_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// In-app Net Promoter Score survey (US-178). Asks "how likely are you to
/// recommend Replider" on the standard 0-10 scale plus an optional free-text
/// comment, then POSTs to `/me/nps`. Shown at most once per quarter and always
/// dismissible ("maybe later") so it nudges without nagging.
class _NpsSurveyModal extends ConsumerStatefulWidget {
  const _NpsSurveyModal();

  @override
  ConsumerState<_NpsSurveyModal> createState() => _NpsSurveyModalState();
}

class _NpsSurveyModalState extends ConsumerState<_NpsSurveyModal> {
  int? _score;
  final _commentController = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _commentController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final score = _score;
    if (score == null || _submitting) return;
    setState(() => _submitting = true);
    final err = await ref
        .read(npsPromptControllerProvider.notifier)
        .submit(score, _commentController.text.trim());
    if (!mounted) return;
    setState(() => _submitting = false);
    if (err != null) {
      SaveFeedback.showError(context, err);
      return;
    }
    Navigator.of(context).pop();
    SaveFeedback.showSuccess(context, t.feedback.nps_thanks);
  }

  Future<void> _dismiss() async {
    await ref.read(npsPromptControllerProvider.notifier).dismiss();
    if (mounted) Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Dialog(
      backgroundColor: colors.backgroundPrimary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      insetPadding: const EdgeInsets.all(24),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 480),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(24, 24, 24, 18),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                t.feedback.nps_title,
                style: TextStyle(
                  color: colors.textPrimary,
                  fontSize: 18,
                  fontWeight: FontWeight.w800,
                ),
              ),
              const SizedBox(height: 8),
              Text(
                t.feedback.nps_subtitle,
                style: TextStyle(
                    color: colors.textSecondary, fontSize: 14, height: 1.4),
              ),
              const SizedBox(height: 18),
              _ScoreSelector(
                colors: colors,
                selected: _score,
                onSelected:
                    _submitting ? null : (v) => setState(() => _score = v),
              ),
              const SizedBox(height: 6),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(t.feedback.nps_scale_low,
                      style: TextStyle(color: colors.textMuted, fontSize: 11)),
                  Text(t.feedback.nps_scale_high,
                      style: TextStyle(color: colors.textMuted, fontSize: 11)),
                ],
              ),
              const SizedBox(height: 16),
              TextField(
                controller: _commentController,
                enabled: !_submitting,
                minLines: 2,
                maxLines: 4,
                maxLength: 500,
                style: TextStyle(color: colors.textPrimary, fontSize: 14),
                decoration: InputDecoration(
                  hintText: t.feedback.nps_comment_hint,
                  hintStyle: TextStyle(color: colors.textMuted, fontSize: 13),
                  filled: true,
                  fillColor: colors.backgroundSecondary,
                  counterText: '',
                ),
              ),
              const SizedBox(height: 16),
              SizedBox(
                height: 48,
                child: ElevatedButton(
                  onPressed: (_score == null || _submitting) ? null : _submit,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: colors.bluePrimary,
                    foregroundColor: Colors.white,
                    disabledBackgroundColor:
                        colors.bluePrimary.withValues(alpha: 0.4),
                    elevation: 0,
                    shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(10)),
                  ),
                  child: _submitting
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            valueColor:
                                AlwaysStoppedAnimation<Color>(Colors.white),
                          ),
                        )
                      : Text(t.feedback.nps_submit,
                          style: const TextStyle(
                              fontWeight: FontWeight.w700, fontSize: 14)),
                ),
              ),
              const SizedBox(height: 6),
              TextButton(
                onPressed: _submitting ? null : _dismiss,
                child: Text(
                  t.feedback.nps_later,
                  style: TextStyle(
                      color: colors.textMuted,
                      fontSize: 13,
                      fontWeight: FontWeight.w600),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 0-10 score chips. The selected one fills with the brand blue; the rest
/// stay outlined. Kept compact so all 11 fit on a phone-width dialog.
class _ScoreSelector extends StatelessWidget {
  const _ScoreSelector({
    required this.colors,
    required this.selected,
    required this.onSelected,
  });

  final AppColorsExtension colors;
  final int? selected;
  final ValueChanged<int>? onSelected;

  @override
  Widget build(BuildContext context) {
    return Wrap(
      spacing: 6,
      runSpacing: 6,
      alignment: WrapAlignment.center,
      children: List.generate(11, (i) {
        final isSelected = selected == i;
        return InkWell(
          onTap: onSelected == null ? null : () => onSelected!(i),
          borderRadius: BorderRadius.circular(8),
          child: Container(
            width: 34,
            height: 38,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color:
                  isSelected ? colors.bluePrimary : colors.backgroundSecondary,
              borderRadius: BorderRadius.circular(8),
              border: Border.all(
                color: isSelected ? colors.bluePrimary : colors.borderLight,
              ),
            ),
            child: Text(
              '$i',
              style: TextStyle(
                color: isSelected ? Colors.white : colors.textSecondary,
                fontWeight: FontWeight.w700,
                fontSize: 14,
              ),
            ),
          ),
        );
      }),
    );
  }
}

/// Opens the NPS survey. Caller decides *when* (see [maybeShowNpsPrompt]).
Future<void> showNpsSurveyModal(BuildContext context) {
  return showDialog<void>(
    context: context,
    barrierDismissible: false,
    builder: (_) => const _NpsSurveyModal(),
  );
}

/// Session-scoped dedupe so the survey shows at most once per app launch even
/// if the dashboard rebuilds. Mirrors the quota-wall pattern (a top-level bool
/// rather than an ad-hoc StateProvider).
bool _npsShownThisSession = false;

@visibleForTesting
void resetNpsShownForTest() {
  _npsShownThisSession = false;
}

/// Fire-and-forget helper: shows the NPS survey iff (a) it hasn't been handled
/// this quarter (per [npsPromptControllerProvider]) and (b) we haven't already
/// shown it this session. Safe to call from `build` — defers to a post-frame
/// callback.
void maybeShowNpsPrompt(BuildContext context, WidgetRef ref) {
  if (_npsShownThisSession) return;
  final handled = ref.watch(npsPromptControllerProvider).asData?.value;
  // Only act once the stored state has resolved; `null` = still loading.
  if (handled == null || handled) return;
  _npsShownThisSession = true;
  WidgetsBinding.instance.addPostFrameCallback((_) {
    if (!context.mounted) return;
    showAsOneShotPrompt(
      () => showNpsSurveyModal(context),
      onDeclined: () => _npsShownThisSession = false,
    );
  });
}
