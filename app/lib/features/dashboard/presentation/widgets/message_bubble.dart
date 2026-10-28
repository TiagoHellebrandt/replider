import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:app/features/dashboard/domain/models/message_model.dart';
import 'package:app/features/dashboard/presentation/widgets/audio_player_bubble.dart';
import 'package:app/features/dashboard/presentation/widgets/image_viewer.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/design_system/components/whatsapp_text.dart';
import 'package:app/features/dashboard/presentation/widgets/attachment_list.dart';
import 'package:app/features/dashboard/presentation/widgets/email_risk_banner.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/dashboard/presentation/widgets/sender_auth_badge.dart';
import 'package:app/features/dashboard/presentation/widgets/view_original_button.dart';
import 'package:app/i18n/strings.g.dart';

class MessageBubble extends StatelessWidget {
  final MessageModel message;
  final String? searchQuery;
  final bool isCurrentMatch;

  /// Which channel this conversation arrived on.
  ///
  /// Needed only for the sender-authenticity badge, which is an EMAIL fact
  /// (SPF/DKIM/DMARC). Optional so the many places that build a bubble without
  /// a conversation in hand keep working — absent means "not email", which is
  /// the safe direction: it hides a badge rather than inventing one.
  final ConversationSource? source;

  const MessageBubble({
    super.key,
    required this.message,
    this.searchQuery,
    this.isCurrentMatch = false,
    this.source,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final isCustomer = message.senderType == SenderType.customer;
    final isBot = message.senderType == SenderType.bot;

    // Content Meta refused to deliver (poll, live location, view-once) is an
    // EVENT, not a customer utterance. The server stamps
    // metadata.unsupported_type and ships a text placeholder; rendering that
    // placeholder as a speech bubble put a warning in the customer's mouth.
    // It stays in the flow (the WHEN matters) but visibly as a notice card.
    if (isCustomer && message.metadata?['unsupported_type'] != null) {
      return _UnsupportedContentCard(message: message, colors: colors);
    }

    // Customer = left, bot/agent = right
    final alignment = isCustomer ? Alignment.centerLeft : Alignment.centerRight;
    final crossAxis =
        isCustomer ? CrossAxisAlignment.start : CrossAxisAlignment.end;

    // Bubble background — derived from theme tokens so future tweaks
    // (custom client themes, dark/light toggle) flow through a single
    // source of truth. The bot tint used to be a one-off 0xFF1A3A5C;
    // bluePrimary at 35% opacity reads identically on the current dark
    // background and stays consistent if the primary color changes.
    final bubbleColor = isCustomer
        ? colors.glassMedium
        : isBot
            ? colors.bluePrimary.withValues(alpha: 0.35)
            : colors.bluePrimary.withValues(alpha: 0.25);

    final borderRadius = BorderRadius.only(
      topLeft: const Radius.circular(16),
      topRight: const Radius.circular(16),
      bottomLeft: isCustomer ? Radius.zero : const Radius.circular(16),
      bottomRight: isCustomer ? const Radius.circular(16) : Radius.zero,
    );

    final hour = message.createdAt.hour.toString().padLeft(2, '0');
    final minute = message.createdAt.minute.toString().padLeft(2, '0');
    final timeStr = '$hour:$minute';

    // Sender label
    String label;
    Color labelColor;
    IconData? labelIcon;
    if (isBot) {
      label = message.senderLabel.isNotEmpty ? message.senderLabel : 'Bot';
      labelColor = colors.cyanGlow;
      labelIcon = Icons.smart_toy;
    } else if (message.senderType == SenderType.agent) {
      label =
          message.senderLabel.isNotEmpty ? message.senderLabel : 'Atendente';
      labelColor = colors.bluePrimary;
      labelIcon = Icons.support_agent;
    } else {
      label = message.senderLabel.isNotEmpty ? message.senderLabel : 'Cliente';
      labelColor = colors.textMuted;
      labelIcon = null;
    }

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 3),
      child: LayoutBuilder(builder: (context, constraints) {
        return Align(
          alignment: alignment,
          child: Column(
            crossAxisAlignment: crossAxis,
            children: [
              // Sender label row
              Padding(
                padding: const EdgeInsets.only(bottom: 2),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (labelIcon != null) ...[
                      Icon(labelIcon, size: 12, color: labelColor),
                      const SizedBox(width: 3),
                    ],
                    Text(
                      label,
                      style: TextStyle(
                        fontSize: 10,
                        fontWeight: FontWeight.w600,
                        color: labelColor,
                      ),
                    ),
                  ],
                ),
              ),
              // Bubble. Wrapped in a GestureDetector so long-press opens
              // the context menu (copy text, copy id). Kept inside the
              // visual container, not the whole row, so tapping an avatar
              // or timestamp doesn't trigger it.
              GestureDetector(
                behavior: HitTestBehavior.translucent,
                onLongPress: () => _showContextMenu(context),
                child: Container(
                  key: const Key('message_bubble_body'),
                  constraints: BoxConstraints(
                    // 65% of the space THIS panel has, not of the whole window.
                    // The dashboard is two columns, so measuring the window made
                    // a bubble as wide as the panel itself: every message filled
                    // the full width and the left/right alignment stopped saying
                    // who spoke. `maxWidth` is finite here (the list constrains
                    // it); the window is the fallback for anywhere it isn't.
                    maxWidth: (constraints.maxWidth.isFinite
                            ? constraints.maxWidth
                            : MediaQuery.of(context).size.width) *
                        0.65,
                  ),
                  padding:
                      const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                  decoration: BoxDecoration(
                    color: isCurrentMatch
                        ? colors.bluePrimary.withValues(alpha: 0.12)
                        : bubbleColor,
                    borderRadius: borderRadius,
                    border: isCurrentMatch
                        ? Border.all(color: colors.bluePrimary, width: 1.5)
                        : Border.all(color: colors.borderLight, width: 0.5),
                  ),
                  child: Column(
                    // The text and the clock follow whoever spoke. Pinned to
                    // `.end` for everyone, the customer's own message sat
                    // shoved against the right edge of a bubble anchored on
                    // the left.
                    crossAxisAlignment: crossAxis,
                    children: [
                      _buildContent(context, colors),
                      // Files that arrived with this message. Rendered inside
                      // the bubble because an attachment IS the message when
                      // someone writes "segue em anexo" — putting it elsewhere
                      // is how it went unnoticed in the first place.
                      // What the impersonation checks found. Above the sender
                      // badge and above the attachments, because when it fires
                      // it is the most important thing on the bubble — a
                      // supplier asking for money at a new account is the last
                      // moment anyone can stop the transfer.
                      if (message.risk != null)
                        Align(
                          alignment: Alignment.centerLeft,
                          child: EmailRiskBanner(
                            risk: message.risk!,
                            messageId: message.id,
                            conversationId: message.conversationId,
                          ),
                        ),
                      // Says when the platform could NOT confirm who sent this.
                      // Silent on the normal case: marking every verified
                      // sender makes the badge wallpaper, and then the one
                      // that matters reads as more of the same.
                      SenderAuthBadge.forMessage(message.metadata,
                              isInbound: isCustomer, source: source) ??
                          const SizedBox.shrink(),
                      // NOT wrapped in an Align: an Align with nothing in it
                      // still expands to the full width it is offered, and
                      // this one is on every bubble — so every message, however
                      // short, was stretched to the 65% cap above and the
                      // left/right alignment stopped saying who spoke, which is
                      // the exact failure that comment was written about.
                      AttachmentList(attachments: message.attachments),
                      // Only for messages whose markup was actually stored: the
                      // body is plain text by design, and this is how the
                      // reader gets the formatting back without weakening that
                      // conversion for everyone.
                      if (ViewOriginalButton.isAvailable(message.metadata))
                        Align(
                          alignment: Alignment.centerLeft,
                          child: ViewOriginalButton(messageId: message.id),
                        ),
                      const SizedBox(height: 4),
                      Text(
                        timeStr,
                        style: TextStyle(fontSize: 10, color: colors.textMuted),
                      ),
                    ],
                  ),
                ),
              ),
              // Send-failure indicator (US-009): the channel adapter never
              // delivered this outbound message — without this marker the
              // bubble reads as sent and the customer is silently left
              // hanging. The reason (provider error) rides on a tooltip.
              if (message.sendFailed)
                Padding(
                  padding: const EdgeInsets.only(top: 2),
                  child: Tooltip(
                    message: message.failedReason ?? '',
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.error_outline,
                            size: 12, color: colors.redAlert),
                        const SizedBox(width: 3),
                        Text(
                          t.conversation.message_send_failed,
                          style: TextStyle(
                            fontSize: 10,
                            fontWeight: FontWeight.w600,
                            color: colors.redAlert,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
            ],
          ),
        );
      }),
    );
  }

