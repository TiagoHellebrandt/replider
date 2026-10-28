/// Web implementation of WhatsApp Embedded Signup using the Facebook
/// JS SDK.
///
/// The SDK and bridge helpers are injected by `web/index.html` under
/// `window.replider_fb`. This file only consumes them via js_interop —
/// no direct `<script>` manipulation from Dart, which keeps the
/// trusted-types posture clean and lets us swap SDK versions by
/// editing HTML alone.
library;

import 'dart:async';
import 'dart:js_interop';
import 'dart:js_interop_unsafe';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/i18n/strings.g.dart';

import 'fb_signup_stub.dart' show FbSignupResult;

export 'fb_signup_stub.dart' show FbSignupResult;

@JS('replider_fb.initSdk')
external JSPromise<JSAny?> _initSdk(JSString appId);

@JS('replider_fb.login')
external JSPromise<JSAny?> _login(JSString configId);

Future<FbSignupResult> launchWhatsAppEmbeddedSignup({
  required String appId,
  required String configId,
}) async {
  if (appId.isEmpty || configId.isEmpty) {
    return FbSignupResult(error: t.channels.embedded_signup_missing_env);
  }
  try {
    await _initSdk(appId.toJS).toDart;
    final jsResult = await _login(configId.toJS).toDart;
    if (jsResult == null) {
      return FbSignupResult(error: t.channels.embedded_signup_no_response);
    }
    final obj = jsResult as JSObject;
    final status = (obj['status'] as JSString?)?.toDart;
    final code = (obj['code'] as JSString?)?.toDart;
    final session = obj['session'];
    String? phoneNumberId;
    String? wabaId;
    if (session != null && session.isA<JSObject>()) {
      final s = session as JSObject;
      phoneNumberId = (s['phone_number_id'] as JSString?)?.toDart;
      wabaId = (s['waba_id'] as JSString?)?.toDart;
    }
    if (code == null || code.isEmpty) {
      if (status == 'not_authorized' || status == 'unknown') {
        return FbSignupResult(error: t.channels.embedded_signup_user_cancelled);
      }
      return FbSignupResult(
        error: t.channels.embedded_signup_no_code(status: status ?? ''),
      );
    }
    return FbSignupResult(
      code: code,
      phoneNumberId: phoneNumberId,
      wabaId: wabaId,
    );
  } catch (e) {
    return FbSignupResult(
        error: t.channels.embedded_signup_failure(error: parseApiError(e)));
  }
}
