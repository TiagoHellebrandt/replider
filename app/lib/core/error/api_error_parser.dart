import 'dart:async';

import 'package:dio/dio.dart';
import 'package:app/i18n/strings.g.dart';

/// Parses any thrown error into a localized, user-friendly message.
///
/// Resolution order:
///   1. Backend `code` (mapped via [_localizedMessage] to localized copy).
///   2. Backend `message` field — if the code is unknown but the API
///      provided a free-form message we surface it instead of a generic
///      fallback. The backend is authoritative for cases the client
///      hasn't enumerated yet.
///   3. DioException type → connection-timeout / connection-error /
///      server-error / canceled / bad-request, each with its own
///      localized i18n key. This is the part that used to read
///      "Erro de rede: null" — a null `e.message` rendered literally as
///      the word "null", which was both unhelpful and untranslated.
///   4. Bare (non-Dio) exceptions — TimeoutException, SocketException,
///      FormatException — that escape the HTTP layer (e.g. a manual
///      `.timeout()`, a raw socket read, or a JSON decode). DioException
///      also wraps these in `.error`, so the same mapping is applied to
///      the wrapped cause.
///   5. Last-resort UNKNOWN.
String parseApiError(Object? error) {
  if (error is! DioException) return _parseNonDio(error);

  final data = error.response?.data;

  // Backend usually returns {"code": "AUTH_INVALID_CREDENTIALS", "message": "..."}
  // but some legacy paths return {"error": "..."}. Read both so a JSON
  // body that uses either key still surfaces a useful message.
  String? code;
  String? message;
  if (data is Map<String, dynamic>) {
    code = data['code'] as String?;
    final legacy = data['error'];
    message =
        (data['message'] as String?) ?? (legacy is String ? legacy : null);
    // Some responses nest the envelope ({"error": {"code": ...}}) — read the
    // code from there rather than throwing a cast error mid-render.
    if (code == null && legacy is Map) {
      code = legacy['code'] as String?;
      message ??= legacy['message'] as String?;
    }
  }

  if (code != null) {
    return _localizedMessage(code, fallbackMessage: message);
  }

  // Backend responded but without a code — fall back to the free-form
  // message if it's present and non-empty.
  if (message != null && message.isNotEmpty) {
    return message;
  }

  // No response at all (or response without body): translate the Dio
  // exception type into a specific localized message instead of the
  // generic UNKNOWN fallback. This is what fixes "Erro de rede: null".
  switch (error.type) {
    case DioExceptionType.connectionTimeout:
    case DioExceptionType.sendTimeout:
    case DioExceptionType.receiveTimeout:
      return t.errors.CONNECTION_TIMEOUT;
    case DioExceptionType.connectionError:
      return t.errors.CONNECTION_ERROR;
    case DioExceptionType.cancel:
      return t.errors.REQUEST_CANCELED;
    case DioExceptionType.badResponse:
      // Server returned a non-2xx but no parseable error envelope —
      // 5xx is a server problem, 4xx is client-side bad input.
      final status = error.response?.statusCode ?? 0;
      if (status >= 500) return t.errors.SERVER_ERROR;
      if (status >= 400) return t.errors.BAD_REQUEST;
      return t.errors.UNKNOWN;
    case DioExceptionType.badCertificate:
    case DioExceptionType.unknown:
      // Dio wraps the underlying cause (SocketException, TimeoutException,
      // FormatException on a bad JSON body, …) in `.error`. Map that
      // instead of collapsing every wrapped failure into UNKNOWN.
      return _parseNonDio(error.error);
  }
}

/// Maps bare (non-Dio) exceptions to localized copy. Used both for errors
/// thrown outside the HTTP layer and for the cause Dio wraps in
/// [DioException.error].
///
/// `SocketException` is matched by runtime type name rather than `is`
/// because it lives in `dart:io`, which can't be imported in a web build.
String _parseNonDio(Object? error) {
  if (error == null) return t.errors.UNKNOWN;
  if (error is TimeoutException) return t.errors.CONNECTION_TIMEOUT;
  if (error is FormatException) return t.errors.BAD_REQUEST;
  if (error.runtimeType.toString() == 'SocketException') {
    return t.errors.CONNECTION_ERROR;
  }
  return t.errors.UNKNOWN;
}

