import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/owner/data/datasources/owner_remote_datasource.dart';
import 'package:app/features/owner/data/impersonation_session.dart';
import 'package:app/features/owner/domain/models/owner_account_models.dart';

part 'owner_controller.g.dart';

@riverpod
OwnerRemoteDataSource ownerDataSource(Ref ref) {
  return OwnerRemoteDataSource(ref.watch(dioProvider));
}

@Riverpod(keepAlive: true)
ImpersonationStorage impersonationStorage(Ref ref) {
  return ImpersonationStorage(const FlutterSecureStorage());
}

/// Active impersonation session (or null). Watched by the global
/// banner so a write/clear in any screen surfaces immediately.
@riverpod
class ImpersonationController extends _$ImpersonationController {
  @override
  Future<ImpersonationSession?> build() async {
    return ref.watch(impersonationStorageProvider).read();
  }

  Future<void> start(ImpersonateResponse out, String operatorEmail) async {
    final session = ImpersonationSession(
      token: out.token,
      expiresAt: out.expiresAt,
      actor: operatorEmail,
      accountId: out.accountId,
      accountName: out.accountName,
      impersonatingUser: out.impersonatingUser,
    );
    await ref.read(impersonationStorageProvider).write(session);
    state = AsyncValue.data(session);
  }
}

/// Result of probing the /owner namespace.
///   allowed              — full access; render the panel.
///   requiresEnrollment   — super-admin but missing 2FA; route them to
///                           the settings 2FA card.
///   requiresFreshProof   — super-admin but the last 2FA proof is
///                           older than the freshness window; the
///                           operator needs to log out + back in.
///   forbidden            — not on the SUPER_ADMIN_EMAILS allowlist;
///                           hide the menu entry entirely.
enum OwnerAccessStatus {
  allowed,
  requiresEnrollment,
  requiresFreshProof,
  forbidden,
}

/// Probe the /owner namespace once and resolve the operator's access
/// level. The sidebar shows the entry for any non-forbidden status so
/// the user can complete their setup; the owner screen itself handles
/// the routing to settings / re-login when the status is not allowed.
@Riverpod(keepAlive: true)
Future<OwnerAccessStatus> ownerAccessStatus(Ref ref) async {
  try {
    await ref.watch(ownerDataSourceProvider).list(limit: 1);
    return OwnerAccessStatus.allowed;
  } on DioException catch (e) {
    final status = e.response?.statusCode ?? 0;
    final data = e.response?.data;
    String? code;
    if (data is Map) {
      code = data['code'] as String?;
    }
    if (code == 'OWNER_2FA_REQUIRED') {
      return OwnerAccessStatus.requiresEnrollment;
    }
    if (code == 'OWNER_2FA_STALE') {
      return OwnerAccessStatus.requiresFreshProof;
    }
    if (status == 401 || status == 403) {
      return OwnerAccessStatus.forbidden;
    }
    // Treat network/server errors as forbidden — admins can refresh.
    return OwnerAccessStatus.forbidden;
  } catch (_) {
    return OwnerAccessStatus.forbidden;
  }
}

/// Backwards-compat shim — true whenever the operator has any path
/// to the panel (allowed OR a setup/refresh step away). Existing
/// sidebar code reads this to decide whether to render the entry.
@Riverpod(keepAlive: true)
Future<bool> ownerIsSuperAdmin(Ref ref) async {
  final status = await ref.watch(ownerAccessStatusProvider.future);
  return status != OwnerAccessStatus.forbidden;
}

/// Paginated owner-panel listing. Search/status debouncing happens
/// in the screen — the controller just refetches whenever the args
/// change.
@riverpod
class OwnerAccounts extends _$OwnerAccounts {
  @override
  Future<OwnerAccountsListResponse> build({
    String search = '',
    String status = '',
    int limit = 50,
    int offset = 0,
  }) async {
    return ref.watch(ownerDataSourceProvider).list(
          search: search,
          status: status,
          limit: limit,
          offset: offset,
        );
  }
}

@riverpod
Future<OwnerAccountDetail> ownerAccountDetail(Ref ref, String id) {
  return ref.watch(ownerDataSourceProvider).getOne(id);
}

/// Top spenders this period — drives the "biggest accounts" widget on
/// the owner-panel home.
@riverpod
Future<TopAccountsResponse> ownerTopAccounts(Ref ref, {int limit = 10}) {
  return ref.watch(ownerDataSourceProvider).top(limit: limit);
}

/// Platform-wide aggregate counts for the owner-panel "system" widget.
@riverpod
Future<OwnerSystemHealth> ownerSystemHealth(Ref ref) {
  return ref.watch(ownerDataSourceProvider).systemHealth();
}

/// 30-day daily token + conversation rollup for a target account —
/// drives the mini bar chart on the detail screen.
@riverpod
Future<OwnerUsageTrendResponse> ownerUsageTrend(Ref ref, String accountId) {
  return ref.watch(ownerDataSourceProvider).usageTrend(accountId);
}

