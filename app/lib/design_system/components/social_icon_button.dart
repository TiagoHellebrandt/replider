import 'package:flutter/material.dart';
import 'package:app/design_system/theme_extensions.dart';

class SocialIconButton extends StatelessWidget {
  final IconData icon;
  final VoidCallback onPressed;
  final Color? color;

  const SocialIconButton({
    super.key,
    required this.icon,
    required this.onPressed,
    this.color,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final radii = Theme.of(context).extension<AppRadiiExtension>()!;

    return InkWell(
      onTap: onPressed,
      borderRadius: BorderRadius.circular(radii.full),
      child: Container(
        width: 56,
        height: 56,
        decoration: BoxDecoration(
          color: colors.glassLight,
          shape: BoxShape.circle,
          border: Border.all(
            color: colors.borderMedium,
            width: 1.0,
          ),
        ),
        child: Center(
          child: Icon(
            icon,
            color: color ?? colors.textPrimary,
            size: 24,
          ),
        ),
      ),
    );
  }
}