String _localizedMessage(String code, {String? fallbackMessage}) {
  final errors = t.errors;
  switch (code) {
    case 'AUTH_INVALID_CREDENTIALS':
      return errors.AUTH_INVALID_CREDENTIALS;
    case 'MCP_TOKEN_NOT_FOUND':
      return errors.MCP_TOKEN_NOT_FOUND;
    case 'MCP_TOKEN_LIMIT_REACHED':
      return errors.MCP_TOKEN_LIMIT_REACHED;
    case 'AUTH_EMAIL_ALREADY_EXISTS':
      return errors.AUTH_EMAIL_ALREADY_EXISTS;
    // US-F001: signup closed. The API answers 503, and respondError
    // sanitizes every 5xx message — the code is all the client gets, so
    // the copy has to live here.
    case 'REGISTRATION_PAUSED':
      return errors.REGISTRATION_PAUSED;
    // US-F011: SSRF guard refused a customer-supplied webhook target.
    case 'WEBHOOK_URL_BLOCKED':
      return errors.WEBHOOK_URL_BLOCKED;
    case 'OWNER_EMAIL_EXISTS':
      return errors.OWNER_EMAIL_EXISTS;
    case 'OWNER_SELLER_NOT_FOUND':
      return errors.OWNER_SELLER_NOT_FOUND;
    case 'COMMISSION_SELLER_NOT_FOUND':
      return errors.COMMISSION_SELLER_NOT_FOUND;
    case 'OWNER_SELLER_INACTIVE':
      return errors.OWNER_SELLER_INACTIVE;
    case 'OWNER_INVALID_LEAD_SOURCE':
      return errors.OWNER_INVALID_LEAD_SOURCE;
    // Outbound-sales lead errors. Unmapped, these fall through to the
    // default branch and print the API's own English text under a form
    // field — which is what a salesperson typing "51 98888-7766" was
    // getting: "lead phone must be a valid E.164 number", in English,
    // with no hint of what to type instead.
    case 'LEAD_INVALID_PHONE':
      return errors.LEAD_INVALID_PHONE;
    // Per-plan spend caps (K4). "Not included" is an upgrade conversation, so
    // the copy lives here; "used up" already carries the numbers and the reset
    // date from the server, and a generic sentence would throw both away.
    case 'AI_DRAFTS_NOT_ALLOWED':
      return errors.AI_DRAFTS_NOT_ALLOWED;
    case 'DOC_EXTRACTION_NOT_ALLOWED':
      return errors.DOC_EXTRACTION_NOT_ALLOWED;
    case 'AI_DRAFTS_BUDGET_EXCEEDED':
    case 'DOC_EXTRACTION_BUDGET_EXCEEDED':
      return fallbackMessage?.trim().isNotEmpty ?? false
          ? fallbackMessage!.trim()
          : errors.SPEND_BUDGET_EXCEEDED;
    // The catalogue reader's two answers that have an action behind them. Both
    // carry it in the server's own words — the quota one names the provider's
    // wait ("o provedor pede 57s"), the timeout one names what to do with this
    // specific file — and a generic sentence would throw exactly that away.
    case 'PRODUCT_PDF_QUOTA_EXCEEDED':
    case 'PRODUCT_PDF_TIMEOUT':
      return fallbackMessage?.trim().isNotEmpty ?? false
          ? fallbackMessage!.trim()
          : errors.UNKNOWN;
    // The server names the offending fragment in its message ("...: {{nome}}"),
    // and that is the useful half — a generic sentence would send the person
    // hunting through a long message for a field they cannot see.
    case 'APPROACH_HAS_PLACEHOLDER':
      return fallbackMessage?.trim().isNotEmpty ?? false
          ? fallbackMessage!.trim()
          : errors.APPROACH_HAS_PLACEHOLDER;
    // Names the address and why, because "I could not send it" would send
    // the person retrying a send that will never work.
    case 'EMAIL_ADDRESS_SUPPRESSED':
      return errors.EMAIL_ADDRESS_SUPPRESSED;
    case 'APPROACH_NEEDS_PRODUCTS':
      return errors.APPROACH_NEEDS_PRODUCTS;
    // Each of these names the FIX, not the fault: the person cannot change
    // the request, only the setup behind it (connect a mailbox, add an
    // e-mail to the contact, paste an AI key). Unmapped they printed the
    // server's English text under a Portuguese button.
    case 'LEAD_FUNNEL_NO_CHANNEL':
      return errors.LEAD_FUNNEL_NO_CHANNEL;
    case 'FUNNEL_STAGE_BACKWARD':
      return errors.FUNNEL_STAGE_BACKWARD;
    case 'LEAD_CONTACT_NOT_MESSAGEABLE':
      return errors.LEAD_CONTACT_NOT_MESSAGEABLE;
    case 'LEAD_ALREADY_CONTACTED':
      return errors.LEAD_ALREADY_CONTACTED;
    case 'CONTACT_OPTED_OUT':
      return errors.CONTACT_OPTED_OUT;
    case 'BYOK_NOT_CONFIGURED':
    // Same cure, different door: the factory raises BYOK_KEY_MISSING for a
    // contract account with no key, and it used to reach the screen as a
    // generic "unavailable" that blamed the deployment.
    case 'BYOK_KEY_MISSING':
      return errors.BYOK_NOT_CONFIGURED;
    case 'FORBIDDEN':
      return errors.FORBIDDEN;
    case 'LEAD_NO_CONVERSATION':
      return errors.LEAD_NO_CONVERSATION;
    // 502 — the server sanitizes every 5xx body, so the code is all we get.
    case 'AI_PROVIDER_UNAVAILABLE':
      return errors.AI_PROVIDER_UNAVAILABLE;
    case 'LEAD_NOT_PENDING_APPROVAL':
      return errors.LEAD_NOT_PENDING_APPROVAL;
    case 'LEAD_SEND_FAILED':
      return errors.LEAD_SEND_FAILED;
    case 'LEAD_NOT_FOUND':
      return errors.LEAD_NOT_FOUND;
    case 'OWNER_PLATFORM_NOT_CONFIGURED':
      return errors.OWNER_PLATFORM_NOT_CONFIGURED;
    case 'SELLER_CONV_UNASSIGNED':
      return errors.SELLER_CONV_UNASSIGNED;
    case 'SELLER_NOT_CONV_ASSIGNEE':
      return errors.SELLER_NOT_CONV_ASSIGNEE;
    case 'SELLER_ACCOUNT_ALREADY_LINKED':
      return errors.SELLER_ACCOUNT_ALREADY_LINKED;
    case 'SELLER_CANNOT_CONVERT_PLATFORM':
      return errors.SELLER_CANNOT_CONVERT_PLATFORM;
    case 'SELLER_NOT_FLAGGED':
      return errors.SELLER_NOT_FLAGGED;
    case 'SELLER_CONV_TARGET_UNRELATED':
      return errors.SELLER_CONV_TARGET_UNRELATED;
    case 'PROVIDER_COST_NOT_FOUND':
      return errors.PROVIDER_COST_NOT_FOUND;
    case 'PROVIDER_COST_DUPLICATE':
      return errors.PROVIDER_COST_DUPLICATE;
    case 'PROVIDER_COST_INVALID_RATE':
      return errors.PROVIDER_COST_INVALID_RATE;
    case 'AUTH_UNAUTHORIZED':
      return errors.AUTH_UNAUTHORIZED;
    case 'AUTH_SOCIAL_FAILED':
      return errors.AUTH_SOCIAL_FAILED;
    case 'AUTH_INVALID_RESET_TOKEN':
      return errors.AUTH_INVALID_RESET_TOKEN;
    case 'AUTH_EMAIL_NOT_VERIFIED':
      return errors.AUTH_EMAIL_NOT_VERIFIED;
    case 'AGENT_NOT_FOUND':
      return errors.AGENT_NOT_FOUND;
    case 'AGENT_LIMIT_REACHED':
      return errors.AGENT_LIMIT_REACHED;
    case 'CONV_NOT_FOUND':
      return errors.CONV_NOT_FOUND;
    case 'CONV_OUTSIDE_24H':
      return errors.CONV_OUTSIDE_24H;
    case 'USAGE_INSUFFICIENT_TOKENS':
      return errors.USAGE_INSUFFICIENT_TOKENS;
    case 'USAGE_NO_CREDITS':
      return errors.USAGE_NO_CREDITS;
    case 'USAGE_LIMIT_EXCEEDED':
      return errors.USAGE_LIMIT_EXCEEDED;
    case 'ACCOUNT_SUSPENDED':
      return errors.ACCOUNT_SUSPENDED;
    case 'TEAM_CANNOT_REMOVE_LAST_ADMIN':
      return errors.TEAM_CANNOT_REMOVE_LAST_ADMIN;
    case 'TEAM_INVALID_PARTICIPANT':
      return errors.TEAM_INVALID_PARTICIPANT;
    case 'CANNED_RESPONSE_FORBIDDEN':
      return errors.CANNED_RESPONSE_FORBIDDEN;
    case 'RATE_LIMITED':
      return errors.RATE_LIMITED;
    case 'TRIAL_EXPIRED':
      return errors.TRIAL_EXPIRED;
    case 'SUBSCRIPTION_INACTIVE':
      return errors.SUBSCRIPTION_INACTIVE;
    case 'TERMS_NOT_ACCEPTED':
      return errors.TERMS_NOT_ACCEPTED;
    case 'OWNER_2FA_STALE':
      return errors.OWNER_2FA_STALE;
    case 'OWNER_2FA_REQUIRED':
      return errors.OWNER_2FA_REQUIRED;
    case 'TOTP_INVALID_CODE':
      return errors.TOTP_INVALID_CODE;
    case 'TOTP_NOT_STAGED':
      return errors.TOTP_NOT_STAGED;
    case 'TOTP_NOT_ENABLED':
      return errors.TOTP_NOT_ENABLED;
    case 'TOTP_CIPHER_UNCONFIGURED':
      return errors.TOTP_CIPHER_UNCONFIGURED;
    case 'MESSAGE_SEND_FAILED':
      return errors.MESSAGE_SEND_FAILED;
    case 'PLAN_NOT_FOUND':
      return errors.PLAN_NOT_FOUND;
    case 'PLAN_NOT_PURCHASABLE':
      // Contract plans (Enterprise/Partner) have no Stripe price. The
      // backend's fallback `message` is pt-BR — mapping the code here
      // keeps USD/English users on localized copy (US-018).
      return errors.PLAN_NOT_PURCHASABLE;
    case 'PLAN_CURRENCY_MISMATCH':
      return errors.PLAN_CURRENCY_MISMATCH;
    case 'COUPON_EXHAUSTED':
      return errors.COUPON_EXHAUSTED;
    case 'ADDON_CURRENCY_UNSUPPORTED':
      return errors.ADDON_CURRENCY_UNSUPPORTED;
    // Sales-led signup invites: ONE code for every refusal, on purpose.
    case 'INVITE_INVALID':
      return errors.INVITE_INVALID;
    // Warmup fail-closed (Item 1): NULL never releases a cold send.
    case 'EMAIL_WARMUP_NOT_STARTED':
      return errors.EMAIL_WARMUP_NOT_STARTED;
    // The daily cap prefers the server message (it carries the actual
    // used/cap/ramp-day numbers a static translation would lose) but no
    // longer falls through to UNKNOWN when the message is absent — this is
    // the most predictable refusal the product makes, and "unexpected
    // error" for it sent people retrying a send that cannot work today.
    case 'EMAIL_DAILY_CAP_REACHED':
      return fallbackMessage?.trim().isNotEmpty ?? false
          ? fallbackMessage!.trim()
          : errors.EMAIL_DAILY_CAP_REACHED;
    // Actionable, not just descriptive: says WHERE the template is chosen.
    case 'LEAD_APPROACH_TEMPLATE_REQUIRED':
      return errors.LEAD_APPROACH_TEMPLATE_REQUIRED;
    // Hybrid sending: NOT_READY names the fix (configure volume sending on
    // the channel); UNAVAILABLE is a platform gap, not the customer's.
    case 'BULK_SENDING_NOT_READY':
      return errors.BULK_SENDING_NOT_READY;
    case 'BULK_SENDING_UNAVAILABLE':
      return errors.BULK_SENDING_UNAVAILABLE;
    // Backstop for the hidden Delete on the default funnel — the server
    // message is English.
    case 'SALES_FUNNEL_IS_DEFAULT':
      return errors.SALES_FUNNEL_IS_DEFAULT;
    // Prospecting (hunt tab): NOT_CONFIGURED is a platform problem the
    // customer cannot fix; the others say what to do next. Unmapped, all
    // four leaked raw Go strings into a red toast.
    case 'PROSPECTING_NOT_CONFIGURED':
      return errors.PROSPECTING_NOT_CONFIGURED;
    case 'PROSPECTING_RATE_LIMITED':
      return errors.PROSPECTING_RATE_LIMITED;
    case 'PROSPECTING_INPUT_REQUIRED':
      return errors.PROSPECTING_INPUT_REQUIRED;
    case 'PROSPECTING_SEARCH_FAILED':
      return errors.PROSPECTING_SEARCH_FAILED;
    // PDF brochure import (Bloco C). TOO_LARGE and TOO_MANY_PAGES are
    // deliberately NOT mapped: their server messages carry the actual
    // numbers (file pages, configured limit) that a static translation
    // would lose — they fall through to fallbackMessage below.
    case 'PRODUCT_PDF_UNREADABLE':
      return errors.PRODUCT_PDF_UNREADABLE;
    case 'PRODUCT_PDF_SCAN_UNAVAILABLE':
      return errors.PRODUCT_PDF_SCAN_UNAVAILABLE;
    case 'PRODUCT_PDF_INFECTED':
      return errors.PRODUCT_PDF_INFECTED;
    case 'PRODUCT_PDF_EXTRACT_FAILED':
      return errors.PRODUCT_PDF_EXTRACT_FAILED;
    case 'PRODUCT_PDF_NO_PRODUCTS':
      return errors.PRODUCT_PDF_NO_PRODUCTS;
    case 'PRODUCT_PDF_PROVIDER_CANT_READ':
      return errors.PRODUCT_PDF_PROVIDER_CANT_READ;
    case 'PRODUCT_PDF_IMPORT_UNAVAILABLE':
      return errors.PRODUCT_PDF_IMPORT_UNAVAILABLE;
    default:
      // Unknown code — prefer the backend's free-form `message` if it
      // came along, otherwise fall back to the localized UNKNOWN copy.
      // Keeps surprise codes from new backend features readable while
      // we add formal translations.
      if (fallbackMessage != null && fallbackMessage.isNotEmpty) {
        return fallbackMessage;
      }
      return errors.UNKNOWN;
  }
}

/// Exposes the raw error code so callers can branch (e.g. 402 → redirect
/// to /billing) before falling back to the generic snackbar. Returns
/// null when the payload doesn't carry a code field.
String? parseApiErrorCode(Object? error) {
  if (error is DioException) {
    if (error.response?.data is Map<String, dynamic>) {
      return (error.response!.data as Map<String, dynamic>)['code'] as String?;
    }
  }
  return null;
}
