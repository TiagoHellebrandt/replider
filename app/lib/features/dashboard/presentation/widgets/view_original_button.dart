import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/dashboard/presentation/controllers/attachment_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// "View original" — the message as the sender formatted it.
///
/// The body a bubble renders is plain text, converted from HTML on the way
/// in. That is a deliberate security choice, and it costs the reader
/// formatting, inline images and the sender's signature. This is how they get
/// those back, without the conversion being weakened for everyone.
///
/// It opens in a BROWSER rather than inside the app: the protection is a
/// Content-Security-Policy the server sets on that response, and only a
/// browser enforces it.
class ViewOriginalButton extends ConsumerStatefulWidget {
  const ViewOriginalButton(
      {super.key,
      required this.messageId,
      this.imagesOnly = false,
      this.compact = false});

  final String messageId;

  /// Render only the "com imagens" action. Used where an inline sandboxed
  /// frame already covers the no-images read (the email inbox view) — three
  /// "original" buttons on one mail was two too many.
  final bool imagesOnly;

  /// Icon-only rendering of the "com imagens" action (tooltip carries the
  /// label). Used in the reader header, where the segmented view toggle
  /// already owns the horizontal space.
  final bool compact;

  /// Whether a message has markup worth offering. The key is present only
  /// when the original was actually stored, so a missing key means the button
  /// does not appear rather than appearing and failing.
  static bool isAvailable(Map<String, dynamic>? metadata) {
    final path = metadata?['email_html_path'];
    return path is String && path.trim().isNotEmpty;
  }

  @override
  ConsumerState<ViewOriginalButton> createState() => _ViewOriginalButtonState();
}

class _ViewOriginalButtonState extends ConsumerState<ViewOriginalButton> {
  bool _busy = false;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    if (widget.compact) {
      return IconButton(
        key: Key('view_original_images_${widget.messageId}'),
        tooltip: t.attachment.view_original_images,
        onPressed: _busy ? null : _confirmImages,
        icon: Icon(Icons.open_in_new, size: 17, color: colors.textSecondary),
        splashRadius: 18,
      );
    }

    return Padding(
      padding: const EdgeInsets.only(top: 6),
      child: Wrap(
        spacing: 4,
        children: [
          if (!widget.imagesOnly)
            TextButton.icon(
              key: Key('view_original_${widget.messageId}'),
              onPressed: _busy ? null : () => _open(loadImages: false),
              icon: Icon(Icons.article_outlined,
                  size: 16, color: colors.textSecondary),
              label: Text(t.attachment.view_original),
            ),
          TextButton(
            key: Key('view_original_images_${widget.messageId}'),
            onPressed: _busy ? null : _confirmImages,
            child: Text(t.attachment.view_original_images),
          ),
        ],
      ),
    );
  }

  /// Loading remote images is a decision with a consequence the reader cannot
  /// see: it tells the sender the message was opened, at a moment, from an IP.
  /// So it is asked, once, in those words.
  Future<void> _confirmImages() async {
    final ok = await showConfirmDialog(
      context,
      title: t.attachment.view_original_images,
      message: t.attachment.view_original_images_warning,
      confirmText: t.attachment.view_original_images,
    );
    if (ok) await _open(loadImages: true);
  }

  Future<void> _open({required bool loadImages}) async {
    setState(() => _busy = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final url = await ref
          .read(attachmentActionsProvider)
          .originalViewUrl(widget.messageId, loadImages: loadImages);
      await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: Key('view_original_error_${widget.messageId}'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }
}
