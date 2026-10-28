import 'package:flutter/material.dart';

import 'package:app/design_system/theme_extensions.dart';

/// The one card every settings screen uses for a section of related
/// controls: solid surface, 16px radius, a header row with icon + bold
/// title, and the section body below.
///
/// Lifted from the account screen's private `_SectionCard` (2026-08-20)
/// when Brand/Profile/Claude each carried their own slightly different
/// card — same information, three paddings, two radii, one screen with
/// no section titles at all. One widget keeps the settings area reading
/// as one place.
///
/// [accent] tints the header AND the border — used by sections whose
/// content carries a warning (the brand kit's forbidden-claims list is
/// red because a violation there is a regulatory problem, not a style
/// choice).
class SettingsSectionCard extends StatelessWidget {
  const SettingsSectionCard({
    super.key,
    required this.title,
    required this.icon,
    required this.child,
    this.accent,
    this.subtitle,
    this.subtitleKey,
  });

  final String title;
  final IconData icon;
  final Widget child;
  final Color? accent;

  /// One quiet line under the title for sections whose purpose is not
  /// obvious from the name ("matched literally against every draft").
  final String? subtitle;

  /// Key for the subtitle Text, for tests that pin a specific caption
  /// (e.g. the brand kit's "cannot unlock what is forbidden" line).
  final Key? subtitleKey;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final titleColor = accent ?? colors.textPrimary;
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
            color: accent?.withValues(alpha: 0.35) ?? colors.borderLight),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Icon(icon, size: 18, color: titleColor),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  title,
                  style: TextStyle(
                    color: titleColor,
                    fontWeight: FontWeight.w700,
                    fontSize: 15,
                  ),
                ),
              ),
            ],
          ),
          if (subtitle != null) ...[
            const SizedBox(height: 6),
            Text(
              subtitle!,
              key: subtitleKey,
              style: TextStyle(
                  color: colors.textSecondary, fontSize: 12, height: 1.4),
            ),
          ],
          const SizedBox(height: 16),
          child,
        ],
      ),
    );
  }
}

/// The settings screens' shared page frame: scrolls, pads 24, and holds
/// the content to a centered 640px column so a wide monitor gets a form,
/// not a banner. Screens with tables (team metrics) may pass a wider
/// [maxWidth]; everything else takes the default.
class SettingsPageBody extends StatelessWidget {
  const SettingsPageBody({
    super.key,
    required this.children,
    this.maxWidth = 640,
  });

  final List<Widget> children;
  final double maxWidth;

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Center(
        child: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: maxWidth),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: children,
          ),
        ),
      ),
    );
  }
}