  // Long-press context menu. Copy full text is by far the most-
  // requested action in day-to-day atendimento (quoting a customer
  // back to a supervisor via Slack). Copy id helps support traceability
  // when we ship incidents — the wamid or internal id is on
  // message.id; exposing it here means QA doesn't need to open
  // devtools to grab it.
  void _showContextMenu(BuildContext context) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: Theme.of(context)
          .extension<AppColorsExtension>()!
          .backgroundSecondary,
      builder: (ctx) {
        final colors = Theme.of(ctx).extension<AppColorsExtension>()!;
        return SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (message.content.isNotEmpty)
                ListTile(
                  leading: Icon(Icons.copy, color: colors.textSecondary),
                  title: Text(t.message_actions.copy_text),
                  onTap: () async {
                    await Clipboard.setData(
                        ClipboardData(text: message.content));
                    if (ctx.mounted) Navigator.pop(ctx);
                    if (ctx.mounted) {
                      ScaffoldMessenger.of(ctx).showSnackBar(SnackBar(
                        content: Text(t.message_actions.message_copied),
                        behavior: SnackBarBehavior.floating,
                      ));
                    }
                  },
                ),
              ListTile(
                leading: Icon(Icons.tag, color: colors.textSecondary),
                title: Text(t.message_actions.copy_message_id),
                subtitle: Text(
                  message.id,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: colors.textMuted, fontSize: 11),
                ),
                onTap: () async {
                  await Clipboard.setData(ClipboardData(text: message.id));
                  if (ctx.mounted) Navigator.pop(ctx);
                  if (ctx.mounted) {
                    ScaffoldMessenger.of(ctx).showSnackBar(SnackBar(
                      content: Text(t.message_actions.id_copied),
                      behavior: SnackBarBehavior.floating,
                    ));
                  }
                },
              ),
            ],
          ),
        );
      },
    );
  }

  Widget _buildHighlightedText(
      String text, String query, AppColorsExtension colors) {
    final base =
        TextStyle(fontSize: 14, color: colors.textPrimary, height: 1.4);
    final lower = text.toLowerCase();
    final spans = <TextSpan>[];
    int start = 0;
    int idx;
    while ((idx = lower.indexOf(query, start)) != -1) {
      if (idx > start) spans.add(TextSpan(text: text.substring(start, idx)));
      spans.add(TextSpan(
        text: text.substring(idx, idx + query.length),
        style: base.copyWith(
          backgroundColor: Colors.amber.withValues(alpha: 0.45),
          color: colors.textPrimary,
          fontWeight: FontWeight.w700,
        ),
      ));
      start = idx + query.length;
    }
    if (start < text.length) spans.add(TextSpan(text: text.substring(start)));
    return RichText(text: TextSpan(style: base, children: spans));
  }

  List<SharedContact> _sharedContacts() {
    final raw = message.metadata?['contacts'];
    if (raw is! List) return const [];
    return raw.map(SharedContact.fromJson).whereType<SharedContact>().toList();
  }

  Widget _buildContent(BuildContext context, AppColorsExtension colors) {
    // A shared contact card arrives as `type: text` with the card itself in
    // metadata.contacts (the text is the fallback for clients that cannot
    // render it). Rendered as a CARD — name, business, number to copy — not
    // as a line of prose: it read as an ordinary message and nobody saw
    // that there was a number in it.
    final contacts = _sharedContacts();
    if (contacts.isNotEmpty) {
      return _ContactCards(contacts: contacts, colors: colors);
    }
    switch (message.messageType) {
      case MessageType.image:
      case MessageType.sticker:
        return _MediaImage(
          url: message.mediaUrl,
          caption: message.content.isNotEmpty ? message.content : null,
          createdAt: message.createdAt,
          colors: colors,
        );

      case MessageType.video:
        return _MediaFile(
          icon: Icons.videocam_rounded,
          label: message.content.isNotEmpty
              ? message.content
              : t.conversation.media_video,
          url: message.mediaUrl,
          colors: colors,
        );

      case MessageType.audio:
      case MessageType.voice:
        return AudioPlayerBubble(
          messageId: message.id,
          url: message.mediaUrl,
          colors: colors,
          isVoice: message.messageType == MessageType.voice,
          initialTranscript: message.audioTranscript,
        );

      case MessageType.document:
        return _MediaFile(
          icon: Icons.insert_drive_file_rounded,
          label: message.content.isNotEmpty
              ? message.content
              : t.dashboard.media_file_label,
          url: message.mediaUrl,
          colors: colors,
        );

      case MessageType.location:
        return _LocationCard(
          latLon: message.mediaUrl ?? message.content,
          colors: colors,
        );

      case MessageType.text:
        final query = searchQuery?.toLowerCase().trim() ?? '';
        if (query.isNotEmpty) {
          return _buildHighlightedText(message.content, query, colors);
        }
        return WhatsAppText(
          message.content,
          // The bubble's own long press opens the copy menu (text + id), which
          // is more useful here than a free selection AND does not fight the
          // list's scroll gesture. Leaving both on put two long-press
          // recognizers on the same pixels.
          selectable: false,
          baseStyle: TextStyle(
            fontSize: 14,
            color: colors.textPrimary,
            height: 1.4,
          ),
        );
    }
  }
}

