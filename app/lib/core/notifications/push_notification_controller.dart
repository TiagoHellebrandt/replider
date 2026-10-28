/// Controller that owns the user-facing push-notification toggle state:
/// browser permission + whether the current device is subscribed
/// server-side. Callers flip it via [enable]/[disable]; subscription CRUD
/// against the Replider API happens here so the UI layer stays state-only.
///
/// Hand-written Riverpod (no @riverpod codegen) so adding push doesn't
/// require a `build_runner build` pass to compile.
library;

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/core/notifications/push_notification_service.dart';
import 'package:app/core/pwa/pwa_install_service.dart';

/// High-level lifecycle of the local push subscription. Maps loosely to
/// the browser's Notification.permission × server-subscribed matrix so
/// the UI can render a single tri-state toggle.
enum PushSubscriptionStatus {
  /// Platform cannot do web push (mobile stub today).
  unsupported,

  /// iOS Safari running outside the installed PWA. iOS 16.4+ supports
  /// Web Push, but ONLY inside an app added to the home screen. The
  /// settings UI flips into "instale o app primeiro" copy in this
  /// state instead of letting the user hit a permission prompt that
  /// will never fire.
  iosNeedsInstall,

  /// Browser has denied permission; user must flip it in site settings.
  denied,

  /// Not subscribed yet. User can opt in.
  notSubscribed,

  /// Subscribed locally AND recorded on the server.
  subscribed,
}

class PushState {
  const PushState({
    required this.status,
    this.endpoint,
    this.error,
  });

  final PushSubscriptionStatus status;
  final String? endpoint;
  final String? error;
}

class PushNotificationController extends AsyncNotifier<PushState> {
  PushNotificationService? _svc;
  PwaInstallService? _pwaSvc;
  String? _vapidKey;
  void Function()? _changeDisposer;

  PushNotificationService get _service => _svc ??= PushNotificationService();
  PwaInstallService get _pwa => _pwaSvc ??= PwaInstallService();

  /// iOS Safari outside the installed PWA can't take a push subscription
  /// even when running on iOS 16.4+. We surface that as a distinct
  /// status so the toggle copy can read "instale o app primeiro"
  /// instead of "push not supported in this browser".
  bool get _iosNeedsInstall => _pwa.os == PwaOs.ios && !_pwa.isStandalone;

  @override
  Future<PushState> build() async {
    if (!kIsWeb) {
      return const PushState(status: PushSubscriptionStatus.unsupported);
    }
    if (_iosNeedsInstall) {
      return const PushState(status: PushSubscriptionStatus.iosNeedsInstall);
    }
    if (!_service.isSupported) {
      return const PushState(status: PushSubscriptionStatus.unsupported);
    }
    // Wire the SW rotation listener exactly once per provider lifetime.
    // keepAlive: true is set on the provider so this disposer survives
    // navigation; ref.onDispose tears it down on logout invalidation.
    _changeDisposer ??=
        _service.onSubscriptionChange(_handleSubscriptionChange);
    ref.onDispose(() {
      _changeDisposer?.call();
      _changeDisposer = null;
    });
    final permission = _service.currentPermission;
    if (permission == PushPermission.denied) {
      return const PushState(status: PushSubscriptionStatus.denied);
    }
    final existing = await _service.currentSubscription();
    if (existing == null) {
      return const PushState(status: PushSubscriptionStatus.notSubscribed);
    }
    return PushState(
      status: PushSubscriptionStatus.subscribed,
      endpoint: existing.endpoint,
    );
  }

  /// Handles a SW-fired subscription rotation. When the SW already
  /// re-subscribed for us, POST the fresh row to the API and flip state
  /// back to subscribed. When it couldn't, mark the toggle as
  /// notSubscribed with the rotation error code so the Settings card
  /// surfaces a "re-enable" hint instead of leaving the toggle stuck on
  /// the now-dead endpoint.
  Future<void> _handleSubscriptionChange(SubscriptionChange ev) async {
    if (ev.resubscribed && ev.subscription != null) {
      try {
        await _postSubscription(ev.subscription!);
        state = AsyncValue.data(PushState(
          status: PushSubscriptionStatus.subscribed,
          endpoint: ev.subscription!.endpoint,
        ));
      } catch (_) {
        // POST failed (offline, server hiccup) — fall through to the
        // "needs re-enable" path so the user notices.
        state = const AsyncValue.data(PushState(
          status: PushSubscriptionStatus.notSubscribed,
          error: 'PUSH_RESUBSCRIBE_POST_FAILED',
        ));
      }
      return;
    }
    state = const AsyncValue.data(PushState(
      status: PushSubscriptionStatus.notSubscribed,
      error: 'PUSH_NEEDS_RESUBSCRIBE',
    ));
  }

