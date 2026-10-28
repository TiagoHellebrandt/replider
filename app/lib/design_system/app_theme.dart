import 'package:flutter/material.dart';
import 'theme_extensions.dart';

class AppTheme {
  static ThemeData get darkTheme {
    // Colors
    const backgroundPrimary = Color(0xFF0A0A0A);
    // Elevated surfaces are LIGHTER than the page. They used to be #030303 —
    // darker than the page they sat on — so every card, column, app bar and
    // input read as a flat black rectangle separated from a flat black page
    // by a 5-10% hairline. That inversion is most of what made the product
    // look like a 2012 admin panel; the fix is a token, not a redesign.
    //
    // #141414 is the smallest step that reads as raised on an OLED phone in
    // daylight while keeping white text at 18:1 and the error red at 4.9:1.
    const backgroundSecondary = Color(0xFF141414);
    const backgroundTertiary = Color(0xFF1F1F1F);

    const glassLight = Color(0x05FFFFFF); // rgba(255,255,255,0.02)
    const glassMedium = Color(0x0DFFFFFF); // rgba(255,255,255,0.05)
    const glassHeavy = Color(0x80000000); // rgba(0,0,0,0.5)

    const textPrimary = Color(0xFFFFFFFF);
    const textSecondary = Color(0xCCFFFFFF); // 0.8
    const textTertiary = Color(0x99FFFFFF); // 0.6
    // 40% white measured 3.77:1 on the page — below WCAG AA for body text,
    // and this is the token that labels nearly every field in the product.
    const textMuted = Color(0x80FFFFFF); // 0.5 → 5.01:1
    const textDisabled = Color(0x33FFFFFF); // 0.2
    const textInverted = Color(0xFF000000);

    const whatsappGreen = Color(0xFF25D366);
    const bluePrimary = Color(0xFF3B82F6);
    const blueDark = Color(0xFF2563EB);
    const indigoDark = Color(0xFF4F46E5);
    // DC2626 measured 4.10:1 on the page: the error colour failed AA, on the
    // one text a user most needs to read, for an audience in their fifties.
    const redAlert = Color(0xFFEF4444); // 5.26:1
    const orangeAlert = Color(0xFFF97316);
    const cyanGlow = Color(0xFF06B6D4);
    const limeGlow = Color(0xFF84CC16);

    // Borders climb with the surface: on a lighter card a 5% hairline is
    // invisible, which is why several screens ended up drawing what looked
    // like borderless blocks.
    const borderLight = Color(0x14FFFFFF); // 0.08
    const borderMedium = Color(0x29FFFFFF); // 0.16
    const borderHeavy = Color(0x33FFFFFF); // 0.20

    final appColors = const AppColorsExtension(
      backgroundPrimary: backgroundPrimary,
      backgroundSecondary: backgroundSecondary,
      backgroundTertiary: backgroundTertiary,
      glassLight: glassLight,
      glassMedium: glassMedium,
      glassHeavy: glassHeavy,
      textPrimary: textPrimary,
      textSecondary: textSecondary,
      textTertiary: textTertiary,
      textMuted: textMuted,
      textDisabled: textDisabled,
      textInverted: textInverted,
      whatsappGreen: whatsappGreen,
      bluePrimary: bluePrimary,
      blueDark: blueDark,
      indigoDark: indigoDark,
      redAlert: redAlert,
      orangeAlert: orangeAlert,
      cyanGlow: cyanGlow,
      limeGlow: limeGlow,
      borderLight: borderLight,
      borderMedium: borderMedium,
      borderHeavy: borderHeavy,
    );

    final appRadii = const AppRadiiExtension(
      small: 8,
      medium: 12,
      large: 16,
      xl: 24,
      full: 9999,
    );

    final appShadows = const AppShadowsExtension(
      glowBlue: [
        BoxShadow(
          color: Color(0x333B82F6), // rgba(59, 130, 246, 0.2)
          blurRadius: 40,
        ),
      ],
      glowWhite: [
        BoxShadow(
          color: Color(0x33FFFFFF), // 0.2
          blurRadius: 40,
        ),
      ],
      glowRed: [
        BoxShadow(
          color: Color(0xB2DC2626), // 0.7
          offset: Offset(0, 15),
          blurRadius: 40,
        ),
      ],
      glass: [
        BoxShadow(
          color: Color(0x80000000), // 0.5
          offset: Offset(0, 10),
          blurRadius: 30,
        ),
      ],
    );

    final appGradients = const AppGradientsExtension(
      textGlowRedOrange: LinearGradient(
        colors: [Color(0xFFDC2626), Color(0xFFF97316)],
        begin: Alignment.centerLeft,
        end: Alignment.centerRight,
      ),
      textFadeWhite: LinearGradient(
        colors: [Color(0xFFFFFFFF), Color(0x66FFFFFF)],
        begin: Alignment.topCenter,
        end: Alignment.bottomCenter,
      ),
      primaryButton: LinearGradient(
        colors: [Color(0xFF2563EB), Color(0xFF4F46E5)],
        begin: Alignment.centerLeft,
        end: Alignment.centerRight,
      ),
    );

    // Sized for who reads this: a salesperson in their fifties, on a phone,
    // between appointments. Nothing below 13; body is 15, not the 12-13 the
    // screens had settled into. Weights top out at w600 — the old code had
    // 290 uses of w700 against 6 of w400, and when everything is bold
    // nothing is emphasised.
    const inter = 'Inter';
    final appTypography = const AppTypographyExtension(
      display: TextStyle(
        fontFamily: inter,
        fontSize: 30,
        fontWeight: FontWeight.w600,
        height: 1.15,
        letterSpacing: -0.6,
        color: textPrimary,
      ),
      title: TextStyle(
        fontFamily: inter,
        fontSize: 22,
        fontWeight: FontWeight.w600,
        height: 1.25,
        letterSpacing: -0.2,
        color: textPrimary,
      ),
      heading: TextStyle(
        fontFamily: inter,
        fontSize: 18,
        fontWeight: FontWeight.w600,
        height: 1.30,
        letterSpacing: -0.1,
        color: textPrimary,
      ),
      // Text someone actually reads end to end — a conversation message, an
      // approach draft — gets more room than UI chrome does.
      bodyLarge: TextStyle(
        fontFamily: inter,
        fontSize: 16,
        fontWeight: FontWeight.w400,
        height: 1.50,
        color: textPrimary,
      ),
      body: TextStyle(
        fontFamily: inter,
        fontSize: 15,
        fontWeight: FontWeight.w400,
        height: 1.50,
        color: textPrimary,
      ),
      label: TextStyle(
        fontFamily: inter,
        fontSize: 14,
        fontWeight: FontWeight.w500,
        height: 1.40,
        color: textPrimary,
      ),
      caption: TextStyle(
        fontFamily: inter,
        fontSize: 13,
        fontWeight: FontWeight.w400,
        height: 1.40,
        color: textSecondary,
      ),
      eyebrow: TextStyle(
        fontFamily: inter,
        fontSize: 12,
        fontWeight: FontWeight.w600,
        height: 1.20,
        letterSpacing: 0.7,
        color: textSecondary,
      ),
    );

    return ThemeData(
      brightness: Brightness.dark,
      scaffoldBackgroundColor: backgroundPrimary,
      primaryColor: bluePrimary,
      colorScheme: const ColorScheme.dark(
        primary: bluePrimary,
        secondary: blueDark,
        surface: backgroundSecondary,
        error: redAlert,
      ),
      fontFamily: 'Inter',

      // THE SHAPE OF A TEXT FIELD LIVES HERE, and only here.
      //
      // Each screen used to write its own: 143 hand-rolled borders across ~50
      // files, with radius 4 (Flutter's default, which comes out square), 8, 10,
      // 12, 14 and
      // 100. The result was what you saw on screen — a square field beside a
      // rounded one, with no difference in meaning between them.
      //
      // 12 is the radius of the rest of the system (cards, buttons, chips). A
      // screen that needs something else now has to justify it, instead of
      // deciding alone and by accident.
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: Colors.transparent,
        hintStyle: TextStyle(color: textSecondary, fontSize: 14),
        labelStyle: TextStyle(color: textSecondary, fontSize: 14),
        contentPadding:
            const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
        border: _fieldBorder(borderMedium),
        enabledBorder: _fieldBorder(borderMedium),
        focusedBorder: _fieldBorder(bluePrimary, width: 1.5),
        disabledBorder: _fieldBorder(borderLight),
        errorBorder: _fieldBorder(redAlert),
        focusedErrorBorder: _fieldBorder(redAlert, width: 1.5),
      ),

      extensions: [
        appColors,
        appRadii,
        appShadows,
        appGradients,
        appTypography,
        const AppSpacingExtension(),
      ],
    );
  }
}

/// The border of every text field in the app. One function, so the radius exists
/// in a single place — the absence of that is what spread six different radii
/// across the screen.
OutlineInputBorder _fieldBorder(Color color, {double width = 1}) =>
    OutlineInputBorder(
      borderRadius: BorderRadius.circular(12),
      borderSide: BorderSide(color: color, width: width),
    );
