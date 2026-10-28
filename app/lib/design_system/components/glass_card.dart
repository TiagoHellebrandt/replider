import 'dart:ui';
import 'package:flutter/material.dart';
import 'package:app/design_system/theme_extensions.dart';

class GlassCard extends StatelessWidget {
  final Widget child;
  final double width;
  final double? height;
  final EdgeInsetsGeometry padding;

  const GlassCard({
    super.key,
    required this.child,
    this.width = double.infinity,
    this.height,
    this.padding = const EdgeInsets.all(24.0),
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final radii = Theme.of(context).extension<AppRadiiExtension>()!;
    final shadows = Theme.of(context).extension<AppShadowsExtension>()!;

    return ClipRRect(
      borderRadius: BorderRadius.circular(radii.large),
      child: BackdropFilter(
        filter: ImageFilter.blur(sigmaX: 16.0, sigmaY: 16.0),
        child: Container(
          width: width,
          height: height,
          padding: padding,
          decoration: BoxDecoration(
            color: colors.glassMedium,
            borderRadius: BorderRadius.circular(radii.large),
            border: Border.all(
              color: colors.borderMedium,
              width: 1.0,
            ),
            boxShadow: shadows.glass,
          ),
          child: child,
        ),
      ),
    );
  }
}
