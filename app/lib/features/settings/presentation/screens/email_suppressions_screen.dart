import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/components/empty_state_card.dart';
import 'package:app/design_system/components/glass_card.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/components/settings_section_card.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/settings/presentation/controllers/email_suppression_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Addresses that no longer receive email from this account.
///
/// This screen exists because a silent block is worse than no block. Without
/// it a salesperson's message simply never goes out, with no list to consult
/// and no way to undo it — and they learn to distrust the whole tool. So
/// every blocked address is here, each with the remote server's own words,
/// and any of them can be released.
class EmailSuppressionsScreen extends ConsumerWidget {
  const EmailSuppressionsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final state = ref.watch(emailSuppressionsProvider);

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        // A deep link outside the shell: without this the screen has neither a
        // back button nor a menu — you get in and only get out by editing the
        // URL.
        leading: IconButton(
          tooltip: t.common.back,
          icon: const Icon(Icons.arrow_back),
          onPressed: () =>
              context.canPop() ? context.pop() : context.go('/settings'),
        ),
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        title: Text(
          t.suppressions.title,
          style: TextStyle(
            fontSize: 18,
            fontWeight: FontWeight.w700,
            color: colors.textPrimary,
          ),
        ),
      ),
      body: state.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => RetryableError(
          error: err,
          onRetry: () => ref.invalidate(emailSuppressionsProvider),
        ),
        data: (items) => SettingsPageBody(
          children: [
            Text(t.suppressions.subtitle,
                style: typography.body.copyWith(
                    color: colors.textSecondary, fontSize: 13, height: 1.4)),
            const SizedBox(height: 16),
            if (items.isEmpty)
              EmptyStateCard(
                key: const Key('suppressions_empty'),
                icon: Icons.mark_email_unread_outlined,
                title: t.suppressions.empty,
                subtitle: t.suppressions.empty_hint,
              )
            else
              for (final s in items)
                _SuppressionRow(
                  key: Key('suppression_${s.address}'),
                  suppression: s,
                  onRelease: () => _release(context, ref, s),
                ),
          ],
        ),
      ),
    );
  }

  Future<void> _release(
      BuildContext context, WidgetRef ref, EmailSuppression s) async {
    final messenger = ScaffoldMessenger.of(context);
    // Confirmed, because releasing puts an address a mail server refused —
    // or one that reported us as spam — back into circulation, and writing
    // to it again is what costs the sending domain its reputation.
    final confirmed = await showConfirmDialog(
      context,
      title: t.suppressions.release_title,
      message: '${s.address}\n\n${t.suppressions.release_warning}',
      confirmText: t.suppressions.release,
    );
    if (!confirmed) return;
    // In-flight guard: the row button stays enabled through the round-trip,
    // and a double tap fired two DELETEs.
    if (!_releasing.add(s.address)) return;

    try {
      await ref.read(emailSuppressionActionsProvider).release(s.address);
      ref.invalidate(emailSuppressionsProvider);
      messenger.showSnackBar(SnackBar(content: Text(t.suppressions.released)));
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: const Key('suppression_error'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      _releasing.remove(s.address);
    }
  }
}

/// Addresses with a release in flight (screen is a ConsumerWidget — this is
/// the minimal double-tap guard, not UI state worth a StatefulWidget).
final _releasing = <String>{};

class _SuppressionRow extends StatelessWidget {
  const _SuppressionRow({
    super.key,
    required this.suppression,
    required this.onRelease,
  });

  final EmailSuppression suppression;
  final VoidCallback onRelease;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: GlassCard(
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      suppression.address,
                      style: const TextStyle(
                          color: Colors.white,
                          fontSize: 14,
                          fontWeight: FontWeight.w600),
                    ),
                  ),
                  TextButton(
                    key: Key('release_${suppression.address}'),
                    onPressed: onRelease,
                    child: Text(t.suppressions.release),
                  ),
                ],
              ),
              const SizedBox(height: 4),
              Text(
                // A complaint is not a bounce: the address WORKS, and the
                // person told their provider we are abusive. Reading the two
                // as the same thing hides which mistake was made.
                suppression.isComplaint
                    ? t.suppressions.reason_complaint
                    : t.suppressions.reason_hard_bounce,
                style: typography.body.copyWith(
                  color: suppression.isComplaint
                      ? colors.redAlert
                      : colors.textSecondary,
                  fontSize: 12,
                ),
              ),
              if (suppression.diagnostic.isNotEmpty) ...[
                const SizedBox(height: 6),
                // Verbatim, monospaced: this is the answer to "why?", and
                // paraphrasing it would lose the code a support conversation
                // is won or lost on.
                Text(
                  suppression.diagnostic,
                  style: TextStyle(
                      color: colors.textMuted,
                      fontSize: 11,
                      fontFamily: 'monospace'),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
