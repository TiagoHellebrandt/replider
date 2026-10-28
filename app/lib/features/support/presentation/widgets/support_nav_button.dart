import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/support/presentation/controllers/support_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Dashboard header entry for the internal support thread (US-005),
/// badged with the unread count. Hidden for platform-account users —
/// the platform team has no thread with itself (the API responds
/// SUPPORT_NOT_AVAILABLE); they see these threads in their normal inbox.
class SupportNavButton extends ConsumerWidget {
  const SupportNavButton({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final user = ref.watch(authControllerProvider).asData?.value;
    if (user == null || user.isPlatformAccount) {
      return const SizedBox.shrink();
    }
    final unread = ref.watch(supportUnreadCounterProvider);
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return IconButton(
      icon: Badge(
        isLabelVisible: unread > 0,
        label: Text(unread > 99 ? '99+' : '$unread'),
        child: Icon(
          Icons.support_agent,
          color: colors.textSecondary,
          size: 20,
        ),
      ),
      onPressed: () => context.go('/support'),
      splashRadius: 20,
      tooltip: t.support.nav_tooltip,
    );
  }
}
