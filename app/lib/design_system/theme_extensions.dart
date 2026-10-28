import 'dart:ui' show FontFeature;
import 'package:flutter/material.dart';

class AppColorsExtension extends ThemeExtension<AppColorsExtension> {
  final Color backgroundPrimary;
  final Color backgroundSecondary;
  final Color backgroundTertiary;

  final Color glassLight;
  final Color glassMedium;
  final Color glassHeavy;

  final Color textPrimary;
  final Color textSecondary;
  final Color textTertiary;
  final Color textMuted;
  final Color textDisabled;
  final Color textInverted;

  final Color whatsappGreen;
  final Color bluePrimary;
  final Color blueDark;
  final Color indigoDark;
  final Color redAlert;
  final Color orangeAlert;
  final Color cyanGlow;
  final Color limeGlow;

  final Color borderLight;
  final Color borderMedium;
  final Color borderHeavy;

  const AppColorsExtension({
    required this.backgroundPrimary,
    required this.backgroundSecondary,
    required this.backgroundTertiary,
    required this.glassLight,
    required this.glassMedium,
    required this.glassHeavy,
    required this.textPrimary,
    required this.textSecondary,
    required this.textTertiary,
    required this.textMuted,
    required this.textDisabled,
    required this.textInverted,
    required this.whatsappGreen,
    required this.bluePrimary,
    required this.blueDark,
    required this.indigoDark,
    required this.redAlert,
    required this.orangeAlert,
    required this.cyanGlow,
    required this.limeGlow,
    required this.borderLight,
    required this.borderMedium,
    required this.borderHeavy,
  });

  @override
  ThemeExtension<AppColorsExtension> copyWith({
    Color? backgroundPrimary,
    Color? backgroundSecondary,
    Color? backgroundTertiary,
    Color? glassLight,
    Color? glassMedium,
    Color? glassHeavy,
    Color? textPrimary,
    Color? textSecondary,
    Color? textTertiary,
    Color? textMuted,
    Color? textDisabled,
    Color? textInverted,
    Color? whatsappGreen,
    Color? bluePrimary,
    Color? blueDark,
    Color? indigoDark,
    Color? redAlert,
    Color? orangeAlert,
    Color? cyanGlow,
    Color? limeGlow,
    Color? borderLight,
    Color? borderMedium,
    Color? borderHeavy,
  }) {
    return AppColorsExtension(
      backgroundPrimary: backgroundPrimary ?? this.backgroundPrimary,
      backgroundSecondary: backgroundSecondary ?? this.backgroundSecondary,
      backgroundTertiary: backgroundTertiary ?? this.backgroundTertiary,
      glassLight: glassLight ?? this.glassLight,
      glassMedium: glassMedium ?? this.glassMedium,
      glassHeavy: glassHeavy ?? this.glassHeavy,
      textPrimary: textPrimary ?? this.textPrimary,
      textSecondary: textSecondary ?? this.textSecondary,
      textTertiary: textTertiary ?? this.textTertiary,
      textMuted: textMuted ?? this.textMuted,
      textDisabled: textDisabled ?? this.textDisabled,
      textInverted: textInverted ?? this.textInverted,
      whatsappGreen: whatsappGreen ?? this.whatsappGreen,
      bluePrimary: bluePrimary ?? this.bluePrimary,
      blueDark: blueDark ?? this.blueDark,
      indigoDark: indigoDark ?? this.indigoDark,
      redAlert: redAlert ?? this.redAlert,
      orangeAlert: orangeAlert ?? this.orangeAlert,
      cyanGlow: cyanGlow ?? this.cyanGlow,
      limeGlow: limeGlow ?? this.limeGlow,
      borderLight: borderLight ?? this.borderLight,
      borderMedium: borderMedium ?? this.borderMedium,
      borderHeavy: borderHeavy ?? this.borderHeavy,
    );
  }

