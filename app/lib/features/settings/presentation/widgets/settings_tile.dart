import 'package:flutter/material.dart';

import 'package:app/design_system/theme_extensions.dart';

/// One row in Settings: icon, name, one line saying what it is for.
///
/// Every row on that screen was 26 lines of the same `ListTile` with the same
/// container, the same radius and the same chevron. Repeated forty times it made
/// a 1488-line file where regrouping anything meant moving forty blocks by hand,
/// and where the only way to see what Settings actually offered was to read all
/// of it.
class SettingsTile extends StatelessWidget {
  const SettingsTile({
    super.key,
    required this.icon,
    required this.title,
    required this.onTap,
    this.subtitle,
    this.iconColor,
    this.trailing,
    this.danger = false,
  });

  final IconData icon;
  final String title;

  /// What the row is for, in a person's words. Optional, but a row without one
  /// makes the reader open it to find out — which is the cost this screen is
  /// trying to remove.
  final String? subtitle;
  final VoidCallback onTap;

  /// Defaults to the theme's primary blue. Set it only where the colour carries
  /// meaning (red for compliance and logout, green for channels).
  final Color? iconColor;

  /// Replaces the chevron. Used where the row carries live state — the channel
  /// row shows how many are offline.
  final Widget? trailing;

  /// Paints the title in the alert colour. For rows that end something.
  final bool danger;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final tint = iconColor ?? colors.bluePrimary;

    return ListTile(
      onTap: onTap,
      leading: Container(
        width: 40,
        height: 40,
        alignment: Alignment.center,
        decoration: BoxDecoration(
          color: tint.withValues(alpha: 0.15),
          borderRadius: BorderRadius.circular(10),
        ),
        child: Icon(icon, color: tint, size: 22),
      ),
      title: Text(
        title,
        style: TextStyle(
          color: danger ? colors.redAlert : colors.textPrimary,
          fontWeight: FontWeight.w600,
        ),
      ),
      subtitle: subtitle == null
          ? null
          : Text(
              subtitle!,
              style: TextStyle(color: colors.textMuted, fontSize: 13),
            ),
      trailing: trailing ??
          Icon(Icons.chevron_right, color: colors.textMuted, size: 20),
    );
  }
}

/// A card holding related rows, drawing its own separators.
///
/// The separators are the reason this exists rather than a bare container.
/// Hand-written dividers between hand-written tiles have to be gated by the
/// same condition as the tile next to them, and one that isn't leaves a line
/// with nothing under it — the old screen carried an `if (isAdmin) Divider(…)`
/// purely to work around that. A group that draws its own separators cannot
/// strand one.
class SettingsGroup extends StatelessWidget {
  const SettingsGroup({super.key, required this.children});

  /// Nulls are dropped, so a caller can write `isAdmin ? tile : null` inline
  /// without also reasoning about the divider that would have followed it.
  final List<Widget?> children;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final rows = children.whereType<Widget>().toList();
    if (rows.isEmpty) return const SizedBox.shrink();

    final separated = <Widget>[];
    for (var i = 0; i < rows.length; i++) {
      if (i > 0) separated.add(Divider(height: 1, color: colors.borderLight));
      separated.add(rows[i]);
    }

    return Container(
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      child: Column(children: separated),
    );
  }
}

/// The `SECTION NAME` label above a group.
///
/// Uppercased here rather than in the translation files: three of these labels
/// used to shout from inside the JSON and four did not, so one screen carried
/// two conventions.
class SettingsSectionLabel extends StatelessWidget {
  const SettingsSectionLabel(this.title, {super.key});

  final String title;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Text(
        title.toUpperCase(),
        style: TextStyle(
          fontSize: 13,
          fontWeight: FontWeight.w700,
          color: colors.textMuted,
          letterSpacing: 1.0,
        ),
      ),
    );
  }
}
