import 'package:flutter/material.dart';
import 'package:app/design_system/theme_extensions.dart';

/// Eyebrow-style section header used across Settings sub-screens
/// (Compliance, WhatsApp, Account). Centralises the tracking, weight,
/// and spacing so new screens don't drift away from the established
/// look. Pre-this, each screen inlined its own `Text` with subtly
/// different styles.
class SettingsSectionHeader extends StatelessWidget {
  final String label;
  final String? subtitle;
  final EdgeInsets padding;

  const SettingsSectionHeader({
    super.key,
    required this.label,
    this.subtitle,
    this.padding = const EdgeInsets.symmetric(vertical: 4),
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Padding(
      padding: padding,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label.toUpperCase(),
            style: TextStyle(
              color: colors.textMuted,
              fontSize: 11,
              fontWeight: FontWeight.w700,
              letterSpacing: 0.8,
            ),
          ),
          if (subtitle != null) ...[
            const SizedBox(height: 4),
            Text(
              subtitle!,
              style:
                  TextStyle(color: colors.textMuted, fontSize: 12, height: 1.4),
            ),
          ],
        ],
      ),
    );
  }
}
