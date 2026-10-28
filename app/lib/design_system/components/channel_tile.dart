import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';

/// Selectable channel tile shared between the onboarding "connect channel"
/// step and the settings "add channel" dialog so both surfaces stay in
/// visual lock-step: SVG logo, animated selection state, optional marketing
/// badge (e.g. "2 min", "Popular", "Em breve").
class ChannelTile extends StatelessWidget {
  final ChannelType type;
  final bool isSelected;
  final VoidCallback onTap;
  final String? badgeText;
  final Color? badgeColor;
  final IconData? badgeIcon;
  final bool disabled;

  const ChannelTile({
    super.key,
    required this.type,
    required this.isSelected,
    required this.onTap,
    this.badgeText,
    this.badgeColor,
    this.badgeIcon,
    this.disabled = false,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final color = type.brandColor;
    final effectiveBadgeColor = badgeColor ?? color;
    final opacity = disabled ? 0.55 : 1.0;

    return Opacity(
      opacity: opacity,
      child: GestureDetector(
        onTap: disabled ? null : onTap,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 200),
          padding: const EdgeInsets.symmetric(vertical: 16, horizontal: 8),
          decoration: BoxDecoration(
            color:
                isSelected ? color.withValues(alpha: 0.15) : colors.glassMedium,
            borderRadius: BorderRadius.circular(16),
            border: Border.all(
              color: isSelected ? color : colors.borderLight,
              width: isSelected ? 2 : 1,
            ),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              _logo(colors),
              const SizedBox(height: 8),
              Text(
                type.label,
                style: TextStyle(
                  color: isSelected ? color : colors.textMuted,
                  fontSize: 14,
                  fontWeight: isSelected ? FontWeight.w700 : FontWeight.w400,
                ),
                textAlign: TextAlign.center,
                overflow: TextOverflow.ellipsis,
              ),
              if (badgeText != null) ...[
                const SizedBox(height: 6),
                Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                  decoration: BoxDecoration(
                    color: effectiveBadgeColor.withValues(alpha: 0.14),
                    borderRadius: BorderRadius.circular(6),
                    border: Border.all(
                      color: effectiveBadgeColor.withValues(alpha: 0.45),
                      width: 0.5,
                    ),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (badgeIcon != null) ...[
                        Icon(badgeIcon, size: 10, color: effectiveBadgeColor),
                        const SizedBox(width: 3),
                      ],
                      Flexible(
                        child: Text(
                          badgeText!,
                          style: TextStyle(
                            color: effectiveBadgeColor,
                            fontSize: 10,
                            fontWeight: FontWeight.w600,
                            height: 1.2,
                          ),
                          textAlign: TextAlign.center,
                          softWrap: true,
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  Widget _logo(AppColorsExtension colors) {
    // Webchat and email have no vendor SVG — fall back to a Material glyph
    // so the tile doesn't render a broken-image icon.
    if (type == ChannelType.webchat || type == ChannelType.email) {
      return Icon(
        type == ChannelType.email ? Icons.alternate_email : Icons.language,
        size: 32,
        color: isSelected ? type.brandColor : colors.textMuted,
      );
    }
    return SvgPicture.asset(
      type.logoAsset,
      width: 32,
      height: 32,
      colorFilter: isSelected
          ? null
          : ColorFilter.mode(colors.textMuted, BlendMode.srcIn),
    );
  }
}