  @override
  ThemeExtension<AppColorsExtension> lerp(
    covariant ThemeExtension<AppColorsExtension>? other,
    double t,
  ) {
    if (other is! AppColorsExtension) {
      return this;
    }
    return AppColorsExtension(
      backgroundPrimary:
          Color.lerp(backgroundPrimary, other.backgroundPrimary, t)!,
      backgroundSecondary:
          Color.lerp(backgroundSecondary, other.backgroundSecondary, t)!,
      backgroundTertiary:
          Color.lerp(backgroundTertiary, other.backgroundTertiary, t)!,
      glassLight: Color.lerp(glassLight, other.glassLight, t)!,
      glassMedium: Color.lerp(glassMedium, other.glassMedium, t)!,
      glassHeavy: Color.lerp(glassHeavy, other.glassHeavy, t)!,
      textPrimary: Color.lerp(textPrimary, other.textPrimary, t)!,
      textSecondary: Color.lerp(textSecondary, other.textSecondary, t)!,
      textTertiary: Color.lerp(textTertiary, other.textTertiary, t)!,
      textMuted: Color.lerp(textMuted, other.textMuted, t)!,
      textDisabled: Color.lerp(textDisabled, other.textDisabled, t)!,
      textInverted: Color.lerp(textInverted, other.textInverted, t)!,
      whatsappGreen: Color.lerp(whatsappGreen, other.whatsappGreen, t)!,
      bluePrimary: Color.lerp(bluePrimary, other.bluePrimary, t)!,
      blueDark: Color.lerp(blueDark, other.blueDark, t)!,
      indigoDark: Color.lerp(indigoDark, other.indigoDark, t)!,
      redAlert: Color.lerp(redAlert, other.redAlert, t)!,
      orangeAlert: Color.lerp(orangeAlert, other.orangeAlert, t)!,
      cyanGlow: Color.lerp(cyanGlow, other.cyanGlow, t)!,
      limeGlow: Color.lerp(limeGlow, other.limeGlow, t)!,
      borderLight: Color.lerp(borderLight, other.borderLight, t)!,
      borderMedium: Color.lerp(borderMedium, other.borderMedium, t)!,
      borderHeavy: Color.lerp(borderHeavy, other.borderHeavy, t)!,
    );
  }
}

class AppRadiiExtension extends ThemeExtension<AppRadiiExtension> {
  final double small;
  final double medium;
  final double large;
  final double xl;
  final double full;

  const AppRadiiExtension({
    required this.small,
    required this.medium,
    required this.large,
    required this.xl,
    required this.full,
  });

  @override
  ThemeExtension<AppRadiiExtension> copyWith({
    double? small,
    double? medium,
    double? large,
    double? xl,
    double? full,
  }) {
    return AppRadiiExtension(
      small: small ?? this.small,
      medium: medium ?? this.medium,
      large: large ?? this.large,
      xl: xl ?? this.xl,
      full: full ?? this.full,
    );
  }

  @override
  ThemeExtension<AppRadiiExtension> lerp(
    covariant ThemeExtension<AppRadiiExtension>? other,
    double t,
  ) {
    if (other is! AppRadiiExtension) {
      return this;
    }
    return AppRadiiExtension(
      small: small + (other.small - small) * t,
      medium: medium + (other.medium - medium) * t,
      large: large + (other.large - large) * t,
      xl: xl + (other.xl - xl) * t,
      full: full + (other.full - full) * t,
    );
  }
}

class AppShadowsExtension extends ThemeExtension<AppShadowsExtension> {
  final List<BoxShadow> glowBlue;
  final List<BoxShadow> glowWhite;
  final List<BoxShadow> glowRed;
  final List<BoxShadow> glass;

  const AppShadowsExtension({
    required this.glowBlue,
    required this.glowWhite,
    required this.glowRed,
    required this.glass,
  });

