/// Banner shown on the dashboard when a mobile user is browsing in a
/// regular tab instead of the installed PWA. Two flavors:
///
///   * Android Chrome: a single "Instalar app" button that triggers the
///     captured beforeinstallprompt event.
///   * iOS Safari: a step-by-step "compartilhar → adicionar à tela de
///     início" instructions card. iOS deliberately offers no JS API for
///     this, so we lean on illustrative copy.
///
/// Hidden when:
///   * Running in the installed PWA (display-mode standalone).
///   * On desktop / unsupported platforms.
///   * After the user dismisses it (per-session via
///     [pwaInstallDismissedProvider]).
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';
import 'pwa_install_controller.dart';
import 'pwa_install_service.dart';

/// Per-session dismissal flag. Reset on app reload — we don't persist
/// it to secure storage because the user is more likely to install
/// after a few sessions of considering it. Annoying-but-effective is
/// fine for this surface.
class PwaInstallDismissed extends Notifier<bool> {
  @override
  bool build() => false;
  void dismiss() => state = true;
}

final pwaInstallDismissedProvider =
    NotifierProvider<PwaInstallDismissed, bool>(PwaInstallDismissed.new);

class PwaInstallBanner extends ConsumerWidget {
  const PwaInstallBanner({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(pwaInstallControllerProvider);
    final dismissed = ref.watch(pwaInstallDismissedProvider);
    if (dismissed) return const SizedBox.shrink();
    if (state == PwaInstallState.notInstallable ||
        state == PwaInstallState.installed) {
      return const SizedBox.shrink();
    }
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 12, 12, 0),
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: colors.bluePrimary.withValues(alpha: 0.10),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: colors.bluePrimary.withValues(alpha: 0.4)),
        ),
        child: state == PwaInstallState.androidPromptable
            ? _AndroidPromptBody(colors: colors, ref: ref)
            : _IosInstructionsBody(colors: colors, ref: ref),
      ),
    );
  }
}

class _AndroidPromptBody extends StatelessWidget {
  const _AndroidPromptBody({required this.colors, required this.ref});

  final AppColorsExtension colors;
  final WidgetRef ref;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(
            color: colors.bluePrimary.withValues(alpha: 0.18),
            borderRadius: BorderRadius.circular(10),
          ),
          child:
              Icon(Icons.install_mobile, color: colors.bluePrimary, size: 22),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(t.pwa.install_title,
                  style: TextStyle(
                      color: colors.textPrimary,
                      fontSize: 14,
                      fontWeight: FontWeight.w700)),
              const SizedBox(height: 2),
              Text(
                t.pwa.install_body,
                style: TextStyle(color: colors.textSecondary, fontSize: 12),
              ),
            ],
          ),
        ),
        const SizedBox(width: 8),
        Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            FilledButton(
              onPressed: () async {
                final accepted = await ref
                    .read(pwaInstallControllerProvider.notifier)
                    .promptInstall();
                if (accepted) {
                  ref.read(pwaInstallDismissedProvider.notifier).dismiss();
                }
              },
              style: FilledButton.styleFrom(
                backgroundColor: colors.bluePrimary,
                padding:
                    const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
              ),
              child: Text(t.pwa.install_btn),
            ),
            TextButton(
              onPressed: () =>
                  ref.read(pwaInstallDismissedProvider.notifier).dismiss(),
              style: TextButton.styleFrom(minimumSize: const Size(0, 28)),
              child: Text(t.pwa.not_now,
                  style: TextStyle(color: colors.textMuted, fontSize: 11)),
            ),
          ],
        ),
      ],
    );
  }
}

class _IosInstructionsBody extends StatelessWidget {
  const _IosInstructionsBody({required this.colors, required this.ref});

  final AppColorsExtension colors;
  final WidgetRef ref;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: colors.bluePrimary.withValues(alpha: 0.18),
                borderRadius: BorderRadius.circular(10),
              ),
              child: Icon(Icons.ios_share, color: colors.bluePrimary, size: 22),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(t.pwa.ios_title,
                      style: TextStyle(
                          color: colors.textPrimary,
                          fontSize: 14,
                          fontWeight: FontWeight.w700)),
                  const SizedBox(height: 2),
                  Text(
                    t.pwa.ios_body,
                    style: TextStyle(color: colors.textSecondary, fontSize: 12),
                  ),
                ],
              ),
            ),
            IconButton(
              tooltip: t.pwa.dismiss_tooltip,
              icon: Icon(Icons.close, color: colors.textMuted, size: 18),
              onPressed: () =>
                  ref.read(pwaInstallDismissedProvider.notifier).dismiss(),
            ),
          ],
        ),
        const SizedBox(height: 12),
        _IosStep(
          number: '1',
          icon: Icons.ios_share,
          text: t.pwa.ios_step_1,
          colors: colors,
        ),
        const SizedBox(height: 6),
        _IosStep(
          number: '2',
          icon: Icons.add_box_outlined,
          text: t.pwa.ios_step_2,
          colors: colors,
        ),
        const SizedBox(height: 6),
        _IosStep(
          number: '3',
          icon: Icons.notifications_active_outlined,
          text: t.pwa.ios_step_3,
          colors: colors,
        ),
      ],
    );
  }
}

class _IosStep extends StatelessWidget {
  const _IosStep({
    required this.number,
    required this.icon,
    required this.text,
    required this.colors,
  });

  final String number;
  final IconData icon;
  final String text;
  final AppColorsExtension colors;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Container(
          width: 22,
          height: 22,
          alignment: Alignment.center,
          decoration: BoxDecoration(
            color: colors.bluePrimary.withValues(alpha: 0.18),
            borderRadius: BorderRadius.circular(11),
          ),
          child: Text(number,
              style: TextStyle(
                  color: colors.bluePrimary,
                  fontSize: 11,
                  fontWeight: FontWeight.w800)),
        ),
        const SizedBox(width: 8),
        Icon(icon, color: colors.textMuted, size: 16),
        const SizedBox(width: 6),
        Expanded(
          child: Text(
            text,
            style: TextStyle(color: colors.textSecondary, fontSize: 12),
          ),
        ),
      ],
    );
  }
}
