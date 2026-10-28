import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/billing/data/datasources/billing_remote_datasource.dart';
import 'package:app/features/billing/domain/models/effective_plan_limits_model.dart';
import 'package:app/features/billing/domain/models/plan_model.dart';
import 'package:app/features/billing/domain/models/usage_model.dart';
import 'package:app/i18n/strings.g.dart';

part 'billing_controller.g.dart';

/// Callback URLs after Stripe checkout completes or is cancelled.
///
/// PATH URLs, not hash: main.dart runs `usePathUrlStrategy()`, so a
/// `/#/billing?...` return had its fragment discarded by the router — the
/// user landed on `/`, the post-checkout refresh never ran and the
/// cancellation survey never fired.
const _successUrl = kIsWeb
    ? 'https://app.replider.com/billing?status=success'
    : 'replider://billing?status=success';

// Return URL for the "cancel subscription" portal flow (US-180). After the
// user cancels in the Stripe portal they land back here and the billing
// screen surfaces the cancellation-reason survey (`cancel_survey=1`).
const _cancelSurveyReturnUrl = kIsWeb
    ? 'https://app.replider.com/billing?cancel_survey=1'
    : 'replider://billing?cancel_survey=1';

// ── Datasource Provider ───────────────────────────────────────────────────

@riverpod
BillingRemoteDataSource billingRemoteDataSource(Ref ref) {
  final dio = ref.watch(dioProvider);
  return BillingRemoteDataSource(dio);
}

// ── Plans Provider ────────────────────────────────────────────────────────

@riverpod
class PlansList extends _$PlansList {
  @override
  Future<List<PlanModel>> build() async {
    final datasource = ref.watch(billingRemoteDataSourceProvider);
    return await datasource.getPlans();
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(billingRemoteDataSourceProvider);
      return await datasource.getPlans();
    });
  }
}

// ── Usage Provider ────────────────────────────────────────────────────────

// keepAlive: usage is read by the dashboard, billing screen, settings hub,
// and the WhatsApp diagnostics card. Without keepAlive every navigation
// between those surfaces re-hits /accounts/usage. Logout invalidates it
// in auth_controller.dart so a second user on the same browser tab does
// not see the previous account's quotas.
@Riverpod(keepAlive: true)
class AccountUsage extends _$AccountUsage {
  @override
  Future<UsageModel> build() async {
    final datasource = ref.watch(billingRemoteDataSourceProvider);
    return await datasource.getUsage();
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(billingRemoteDataSourceProvider);
      return await datasource.getUsage();
    });
  }
}

// ── Effective Plan Limits Provider ────────────────────────────────────

/// Resolves the calling account's effective limits via /me/plan-limits.
/// Distinct from [PlansList] (the public catalogue) — this surfaces the
/// per-account overrides that drive Partner / Enterprise contract caps.
/// keepAlive: read by the billing screen + (potentially) the upgrade
/// modal; logout invalidates from auth_controller.dart alongside the
/// other user-scoped providers.
@Riverpod(keepAlive: true)
class AccountPlanLimits extends _$AccountPlanLimits {
  @override
  Future<EffectivePlanLimitsModel> build() async {
    final datasource = ref.watch(billingRemoteDataSourceProvider);
    return await datasource.getPlanLimits();
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(billingRemoteDataSourceProvider);
      return await datasource.getPlanLimits();
    });
  }
}

// ── Checkout Action ───────────────────────────────────────────────────────

@riverpod
class BillingActions extends _$BillingActions {
  @override
  FutureOr<void> build() {}

  /// Opens the Stripe-hosted Customer Portal. Returns null on success,
  /// an error string otherwise. Caller uses null as "URL launched, we're
  /// done here" — the portal itself lives on stripe.com.
  Future<String?> openBillingPortal() async {
    final datasource = ref.read(billingRemoteDataSourceProvider);
    try {
      final response = await datasource.openBillingPortal({
        'return_url': _successUrl,
      });
      final uri = Uri.parse(response.url);
      if (await canLaunchUrl(uri)) {
        await launchUrl(uri, mode: LaunchMode.externalApplication);
      }
      return null;
    } on DioException catch (e) {
      // BILLING_NO_CUSTOMER is a UX-specific message — user needs to
      // complete a first checkout before the Stripe portal exists.
      // Other errors fall through to the shared parseApiError.
      if (parseApiErrorCode(e) == 'BILLING_NO_CUSTOMER') {
        return t.billing.no_customer_yet;
      }
      return parseApiError(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }

  /// Opens the Stripe portal for cancellation (US-180). Identical to
  /// [openBillingPortal] but points the return URL at the billing screen with
  /// `cancel_survey=1` so the churn-reason survey is presented when the user
  /// returns after cancelling. Returns null on success, an error string
  /// otherwise.
  Future<String?> openBillingPortalForCancellation() async {
    final datasource = ref.read(billingRemoteDataSourceProvider);
    try {
      final response = await datasource.openBillingPortal({
        'return_url': _cancelSurveyReturnUrl,
      });
      final uri = Uri.parse(response.url);
      if (await canLaunchUrl(uri)) {
        await launchUrl(uri, mode: LaunchMode.externalApplication);
      }
      return null;
    } on DioException catch (e) {
      if (parseApiErrorCode(e) == 'BILLING_NO_CUSTOMER') {
        return t.billing.no_customer_yet;
      }
      return parseApiError(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }
}