  @override
  ThemeExtension<AppShadowsExtension> copyWith({
    List<BoxShadow>? glowBlue,
    List<BoxShadow>? glowWhite,
    List<BoxShadow>? glowRed,
    List<BoxShadow>? glass,
  }) {
    return AppShadowsExtension(
      glowBlue: glowBlue ?? this.glowBlue,
      glowWhite: glowWhite ?? this.glowWhite,
      glowRed: glowRed ?? this.glowRed,
      glass: glass ?? this.glass,
    );
  }

  @override
  ThemeExtension<AppShadowsExtension> lerp(
    covariant ThemeExtension<AppShadowsExtension>? other,
    double t,
  ) {
    if (other is! AppShadowsExtension) return this;
    return AppShadowsExtension(
      glowBlue: BoxShadow.lerpList(glowBlue, other.glowBlue, t) ?? glowBlue,
      glowWhite: BoxShadow.lerpList(glowWhite, other.glowWhite, t) ?? glowWhite,
      glowRed: BoxShadow.lerpList(glowRed, other.glowRed, t) ?? glowRed,
      glass: BoxShadow.lerpList(glass, other.glass, t) ?? glass,
    );
  }
}

class AppGradientsExtension extends ThemeExtension<AppGradientsExtension> {
  final LinearGradient textGlowRedOrange;
  final LinearGradient textFadeWhite;
  final LinearGradient primaryButton;

  const AppGradientsExtension({
    required this.textGlowRedOrange,
    required this.textFadeWhite,
    required this.primaryButton,
  });

  @override
  ThemeExtension<AppGradientsExtension> copyWith({
    LinearGradient? textGlowRedOrange,
    LinearGradient? textFadeWhite,
    LinearGradient? primaryButton,
  }) {
    return AppGradientsExtension(
      textGlowRedOrange: textGlowRedOrange ?? this.textGlowRedOrange,
      textFadeWhite: textFadeWhite ?? this.textFadeWhite,
      primaryButton: primaryButton ?? this.primaryButton,
    );
  }

  @override
  ThemeExtension<AppGradientsExtension> lerp(
    covariant ThemeExtension<AppGradientsExtension>? other,
    double t,
  ) {
    if (other is! AppGradientsExtension) return this;
    return AppGradientsExtension(
      textGlowRedOrange:
          LinearGradient.lerp(textGlowRedOrange, other.textGlowRedOrange, t)!,
      textFadeWhite:
          LinearGradient.lerp(textFadeWhite, other.textFadeWhite, t)!,
      primaryButton:
          LinearGradient.lerp(primaryButton, other.primaryButton, t)!,
    );
  }
}

/// The type scale.
///
/// It used to have three styles against 21 distinct `fontSize:` literals
/// scattered through the app — so in practice there was no scale, and the
/// four sizes carrying 78% of the product's text (12, 13, 11, 14) had no
/// token at all.
///
/// The sizes below are chosen for who reads this: a salesperson in their
/// fifties, on a phone, between appointments. Presbyopia starts around 42.
/// Nothing here goes below 13, and `body` is 15 rather than the 12-13 the
/// app had settled into.
///
/// Weight matters as much as size. The old code had 290 uses of w700 and 6
/// of w400 — when almost everything is bold, nothing is emphasised, and the
/// result reads as shouting rather than hierarchy. Here w400 is the default
/// and w600 is the heaviest thing in normal use.
class AppTypographyExtension extends ThemeExtension<AppTypographyExtension> {
  /// Screen title, hero number. 30/600.
  final TextStyle display;

  /// Company name in a detail panel, sheet title. 22/600.
  final TextStyle title;

  /// Card header, Kanban column header. 18/600.
  final TextStyle heading;

  /// Text someone actually READS: a conversation message, an approach
  /// draft. 16/400 — deliberately larger than UI body.
  final TextStyle bodyLarge;

  /// The default. 15/400.
  final TextStyle body;

  /// Field label, name on a lead card, navigation item. 14/500.
  final TextStyle label;

  /// Metadata: "há 2 dias", counters, timestamps. 13/400. The floor.
  final TextStyle caption;

  /// UPPERCASE section header. 12/600 with tracking.
  final TextStyle eyebrow;

