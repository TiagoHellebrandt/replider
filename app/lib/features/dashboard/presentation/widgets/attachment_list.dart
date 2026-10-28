import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/dashboard/domain/models/message_attachment.dart';
import 'package:app/features/dashboard/presentation/controllers/attachment_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// The files that arrived with a message.
///
/// Every row states its scan verdict BEFORE the user clicks. Finding out that
/// a file is blocked by clicking it is how a product ends up implying a safety
/// it never checked — the whole point of the five states is that the reader
/// can tell "checked and clean" from "nobody looked".
class AttachmentList extends StatelessWidget {
  const AttachmentList(
      {super.key, required this.attachments, this.includeLargeInline = false});

  /// Also list inline images big enough to be content (a pasted photo).
  /// The email TEXT mode turns this on — it cannot render them in place,
  /// and without a row here the photo the mail exists for is unreachable.
  /// The formatted view keeps it off: there the image already renders.
  final bool includeLargeInline;

  /// Comes from the message itself — the server batches them onto the
  /// transcript. A provider per bubble would be an N+1, and a conversation
  /// with fifty emails would fire fifty requests.
  final List<MessageAttachment> attachments;

  @override
  Widget build(BuildContext context) {
    // Inline furniture (signature logos, tracking pixels) is stored but is
    // not what a person means by "anexo". Counting it teaches them to ignore
    // the indicator, and that is exactly when the real contract slips past.
    final items = attachments
        .where(
            (a) => !a.isInline || (includeLargeInline && a.isLargeInlineImage))
        .toList();
    if (items.isEmpty) return const SizedBox.shrink();

    return Padding(
      padding: const EdgeInsets.only(top: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          for (final a in items)
            _AttachmentRow(key: Key('attachment_${a.id}'), attachment: a),
        ],
      ),
    );
  }
}

class _AttachmentRow extends ConsumerStatefulWidget {
  const _AttachmentRow({super.key, required this.attachment});

  final MessageAttachment attachment;

  @override
  ConsumerState<_AttachmentRow> createState() => _AttachmentRowState();
}

class _AttachmentRowState extends ConsumerState<_AttachmentRow> {
  bool _busy = false;

  MessageAttachment get a => widget.attachment;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    return Container(
      margin: const EdgeInsets.only(bottom: 6),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: colors.backgroundTertiary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: a.scanStatus == AttachmentScanStatus.infected
              ? colors.redAlert
              : colors.borderLight,
        ),
      ),
      child: Row(
        children: [
          Icon(_icon(), size: 20, color: _statusColor(colors)),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  a.filename,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: typography.body.copyWith(
                      color: Colors.white,
                      fontSize: 13,
                      fontWeight: FontWeight.w500),
                ),
                Text(
                  '${a.readableSize} · ${_statusLabel()}',
                  key: Key('attachment_status_${a.id}'),
                  style: typography.body
                      .copyWith(color: _statusColor(colors), fontSize: 12),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          if (_busy)
            const SizedBox(
                width: 18,
                height: 18,
                child: CircularProgressIndicator(strokeWidth: 2))
          else
            TextButton(
              key: Key('attachment_open_${a.id}'),
              onPressed: _action(),
              child: Text(_actionLabel()),
            ),
        ],
      ),
    );
  }

  /// An infected file gets an explanation, never a way through. Anything
  /// else unscanned gets an explicit acknowledgement; a clean file just
  /// opens.
  VoidCallback _action() => switch (a.scanStatus) {
        AttachmentScanStatus.infected => _explainBlocked,
        AttachmentScanStatus.clean => _open,
        _ => _openWithAcknowledgement,
      };

  String _actionLabel() => switch (a.scanStatus) {
        AttachmentScanStatus.infected => t.attachment.why_blocked,
        AttachmentScanStatus.clean => t.attachment.open,
        _ => t.attachment.download_anyway,
      };

  IconData _icon() => switch (a.scanStatus) {
        AttachmentScanStatus.clean => Icons.attach_file,
        AttachmentScanStatus.infected => Icons.gpp_bad_outlined,
        _ => Icons.help_outline,
      };

  Color _statusColor(AppColorsExtension colors) => switch (a.scanStatus) {
        AttachmentScanStatus.clean => colors.textMuted,
        AttachmentScanStatus.infected => colors.redAlert,
        _ => colors.orangeAlert,
      };

  String _statusLabel() => switch (a.scanStatus) {
        AttachmentScanStatus.clean => t.attachment.status_clean,
        AttachmentScanStatus.pending => t.attachment.status_pending,
        AttachmentScanStatus.skipped => t.attachment.status_skipped,
        AttachmentScanStatus.failed => t.attachment.status_failed,
        AttachmentScanStatus.infected => t.attachment.status_infected,
      };

  /// Explains, and offers nothing. The dialog has one button because there
  /// is exactly one thing to do with a flagged file: not open it.
  Future<void> _explainBlocked() async {
    await showConfirmDialog(
      context,
      title: t.attachment.blocked_title,
      message: t.attachment.blocked_infected(signature: a.scanSignature ?? '—'),
      confirmText: t.common.close,
      cancelText: '',
    );
  }

  /// Opening a file nobody scanned is a real decision, so it is a real
  /// confirmation — not a second click on the same button.
  Future<void> _openWithAcknowledgement() async {
    final message = switch (a.scanStatus) {
      AttachmentScanStatus.pending => t.attachment.blocked_pending,
      AttachmentScanStatus.failed => t.attachment.blocked_failed,
      _ => t.attachment.blocked_skipped,
    };
    final ok = await showConfirmDialog(
      context,
      title: t.attachment.blocked_title,
      message: '$message\n\n${t.attachment.download_anyway_confirm}',
      confirmText: t.attachment.download_anyway,
    );
    if (ok) await _open(acknowledge: true);
  }

  Future<void> _open({bool acknowledge = false}) async {
    if (_busy) return;
    setState(() => _busy = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final url = await ref
          .read(attachmentActionsProvider)
          .downloadUrl(a.id, acknowledgeUnscanned: acknowledge);
      if (url.isEmpty) throw StateError('empty url');
      await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: Key('attachment_error_${a.id}'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }
}
