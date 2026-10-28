import 'package:flutter/widgets.dart';

/// Stub used by the conditional import in [google_sign_in_button.dart].
///
/// On non-web platforms (and in `flutter test` running on the VM), this is
/// what gets imported instead of the real GIS-based implementation. The
/// `web_only.dart` library from `google_sign_in_web` transitively pulls in
/// `dart:js_interop`, which is unavailable on the Dart VM and would otherwise
/// fail compilation of any test that touches the auth screens.
class WebGoogleSignInButton extends StatelessWidget {
  const WebGoogleSignInButton({super.key});

  @override
  Widget build(BuildContext context) {
    // Should never actually be rendered: callers gate this widget behind
    // `kIsWeb`. The throw makes mistakes loud rather than silent.
    throw UnsupportedError(
      'WebGoogleSignInButton is web-only. Guard usage with kIsWeb.',
    );
  }
}