// ── Shared contact card ───────────────────────────────────────────────────────

/// One contact out of a WhatsApp "contacts" message, as the API stores it in
/// metadata.contacts: `name.formatted_name`, `phones[].phone`, and the
/// business name/description the adapter lifts out of the vCard.
class SharedContact {
  const SharedContact({
    required this.name,
    this.businessName = '',
    this.businessDescription = '',
    this.phones = const [],
  });

  final String name;
  final String businessName;
  final String businessDescription;
  final List<String> phones;

  static SharedContact? fromJson(dynamic raw) {
    if (raw is! Map) return null;
    final m = raw.cast<String, dynamic>();
    final nameMap = (m['name'] as Map?)?.cast<String, dynamic>();
    var name = (nameMap?['formatted_name'] as String?)?.trim() ?? '';
    if (name.isEmpty) {
      name = [nameMap?['first_name'], nameMap?['last_name']]
          .whereType<String>()
          .map((s) => s.trim())
          .where((s) => s.isNotEmpty)
          .join(' ');
    }
    final phones = ((m['phones'] as List?) ?? const [])
        .whereType<Map>()
        .map((p) => (p['phone'] as String?)?.trim() ?? '')
        .where((p) => p.isNotEmpty)
        .toList();
    if (name.isEmpty && phones.isEmpty) return null;
    return SharedContact(
      name: name,
      businessName: (m['business_name'] as String?)?.trim() ?? '',
      businessDescription: (m['business_description'] as String?)?.trim() ?? '',
      phones: phones,
    );
  }

