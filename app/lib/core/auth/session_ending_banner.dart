import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Warns that the session is about to end, before it ends.
///
/// The token lasts 24 hours and does not renew, so once a day the person is
/// thrown out. What can be fixed from here is not the eviction — it is the
/// surprise: being dropped halfway through a written approach loses the
/// approach. Com quinze minutos de aviso, a hora de entrar de novo passa a
/// ser dela.
class SessionEndingBanner extends ConsumerWidget {
  const SessionEndingBanner({super.key, required this.remaining});

  final Duration remaining;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // Rounds up: telling somebody with forty seconds left "0 min" is giving the
    // wrong news at the last possible moment.
    final minutes = (remaining.inSeconds / 60).ceil();

    return Material(
      color: colors.orangeAlert.withValues(alpha: 0.12),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        child: Row(
          children: [
            Icon(Icons.schedule, size: 18, color: colors.orangeAlert),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    t.auth.session_ending_title(minutes: '$minutes'),
                    key: const Key('session_ending_title'),
                    style: TextStyle(
                        color: colors.textPrimary,
                        fontSize: 13,
                        fontWeight: FontWeight.w600),
                  ),
                  Text(
                    t.auth.session_ending_body,
                    style: TextStyle(color: colors.textSecondary, fontSize: 12),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 12),
            TextButton(
              key: const Key('session_ending_cta'),
              onPressed: () async {
                // Stores where to return BEFORE leaving, the same way the 401
                // path does — whoever signs in again lands where they were, not
                // on the home screen.
                final from = GoRouterState.of(context).uri.toString();
                final router = GoRouter.of(context);
                await ref.read(authControllerProvider.notifier).logout();
                router.go('/?from=${Uri.encodeComponent(from)}');
              },
              child: Text(t.auth.session_ending_cta),
            ),
          ],
        ),
      ),
    );
  }
}
