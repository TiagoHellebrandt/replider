import 'package:flutter/material.dart';

import 'package:app/design_system/theme_extensions.dart';

/// The primary-action pill from the Ofertas mockups: the button gradient in
/// a 999 radius, compact enough for a row, loud enough to be THE action.
class GradientPill extends StatelessWidget {
  const GradientPill({
    super.key,
    required this.label,
    required this.onTap,
    this.icon,
    this.fontSize = 12.5,
    this.padding = const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
    this.height,
    this.fitLabel = false,
  });

  final String label;
  final VoidCallback? onTap;
  final IconData? icon;
  final double fontSize;
  final EdgeInsets padding;

  /// A fixed height makes a pill ROW line up: siblings with different
  /// internal metrics (gradient ink vs OutlinedButton) otherwise render a
  /// staircase.
  final double? height;

  /// Shrink the label instead of overflowing when the pill is given less
  /// width than its words need. Off by default: a pill in a Wrap sizes to
  /// its content and should keep its type scale. On by the caller that puts
  /// it in an Expanded on a phone, where the width is whatever is left.
  final bool fitLabel;

  @override
  Widget build(BuildContext context) {
    final gradients = Theme.of(context).extension<AppGradientsExtension>()!;
    return Opacity(
      opacity: onTap == null ? 0.5 : 1,
      child: Material(
        color: Colors.transparent,
        child: Ink(
          decoration: BoxDecoration(
            gradient: gradients.primaryButton,
            borderRadius: BorderRadius.circular(999),
          ),
          child: InkWell(
            onTap: onTap,
            borderRadius: BorderRadius.circular(999),
            child: Container(
              height: height,
              padding: padding,
              // NO `alignment` here: on a Container it means "expand to the
              // incoming constraints and place the child" — inside a Wrap or
              // Row that stretched the pill across the whole card. The Row
              // centers its own children vertically within the height.
              child: _row(context),
            ),
          ),
        ),
      ),
    );
  }

  Widget _row(BuildContext context) {
    final row = Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (icon != null) ...[
          Icon(icon, size: fontSize + 2, color: Colors.white),
          const SizedBox(width: 7),
        ],
        Text(label,
            style: TextStyle(
                fontSize: fontSize,
                fontWeight: FontWeight.w600,
                color: Colors.white)),
      ],
    );
    if (!fitLabel) return row;
    return FittedBox(fit: BoxFit.scaleDown, child: row);
  }
}

/// The quiet secondary beside a [GradientPill]: outline, same height.
class GhostPill extends StatelessWidget {
  const GhostPill({
    super.key,
    required this.label,
    required this.onTap,
    this.fontSize = 12.5,
    this.padding = const EdgeInsets.symmetric(horizontal: 15, vertical: 8),
    this.height,
  });

  final String label;
  final VoidCallback? onTap;
  final double fontSize;
  final EdgeInsets padding;
  final double? height;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final button = OutlinedButton(
      onPressed: onTap,
      style: OutlinedButton.styleFrom(
        foregroundColor: colors.textPrimary,
        side: BorderSide(color: colors.borderHeavy),
        padding: padding,
        // Size.fromHeight carries an INFINITE width — the button ignores the
        // whole thing and collapses. A minimum height with zero-width floor
        // is what "same height as the pill beside me" actually means.
        minimumSize: height == null ? Size.zero : Size(0, height!),
        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(999)),
        textStyle: TextStyle(fontSize: fontSize, fontWeight: FontWeight.w600),
      ),
      child: Text(label),
    );
    // A hard SizedBox, not minimumSize: the theme's visual density quietly
    // shaves a few pixels off button minimums, and "same height as the pill
    // beside me" cannot be approximate.
    return height == null ? button : SizedBox(height: height, child: button);
  }
}