  /// The business name is shown only when it says something the saved name
  /// does not — the same rule the server applies to the fallback text.
  bool get showsBusinessName =>
      businessName.isNotEmpty &&
      businessName.toLowerCase().replaceAll(RegExp(r'\s+'), ' ') !=
          name.toLowerCase().replaceAll(RegExp(r'\s+'), ' ');

  String get initial => name.isNotEmpty ? name.trim()[0].toUpperCase() : '#';
}

class _ContactCards extends StatelessWidget {
  const _ContactCards({required this.contacts, required this.colors});
  final List<SharedContact> contacts;
  final AppColorsExtension colors;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (var i = 0; i < contacts.length; i++) ...[
          if (i > 0) const SizedBox(height: 8),
          _ContactCard(contact: contacts[i], colors: colors),
        ],
      ],
    );
  }
}

class _ContactCard extends StatelessWidget {
  const _ContactCard({required this.contact, required this.colors});
  final SharedContact contact;
  final AppColorsExtension colors;

  @override
  Widget build(BuildContext context) {
    return Container(
      key: const Key('shared_contact_card'),
      constraints: const BoxConstraints(minWidth: 220, maxWidth: 320),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.glassMedium,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              CircleAvatar(
                radius: 20,
                backgroundColor: colors.bluePrimary.withValues(alpha: 0.18),
                child: Text(
                  contact.initial,
                  style: TextStyle(
                    color: colors.bluePrimary,
                    fontWeight: FontWeight.w700,
                    fontSize: 16,
                  ),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      t.conversation.shared_contact,
                      style: TextStyle(
                        fontSize: 11,
                        color: colors.textMuted,
                        letterSpacing: 0.4,
                      ),
                    ),
                    Text(
                      contact.name.isNotEmpty
                          ? contact.name
                          : contact.phones.first,
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                        color: colors.textPrimary,
                      ),
                    ),
                    if (contact.showsBusinessName)
                      Text(
                        contact.businessName,
                        style: TextStyle(
                            fontSize: 12, color: colors.textSecondary),
                      ),
                  ],
                ),
              ),
            ],
          ),
          if (contact.businessDescription.isNotEmpty) ...[
            const SizedBox(height: 8),
            Text(
              contact.businessDescription,
              style: TextStyle(
                  fontSize: 12, color: colors.textSecondary, height: 1.35),
            ),
          ],
          for (final phone in contact.phones) ...[
            const SizedBox(height: 8),
            _PhoneRow(phone: phone, colors: colors),
          ],
        ],
      ),
    );
  }
}

