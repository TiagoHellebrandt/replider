import 'package:flutter/material.dart';
import 'package:app/design_system/theme_extensions.dart';

/// Shared empty-state layout used across Settings sub-screens (Tags,
/// Canned Responses, Agents list, Team, etc.). Centralises the icon
/// size, spacing, and button style so new screens pick up the same
/// pattern for free — and so a future visual tweak happens in one
/// place instead of four.
///
/// Render as the full body of a screen (wrap in Center yourself if
/// rendering inside a Stack/Column).
class EmptyStateCard extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final String? ctaLabel;
  final VoidCallback? onCtaTap;

  const EmptyStateCard({
    super.key,
    required this.icon,
    required this.title,
    required this.subtitle,
    this.ctaLabel,
    this.onCtaTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 32),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, size: 64, color: colors.textDisabled),
            const SizedBox(height: 16),
            Text(
              title,
              style: TextStyle(
                color: colors.textPrimary,
                fontSize: 16,
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              subtitle,
              textAlign: TextAlign.center,
              style: TextStyle(
                color: colors.textMuted,
                fontSize: 13,
                height: 1.5,
              ),
            ),
            if (ctaLabel != null && onCtaTap != null) ...[
              const SizedBox(height: 20),
              ElevatedButton.icon(
                onPressed: onCtaTap,
                icon: const Icon(Icons.add, size: 18),
                label: Text(ctaLabel!),
                style: ElevatedButton.styleFrom(
                  backgroundColor: colors.bluePrimary,
                  foregroundColor: Colors.white,
                  padding:
                      const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
