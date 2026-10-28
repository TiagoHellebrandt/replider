import 'package:flutter/material.dart';
import 'package:app/design_system/theme_extensions.dart';

class PrimaryGradientButton extends StatelessWidget {
  final String text;
  final VoidCallback? onPressed;
  final bool isLoading;

  const PrimaryGradientButton({
    super.key,
    required this.text,
    required this.onPressed,
    this.isLoading = false,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final gradients = Theme.of(context).extension<AppGradientsExtension>()!;
    final radii = Theme.of(context).extension<AppRadiiExtension>()!;
    final shadows = Theme.of(context).extension<AppShadowsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    final isDisabled = onPressed == null || isLoading;

    return Container(
      width: double.infinity,
      height: 56,
      decoration: BoxDecoration(
        gradient: isDisabled ? null : gradients.primaryButton,
        color: isDisabled ? colors.glassHeavy : null,
        borderRadius: BorderRadius.circular(radii.large),
        boxShadow: isDisabled ? null : shadows.glowBlue,
      ),
      child: ElevatedButton(
        onPressed: isLoading ? null : onPressed,
        style: ElevatedButton.styleFrom(
          backgroundColor: Colors.transparent,
          shadowColor: Colors.transparent,
          disabledBackgroundColor: Colors.transparent,
          disabledForegroundColor: colors.textMuted,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(radii.large),
          ),
        ),
        child: isLoading
            ? const SizedBox(
                width: 24,
                height: 24,
                child: CircularProgressIndicator(
                  color: Colors.white,
                  strokeWidth: 2,
                ),
              )
            : Text(
                text,
                style: typography.body.copyWith(
                  fontWeight: FontWeight.w600,
                  color: isDisabled ? colors.textMuted : Colors.white,
                ),
              ),
      ),
    );
  }
}
