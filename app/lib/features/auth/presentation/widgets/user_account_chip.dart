import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/i18n/strings.g.dart';

class UserAccountChip extends ConsumerStatefulWidget {
  const UserAccountChip({super.key});

  @override
  ConsumerState<UserAccountChip> createState() => _UserAccountChipState();
}

class _UserAccountChipState extends ConsumerState<UserAccountChip> {
  bool _menuOpen = false;

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    final user = authState.asData?.value;
    if (user == null) return const SizedBox.shrink();

    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final isCompact = MediaQuery.of(context).size.width < 480;

    return PopupMenuButton<_ChipAction>(
      tooltip: user.email,
      offset: const Offset(0, 48),
      padding: EdgeInsets.zero,
      color: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: colors.borderLight),
      ),
      onOpened: () => setState(() => _menuOpen = true),
      onCanceled: () => setState(() => _menuOpen = false),
      onSelected: (action) async {
        setState(() => _menuOpen = false);
        switch (action) {
          case _ChipAction.logout:
            await ref.read(authControllerProvider.notifier).logout();
            if (context.mounted) context.go('/');
        }
      },
      itemBuilder: (context) => [
        PopupMenuItem<_ChipAction>(
          enabled: false,
          padding: EdgeInsets.zero,
          child: _UserHeader(user: user),
        ),
        const PopupMenuDivider(),
        PopupMenuItem<_ChipAction>(
          value: _ChipAction.logout,
          child: Row(
            children: [
              Icon(Icons.logout, size: 18, color: colors.redAlert),
              const SizedBox(width: 12),
              Text(
                t.settings.logout,
                style: TextStyle(
                  color: colors.redAlert,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
        ),
      ],
      child: Container(
        padding: EdgeInsets.fromLTRB(4, 4, isCompact ? 4 : 10, 4),
        decoration: BoxDecoration(
          color: colors.backgroundSecondary,
          borderRadius: BorderRadius.circular(100),
          border: Border.all(color: colors.borderLight),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            _Avatar(user: user, radius: 14),
            if (!isCompact) ...[
              const SizedBox(width: 8),
              ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 120),
                child: Text(
                  user.name,
                  style: TextStyle(
                    color: colors.textSecondary,
                    fontSize: 13,
                    fontWeight: FontWeight.w500,
                  ),
                  overflow: TextOverflow.ellipsis,
                  maxLines: 1,
                ),
              ),
            ],
            const SizedBox(width: 4),
            AnimatedRotation(
              turns: _menuOpen ? 0.5 : 0.0,
              duration: const Duration(milliseconds: 200),
              child: Icon(
                Icons.expand_more,
                size: 20,
                color: colors.textMuted,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

enum _ChipAction { logout }

class _UserHeader extends StatelessWidget {
  const _UserHeader({required this.user});

  final UserModel user;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 8, 12, 12),
      child: Row(
        children: [
          _Avatar(user: user, radius: 22),
          const SizedBox(width: 12),
          Flexible(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  user.name,
                  style: TextStyle(
                    color: colors.textPrimary,
                    fontWeight: FontWeight.w600,
                    fontSize: 14,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(
                  user.email,
                  style: TextStyle(
                    color: colors.textMuted,
                    fontSize: 12,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
                if (user.isGoogleUser) ...[
                  const SizedBox(height: 4),
                  Text(
                    'Google account',
                    style: TextStyle(
                      color: colors.textMuted,
                      fontSize: 11,
                      fontStyle: FontStyle.italic,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _Avatar extends StatelessWidget {
  const _Avatar({required this.user, required this.radius});

  final UserModel user;
  final double radius;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final initial = user.name.isNotEmpty ? user.name[0].toUpperCase() : '?';

    final initialAvatar = CircleAvatar(
      radius: radius,
      backgroundColor:
          user.isGoogleUser ? colors.backgroundSecondary : colors.bluePrimary,
      child: Text(
        initial,
        style: TextStyle(
          color: Colors.white,
          fontWeight: FontWeight.w600,
          fontSize: radius * 0.9,
        ),
      ),
    );

    if (!user.isGoogleUser) return initialAvatar;

    // Render the Google profile picture through AppNetworkImage so the decoded
    // bitmap is served from Flutter's ImageCache instead of being refetched and
    // redecoded on every rebuild. CircleAvatar.foregroundImage needs an
    // ImageProvider, so we render a circular AppNetworkImage with the
    // initial-letter avatar as the fallback instead.
    return AppNetworkImage(
      url: user.picture,
      width: radius * 2,
      height: radius * 2,
      renderWidth: radius * 2,
      borderRadius: BorderRadius.circular(radius),
      fallback: initialAvatar,
    );
  }
}
