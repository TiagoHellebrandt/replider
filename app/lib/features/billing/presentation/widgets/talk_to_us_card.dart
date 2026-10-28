import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/billing/domain/models/plan_model.dart';
import 'package:app/i18n/strings.g.dart';

part 'talk_to_us_card.g.dart';

/// The sales team's channels: a booking page (when configured), WhatsApp
/// and e-mail. One read per session — the values change on deploy, not in
/// conversation.
@Riverpod(keepAlive: true)
Future<ContactSalesChannels> contactSales(Ref ref) async {
  final dio = ref.read(dioProvider);
  final res = await dio.get<dynamic>('/plans/contact-channels');
  return ContactSalesChannels.fromJson(
      (res.data as Map).cast<String, dynamic>());
}

/// "Talk to us" — what replaced the price list (sales-led, 2026-08-19).
///
/// A hospital-equipment distributor does not put a card into a pricing
/// page; she books a call and hears a number that fits her. So the product
/// shows no plans. Wherever it used to offer an upgrade — the billing
/// screen, the trial banner, the quota wall — it now offers the
/// conversation: book a slot (the calendar page, when configured),
/// WhatsApp, or e-mail. The contract itself is applied by the owner panel.
class TalkToUsCard extends ConsumerWidget {
  const TalkToUsCard({super.key, this.compact = false});

  /// Tighter padding and no icon, for a modal or a banner.
  final bool compact;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final channelsAsync = ref.watch(contactSalesProvider);
    final channels = channelsAsync.asData?.value;
    final bookingUrl = channels?.bookingUrl ?? '';
    // The fetch failing (or still loading) must never leave this card with
    // ZERO buttons — inside the non-dismissible quota wall that meant the
    // product's only path to a bigger plan silently vanished. A retry, and
    // a hardcoded mail fallback that needs no server at all.
    final fetchDead = channelsAsync.hasError;

    return Container(
      key: const Key('talk_to_us_card'),
      padding: EdgeInsets.all(compact ? 16 : 20),
      decoration: BoxDecoration(
        color: colors.bluePrimary.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: colors.bluePrimary.withValues(alpha: 0.25)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              if (!compact) ...[
                Icon(Icons.handshake_outlined,
                    size: 22, color: colors.bluePrimary),
                const SizedBox(width: 10),
              ],
              Expanded(
                child: Text(
                  t.billing.talk_title,
                  style: TextStyle(
                    color: colors.textPrimary,
                    fontWeight: FontWeight.w700,
                    fontSize: compact ? 15 : 17,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 6),
          Text(
            t.billing.talk_body,
            style: TextStyle(
                color: colors.textSecondary, fontSize: 13, height: 1.4),
          ),
          const SizedBox(height: 14),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              // The booking page is the primary action when it exists: it
              // ends in a slot on a calendar, which is what "talk to us"
              // should mean. Without it WhatsApp takes the lead.
              if (bookingUrl.isNotEmpty)
                FilledButton.icon(
                  key: const Key('talk_to_us_book'),
                  onPressed: () => _open(context, Uri.parse(bookingUrl)),
                  icon: const Icon(Icons.event_available_outlined, size: 18),
                  label: Text(t.billing.talk_book_cta),
                  style: FilledButton.styleFrom(
                    backgroundColor: colors.bluePrimary,
                    foregroundColor: Colors.white,
                  ),
                ),
              if (channels != null)
                (bookingUrl.isEmpty ? FilledButton.icon : OutlinedButton.icon)(
                  key: const Key('talk_to_us_whatsapp'),
                  onPressed: () =>
                      _open(context, Uri.parse(channels.whatsappLink)),
                  icon: const Icon(Icons.chat_bubble_outline, size: 18),
                  label: Text(t.billing.contract_whatsapp_cta),
                  style: bookingUrl.isEmpty
                      ? FilledButton.styleFrom(
                          backgroundColor: const Color(0xFF25D366),
                          foregroundColor: Colors.white)
                      : null,
                ),
              if (channels == null && fetchDead) ...[
                OutlinedButton.icon(
                  key: const Key('talk_to_us_retry'),
                  onPressed: () => ref.invalidate(contactSalesProvider),
                  icon: const Icon(Icons.refresh, size: 18),
                  label: Text(t.common.retry),
                ),
                OutlinedButton.icon(
                  key: const Key('talk_to_us_email_fallback'),
                  onPressed: () => _open(
                    context,
                    Uri(
                      scheme: 'mailto',
                      path: 'contato@replider.com',
                      queryParameters: {
                        'subject': t.billing.talk_email_subject
                      },
                    ),
                  ),
                  icon: const Icon(Icons.mail_outline, size: 18),
                  label: Text(t.billing.contract_email_cta),
                ),
              ],
              if (channels != null)
                OutlinedButton.icon(
                  key: const Key('talk_to_us_email'),
                  onPressed: () => _open(
                    context,
                    Uri(
                      scheme: 'mailto',
                      path: channels.email,
                      queryParameters: {
                        'subject': t.billing.talk_email_subject
                      },
                    ),
                  ),
                  icon: const Icon(Icons.mail_outline, size: 18),
                  label: Text(t.billing.contract_email_cta),
                ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _open(BuildContext context, Uri uri) async {
    final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
    if (!ok && context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(t.billing.contract_link_failed)),
      );
    }
  }
}
