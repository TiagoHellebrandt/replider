import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';
import 'package:app/features/settings/presentation/controllers/channels_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// "Conecte a conta que você já usa" — the one-click half of the email setup.
///
/// It sits ABOVE the SMTP fields rather than beside them because these are not
/// two equivalent options: for the person this is written for, the fields below
/// mean finding a hostname, a port and generating an app password, and a
/// @gmail.com address is refused by them outright. This path is the answer for
/// almost everyone; SMTP stays for a company sending from its own domain at
/// volume.
class GoogleMailboxConnectCard extends ConsumerStatefulWidget {
  const GoogleMailboxConnectCard({super.key});

  @override
  ConsumerState<GoogleMailboxConnectCard> createState() =>
      _GoogleMailboxConnectCardState();
}

class _GoogleMailboxConnectCardState
    extends ConsumerState<GoogleMailboxConnectCard> {
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    // The channel list is load-on-demand; in the onboarding wizard nothing
    // else asks for it, so without this the connected state could never
    // render there.
    Future.microtask(() {
      if (mounted) ref.read(channelsListProvider.notifier).load();
    });
  }

  Future<void> _connect() async {
    setState(() => _busy = true);
    try {
      final dio = ref.read(dioProvider);
      final res = await dio.post<Map<String, dynamic>>(
        '/channels/email/google/connect',
      );
      final url = res.data?['auth_url'] as String?;
      if (url == null || url.isEmpty) {
        throw DioException(
          requestOptions:
              RequestOptions(path: '/channels/email/google/connect'),
          message: 'no auth_url',
        );
      }
      if (!mounted) return;
      // externalApplication, not an in-app webview: Google refuses to render
      // its consent screen inside an embedded browser, and the failure is a
      // blank page rather than a message.
      await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          key: const Key('gmail_connect_error'),
          content: Text(parseApiError(e)),
          behavior: SnackBarBehavior.floating,
        ),
      );
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    // The OAuth finishes in ANOTHER TAB and its callback lands on the
    // channels screen — this card (also mounted in the onboarding wizard)
    // never hears the result through any response it asked for. The honest
    // signal is the account's own channel list: an email channel existing
    // IS "connected".
    ChannelModel? mailbox;
    for (final c in ref.watch(channelsListProvider).asData?.value ??
        const <ChannelModel>[]) {
      if (c.type == ChannelType.email) {
        mailbox = c;
        break;
      }
    }

    return Container(
      key: const Key('gmail_connect_card'),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: colors.glassMedium,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (mailbox != null) ...[
            Row(
              children: [
                Icon(Icons.check_circle_outline,
                    size: 18, color: colors.whatsappGreen),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    t.channels.gmail_already_connected(name: mailbox.name),
                    key: const Key('gmail_connected_state'),
                    style: TextStyle(
                        color: colors.textPrimary, fontWeight: FontWeight.w600),
                  ),
                ),
              ],
            ),
          ] else ...[
            Text(
              t.channels.gmail_connect_title,
              style: TextStyle(
                color: colors.textPrimary,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(height: 6),
            Text(
              // States the two things the person is actually deciding: nothing
              // to type, and the replies stay in the inbox they already read.
              t.channels.gmail_connect_help,
              style: TextStyle(color: colors.textMuted, fontSize: 12.5),
            ),
            const SizedBox(height: 12),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                FilledButton.icon(
                  key: const Key('gmail_connect_button'),
                  onPressed: _busy ? null : _connect,
                  icon: _busy
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.link),
                  label: Text(t.channels.gmail_connect_cta),
                ),
                // The consent screen opened in another tab; whoever finished
                // there comes back to a card that has no idea. This re-reads
                // the channel list, which is where the truth lands.
                TextButton(
                  key: const Key('gmail_connect_check'),
                  onPressed: () =>
                      ref.read(channelsListProvider.notifier).load(),
                  child: Text(t.channels.gmail_connect_check),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}

/// Turns the callback's `?gmail=` flag into a message.
///
/// The connection completes in a Google tab and lands back here, so the result
/// arrives as a query parameter rather than as the response to anything this
/// screen asked for. "denied" is deliberately not an error: somebody pressed
/// cancel on the consent screen, and telling them something failed is a lie.
String? gmailConnectMessage(String? flag) {
  switch (flag) {
    case 'connected':
      return t.channels.gmail_connected;
    case 'reconnected':
      return t.channels.gmail_reconnected;
    case 'denied':
      return t.channels.gmail_denied;
    case 'error':
      return t.channels.gmail_failed;
    default:
      return null;
  }
}