/// The number, with the one action a shared contact is for: copying it.
class _PhoneRow extends StatelessWidget {
  const _PhoneRow({required this.phone, required this.colors});
  final String phone;
  final AppColorsExtension colors;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      key: Key('shared_contact_phone_$phone'),
      borderRadius: BorderRadius.circular(8),
      onTap: () async {
        await Clipboard.setData(ClipboardData(text: phone));
        if (!context.mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(t.conversation.shared_contact_copied),
            behavior: SnackBarBehavior.floating,
            duration: const Duration(seconds: 2),
          ),
        );
      },
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 4, horizontal: 2),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.phone_outlined, size: 16, color: colors.cyanGlow),
            const SizedBox(width: 8),
            Text(
              phone,
              style: TextStyle(
                  fontSize: 13,
                  color: colors.textPrimary,
                  fontWeight: FontWeight.w500),
            ),
            const SizedBox(width: 8),
            Icon(Icons.copy_rounded, size: 14, color: colors.textMuted),
          ],
        ),
      ),
    );
  }
}

// ── Media widgets ─────────────────────────────────────────────────────────────

class _MediaImage extends StatelessWidget {
  final String? url;
  final String? caption;
  final DateTime? createdAt;
  final AppColorsExtension colors;

  const _MediaImage({
    required this.url,
    this.caption,
    this.createdAt,
    required this.colors,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        MouseRegion(
          cursor: url != null ? SystemMouseCursors.click : MouseCursor.defer,
          child: GestureDetector(
            onTap: url == null
                ? null
                : () => showImageViewer(
                      context,
                      url: url!,
                      filename: _filenameFromUrl(url!),
                      caption: caption,
                      date: createdAt,
                    ),
            child: ClipRRect(
              borderRadius: BorderRadius.circular(8),
              // Constrain the rendered size so tall portrait photos don't
              // take over the whole conversation panel — WhatsApp Web caps
              // media previews similarly and the full-size version is a
              // tap away.
              child: ConstrainedBox(
                constraints: const BoxConstraints(
                  maxWidth: 320,
                  maxHeight: 360,
                ),
                // AppNetworkImage adds Flutter's ImageCache + a memory-bounded
                // cacheWidth so scrolling a conversation doesn't re-fetch or
                // re-decode every attachment. We keep the bespoke skeleton +
                // crossfade via the frame/loading builders so the bubble never
                // collapses to 0 while the request is in flight.
                child: AppNetworkImage(
                  url: url,
                  fit: BoxFit.contain,
                  renderWidth: 320,
                  fallback: _imageFallback(),
                  frameBuilder: (_, child, frame, wasSynchronouslyLoaded) {
                    if (wasSynchronouslyLoaded || frame != null) {
                      return AnimatedSwitcher(
                        duration: const Duration(milliseconds: 180),
                        child: child,
                      );
                    }
                    return const _ImageSkeleton();
                  },
                  loadingBuilder: (_, child, progress) =>
                      progress == null ? child : const _ImageSkeleton(),
                ),
              ),
            ),
          ),
        ),
        if (caption != null && caption!.isNotEmpty) ...[
          const SizedBox(height: 6),
          Text(
            caption!,
            style: TextStyle(fontSize: 13, color: colors.textPrimary),
          ),
        ],
      ],
    );
  }

  Widget _imageFallback() => Container(
        height: 100,
        color: Colors.white12,
        child: const Center(
            child: Icon(Icons.broken_image, color: Colors.white38)),
      );

  /// Extracts a best-effort filename from a signed GCS URL. The object
  /// path's last segment (e.g. `accounts/ACCT/ULID.jpg` -> `ULID.jpg`)
  /// is what we show in the viewer header. Returns null for non-GCS
  /// URLs so the viewer falls back to its default "Imagem" label.
  static String? _filenameFromUrl(String url) {
    final uri = Uri.tryParse(url);
    if (uri == null || uri.pathSegments.isEmpty) return null;
    final last = uri.pathSegments.last;
    return last.isEmpty ? null : last;
  }
}

class _MediaFile extends StatelessWidget {
  final IconData icon;
  final String label;
  final String? url;
  final AppColorsExtension colors;