  const AppTypographyExtension({
    required this.display,
    required this.title,
    required this.heading,
    required this.bodyLarge,
    required this.body,
    required this.label,
    required this.caption,
    required this.eyebrow,
  });

  /// Tabular figures, for anything comparable down a column: money,
  /// percentages, counters, dates.
  ///
  /// In a proportional face a `1` is narrower than an `8`, so a total that
  /// updates over a WebSocket makes the whole column jump sideways. The
  /// reader does not diagnose the cause; they register "unstable".
  static const List<FontFeature> tabular = [FontFeature.tabularFigures()];

  @override
  ThemeExtension<AppTypographyExtension> copyWith({
    TextStyle? display,
    TextStyle? title,
    TextStyle? heading,
    TextStyle? bodyLarge,
    TextStyle? body,
    TextStyle? label,
    TextStyle? caption,
    TextStyle? eyebrow,
  }) {
    return AppTypographyExtension(
      display: display ?? this.display,
      title: title ?? this.title,
      heading: heading ?? this.heading,
      bodyLarge: bodyLarge ?? this.bodyLarge,
      body: body ?? this.body,
      label: label ?? this.label,
      caption: caption ?? this.caption,
      eyebrow: eyebrow ?? this.eyebrow,
    );
  }

  @override
  ThemeExtension<AppTypographyExtension> lerp(
    covariant ThemeExtension<AppTypographyExtension>? other,
    double t,
  ) {
    if (other is! AppTypographyExtension) return this;
    return AppTypographyExtension(
      display: TextStyle.lerp(display, other.display, t)!,
      title: TextStyle.lerp(title, other.title, t)!,
      heading: TextStyle.lerp(heading, other.heading, t)!,
      bodyLarge: TextStyle.lerp(bodyLarge, other.bodyLarge, t)!,
      body: TextStyle.lerp(body, other.body, t)!,
      label: TextStyle.lerp(label, other.label, t)!,
      caption: TextStyle.lerp(caption, other.caption, t)!,
      eyebrow: TextStyle.lerp(eyebrow, other.eyebrow, t)!,
    );
  }
}

/// Spacing tokens on a 4px grid. Screens used to scatter `SizedBox(height:
/// 8/12/16/24)` literals everywhere, so a designer tweaking rhythm had
/// to grep across dozens of files. The tokens are intentionally few —
/// five is enough for the whole app. Anything truly off-grid should be
/// a deliberate one-off in the widget rather than a new token.
///
/// Values:
///   xs  4 — icon-to-label, badge internal padding
///   sm  8 — between sibling controls
///   md 12 — card internal padding, form row gap
///   lg 16 — section gap, screen horizontal padding
///   xl 24 — between major blocks on a screen
///   xxl 32 — hero sections, before/after a screen's primary CTA
class AppSpacingExtension extends ThemeExtension<AppSpacingExtension> {
  final double xs;
  final double sm;
  final double md;
  final double lg;
  final double xl;
  final double xxl;

  const AppSpacingExtension({
    this.xs = 4,
    this.sm = 8,
    this.md = 12,
    this.lg = 16,
    this.xl = 24,
    this.xxl = 32,
  });

  @override
  ThemeExtension<AppSpacingExtension> copyWith({
    double? xs,
    double? sm,
    double? md,
    double? lg,
    double? xl,
    double? xxl,
  }) =>
      AppSpacingExtension(
        xs: xs ?? this.xs,
        sm: sm ?? this.sm,
        md: md ?? this.md,
        lg: lg ?? this.lg,
        xl: xl ?? this.xl,
        xxl: xxl ?? this.xxl,
      );

  @override
  ThemeExtension<AppSpacingExtension> lerp(
    covariant ThemeExtension<AppSpacingExtension>? other,
    double t,
  ) {
    // Spacing values don't animate in our UI — a hard switch is what
    // we want if the theme is ever swapped mid-frame.
    if (other is! AppSpacingExtension) return this;
    return other;
  }
}
