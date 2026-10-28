import 'package:dio/dio.dart';
import 'package:flutter/material.dart';

import 'package:app/features/billing/presentation/widgets/upgrade_plan_modal.dart';
import 'package:app/i18n/strings.g.dart';

/// Inspects an API error and, if it carries the `PLAN_LIMIT_EXCEEDED`
/// code, auto-opens the upgrade modal and returns a localized message
/// precise to the resource that hit the limit (agents/channels/etc.).
/// Returns null when the error is *not* a plan-limit signal, so
/// callers can fall through to their existing generic error handling.
///
/// Usage at a create call-site:
/// ```dart
/// final err = await controller.createAgent(...);
/// if (err != null) {
///   final handled = handlePlanLimitError(context, lastDioError);
///   SaveFeedback.showError(context, handled ?? err);
/// }
/// ```
///
/// The modal open is deferred a frame so this helper is safe to call
/// from within a `try/catch` immediately followed by a `setState`.
/// Every backend code that should funnel the user into the upgrade
/// modal. Keep this list narrow — only *feature/capacity gates* belong
/// here; things like validation errors or network failures stay with
/// the caller's own snackbar handling.
const _planGateCodes = {
  'PLAN_LIMIT_EXCEEDED',
  'BROADCAST_NOT_ALLOWED',
  'RECIPIENTS_OVER_CAMPAIGN_CAP',
  'MONTHLY_BROADCAST_CAP',
};

String? handlePlanLimitError(BuildContext context, Object? error) {
  if (error is! DioException) return null;
  final data = error.response?.data;
  if (data is! Map<String, dynamic>) return null;
  final code = data['code'] as String?;
  if (code == null || !_planGateCodes.contains(code)) return null;

  final meta = data['metadata'];
  final resource =
      meta is Map<String, dynamic> ? meta['resource'] as String? : null;
  final current =
      meta is Map<String, dynamic> ? (meta['current'] as num?)?.toInt() : null;
  final limit =
      meta is Map<String, dynamic> ? (meta['limit'] as num?)?.toInt() : null;

  // Kick the modal after the current frame so callers can finish their
  // state updates (setState, Navigator pops, etc.) before the bottom
  // sheet slides up.
  WidgetsBinding.instance.addPostFrameCallback((_) {
    if (context.mounted) showUpgradePlanModal(context);
  });

  final errors = t.errors;
  final cur = (current ?? 0).toString();
  final lim = (limit ?? 0).toString();

  // Feature-gate codes carry no resource metadata — map them to
  // user-facing copy directly. PLAN_LIMIT_EXCEEDED still uses the
  // current/limit placeholders from the Slang catalog.
  switch (code) {
    case 'BROADCAST_NOT_ALLOWED':
      return errors.BROADCAST_NOT_ALLOWED;
    case 'RECIPIENTS_OVER_CAMPAIGN_CAP':
      return errors.RECIPIENTS_OVER_CAMPAIGN_CAP(current: cur, limit: lim);
    case 'MONTHLY_BROADCAST_CAP':
      return errors.MONTHLY_BROADCAST_CAP(current: cur, limit: lim);
  }

  // PLAN_LIMIT_EXCEEDED: resource-specific copy when we know it.
  switch (resource) {
    case 'agents':
      return errors.PLAN_LIMIT_EXCEEDED_agents(current: cur, limit: lim);
    case 'channels':
      return errors.PLAN_LIMIT_EXCEEDED_channels(current: cur, limit: lim);
    case 'seats':
      return errors.PLAN_LIMIT_EXCEEDED_seats(current: cur, limit: lim);
    default:
      return errors.PLAN_LIMIT_EXCEEDED;
  }
}
