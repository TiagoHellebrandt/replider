import 'package:flutter/material.dart';

import 'package:app/core/ui/one_shot_prompt.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/components/save_feedback.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/feedback/presentation/controllers/cancellation_survey_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Cancellation-reason survey (US-180). Presented when the user returns from
/// the Stripe portal cancel flow (`/billing?cancel_survey=1`). Asks why they
/// cancelled (closed reason set + optional comment) and POSTs to
/// `/me/cancellation-survey`. Always dismissible so it never traps the user.
class _CancellationSurveyModal extends ConsumerStatefulWidget {
  const _CancellationSurveyModal();

  @override
  ConsumerState<_CancellationSurveyModal> createState() =>
      _CancellationSurveyModalState();
}

class _CancellationSurveyModalState
    extends ConsumerState<_CancellationSurveyModal> {
  String? _reason;
  final _commentController = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _commentController.dispose();
    super.dispose();
  }

  String _reasonLabel(String key) {
    final r = t.feedback.cancel_reasons;
    switch (key) {
      case 'too_expensive':
        return r.too_expensive;
      case 'missing_features':
        return r.missing_features;
      case 'not_using':
        return r.not_using;
      case 'switched_competitor':
        return r.switched_competitor;
      case 'technical_issues':
        return r.technical_issues;
      case 'temporary_pause':
        return r.temporary_pause;
      default:
        return r.other;
    }
  }

  Future<void> _submit() async {
    final reason = _reason;
    if (reason == null || _submitting) return;
    setState(() => _submitting = true);
    final err = await ref
        .read(cancellationSurveyControllerProvider.notifier)
        .submit(reason, _commentController.text.trim());
    if (!mounted) return;
    setState(() => _submitting = false);
    if (err != null) {
      SaveFeedback.showError(context, err);
      return;
    }
    Navigator.of(context).pop();
    SaveFeedback.showSuccess(context, t.feedback.cancel_thanks);
  }

  void _dismiss() => Navigator.of(context).pop();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Dialog(
      backgroundColor: colors.backgroundPrimary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      insetPadding: const EdgeInsets.all(24),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 480),
        child: SingleChildScrollView(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(24, 24, 24, 18),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  t.feedback.cancel_title,
                  style: TextStyle(
                    color: colors.textPrimary,
                    fontSize: 18,
                    fontWeight: FontWeight.w800,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  t.feedback.cancel_subtitle,
                  style: TextStyle(
                      color: colors.textSecondary, fontSize: 14, height: 1.4),
                ),
                const SizedBox(height: 16),
                ...cancellationReasonKeys.map(
                  (key) => _ReasonOption(
                    colors: colors,
                    label: _reasonLabel(key),
                    selected: _reason == key,
                    onTap: _submitting
                        ? null
                        : () => setState(() => _reason = key),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _commentController,
                  enabled: !_submitting,
                  minLines: 2,
                  maxLines: 4,
                  maxLength: 500,
                  style: TextStyle(color: colors.textPrimary, fontSize: 14),
                  decoration: InputDecoration(
                    hintText: t.feedback.cancel_comment_hint,
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
                    onPressed:
                        (_reason == null || _submitting) ? null : _submit,
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
                        : Text(t.feedback.cancel_submit,
                            style: const TextStyle(
                                fontWeight: FontWeight.w700, fontSize: 14)),
                  ),
                ),
                const SizedBox(height: 6),
                TextButton(
                  onPressed: _submitting ? null : _dismiss,
                  child: Text(
                    t.common.close,
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
      ),
    );
  }
}

/// A single selectable churn-reason row. The selected one fills with the brand
/// blue tint and shows a check; the rest stay outlined.
class _ReasonOption extends StatelessWidget {
  const _ReasonOption({
    required this.colors,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final AppColorsExtension colors;
  final String label;
  final bool selected;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(10),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
          decoration: BoxDecoration(
            color: selected
                ? colors.bluePrimary.withValues(alpha: 0.15)
                : colors.backgroundSecondary,
            borderRadius: BorderRadius.circular(10),
            border: Border.all(
              color: selected ? colors.bluePrimary : colors.borderLight,
            ),
          ),
          child: Row(
            children: [
              Icon(
                selected
                    ? Icons.radio_button_checked
                    : Icons.radio_button_unchecked,
                size: 18,
                color: selected ? colors.bluePrimary : colors.textMuted,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  label,
                  style: TextStyle(
                    color: colors.textPrimary,
                    fontSize: 14,
                    fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
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

/// Opens the cancellation survey. Caller decides *when* (see
/// [maybeShowCancellationSurvey]).
Future<void> showCancellationSurveyModal(BuildContext context) {
  return showDialog<void>(
    context: context,
    barrierDismissible: false,
    builder: (_) => const _CancellationSurveyModal(),
  );
}

/// Session-scoped dedupe so the survey shows at most once per app launch even
/// if the billing screen rebuilds. Mirrors the NPS / quota-wall pattern.
bool _cancellationSurveyShownThisSession = false;

@visibleForTesting
void resetCancellationSurveyShownForTest() {
  _cancellationSurveyShownThisSession = false;
}

/// Fire-and-forget helper: shows the cancellation survey iff [trigger] is true
/// (the `cancel_survey=1` return-URL flag) and it hasn't already been shown
/// this session. Safe to call from `initState`/`build` — defers to a
/// post-frame callback.
void maybeShowCancellationSurvey(
  BuildContext context, {
  required bool trigger,
}) {
  if (!trigger || _cancellationSurveyShownThisSession) return;
  _cancellationSurveyShownThisSession = true;
  WidgetsBinding.instance.addPostFrameCallback((_) {
    if (!context.mounted) return;
    showAsOneShotPrompt(
      () => showCancellationSurveyModal(context),
      onDeclined: () => _cancellationSurveyShownThisSession = false,
    );
  });
}