/// Last 10 Stripe invoices for a target account — feeds the read-only
/// invoices section on the owner-panel detail screen. Empty list when
/// the account has no Stripe customer (contract / pre-checkout).
@riverpod
Future<OwnerInvoicesResponse> ownerInvoices(Ref ref, String accountId) {
  return ref.watch(ownerDataSourceProvider).invoices(accountId);
}

/// Latest conversations for a target account (read-only triage list
/// surfaced on the owner-panel detail screen). The detail screen can
/// then nudge operators into impersonation when they need to read the
/// full thread.
@riverpod
Future<OwnerConversationsResponse> ownerConversations(Ref ref, String accountId,
    {int limit = 50}) {
  return ref
      .watch(ownerDataSourceProvider)
      .conversations(accountId, limit: limit);
}

/// Imperative actions exposed as a plain controller. UI calls these
/// from button handlers and invalidates the relevant providers on
/// success.
@riverpod
class OwnerActions extends _$OwnerActions {
  @override
  void build() {}

  Future<void> suspend(String id, String reason) async {
    await ref.read(ownerDataSourceProvider).suspend(id, {'reason': reason});
    ref.invalidate(ownerAccountDetailProvider(id));
    ref.invalidate(ownerAccountsProvider);
  }

  Future<void> reactivate(String id) async {
    await ref.read(ownerDataSourceProvider).reactivate(id);
    ref.invalidate(ownerAccountDetailProvider(id));
    ref.invalidate(ownerAccountsProvider);
  }

  Future<void> setOverrides(String id, Map<String, dynamic> overrides) async {
    await ref.read(ownerDataSourceProvider).setOverrides(id, overrides);
    ref.invalidate(ownerAccountDetailProvider(id));
  }

  Future<void> setStatus(String id, String status) async {
    await ref
        .read(ownerDataSourceProvider)
        .setSubscriptionStatus(id, {'status': status});
    ref.invalidate(ownerAccountDetailProvider(id));
    ref.invalidate(ownerAccountsProvider);
  }

  Future<void> grantCredits(String id, int tokens) async {
    await ref
        .read(ownerDataSourceProvider)
        .grantCredits(id, {'tokens': tokens});
    ref.invalidate(ownerAccountDetailProvider(id));
  }

  Future<void> extendTrial(String id, int days) async {
    await ref.read(ownerDataSourceProvider).extendTrial(id, {'days': days});
    ref.invalidate(ownerAccountDetailProvider(id));
  }

  /// Link/reallocate the seller of a customer account. Pass an empty
  /// [sellerEmail] to unlink (leadSource is ignored on unlink).
  Future<void> setSeller(
    String id, {
    required String sellerEmail,
    String leadSource = '',
  }) async {
    await ref.read(ownerDataSourceProvider).setSeller(id, {
      'seller_email': sellerEmail,
      'lead_source': leadSource,
    });
    ref.invalidate(ownerAccountDetailProvider(id));
  }

  Future<ImpersonateResponse> impersonate(String id, {int minutes = 15}) async {
    return ref
        .read(ownerDataSourceProvider)
        .impersonate(id, {'minutes': minutes});
  }

  /// Apply a contract plan to an existing account (plan + overrides +
  /// trial-bypass + active status in one call).
  Future<void> applyContract(
    String id, {
    required String planId,
    bool trialBypass = false,
    String? bypassReason,
    Map<String, int?>? overrides,
    bool? platformKeyAllowed,
  }) async {
    final body = <String, dynamic>{
      'plan_id': planId,
      'trial_bypass': trialBypass,
      if (bypassReason != null && bypassReason.isNotEmpty)
        'bypass_reason': bypassReason,
      if (overrides != null) 'overrides': overrides,
      if (platformKeyAllowed != null)
        'platform_key_allowed': platformKeyAllowed,
    };
    await ref.read(ownerDataSourceProvider).applyContract(id, body);
    ref.invalidate(ownerAccountDetailProvider(id));
    ref.invalidate(ownerAccountsProvider);
  }

  Future<OwnerAccountSummary> createContract({
    required String companyName,
    required String adminEmail,
    required String adminName,
    required String planId,
    bool trialBypass = false,
    String? bypassReason,
    Map<String, int?>? overrides,
  }) async {
    final body = <String, dynamic>{
      'company_name': companyName,
      'admin_email': adminEmail,
      'admin_name': adminName,
      'plan_id': planId,
      'trial_bypass': trialBypass,
      if (bypassReason != null && bypassReason.isNotEmpty)
        'bypass_reason': bypassReason,
      if (overrides != null) 'overrides': overrides,
    };
    final out = await ref.read(ownerDataSourceProvider).createContract(body);
    ref.invalidate(ownerAccountsProvider);
    return out;
  }
}
