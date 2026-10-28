/// Non-web stub for WhatsApp Embedded Signup.
///
/// The Facebook JS SDK only runs in a browser. On mobile we route users
/// to the manual credential form (see channels_screen) as a fallback —
/// the widget layer surfaces this to the user.
library;

import 'package:app/i18n/strings.g.dart';

class FbSignupResult {
  final String? code;
  final String? phoneNumberId;
  final String? wabaId;
  final String? error;

  const FbSignupResult(
      {this.code, this.phoneNumberId, this.wabaId, this.error});

  bool get isOk => code != null && error == null;
}

Future<FbSignupResult> launchWhatsAppEmbeddedSignup({
  required String appId,
  required String configId,
}) async {
  return FbSignupResult(error: t.channels.fb_signup_mobile_only);
}
