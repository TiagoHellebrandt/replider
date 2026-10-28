import 'package:flutter/material.dart';
import 'package:app/design_system/theme_extensions.dart';

class PrimaryPillButton extends StatelessWidget {
  final String text;
  final VoidCallback? onPressed;
  final bool isLoading;

  const PrimaryPillButton({
    super.key,
    required this.text,
    required this.onPressed,
    this.isLoading = false,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    final isDisabled = onPressed == null || isLoading;

    return SizedBox(
      width: double.infinity,
      height: 54,
      child: ElevatedButton(
        onPressed: isLoading ? null : onPressed,
        style: ElevatedButton.styleFrom(
          // #4A89F3 was picked off a reference image and lived outside the
          // token set. White on it measures 3.41:1 — below WCAG AA — and
          // this is the login CTA, the first thing anyone sees. blueDark
          // carries white at 5.17:1 and is already a token.
          backgroundColor: colors.blueDark,
          foregroundColor: colors.textPrimary,
          disabledBackgroundColor: colors.borderMedium,
          disabledForegroundColor: colors.textSecondary,
          elevation: 0,
          shadowColor: Colors.transparent,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(100),
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
                  fontWeight: FontWeight.w500,
                  color: isDisabled ? colors.textSecondary : Colors.white,
                  fontSize: 16,
                ),
              ),
      ),
    );
  }
}
