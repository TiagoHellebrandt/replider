import 'package:dio/dio.dart';

import 'package:app/i18n/strings.g.dart';

/// Strongly-typed view over the `AppError` envelope the API returns for
/// WhatsApp failures. The backend maps Meta's error codes (190, 100,
/// 131026, etc.) into a small set of kinds (`TOKEN_EXPIRED`,
/// `OUTSIDE_24H_WINDOW`, ...) so the UI can pick a CTA — e.g. show
/// "Reconectar via Meta" when the System User token expired, or
/// "Use template" when the 24h service window closed.
class WhatsAppError {
  final String code;
  final String message;
  final int? httpStatus;

  /// Meta error subcode (e.g. 2388023 = template name exists,
  /// 2388299 = content/category mismatch). Copied verbatim from the
  /// backend metadata envelope so humanize() can pick more specific
  /// copy than the generic "Invalid parameter" string Meta returns.
  final int? metaSubcode;
  final int? metaCode;
  final String? fbTraceId;

  /// Meta's localized user-facing copy (error_user_title / error_user_msg)
  /// surfaced by the backend. These carry the real reason while the
  /// top-level `message` often stays a generic "Invalid parameter".
  final String? metaUserTitle;
  final String? metaUserMsg;

  const WhatsAppError({
    required this.code,
    required this.message,
    this.httpStatus,
    this.metaSubcode,
    this.metaCode,
    this.fbTraceId,
    this.metaUserTitle,
    this.metaUserMsg,
  });

  bool get isTokenExpired => code == 'TOKEN_EXPIRED';
  bool get isOutsideWindow => code == 'OUTSIDE_24H_WINDOW';
  bool get isMissingPermission => code == 'MISSING_PERMISSION';
  bool get isMissingCredentials =>
      code == 'MISSING_WABA_ID' || code == 'MISSING_TOKEN';
  bool get isChannelNotFound => code == 'CHANNEL_NOT_FOUND';

  /// True when [humanize] returns curated, channel-specific copy for this
  /// code. Lets callers prefer humanize() for WA failures and route
  /// cross-cutting codes (TRIAL_EXPIRED, AUTH_*, ...) through the generic
  /// localized error parser instead of echoing the raw API message.
  bool get isKnown => switch (code) {
        'TOKEN_EXPIRED' ||
        'OUTSIDE_24H_WINDOW' ||
        'MISSING_PERMISSION' ||
        'INVALID_PARAM' ||
        'RECIPIENT_ERROR' ||
        'TEMPLATE_ERROR' ||
        'MISSING_WABA_ID' ||
        'MISSING_TOKEN' ||
        'WA_REGISTER_RATE_LIMITED' ||
        'WA_PIN_MISMATCH' ||
        'WA_ALREADY_VERIFIED' ||
        'WA_CERT_2FA_BLOCKED' ||
        'WA_OWNERSHIP_NOT_VERIFIED' ||
        'WA_VERIFY_CODE_INVALID' ||
        'WA_REGISTER_FAILED' ||
        'WA_DEREGISTER_FAILED' ||
        'WA_REQUEST_CODE_FAILED' ||
        'WA_VERIFY_CODE_FAILED' ||
        'WA_SUBSCRIBE_FAILED' ||
        'WEBHOOK_STATUS_FAILED' ||
        'WA_TOKEN_EXCHANGE_FAILED' ||
        'WA_TEMPLATE_CREATE_FAILED' ||
        'WA_TEMPLATE_DELETE_FAILED' ||
        'WA_PROFILE_FETCH_FAILED' ||
        'WA_PROFILE_UPDATE_FAILED' ||
        'WA_PICTURE_UPLOAD_FAILED' ||
        'WA_PHONE_OWNED_ELSEWHERE' ||
        'TRIAL_EXPIRED' ||
        'SUBSCRIPTION_INACTIVE' ||
        'MISSING_CREDS' ||
        'NOT_SUPPORTED' =>
          true,
        _ => false,
      };

