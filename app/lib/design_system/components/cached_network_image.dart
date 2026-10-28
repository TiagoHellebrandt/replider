import 'package:flutter/material.dart';
import 'package:app/design_system/theme_extensions.dart';

/// Thin wrapper around Image.network that adds:
///   * a consistent loading placeholder (circular progress, theme color)
///   * a consistent error fallback (broken-image icon + caption)
///   * automatic cacheWidth so avatars and thumbnails don't decode
///     the full 2000px original into memory just to render at 64px
///
/// We deliberately avoid adding a third-party package (cached_network_image
/// et al.) for now — Flutter's built-in ImageCache is enough for the
/// scales we target, and keeping the dependency footprint small makes
/// the web build faster.
///
/// Pass `renderWidth` = the max width the image will be rendered at
/// on screen (in logical pixels). The wrapper multiplies by devicePixelRatio
/// to pick a sensible cache width. Omit for full-resolution decodes.
class AppNetworkImage extends StatelessWidget {
  final String? url;
  final double? width;
  final double? height;
  final double? renderWidth;
  final BoxFit fit;
  final BorderRadius? borderRadius;
  final Widget? fallback;

  /// Optional overrides for callers that need a richer in-flight UI than the
  /// default centered spinner (e.g. a skeleton shimmer + crossfade for chat
  /// attachments). When provided they are passed straight through to the
  /// underlying [Image.network]; caching (`cacheWidth`) and the error
  /// fallback still apply.
  final ImageLoadingBuilder? loadingBuilder;
  final ImageFrameBuilder? frameBuilder;

  const AppNetworkImage({
    super.key,
    required this.url,
    this.width,
    this.height,
    this.renderWidth,
    this.fit = BoxFit.cover,
    this.borderRadius,
    this.fallback,
    this.loadingBuilder,
    this.frameBuilder,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    if (url == null || url!.isEmpty) {
      return _fallbackOrDefault(colors);
    }

    final dpr = MediaQuery.of(context).devicePixelRatio;
    final cacheW = renderWidth != null ? (renderWidth! * dpr).round() : null;

    Widget img = Image.network(
      url!,
      width: width,
      height: height,
      fit: fit,
      cacheWidth: cacheW,
      frameBuilder: frameBuilder,
      errorBuilder: (_, __, ___) => _fallbackOrDefault(colors),
      loadingBuilder: loadingBuilder ??
          (_, child, progress) {
            if (progress == null) return child;
            return SizedBox(
              width: width,
              height: height ?? 80,
              child: Center(
                child: SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: colors.cyanGlow,
                  ),
                ),
              ),
            );
          },
    );

    if (borderRadius != null) {
      img = ClipRRect(borderRadius: borderRadius!, child: img);
    }
    return img;
  }

  Widget _fallbackOrDefault(AppColorsExtension colors) {
    if (fallback != null) return fallback!;
    return Container(
      width: width,
      height: height ?? 80,
      decoration: BoxDecoration(
        color: colors.glassMedium,
        borderRadius: borderRadius,
      ),
      child:
          Icon(Icons.broken_image_outlined, color: colors.textMuted, size: 24),
    );
  }
}
