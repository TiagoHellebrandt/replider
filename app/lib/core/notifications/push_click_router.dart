/// Bridges service-worker notification clicks into in-app navigation
/// (canal-interno US-006). The SW posts `replider_push_click` with the
/// payload's click_url (e.g. `/support`); push-client.js exposes it via
/// `onClick`, [PushNotificationService.onNotificationClick] wraps it, and
/// this provider is the missing last hop: it routes the target through
/// GoRouter so a click on "your account manager replied" lands directly
/// on the support widget instead of just focusing the tab.
///
/// Hand-written Riverpod (no codegen), same rationale as
/// push_notification_controller.dart. On non-web the stub service returns
/// a no-op disposer, so watching this provider costs nothing.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/notifications/push_notification_service.dart';
import 'package:app/core/router/app_router.dart';

/// Injectable service handle so tests can substitute a fake that fires
/// synthetic clicks. Production resolves the platform implementation.
final pushNotificationServiceProvider = Provider<PushNotificationService>(
  (_) => PushNotificationService(),
);

/// Side-effect provider: watch it once from the app shell to keep the
/// click→router bridge alive for the whole session.
final pushClickRouterProvider = Provider<void>((ref) {
  final service = ref.watch(pushNotificationServiceProvider);
  final dispose = service.onNotificationClick((target) {
    final path =
        target.isEmpty ? '/' : (target.startsWith('/') ? target : '/$target');
    // read (not watch): the router provider is keepAlive and the handler
    // runs outside build.
    ref.read(appRouterProvider).go(path);
  });
  ref.onDispose(dispose);
});
