import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

/// What a customer sees when a URL doesn't resolve.
///
/// Without a router `errorBuilder`, GoRouter falls back to dumping its own
/// exception on screen — customers were meeting
/// `GoException: no routes for location: /settings/route-that-does-not-exist`, in
/// English, in a product that is otherwise in Portuguese. That text tells the
/// person nothing they can act on and tells anyone else that the app crashed.
///
/// The path is shown because a wrong URL is usually a typo or a stale
/// bookmark, and seeing it is what lets someone recognise their own mistake.
class NotFoundScreen extends StatelessWidget {
  const NotFoundScreen({super.key, this.location});

  /// The URL that didn't match, as typed.
  final String? location;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.explore_off_outlined,
                  size: 44, color: colors.textMuted),
              const SizedBox(height: 16),
              Text(
                t.errors.route_not_found_title,
                key: const Key('not_found_title'),
                textAlign: TextAlign.center,
                style: TextStyle(
                  color: colors.textPrimary,
                  fontSize: 18,
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 8),
              Text(
                t.errors.route_not_found_body,
                textAlign: TextAlign.center,
                style: TextStyle(color: colors.textSecondary, fontSize: 14),
              ),
              if (location != null && location!.isNotEmpty) ...[
                const SizedBox(height: 12),
                Text(
                  location!,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    color: colors.textMuted,
                    fontSize: 12,
                    fontFamily: 'monospace',
                  ),
                ),
              ],
              const SizedBox(height: 24),
              FilledButton(
                key: const Key('not_found_home'),
                onPressed: () => context.go('/start'),
                child: Text(t.errors.route_not_found_cta),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
