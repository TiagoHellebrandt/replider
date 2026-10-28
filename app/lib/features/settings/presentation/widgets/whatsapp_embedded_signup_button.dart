import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/settings/domain/models/whatsapp_error.dart';
import 'package:app/features/settings/presentation/controllers/channels_controller.dart';
import 'package:app/i18n/strings.g.dart';

// Conditional import: Dart pulls fb_signup_web.dart on the web and
// fb_signup_stub.dart everywhere else. The stub returns a friendly
// "use the manual form" error so the caller has a single API surface.
import 'fb_signup_stub.dart' if (dart.library.js_interop) 'fb_signup_web.dart';

const _appId = String.fromEnvironment('WHATSAPP_APP_ID');
const _configId = String.fromEnvironment('WHATSAPP_EMBEDDED_CONFIG_ID');

/// Primary connect-WhatsApp entry point. One tap:
///   1. Loads the Facebook JS SDK (web only).
///   2. Opens Meta's Embedded Signup popup.
///   3. Captures the OAuth code + session (phone_number_id, waba_id).
///   4. Hands them to our backend's `/channels/whatsapp/signup` which
///      exchanges the code for a long-lived System User token,
///      registers the phone number, subscribes webhooks, and creates
///      the channel — all in one round-trip.
/// On mobile we degrade to the manual credential form instead.
class WhatsAppEmbeddedSignupButton extends ConsumerStatefulWidget {
  final String? agentId;
  final VoidCallback? onConnected;

  const WhatsAppEmbeddedSignupButton({
    super.key,
    this.agentId,
    this.onConnected,
  });

  @override
  ConsumerState<WhatsAppEmbeddedSignupButton> createState() =>
      _WhatsAppEmbeddedSignupButtonState();
}

class _WhatsAppEmbeddedSignupButtonState
    extends ConsumerState<WhatsAppEmbeddedSignupButton> {
  bool _loading = false;

  Future<void> _handleTap() async {
    if (_loading) return;
    if (_appId.isEmpty || _configId.isEmpty) {
      _snack(
        t.channels.embedded_signup_missing_config,
        isError: true,
      );
      return;
    }
    setState(() => _loading = true);
    final result = await launchWhatsAppEmbeddedSignup(
      appId: _appId,
      configId: _configId,
    );
    if (!mounted) return;
    if (!result.isOk) {
      setState(() => _loading = false);
      _snack(result.error ?? t.channels.embedded_signup_unknown_failure,
          isError: true);
      return;
    }

    // Hand off to backend. Phone/WABA can be null if Meta sent us only
    // the code (some flows defer the session_info event); the API can
    // still exchange the token and look up the phone number itself.
    //
    // Recovery: Meta's authorization codes are valid for ~1h, so a
    // transient backend failure (5xx, network blip) shouldn't force
    // the user to restart the popup. We retry once with a short
    // backoff before surfacing the error. For non-transient errors
    // (4xx — bad payload, conflict, plan limit) we abort on the first
    // attempt because retrying never helps.
    final ds = ref.read(channelsRemoteDataSourceProvider);
    final body = {
      'code': result.code,
      if (result.phoneNumberId != null) 'phone_number_id': result.phoneNumberId,
      if (result.wabaId != null) 'waba_id': result.wabaId,
      if (widget.agentId != null) 'agent_id': widget.agentId,
    };
    Object? lastError;
    for (var attempt = 0; attempt < 2; attempt++) {
      try {
        await ds.completeWhatsAppSignup(body);
        lastError = null;
        break;
      } on DioException catch (e) {
        lastError = e;
        final status = e.response?.statusCode ?? 0;
        // Retry only on 5xx + connection errors. 4xx (esp. 409
        // WA_PHONE_OWNED_ELSEWHERE, 422 plan limit) won't change.
        final retriable = status >= 500 ||
            status == 0 ||
            e.type == DioExceptionType.connectionTimeout ||
            e.type == DioExceptionType.receiveTimeout ||
            e.type == DioExceptionType.connectionError;
        if (!retriable || attempt == 1) break;
        await Future<void>.delayed(const Duration(seconds: 2));
      } catch (e) {
        lastError = e;
        break;
      }
    }
    if (!mounted) {
      return;
    }
    if (lastError == null) {
      await ref.read(channelsListProvider.notifier).refresh();
      if (!mounted) return;
      setState(() => _loading = false);
      _snack(t.channels.embedded_signup_success, isError: false);
      widget.onConnected?.call();
      return;
    }
    setState(() => _loading = false);
    String msg;
    if (lastError is DioException) {
      msg = WhatsAppError.fromDio(lastError)?.humanize() ??
          t.channels
              .embedded_signup_complete_failed(error: lastError.message ?? '');
    } else {
      msg = t.channels
          .embedded_signup_complete_failed(error: lastError.toString());
    }
    _snack(msg, isError: true);
  }

  void _snack(String msg, {required bool isError}) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(msg),
      backgroundColor: isError ? Colors.red.shade700 : null,
      behavior: SnackBarBehavior.floating,
    ));
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return ElevatedButton.icon(
      onPressed: _loading ? null : _handleTap,
      icon: _loading
          ? const SizedBox(
              width: 16,
              height: 16,
              child: CircularProgressIndicator(
                  strokeWidth: 2, color: Colors.white),
            )
          : const Icon(Icons.chat, size: 18),
      label: Text(_loading
          ? t.channels.embedded_signup_connecting
          : t.channels.embedded_signup_cta),
      style: ElevatedButton.styleFrom(
        backgroundColor: colors.whatsappGreen,
        foregroundColor: colors.textInverted,
        padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 12),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
      ),
    );
  }
}
