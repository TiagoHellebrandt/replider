import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/notifications/push_notification_controller.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

/// Settings row that toggles Web Push notifications for this browser.
/// Hidden entirely on non-web platforms until we ship FCM/APNs.
/// Rendered inline inside the profile card (no own Container/border) —
/// the parent card supplies the visual frame.
class NotificationsSettingsCard extends ConsumerWidget {
  const NotificationsSettingsCard({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Mobile path: no UI at all for now. The in-app sound notifier
    // already covers the open-app case; push lands with FCM/APNs.
    if (!kIsWeb) return const SizedBox.shrink();

    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final asyncState = ref.watch(pushNotificationControllerProvider);

    return asyncState.when(
      data: (s) => _buildRow(context, ref, colors, s, loading: false),
      loading: () {
        final current = asyncState.asData?.value ??
            const PushState(status: PushSubscriptionStatus.notSubscribed);
        return _buildRow(context, ref, colors, current, loading: true);
      },
      error: (err, _) => _errorRow(colors, parseApiError(err)),
    );
  }

  Widget _buildRow(
    BuildContext context,
    WidgetRef ref,
    AppColorsExtension colors,
    PushState state, {
    required bool loading,
  }) {
    final status = state.status;
    final subscribed = status == PushSubscriptionStatus.subscribed;
    final unsupported = status == PushSubscriptionStatus.unsupported;
    final denied = status == PushSubscriptionStatus.denied;
    final iosNeedsInstall = status == PushSubscriptionStatus.iosNeedsInstall;

    String subtitle;
    if (iosNeedsInstall) {
      // iOS Safari outside the installed PWA can't take a push
      // subscription even on iOS 16.4+. Send the user to the install
      // path before they fight a permission prompt that won't fire.
      subtitle = t.notifications.subtitle_ios_install;
    } else if (unsupported) {
      subtitle = t.notifications.subtitle_unsupported;
    } else if (denied) {
      subtitle = t.notifications.subtitle_denied;
    } else if (subscribed) {
      subtitle = t.notifications.subtitle_subscribed;
    } else {
      subtitle = t.notifications.subtitle_default;
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        ListTile(
          leading: Container(
            width: 40,
            height: 40,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: colors.bluePrimary.withValues(alpha: 0.15),
              borderRadius: BorderRadius.circular(10),
            ),
            child: Icon(
              subscribed
                  ? Icons.notifications_active
                  : Icons.notifications_outlined,
              color: colors.bluePrimary,
              size: 22,
            ),
          ),
          title: Text(
            t.notifications.title,
            style: TextStyle(
              color: colors.textPrimary,
              fontWeight: FontWeight.w600,
            ),
          ),
          subtitle: Text(
            subtitle,
            style: TextStyle(color: colors.textMuted, fontSize: 13),
          ),
          trailing: loading
              ? const SizedBox(
                  width: 24,
                  height: 24,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : Switch(
                  value: subscribed,
                  onChanged: (unsupported || denied || iosNeedsInstall)
                      ? null
                      : (val) {
                          final ctrl = ref.read(
                              pushNotificationControllerProvider.notifier);
                          if (val) {
                            ctrl.enable();
                          } else {
                            ctrl.disable();
                          }
                        },
                ),
        ),
        if (subscribed)
          Padding(
            padding: const EdgeInsets.fromLTRB(72, 0, 16, 12),
            child: Align(
              alignment: Alignment.centerLeft,
              child: TextButton.icon(
                onPressed: loading ? null : () => _runTest(context, ref),
                icon: const Icon(Icons.send_outlined, size: 16),
                label: Text(t.notifications.test_btn),
              ),
            ),
          ),
      ],
    );
  }

  Future<void> _runTest(BuildContext context, WidgetRef ref) async {
    final messenger = ScaffoldMessenger.of(context);
    final ctrl = ref.read(pushNotificationControllerProvider.notifier);
    final result = await ctrl.sendTest();

    String text;
    Color? bg;
    if (result.hasError) {
      text = t.notifications.test_request_failed(error: result.error!);
      bg = Colors.red.shade700;
    } else if (!result.hadAnySubs) {
      text = t.notifications.test_no_subs;
      bg = Colors.orange.shade700;
    } else if (result.allFailed) {
      text = t.notifications.test_all_failed(count: result.failed.toString());
      bg = Colors.red.shade700;
    } else if (result.disabled > 0) {
      text =
          t.notifications.test_disabled_some(count: result.disabled.toString());
      bg = Colors.orange.shade700;
    } else {
      text = t.notifications.test_success(
        ok: result.ok.toString(),
        failed: result.failed.toString(),
      );
    }
    messenger.showSnackBar(SnackBar(
      backgroundColor: bg,
      content: Text(text),
    ));
    if (result.disabled > 0) {
      ref.invalidate(pushNotificationControllerProvider);
    }
  }

  Widget _errorRow(AppColorsExtension colors, String message) => Padding(
        padding: const EdgeInsets.all(16),
        child: Text(
          t.notifications.error_loading(error: message),
          style: TextStyle(color: colors.textMuted, fontSize: 13),
        ),
      );
}
