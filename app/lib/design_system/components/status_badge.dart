import 'package:flutter/material.dart';

/// Small pill used for status / plan / count labels (e.g. `ACTIVE`,
/// `SUSPENDED`, plan name, cap %). Tinted background + matching text in
/// the supplied semantic color. Replaces the hand-rolled
/// `Container + BoxDecoration + Text` pills that each screen used to
/// duplicate, so every badge across the app reads the same.
class StatusBadge extends StatelessWidget {
  final String label;
  final Color color;

  const StatusBadge({super.key, required this.label, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        label,
        style: TextStyle(
          color: color,
          fontSize: 11,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}
