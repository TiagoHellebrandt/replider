import 'package:flutter/material.dart';
import 'package:app/design_system/theme_extensions.dart';

class SocialOutlineButton extends StatelessWidget {
  final String text;
  final VoidCallback onPressed;

  /// Provide either [icon] + [iconColor] for a simple Material icon,
  /// or [customIcon] for any widget (e.g. an SvgPicture for the Google logo).
  final IconData? icon;
  final Color? iconColor;
  final Widget? customIcon;

  const SocialOutlineButton({
    super.key,
    required this.text,
    required this.onPressed,
    this.icon,
    this.iconColor,
    this.customIcon,
  }) : assert(
          icon != null || customIcon != null,
          'Provide either icon or customIcon',
        );

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    final iconWidget = customIcon ?? Icon(icon, color: iconColor, size: 20);

    return SizedBox(
      width: double.infinity,
      height: 54,
      child: OutlinedButton(
        onPressed: onPressed,
        style: OutlinedButton.styleFrom(
          side: BorderSide(color: colors.borderMedium),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(100),
          ),
          foregroundColor: colors.textPrimary,
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            SizedBox(width: 20, height: 20, child: iconWidget),
            const SizedBox(width: 8),
            Flexible(
              child: Text(
                text,
                style: typography.body.copyWith(
                  fontWeight: FontWeight.w500,
                  fontSize: 14,
                  color: colors.textPrimary,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
