import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:google_sign_in_platform_interface/google_sign_in_platform_interface.dart';

import 'package:app/design_system/components/social_outline_button.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/i18n/strings.g.dart';

// Conditional import: pulls the real GIS-based widget on web, and a stub
// (that throws if rendered) on the VM / mobile / desktop. This is the only
// way to keep the auth screens importable in `flutter test`, because
// `package:google_sign_in_web/web_only.dart` transitively depends on
// `dart:js_interop`, which the VM does not provide.
import 'google_sign_in_button_stub.dart'
    if (dart.library.html) 'google_sign_in_button_web.dart';

/// Read at compile time. On web, the `GoogleSignIn` plugin requires a client
/// ID passed via `--dart-define=GOOGLE_CLIENT_ID=...`. On mobile, the client
/// ID comes from `google-services.json` / `Info.plist` and this constant is
/// allowed to be empty.
const _googleClientId = String.fromEnvironment('GOOGLE_CLIENT_ID');

/// Whether the Google Sign-In button should be visible on the current
/// platform / build configuration. Callers (e.g. login/register screens)
/// can use this to also hide the "Or" divider when there are no social
/// providers to show.
///
/// - **Web**: visible only when `GOOGLE_CLIENT_ID` was provided at build
///   time. Without it, the GIS SDK cannot initialize and rendering anything
///   here would either hang ("Getting ready" forever) or produce a broken
///   button. Better to show nothing.
/// - **Mobile / desktop**: always visible — uses native sign-in flow which
///   doesn't depend on the dart-define.
bool get isGoogleSignInAvailable => kIsWeb ? _googleClientId.isNotEmpty : true;

/// Platform-aware Google Sign-In button.
///
/// **Web:** renders the official GIS button (controlled by the Google JS
/// SDK). The button itself opens the full OAuth flow when clicked — account
/// chooser, login, consent — and the resulting credential is delivered via
/// the [googleUserDataEventsProvider] stream. We listen to that stream and
/// forward the `idToken` to [AuthController.exchangeGoogleIdTokenForSession].
///
/// **Mobile / desktop:** renders the existing custom-styled button that
/// calls [AuthController.loginWithGoogle], which uses the native sign-in
/// flow that returns a credential synchronously.
///
/// **Why this split exists:** on web, `GoogleSignIn.signIn()` is deprecated
/// (returns `idToken == null`) and `signInSilently()` only works if the user
/// is already signed into Google in this browser. The rendered GIS button
/// is the only path that handles the full OAuth flow from a fresh browser
/// session and returns a real id_token.
class GoogleSignInButton extends ConsumerStatefulWidget {
  const GoogleSignInButton({super.key});

  @override
  ConsumerState<GoogleSignInButton> createState() => _GoogleSignInButtonState();
}

class _GoogleSignInButtonState extends ConsumerState<GoogleSignInButton> {
  ProviderSubscription<AsyncValue<GoogleSignInUserData?>>? _subscription;
  String? _lastHandledIdToken;

  @override
  void initState() {
    super.initState();
    if (kIsWeb) {
      // Listen to the GIS credential stream once. Each event with a non-null
      // idToken triggers a backend exchange.
      _subscription = ref.listenManual<AsyncValue<GoogleSignInUserData?>>(
        googleUserDataEventsProvider,
        (previous, next) {
          final data = next.asData?.value;
          if (data == null) return;
          final idToken = data.idToken;
          if (idToken == null || idToken.isEmpty) {
            debugPrint(
                '[GoogleSignInButton] stream emitted account without idToken');
            return;
          }

          // Avoid re-triggering if the same id_token arrives twice (the
          // stream replays the last value on rebuild / hot reload).
          if (_lastHandledIdToken == idToken) {
            debugPrint('[GoogleSignInButton] ignoring duplicate idToken');
            return;
          }
          _lastHandledIdToken = idToken;

          debugPrint('[GoogleSignInButton] forwarding idToken to backend');
          ref
              .read(authControllerProvider.notifier)
              .exchangeGoogleIdTokenForSession(idToken);
        },
        fireImmediately: false,
      );
    }
  }

  @override
  void dispose() {
    _subscription?.close();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // Web with a configured client ID: render the official GIS button.
    // The OAuth roundtrip and credential delivery happen through the
    // userDataEvents stream we listen to in initState — there's no
    // onPressed callback here.
    if (kIsWeb && _googleClientId.isNotEmpty) {
      return const WebGoogleSignInButton();
    }

    // Web without GOOGLE_CLIENT_ID: render nothing. The GIS SDK can't
    // initialize without a client ID, so any button here would be broken
    // (hang on "Getting ready" or fail silently on click). Callers should
    // use `isGoogleSignInAvailable` to also hide the surrounding "Or"
    // divider.
    if (kIsWeb) {
      return const SizedBox.shrink();
    }

    // Mobile / desktop: render the custom-styled button + native flow.
    return SocialOutlineButton(
      key: const Key('google_login_button'),
      text: t.auth.google,
      customIcon: SvgPicture.asset(
        'assets/google_logo.svg',
        width: 20,
        height: 20,
      ),
      onPressed: () {
        ref.read(authControllerProvider.notifier).loginWithGoogle();
      },
    );
  }
}
