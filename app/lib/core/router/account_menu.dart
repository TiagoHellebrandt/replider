import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/router/app_shell.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Account-level navigation, reachable from every destination.
///
/// These entries are not part of any destination: Settings, Billing and
/// Team belong to the ACCOUNT, not to the inbox. They used to hang off the
/// conversation list's toolbar — a leftover from when the inbox was the
/// whole app — which meant reaching Billing required going to Conversas
/// first, and made the inbox's header carry six things that had nothing to
/// do with conversations.

/// The rail's account button sits bottom-left, so on a wide screen the menu
/// belongs beside it. A bottom sheet is a phone pattern: stretched across a
/// desktop it reads as a slab dropped in the middle of the page, nowhere
/// near the control that opened it.
///
/// The threshold is the SHELL'S, not a number of its own. While there were two
/// (600 there, 900 here), every window between them showed the rail on the left
/// and opened the menu as a sheet at the bottom — the answer appearing on the
/// opposite side of the screen from the button that asked for it.
Future<void> showAccountMenu(BuildContext context) {
  final colors = Theme.of(context).extension<AppColorsExtension>()!;
  final wide = MediaQuery.sizeOf(context).width >= AppShell.railBreakpoint;

  if (!wide) {
    return showModalBottomSheet<void>(
      context: context,
      backgroundColor: colors.backgroundSecondary,
      isScrollControlled: true,
      builder: (sheetContext) => const SafeArea(child: _AccountMenu()),
    );
  }

  // Same menu widget, anchored to the rail instead of the bottom edge.
  return showGeneralDialog<void>(
    context: context,
    // showGeneralDialog defaults barrierDismissible to FALSE, unlike
    // showDialog and unlike the bottom sheet this same menu becomes on a
    // narrow window. So the wide-window menu could only be closed by picking
    // an entry: clicking the dimmed area did nothing, and the dimming is
    // exactly what tells a person there is something to click away.
    barrierDismissible: true,
    // Announced as the action of dismissing the barrier — so it names the
    // dismissal, not the menu. It read "Settings" (a menu ENTRY), which told a
    // screen-reader user that tapping outside would open settings.
    barrierLabel: t.common.close,
    barrierColor: Colors.black.withValues(alpha: 0.3),
    pageBuilder: (dialogContext, _, __) => Align(
      alignment: Alignment.bottomLeft,
      child: Padding(
        padding: const EdgeInsets.only(left: 12, bottom: 12),
        child: Material(
          color: colors.backgroundSecondary,
          borderRadius: BorderRadius.circular(14),
          clipBehavior: Clip.antiAlias,
          child: ConstrainedBox(
            constraints: BoxConstraints(
              maxWidth: 260,
              // Never taller than the window: an account with every entry
              // plus a long name must still show the way out at the bottom.
              maxHeight: MediaQuery.sizeOf(dialogContext).height - 24,
            ),
            child: const _AccountMenu(),
          ),
        ),
      ),
    ),
  );
}

class _AccountMenu extends ConsumerWidget {
  const _AccountMenu();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final user = ref.watch(authControllerProvider).asData?.value;
    // Only an admin can change what the account IS (plan, team, channels);
    // showing those to an attendant teaches them to expect a 403.
    final isAdmin = user?.isAdmin ?? false;

    void go(String location) {
      Navigator.of(context).pop();
      context.go(location);
    }

    return ListView(
      shrinkWrap: true,
      padding: const EdgeInsets.symmetric(vertical: 8),
      children: [
        if (user != null)
          Padding(
            padding: const EdgeInsets.fromLTRB(20, 8, 20, 12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  user.name,
                  key: const Key('account_menu_user_name'),
                  style: typography.body.copyWith(
                      color: Colors.white,
                      fontSize: 15,
                      fontWeight: FontWeight.w600),
                ),
                Text(
                  user.email,
                  style: typography.body
                      .copyWith(color: colors.textSecondary, fontSize: 12),
                ),
              ],
            ),
          ),
        const Divider(height: 1),
        _MenuItem(
          keyValue: 'account_menu_profile',
          icon: Icons.person_outline,
          label: t.account_menu.profile,
          onTap: () => go('/settings/profile'),
        ),
        if (isAdmin) ...[
          _MenuItem(
            keyValue: 'account_menu_products',
            icon: Icons.inventory_2_outlined,
            label: t.sales.products.title,
            onTap: () => go('/sales/products'),
          ),
          // The customer list is the Carteira view of Clientes (2026-09-01):
          // `/contacts` was a fourth list of the same people. The entry
          // stays so the base she imported is one tap from anywhere.
          _MenuItem(
            keyValue: 'account_menu_contacts',
            icon: Icons.contacts_outlined,
            label: t.portfolio.title,
            onTap: () => go('/funnel?view=portfolio'),
          ),
          _MenuItem(
            keyValue: 'account_menu_channels',
            icon: Icons.hub_outlined,
            label: t.account_menu.channels,
            onTap: () => go('/settings/channels'),
          ),
          _MenuItem(
            keyValue: 'account_menu_team',
            icon: Icons.group_outlined,
            label: t.account_menu.team,
            onTap: () => go('/settings/team'),
          ),
        ],
        _MenuItem(
          keyValue: 'account_menu_settings',
          icon: Icons.settings_outlined,
          label: t.account_menu.settings,
          onTap: () => go('/settings'),
        ),
        const Divider(height: 1),
        _MenuItem(
          keyValue: 'account_menu_logout',
          icon: Icons.logout,
          label: t.account_menu.logout,
          danger: true,
          onTap: () async {
            Navigator.of(context).pop();
            await ref.read(authControllerProvider.notifier).logout();
          },
        ),
      ],
    );
  }
}

class _MenuItem extends StatelessWidget {
  const _MenuItem({
    required this.keyValue,
    required this.icon,
    required this.label,
    required this.onTap,
    this.danger = false,
  });

  final String keyValue;
  final IconData icon;
  final String label;
  final VoidCallback onTap;
  final bool danger;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final tint = danger ? colors.redAlert : colors.textSecondary;

    return InkWell(
      key: Key(keyValue),
      onTap: onTap,
      child: Padding(
        // 44px minimum touch target, same rule as everywhere else.
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
        child: Row(
          children: [
            Icon(icon, size: 20, color: tint),
            const SizedBox(width: 14),
            // The anchored desktop menu is narrow, so a long label has to
            // give rather than overflow off the edge of the card.
            Expanded(
              child: Text(
                label,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  color: danger ? colors.redAlert : Colors.white,
                  fontSize: 14,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
