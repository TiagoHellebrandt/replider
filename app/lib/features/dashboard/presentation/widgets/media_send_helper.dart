import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/i18n/strings.g.dart';

/// Per-conversation in-flight upload tracker. Lives at module scope so a
/// rebuild between picking a file and the PUT finishing doesn't wipe the
/// guard. If the user picks a second file (paperclip + drag-drop, fast
/// double-tap) while the first PUT is still running, the second call
/// surfaces a toast and returns false instead of racing two PUTs that
/// land in the wrong order — or, worse, both succeeding and showing
/// duplicate messages.
final Set<String> _uploadsInFlight = <String>{};

/// Uploads a file to a conversation using the shared replyWithMedia
/// pipeline. Centralizes MIME detection, size gating, and user feedback
/// so the paperclip button and the drag-and-drop surfaces emit identical
/// UX. Returns true on success, false on any handled failure.
Future<bool> sendMediaToConversation({
  required WidgetRef ref,
  required BuildContext context,
  required String conversationId,
  required Uint8List bytes,
  required String filename,

  /// Overrides the auto-detected messageType ("image"/"audio"/...). Used
  /// by the in-app recorder so a mic-recorded .m4a ships as `voice`
  /// instead of the generic `audio` bubble.
  String? forceMessageType,

  /// When true, skips the preview/caption dialog. Used by flows that
  /// already have their own confirmation UI (e.g. the voice recorder's
  /// listen-before-send preview).
  bool skipPreview = false,
}) async {
  final messenger = ScaffoldMessenger.of(context);
  if (_uploadsInFlight.contains(conversationId)) {
    messenger.showSnackBar(SnackBar(
      behavior: SnackBarBehavior.floating,
      content: Text(t.media_send.upload_in_progress),
    ));
    return false;
  }
  final ext = _extensionOf(filename);
  final contentType = _contentTypeFromExt(ext);
  final messageType =
      forceMessageType ?? _messageTypeFromContentType(contentType);

  final maxBytes = _maxBytesFor(messageType);
  if (bytes.length > maxBytes) {
    final maxMb = (maxBytes / (1024 * 1024)).round();
    final actualMb = (bytes.length / (1024 * 1024)).toStringAsFixed(1);
    messenger.showSnackBar(SnackBar(
      behavior: SnackBarBehavior.floating,
      backgroundColor: Colors.red.shade700,
      content: Text(
        t.media_send
            .file_too_large(actualMb: actualMb, maxMb: maxMb.toString()),
      ),
    ));
    return false;
  }

  String caption = '';
  if (!skipPreview) {
    // Confirmation modal — shows a preview (image thumbnail when possible,
    // otherwise filename + type icon) plus an optional caption field so
    // the user can cancel a wrong drop/pick before anything hits GCS.
    final result = await _showMediaPreviewDialog(
      context: context,
      filename: filename,
      bytes: bytes,
      messageType: messageType,
    );
    if (result == null) {
      // User canceled or dismissed the dialog.
      return false;
    }
    caption = result.caption;
  }

  final icon = switch (messageType) {
    'image' => Icons.image_outlined,
    'video' => Icons.videocam_outlined,
    'audio' => Icons.audiotrack_outlined,
    'voice' => Icons.mic_none_outlined,
    _ => Icons.insert_drive_file_outlined,
  };
  messenger.showSnackBar(SnackBar(
    behavior: SnackBarBehavior.floating,
    duration: const Duration(seconds: 3),
    content: Row(
      children: [
        Icon(icon, color: Colors.white, size: 16),
        const SizedBox(width: 8),
        Expanded(
          child: Text(
            t.media_send.sending_file(filename: filename),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    ),
  ));

  _uploadsInFlight.add(conversationId);
  try {
    await ref.read(conversationActionsProvider.notifier).replyWithMedia(
          conversationId,
          bytes,
          filename,
          contentType,
          messageType,
          caption: caption,
        );
    return true;
  } catch (e) {
    messenger.showSnackBar(
      SnackBar(
          content:
              Text(t.conversation.file_send_error(error: parseApiError(e)))),
    );
    return false;
  } finally {
    _uploadsInFlight.remove(conversationId);
  }
}

class _MediaPreviewResult {
  final String caption;
  const _MediaPreviewResult({required this.caption});
}

Future<_MediaPreviewResult?> _showMediaPreviewDialog({
  required BuildContext context,
  required String filename,
  required Uint8List bytes,
  required String messageType,
}) {
  return showDialog<_MediaPreviewResult>(
    context: context,
    barrierDismissible: true,
    builder: (ctx) => _MediaPreviewDialog(
      filename: filename,
      bytes: bytes,
      messageType: messageType,
    ),
  );
}

class _MediaPreviewDialog extends StatefulWidget {
  final String filename;
  final Uint8List bytes;
  final String messageType;

  const _MediaPreviewDialog({
    required this.filename,
    required this.bytes,
    required this.messageType,
  });

  @override
  State<_MediaPreviewDialog> createState() => _MediaPreviewDialogState();
}

class _MediaPreviewDialogState extends State<_MediaPreviewDialog> {
  final _captionController = TextEditingController();

  @override
  void dispose() {
    _captionController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final sizeMb = (widget.bytes.length / (1024 * 1024)).toStringAsFixed(1);

    return Dialog(
      insetPadding: const EdgeInsets.symmetric(horizontal: 24, vertical: 24),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 420),
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                t.media_send.send_file_dialog_title,
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 12),
              ConstrainedBox(
                constraints: const BoxConstraints(maxHeight: 280),
                child: _previewContent(),
              ),
              const SizedBox(height: 12),
              Text(
                widget.filename,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
              ),
              Text(
                '$sizeMb MB',
                style: theme.textTheme.bodySmall,
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _captionController,
                decoration: InputDecoration(
                  labelText: t.media_send.caption_label,
                  isDense: true,
                ),
                maxLines: 2,
                minLines: 1,
                textInputAction: TextInputAction.done,
                onSubmitted: (_) => _confirm(),
              ),
              const SizedBox(height: 16),
              Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  TextButton(
                    onPressed: () => Navigator.of(context).pop(),
                    child: Text(t.media_send.cancel_btn),
                  ),
                  const SizedBox(width: 8),
                  FilledButton.icon(
                    onPressed: _confirm,
                    icon: const Icon(Icons.send, size: 16),
                    label: Text(t.media_send.send_btn),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _confirm() {
    Navigator.of(context).pop(
      _MediaPreviewResult(caption: _captionController.text.trim()),
    );
  }

  Widget _previewContent() {
    if (widget.messageType == 'image') {
      return ClipRRect(
        borderRadius: BorderRadius.circular(8),
        child: Image.memory(
          widget.bytes,
          fit: BoxFit.contain,
          errorBuilder: (_, __, ___) => _iconFallback(),
        ),
      );
    }
    return _iconFallback();
  }

  Widget _iconFallback() {
    final icon = switch (widget.messageType) {
      'image' => Icons.image_outlined,
      'video' => Icons.videocam_outlined,
      'audio' => Icons.audiotrack_outlined,
      'voice' => Icons.mic_none_outlined,
      _ => Icons.insert_drive_file_outlined,
    };
    return Container(
      height: 160,
      decoration: BoxDecoration(
        color: Colors.black26,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Center(
        child: Icon(icon, size: 64, color: Colors.white70),
      ),
    );
  }
}

String _extensionOf(String filename) {
  final dot = filename.lastIndexOf('.');
  if (dot < 0 || dot == filename.length - 1) return '';
  return filename.substring(dot + 1).toLowerCase();
}

String _contentTypeFromExt(String ext) => switch (ext) {
      'jpg' || 'jpeg' => 'image/jpeg',
      'png' => 'image/png',
      'gif' => 'image/gif',
      'webp' => 'image/webp',
      'mp4' => 'video/mp4',
      'mov' => 'video/quicktime',
      'mp3' => 'audio/mpeg',
      'ogg' || 'oga' => 'audio/ogg',
      'wav' => 'audio/wav',
      'm4a' => 'audio/mp4',
      'pdf' => 'application/pdf',
      'doc' => 'application/msword',
      'docx' =>
        'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
      _ => 'application/octet-stream',
    };

String _messageTypeFromContentType(String contentType) {
  if (contentType.startsWith('image/')) return 'image';
  if (contentType.startsWith('video/')) return 'video';
  if (contentType.startsWith('audio/')) return 'audio';
  return 'document';
}

/// WhatsApp Cloud API media size limits (Meta docs, 2026-04). Rejecting
/// oversize files before the upload starts avoids a long PUT ending in
/// a generic server error.
int _maxBytesFor(String messageType) {
  switch (messageType) {
    case 'image':
    case 'sticker':
      return 5 * 1024 * 1024;
    case 'audio':
    case 'voice':
    case 'video':
      return 16 * 1024 * 1024;
    default:
      return 100 * 1024 * 1024;
  }
}