  const _MediaFile({
    required this.icon,
    required this.label,
    this.url,
    required this.colors,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 28, color: colors.cyanGlow),
        const SizedBox(width: 10),
        Flexible(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                label,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(fontSize: 13, color: colors.textPrimary),
              ),
              if (url != null) ...[
                const SizedBox(height: 2),
                Text(
                  'Toque para abrir',
                  style: TextStyle(fontSize: 10, color: colors.textMuted),
                ),
              ],
            ],
          ),
        ),
      ],
    );
  }
}

class _LocationCard extends StatelessWidget {
  /// Raw value: "geo:lat,lon" or just "lat,lon"
  final String latLon;
  final AppColorsExtension colors;

  const _LocationCard({required this.latLon, required this.colors});

  @override
  Widget build(BuildContext context) {
    final coords = latLon.replaceFirst('geo:', '');
    final parts = coords.split(',');
    final lat = parts.isNotEmpty ? parts[0] : '';
    final lon = parts.length > 1 ? parts[1] : '';

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(Icons.location_on_rounded, size: 28, color: colors.cyanGlow),
        const SizedBox(width: 10),
        Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              t.dashboard.media_location_label,
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: colors.textPrimary,
              ),
            ),
            Text(
              '$lat, $lon',
              style: TextStyle(fontSize: 11, color: colors.textMuted),
            ),
          ],
        ),
      ],
    );
  }
}

/// Shimmering placeholder shown while an inline image is still
/// downloading. Uses a linear gradient swept across the box by a 1.4s
/// repeating animation — matches the usual "skeleton" look most
/// messaging clients use while attachments load.
class _ImageSkeleton extends StatefulWidget {
  const _ImageSkeleton();

  @override
  State<_ImageSkeleton> createState() => _ImageSkeletonState();
}

class _ImageSkeletonState extends State<_ImageSkeleton>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1400),
    )..repeat();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _controller,
      builder: (_, __) {
        final t = _controller.value;
        return Container(
          width: 280,
          height: 200,
          decoration: BoxDecoration(
            color: Colors.white10,
            gradient: LinearGradient(
              begin: Alignment(-1 + 2 * t, -0.2),
              end: Alignment(-0.4 + 2 * t, 0.2),
              colors: const [
                Color(0x11FFFFFF),
                Color(0x33FFFFFF),
                Color(0x11FFFFFF),
              ],
            ),
          ),
          child: const Center(
            child: Icon(
              Icons.image_outlined,
              size: 40,
              color: Colors.white24,
            ),
          ),
        );
      },
    );
  }
}

/// The event card for content Meta refused to deliver (design B,
/// 2026-08-28): left-aligned so its place in the conversation is kept, but
/// visibly a NOTICE — amber outline, a title, no avatar, no sender name.
/// The server's text placeholder is ignored in favour of the localized
/// copy; the metadata stamp is the source of truth.
class _UnsupportedContentCard extends StatelessWidget {
  const _UnsupportedContentCard({required this.message, required this.colors});

  final MessageModel message;
  final AppColorsExtension colors;

  @override
  Widget build(BuildContext context) {
    final hour = message.createdAt.hour.toString().padLeft(2, '0');
    final minute = message.createdAt.minute.toString().padLeft(2, '0');
    return Align(
      alignment: Alignment.centerLeft,
      child: Container(
        key: Key('unsupported_content_${message.id}'),
        margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
        padding: const EdgeInsets.fromLTRB(14, 12, 14, 10),
        constraints: const BoxConstraints(maxWidth: 560),
        decoration: BoxDecoration(
          color: colors.backgroundSecondary,
          border: Border.all(color: colors.orangeAlert.withValues(alpha: 0.5)),
          borderRadius: const BorderRadius.only(
            topLeft: Radius.circular(4),
            topRight: Radius.circular(16),
            bottomLeft: Radius.circular(16),
            bottomRight: Radius.circular(16),
          ),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.only(top: 1),
              child: Icon(Icons.disabled_by_default_outlined,
                  size: 18, color: colors.orangeAlert),
            ),
            const SizedBox(width: 10),
            Flexible(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    t.dashboard.wa_unsupported_title,
                    style: TextStyle(
                      color: colors.textSecondary,
                      fontSize: 12.5,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: 3),
                  Text(
                    t.dashboard.wa_unsupported_body,
                    style: TextStyle(
                      color: colors.textTertiary,
                      fontSize: 12.5,
                      height: 1.4,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    '$hour:$minute',
                    style:
                        TextStyle(color: colors.textDisabled, fontSize: 10.5),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
