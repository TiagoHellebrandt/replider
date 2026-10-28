import 'package:flutter/material.dart';

import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/dashboard/domain/models/sender_authenticity.dart';
import 'package:app/i18n/strings.g.dart';

/// Says when the platform could NOT confirm who sent a message.
///
/// Deliberately silent on the normal case. Marking every verified sender
/// would make the badge wallpaper, and then the one that matters — a domain
/// that did not authorise the send — would read as more of the same. The
/// badge appears only when there is something the reader would want to know.
class SenderAuthBadge extends StatelessWidget {
  const SenderAuthBadge({
    super.key,
    required this.authenticity,
    required this.detail,
  });

  final SenderAuthenticity authenticity;
  final SenderAuthDetail? detail;

  /// Builds the badge for a message, or nothing when there is nothing to say.
  ///
  /// Inbound only: an outbound message was written here, and "we verified you sent
  /// this yourself" is noise.
  ///
  /// EMAIL only, and that restriction is the point: SPF, DKIM and DMARC are
  /// facts about a mail server. On WhatsApp, Telegram, Instagram or SMS there
  /// is no verdict to have, so `fromMetadata` returned `unknown` for every
  /// single message and the badge stamped "Sender not verified" under
  /// every line a customer ever wrote. That is exactly the wallpaper this
  /// widget's own doc warns against — and worse than noise, because on a
  /// channel where the check is impossible it implies a risk that was never
  /// assessed. A conversation with no known source is treated as not-email:
  /// hiding a badge beats inventing one.
  static Widget? forMessage(Map<String, dynamic>? metadata,
      {required bool isInbound, ConversationSource? source}) {
    if (!isInbound) return null;
    if (source != ConversationSource.email) return null;
    final verdict = SenderAuthenticity.fromMetadata(metadata);
    if (!verdict.isWorthShowing) return null;
    return SenderAuthBadge(
      authenticity: verdict,
      detail: SenderAuthDetail.fromMetadata(metadata),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final failed = authenticity == SenderAuthenticity.failed;
    // A failed check is the shape a forged sender takes, so it gets the alarm
    // colour. An unchecked one is merely unknown — orange would cry wolf on
    // every message from an older provider, and a badge that cries wolf gets
    // ignored exactly when it should not be.
    final color = failed ? colors.redAlert : colors.textMuted;

    return Padding(
      padding: const EdgeInsets.only(top: 4),
      child: InkWell(
        key: Key(failed ? 'sender_auth_failed' : 'sender_auth_unknown'),
        onTap: () => _explain(context, failed),
        borderRadius: BorderRadius.circular(8),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 2),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(failed ? Icons.gpp_maybe_outlined : Icons.help_outline,
                  size: 14, color: color),
              const SizedBox(width: 4),
              Text(
                failed
                    ? t.sender_auth.badge_failed
                    : t.sender_auth.badge_unknown,
                style: typography.body.copyWith(color: color, fontSize: 12),
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// The explanation is written for a salesperson, not an administrator: it
  /// says what to DO. The raw check results come after, because that is what
  /// gets forwarded to whoever runs the mail server.
  Future<void> _explain(BuildContext context, bool failed) async {
    final checks = detail;
    final body =
        failed ? t.sender_auth.body_failed : t.sender_auth.body_unknown;
    await showConfirmDialog(
      context,
      title: failed ? t.sender_auth.title_failed : t.sender_auth.title_unknown,
      message: checks == null
          ? body
          : '$body\n\n${t.sender_auth.checks(checks: checks.toString())}',
      confirmText: t.common.close,
      cancelText: '',
    );
  }
}