  /// Short, human-friendly copy to show directly in UI. Localized
  /// because this is all the user will see; the raw Meta message is often
  /// technical and English-only.
  String humanize() {
    final wa = t.whatsapp;
    switch (code) {
      case 'TOKEN_EXPIRED':
        return wa.error_token_expired;
      case 'OUTSIDE_24H_WINDOW':
        return wa.error_outside_window;
      case 'MISSING_PERMISSION':
        return wa.error_missing_permission;
      case 'INVALID_PARAM':
        return _invalidParamHint();
      case 'RECIPIENT_ERROR':
        return wa.error_recipient(message: message);
      case 'TEMPLATE_ERROR':
        return wa.error_template(message: message);
      case 'MISSING_WABA_ID':
        return wa.error_missing_waba;
      case 'MISSING_TOKEN':
        return wa.error_missing_token;
      case 'WA_REGISTER_RATE_LIMITED':
        // Meta locks /register and /deregister for this phone for ~24h
        // after a few failed attempts. Each retry resets the cooldown,
        // so the only useful action is to wait.
        return wa.error_register_rate_limited;
      case 'WA_PIN_MISMATCH':
        return wa.error_pin_mismatch;
      case 'WA_ALREADY_VERIFIED':
        return wa.error_already_verified;
      case 'WA_CERT_2FA_BLOCKED':
        return wa.error_cert_2fa_blocked;
      case 'WA_OWNERSHIP_NOT_VERIFIED':
        return wa.error_ownership_not_verified;
      case 'WA_VERIFY_CODE_INVALID':
        return wa.error_verify_code_invalid;
      // Operational fallbacks emitted by mapMetaError when no Meta Kind
      // matches. We translate the noun in each case so the user sees
      // localized copy instead of "WA_REGISTER_FAILED".
      case 'WA_REGISTER_FAILED':
        return wa.error_register_failed(detail: _metaDetail());
      case 'WA_DEREGISTER_FAILED':
        return wa.error_deregister_failed(detail: _metaDetail());
      case 'WA_REQUEST_CODE_FAILED':
        return wa.error_request_code_failed(detail: _metaDetail());
      case 'WA_VERIFY_CODE_FAILED':
        return wa.error_verify_code_failed;
      case 'WA_SUBSCRIBE_FAILED':
        return wa.error_subscribe_failed(detail: _metaDetail());
      case 'WEBHOOK_STATUS_FAILED':
        return wa.error_webhook_status_failed(detail: _metaDetail());
      case 'WA_TOKEN_EXCHANGE_FAILED':
        return wa.error_token_exchange_failed;
      case 'WA_TEMPLATE_CREATE_FAILED':
        return wa.error_template_create_failed(detail: _metaDetail());
      case 'WA_TEMPLATE_DELETE_FAILED':
        return wa.error_template_delete_failed(detail: _metaDetail());
      case 'WA_PROFILE_FETCH_FAILED':
        return wa.error_profile_fetch_failed;
      case 'WA_PROFILE_UPDATE_FAILED':
        return wa.error_profile_update_failed(detail: _metaDetail());
      case 'WA_PICTURE_UPLOAD_FAILED':
        return wa.error_picture_upload_failed;
      case 'WA_PHONE_OWNED_ELSEWHERE':
        return wa.error_phone_owned_elsewhere;
      case 'MISSING_CREDS':
        return wa.error_missing_creds;
      case 'NOT_SUPPORTED':
        return wa.error_not_supported;
      case 'TRIAL_EXPIRED':
        return wa.error_trial_expired;
      case 'SUBSCRIPTION_INACTIVE':
        return wa.error_subscription_inactive;
      default:
        return message.isNotEmpty
            ? message
            : wa.error_unknown_with_code(code: code);
    }
  }

  /// Picks the most specific Meta-side detail the API surfaced. Falls
  /// back to the top-level message + Meta code so support can grep.
  String _metaDetail() {
    if (metaUserMsg != null && metaUserMsg!.isNotEmpty) {
      return metaUserMsg!;
    }
    if (message.isNotEmpty) {
      return message;
    }
    if (metaCode != null) {
      return t.whatsapp.error_meta_code(code: metaCode!.toString());
    }
    return '';
  }

  /// Subcode-aware copy for the vague "Invalid parameter" error. Meta
  /// returns the same literal message for wildly different failures
  /// (duplicate name, content-vs-category mismatch, malformed components),
  /// so we disambiguate via the numeric subcode surfaced in the backend
  /// metadata envelope.
  String _invalidParamHint() {
    // Meta's localized user_msg is the authoritative reason when
    // present — use it verbatim instead of the subcode heuristic.
    if (metaUserMsg != null && metaUserMsg!.isNotEmpty) {
      final title = metaUserTitle ?? '';
      return title.isNotEmpty ? '$title: ${metaUserMsg!}' : metaUserMsg!;
    }
    final wa = t.whatsapp;
    switch (metaSubcode) {
      case 2388023:
        return wa.error_invalid_param_duplicate_name;
      case 2388024:
        return wa.error_invalid_param_paused;
      case 2388043:
        return wa.error_invalid_param_language;
      case 2388299:
        // The single biggest source of this subcode is content that
        // doesn't fit the declared category — e.g. newsletter copy
        // submitted as UTILITY. Point the user straight at that fix.
        return wa.error_invalid_param_category_mismatch;
      default:
        final trace = fbTraceId != null ? ' (fb_trace=$fbTraceId)' : '';
        return wa.error_invalid_param_default(message: message, trace: trace);
    }
  }

  static WhatsAppError? fromDio(DioException e) {
    final raw = e.response?.data;
    Map<String, dynamic>? data;
    if (raw is Map<String, dynamic>) {
      data = raw;
    } else if (raw is Map) {
      // Dio occasionally hands back Map<dynamic, dynamic> when the
      // response goes through a non-default decoder. Rebuild as
      // Map<String, dynamic> so the field reads below stay typed.
      data = raw.map((k, v) => MapEntry(k.toString(), v));
    }
    if (data != null) {
      final code = data['code'] as String?;
      final rawMsg = data['message'];
      final rawErr = data['error'];
      final message =
          (rawMsg is String) ? rawMsg : (rawErr is String ? rawErr : null);
      if (code != null && message != null) {
        final md = data['metadata'];
        int? metaSubcode;
        int? metaCode;
        String? fbTraceId;
        String? metaUserTitle;
        String? metaUserMsg;
        if (md is Map) {
          metaSubcode = (md['meta_subcode'] as num?)?.toInt();
          metaCode = (md['meta_code'] as num?)?.toInt();
          fbTraceId = md['fb_trace_id'] as String?;
          metaUserTitle = md['meta_user_title'] as String?;
          metaUserMsg = md['meta_user_msg'] as String?;
        }
        return WhatsAppError(
          code: code,
          message: message,
          httpStatus: e.response?.statusCode,
          metaSubcode: metaSubcode,
          metaCode: metaCode,
          fbTraceId: fbTraceId,
          metaUserTitle: metaUserTitle,
          metaUserMsg: metaUserMsg,
        );
      }
    }
    return null;
  }
}
