import 'package:flutter/material.dart';

import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/features/dashboard/presentation/widgets/download_helper.dart';
import 'package:app/i18n/strings.g.dart';

/// Full-screen image viewer with pinch/scroll zoom, metadata footer
/// (filename, date, caption) and a Download action that opens the
/// signed URL in a new tab — right-click-save style, but without
/// needing the user to know about it.
///
/// Built as a fullscreen dialog so Navigator.pop dismisses it back
/// to the conversation view without a route change in GoRouter.
Future<void> showImageViewer(
  BuildContext context, {
  required String url,
  String? filename,
  String? caption,
  DateTime? date,
}) {
  return Navigator.of(context, rootNavigator: true).push(
    PageRouteBuilder<void>(
      opaque: false,
      barrierColor: Colors.black87,
      pageBuilder: (_, __, ___) => ImageViewerScreen(
        url: url,
        filename: filename,
        caption: caption,
        date: date,
      ),
      transitionsBuilder: (_, animation, __, child) => FadeTransition(
        opacity: animation,
        child: child,
      ),
    ),
  );
}

class ImageViewerScreen extends StatelessWidget {
  final String url;
  final String? filename;
  final String? caption;
  final DateTime? date;

  const ImageViewerScreen({
    super.key,
    required this.url,
    this.filename,
    this.caption,
    this.date,
  });

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: Stack(
        fit: StackFit.expand,
        children: [
          // Backdrop: full-screen tap target that dismisses the viewer.
          // Kept BEHIND the image so a tap on the photo itself doesn't
          // fall through and close the overlay — only the empty area
          // around the image triggers pop.
          Positioned.fill(
            child: GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: () => Navigator.of(context).pop(),
            ),
          ),
          Center(
            child: InteractiveViewer(
              minScale: 0.5,
              maxScale: 6,
              child: GestureDetector(
                // Absorb taps on the image itself so the backdrop
                // dismiss doesn't fire. Empty onTap is intentional.
                onTap: () {},
                child: AppNetworkImage(
                  url: url,
                  fit: BoxFit.contain,
                  loadingBuilder: (_, child, progress) => progress == null
                      ? child
                      : const Center(
                          child: CircularProgressIndicator(color: Colors.white),
                        ),
                  fallback: const Center(
                    child: Icon(Icons.broken_image,
                        color: Colors.white54, size: 64),
                  ),
                ),
              ),
            ),
          ),
          // Top bar — filename on the left, actions on the right. Kept
          // in the overlay (not an AppBar) so it floats over the image
          // without reserving body space.
          Positioned(
            top: 0,
            left: 0,
            right: 0,
            child: SafeArea(
              bottom: false,
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    colors: [
                      Colors.black.withValues(alpha: 0.7),
                      Colors.transparent,
                    ],
                  ),
                ),
                child: Row(
                  children: [
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        filename ?? t.image_viewer.default_filename,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    const SizedBox(width: 8),
                    FilledButton.icon(
                      onPressed: () => _download(context),
                      icon: const Icon(
                        Icons.download_rounded,
                        size: 18,
                        color: Colors.black,
                      ),
                      label: Text(
                        t.image_viewer.download_btn,
                        style: const TextStyle(color: Colors.black),
                      ),
                      style: FilledButton.styleFrom(
                        backgroundColor: Colors.white,
                        foregroundColor: Colors.black,
                        iconColor: Colors.black,
                        padding: const EdgeInsets.symmetric(
                            horizontal: 14, vertical: 10),
                      ),
                    ),
                    const SizedBox(width: 4),
                    IconButton(
                      tooltip: t.image_viewer.close_tooltip,
                      icon: const Icon(Icons.close, color: Colors.white),
                      onPressed: () => Navigator.of(context).pop(),
                    ),
                  ],
                ),
              ),
            ),
          ),
          if ((caption != null && caption!.isNotEmpty) || date != null)
            Positioned(
              left: 0,
              right: 0,
              bottom: 0,
              child: IgnorePointer(
                child: Container(
                  padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
                  decoration: BoxDecoration(
                    gradient: LinearGradient(
                      begin: Alignment.topCenter,
                      end: Alignment.bottomCenter,
                      colors: [
                        Colors.transparent,
                        Colors.black.withValues(alpha: 0.75),
                      ],
                    ),
                  ),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (caption != null && caption!.isNotEmpty)
                        Text(
                          caption!,
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 14,
                            height: 1.4,
                          ),
                        ),
                      if (caption != null &&
                          caption!.isNotEmpty &&
                          date != null)
                        const SizedBox(height: 6),
                      if (date != null)
                        Text(
                          _formatDate(date!),
                          style: const TextStyle(
                            color: Colors.white70,
                            fontSize: 12,
                          ),
                        ),
                    ],
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }

  Future<void> _download(BuildContext context) async {
    // On web the helper fetches the signed URL as bytes and triggers a
    // blob-backed <a download> so the browser saves instead of opening
    // in a new tab (cross-origin GCS URLs ignore the download attribute
    // when pointed at directly). On mobile it falls back to
    // url_launcher so the system handler decides what to do.
    final messenger = ScaffoldMessenger.of(context);
    final name = filename ?? t.image_viewer.default_save_filename;
    final ok = await downloadFile(url, name);
    if (!ok) {
      messenger.showSnackBar(
        SnackBar(content: Text(t.image_viewer.download_failed)),
      );
    }
  }

  static String _formatDate(DateTime dt) {
    final local = dt.toLocal();
    String pad(int n) => n.toString().padLeft(2, '0');
    return '${pad(local.day)}/${pad(local.month)}/${local.year} ${pad(local.hour)}:${pad(local.minute)}';
  }
}