  /// Requests permission, subscribes to push, and uploads the endpoint to
  /// the API. Idempotent — re-running it on an already-subscribed device
  /// just refreshes the server row.
  Future<void> enable() async {
    if (!kIsWeb) return;
    if (_iosNeedsInstall) {
      state = const AsyncValue.data(
          PushState(status: PushSubscriptionStatus.iosNeedsInstall));
      return;
    }
    if (!_service.isSupported) return;
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final permission = await _service.requestPermission();
      if (permission == PushPermission.denied) {
        return const PushState(status: PushSubscriptionStatus.denied);
      }
      final vapid = await _fetchVapidKey();
      if (vapid.isEmpty) {
        return const PushState(
          status: PushSubscriptionStatus.notSubscribed,
          error: 'PUSH_VAPID_UNCONFIGURED',
        );
      }
      final sub = await _service.subscribe(vapid);
      if (sub == null) {
        return const PushState(
          status: PushSubscriptionStatus.notSubscribed,
          error: 'PUSH_SUBSCRIBE_FAILED',
        );
      }
      await _postSubscription(sub);
      return PushState(
        status: PushSubscriptionStatus.subscribed,
        endpoint: sub.endpoint,
      );
    });
  }

  /// Removes the local subscription and tells the server to forget it.
  /// Best-effort — a failed DELETE still clears local state so a retry
  /// from a clean browser works.
  Future<void> disable() async {
    if (!kIsWeb || !_service.isSupported) return;
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final endpoint = await _service.unsubscribe();
      if (endpoint != null) {
        await _deleteSubscription(endpoint);
      }
      return const PushState(status: PushSubscriptionStatus.notSubscribed);
    });
  }

  /// Result of [sendTest]. Mirrors the API's per-subscription summary
  /// so the settings card can render an actionable snackbar (delivered
  /// vs. failed counts, plus how many we just disabled because the
  /// push service rejected the auth — typically a VAPID rotation).
  Future<PushTestSummary> sendTest() async {
    final dio = ref.read(dioProvider);
    try {
      final resp = await dio.post<dynamic>(
        '/push/test',
        options: Options(
          validateStatus: (code) => code != null && code < 500,
        ),
      );
      if (resp.statusCode != null && resp.statusCode! >= 400) {
        return PushTestSummary._error('HTTP ${resp.statusCode}');
      }
      final raw = resp.data;
      if (raw is! List) {
        return const PushTestSummary._(ok: 0, failed: 0, disabled: 0);
      }
      var ok = 0, failed = 0, disabled = 0;
      for (final entry in raw) {
        if (entry is! Map) continue;
        if (entry['ok'] == true) {
          ok += 1;
        } else {
          failed += 1;
          if (entry['disabled'] == true) disabled += 1;
        }
      }
      return PushTestSummary._(ok: ok, failed: failed, disabled: disabled);
    } on DioException catch (e) {
      return PushTestSummary._error(e.message ?? e.toString());
    }
  }

  Future<String> _fetchVapidKey() async {
    if (_vapidKey != null) return _vapidKey!;
    final dio = ref.read(dioProvider);
    final resp = await dio.get<dynamic>('/push/vapid-public-key');
    final data = resp.data;
    final key = (data is Map && data['public_key'] is String)
        ? data['public_key'] as String
        : '';
    _vapidKey = key;
    return key;
  }

  Future<void> _postSubscription(PushSubscriptionPayload sub) async {
    final dio = ref.read(dioProvider);
    await dio.post<dynamic>(
      '/push/subscriptions',
      data: sub.toJson(),
      options: Options(
        validateStatus: (code) => code != null && code < 500,
      ),
    );
  }

  Future<void> _deleteSubscription(String endpoint) async {
    final dio = ref.read(dioProvider);
    await dio.delete<dynamic>(
      '/push/subscriptions',
      data: {'endpoint': endpoint},
      options: Options(
        validateStatus: (code) => code != null && code < 500,
      ),
    );
  }
}

/// Per-call summary of the diagnostic /push/test endpoint.
/// [error] is non-null only when the request itself failed (network /
/// 4xx /5xx) — per-subscription failures show up in [failed] / [disabled].
class PushTestSummary {
  const PushTestSummary._({
    required this.ok,
    required this.failed,
    required this.disabled,
  }) : error = null;
  const PushTestSummary._error(this.error)
      : ok = 0,
        failed = 0,
        disabled = 0;

  final int ok;
  final int failed;
  final int disabled;
  final String? error;

  bool get hasError => error != null;
  bool get hadAnySubs => ok + failed > 0;
  bool get allFailed => ok == 0 && failed > 0;
}

/// Keep-alive so the state survives navigating between settings and
/// dashboard — users hit the toggle once and move on.
final pushNotificationControllerProvider =
    AsyncNotifierProvider<PushNotificationController, PushState>(
  PushNotificationController.new,
);
