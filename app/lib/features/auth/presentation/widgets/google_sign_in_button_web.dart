import 'package:flutter/material.dart';
import 'package:google_sign_in_web/web_only.dart' as web_only;

/// The actual GIS-rendered Google Sign-In button. Only loaded on web via the
/// conditional import in [google_sign_in_button.dart].
///
/// **Why this is rendered visibly instead of overlaid behind a custom button:**
///
/// Google's Identity Services SDK only emits credentials when the click
/// originates from a button it rendered itself (anti-fraud / trusted UI).
/// We tried the "invisible overlay" technique used in other frameworks
/// (custom button on top, GIS button beneath at opacity 0), but Flutter Web
/// has a known limitation: pointer events on canvas-rendered widgets do not
/// reliably pass through to platform views below them, even with
/// `IgnorePointer` or `Opacity(0)`. See:
/// https://github.com/flutter/flutter/issues/41142
///
/// The pragmatic solution is to render the GIS button visibly with the
/// configuration that best matches our dark design system. The
/// `filledBlack` theme + `pill` shape + `large` size yields a black pill
/// button that fits naturally in a dark-themed app.
///
/// If we ever need pixel-perfect matching, the alternative is to implement
/// the OAuth 2.0 flow manually (build the auth URL, open a popup, parse the
/// callback, exchange the code for tokens) — about 300 lines of code plus a
/// callback page. Not worth it for a single button.
class WebGoogleSignInButton extends StatelessWidget {
  const WebGoogleSignInButton({super.key});

  // Matches SocialOutlineButton's height so this button has the same vertical
  // footprint as the email/password ones above it.
  static const double _height = 54;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: double.infinity,
      height: _height,
      child: Center(
        child: web_only.renderButton(
          configuration: web_only.GSIButtonConfiguration(
            type: web_only.GSIButtonType.standard,
            // filledBlack: solid black background, white text, multi-color
            // "G" logo. The closest GIS preset to a dark-themed app.
            // Other options (`outline`, `filledBlue`) all assume a light bg.
            theme: web_only.GSIButtonTheme.filledBlack,
            size: web_only.GSIButtonSize.large,
            // Pill matches the rounded design of the email/password fields.
            shape: web_only.GSIButtonShape.pill,
            text: web_only.GSIButtonText.continueWith,
            logoAlignment: web_only.GSIButtonLogoAlignment.left,
          ),
        ),
      ),
    );
  }
}
