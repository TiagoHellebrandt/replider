import 'package:desktop_drop/desktop_drop.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/dashboard/presentation/widgets/media_send_helper.dart';
import 'package:app/features/dashboard/presentation/widgets/source_badge.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

class ConversationTile extends ConsumerStatefulWidget {
  final ConversationModel conversation;

  const ConversationTile({super.key, required this.conversation});

  @override
  ConsumerState<ConversationTile> createState() => _ConversationTileState();
}

class _ConversationTileState extends ConsumerState<ConversationTile> {
  bool _isDraggingOver = false;

  ConversationModel get conversation => widget.conversation;

  String _formatTime(DateTime dt) {
    final now = DateTime.now();
    final diff = now.difference(dt);

    if (diff.inMinutes < 1) return t.dashboard.now;
    if (diff.inHours < 1) return '${diff.inMinutes}min';

    final isToday =
        dt.day == now.day && dt.month == now.month && dt.year == now.year;
    final yesterday = now.subtract(const Duration(days: 1));
    final isYesterday = dt.day == yesterday.day &&
        dt.month == yesterday.month &&
        dt.year == yesterday.year;

    if (isToday) {
      return '${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
    }
    if (isYesterday) return t.dashboard.day_yesterday;
    return '${dt.day.toString().padLeft(2, '0')}/${dt.month.toString().padLeft(2, '0')}';
  }

  Future<void> _handleDrop(DropDoneDetails detail) async {
    if (detail.files.isEmpty) return;
    // Route the drop to this tile's conversation: select it first so the
    // detail panel opens to the same thread the user targeted, then
    // upload the first file through the shared helper.
    ref.read(selectedConversationProvider.notifier).select(conversation.id);
    final file = detail.files.first;
    final bytes = await file.readAsBytes();
    if (!mounted) return;
    await sendMediaToConversation(
      ref: ref,
      context: context,
      conversationId: conversation.id,
      bytes: bytes,
      filename: file.name,
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // `.select(==)` scopes the rebuild to THIS tile's selection state.
    // Without it every tile rebuilt whenever the user tapped any other
    // tile (since the provider value changed), which showed up as
    // jank on lists >200 rows.
    final isSelected = ref.watch(
      selectedConversationProvider.select((id) => id == conversation.id),
    );
    final selection = ref.watch(conversationSelectionProvider);
    final multiselectActive = selection.isNotEmpty;
    final isInBulk = selection.contains(conversation.id);

    return DropTarget(
      onDragEntered: (_) {
        if (!_isDraggingOver) setState(() => _isDraggingOver = true);
      },
      onDragExited: (_) {
        if (_isDraggingOver) setState(() => _isDraggingOver = false);
      },
      onDragDone: (detail) async {
        setState(() => _isDraggingOver = false);
        await _handleDrop(detail);
      },
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          // Long-press enters multiselect mode + selects the row.
          // Tap toggles when multiselect is active, otherwise opens the
          // conversation. Mirrors the WhatsApp/iMessage gesture so the
          // affordance is discoverable without a tooltip.
          onLongPress: () {
            ref
                .read(conversationSelectionProvider.notifier)
                .toggle(conversation.id);
          },
          onTap: () {
            final selection = ref.read(conversationSelectionProvider);
            if (selection.isNotEmpty) {
              ref
                  .read(conversationSelectionProvider.notifier)
                  .toggle(conversation.id);
              return;
            }
            ref
                .read(selectedConversationProvider.notifier)
                .select(conversation.id);
            ref.read(messageSearchProvider.notifier).clear();
            // Opening IS reading. This call had zero call sites, so the
            // WhatsApp blue ticks never fired and unread state never
            // cleared server-side.
            ref
                .read(conversationActionsProvider.notifier)
                .markAsRead(conversation.id);
          },
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 200),
            decoration: BoxDecoration(
              color: _isDraggingOver
                  ? colors.bluePrimary.withValues(alpha: 0.18)
                  : (isInBulk
                      ? colors.bluePrimary.withValues(alpha: 0.12)
                      : (isSelected ? colors.glassMedium : Colors.transparent)),
              border: Border(
                left: BorderSide(
                  color: _isDraggingOver
                      ? colors.bluePrimary
                      : (isInBulk || isSelected
                          ? colors.bluePrimary
                          : Colors.transparent),
                  width: 3,
                ),
                bottom: BorderSide(
                  color: colors.borderLight,
                  width: 0.5,
                ),
              ),
            ),
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
            child: Row(
              children: [
                // Multiselect checkbox — visible only when the user is in
                // bulk-edit mode. Saves real estate the rest of the time.
                if (multiselectActive) ...[
                  Padding(
                    padding: const EdgeInsets.only(right: 10),
                    child: Icon(
                      isInBulk
                          ? Icons.check_circle
                          : Icons.radio_button_unchecked,
                      color: isInBulk ? colors.bluePrimary : colors.textMuted,
                      size: 22,
                    ),
                  ),
                ],
                // Avatar
                Stack(
                  children: [
                    _TileAvatar(
                      name: conversation.contactName,
                      avatarUrl: conversation.contactAvatarUrl,
                      color: _avatarColor(conversation.contactName),
                    ),
                    // Source badge on avatar
                    Positioned(
                      bottom: -2,
                      right: -2,
                      child: SourceBadge(source: conversation.source, size: 12),
                    ),
                  ],
                ),
                const SizedBox(width: 12),
                // Content
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // Name + time
                      Row(
                        children: [
                          if (_showPriority(conversation.priority)) ...[
                            _PriorityBadge(priority: conversation.priority),
                            const SizedBox(width: 6),
                          ],
                          if (_sentimentBadge(conversation.sentiment)
                              case final badge?) ...[
                            badge,
                            const SizedBox(width: 6),
                          ],
                          Expanded(
                            child: Text(
                              conversation.contactName,
                              style: TextStyle(
                                fontSize: 15,
                                fontWeight: conversation.unreadCount > 0
                                    ? FontWeight.w700
                                    : FontWeight.w500,
                                color: colors.textPrimary,
                              ),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                          const SizedBox(width: 8),
                          Text(
                            _formatTime(conversation.lastMessageAt),
                            style: TextStyle(
                              fontSize: 11,
                              color: conversation.unreadCount > 0
                                  ? colors.bluePrimary
                                  : colors.textMuted,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 4),
                      // Last message + unread badge
                      Row(
                        children: [
                          Expanded(
                            child: _LastMessagePreview(
                              text: conversation.lastMessage,
                              type: conversation.lastMessageType,
                              unread: conversation.unreadCount > 0,
                              colors: colors,
                            ),
                          ),
                          if (conversation.unreadCount > 0) ...[
                            const SizedBox(width: 8),
                            Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 7,
                                vertical: 2,
                              ),
                              decoration: BoxDecoration(
                                color: colors.bluePrimary,
                                borderRadius: BorderRadius.circular(10),
                              ),
                              child: Text(
                                conversation.unreadCount.toString(),
                                style: const TextStyle(
                                  color: Colors.white,
                                  fontSize: 11,
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                            ),
                          ],
                        ],
                      ),
                      // Tag chips + assignee. Lumping them in the same
                      // row because both are "who/what is this
                      // conversation about"-signals the attendant scans
                      // before deciding to open. Assignee was previously
                      // only visible after opening the thread, which
                      // defeated a supervisor's ability to triage the
                      // list at a glance.
                      if (conversation.tags.isNotEmpty ||
                          conversation.leadStageLabel.isNotEmpty ||
                          _categoryChip(conversation.inboxCategory) != null ||
                          conversation.assignedToName != null) ...[
                        const SizedBox(height: 4),
                        Row(
                          children: [
                            // Quarantine before everything, including the
                            // pipeline stage: a message we believe is a scam
                            // must not read as an ordinary supplier invoice
                            // in the list, which is how the fraud works.
                            if (_categoryChip(conversation.inboxCategory)
                                case final chip?)
                              Padding(
                                padding: const EdgeInsets.only(right: 4),
                                child: Container(
                                  key: Key(
                                      'conversation_category_chip_${conversation.id}'),
                                  padding: const EdgeInsets.symmetric(
                                      horizontal: 6, vertical: 1),
                                  decoration: BoxDecoration(
                                    color: (chip.danger
                                            ? colors.redAlert
                                            : colors.orangeAlert)
                                        .withValues(alpha: 0.15),
                                    borderRadius: BorderRadius.circular(4),
                                    border: Border.all(
                                        color: (chip.danger
                                                ? colors.redAlert
                                                : colors.orangeAlert)
                                            .withValues(alpha: 0.35)),
                                  ),
                                  child: Text(
                                    chip.label,
                                    style: TextStyle(
                                      color: chip.danger
                                          ? colors.redAlert
                                          : colors.orangeAlert,
                                      fontSize: 10,
                                      fontWeight: FontWeight.w600,
                                    ),
                                  ),
                                ),
                              ),
                            // Pipeline stage first (US-M024): an attendant
                            // about to answer needs to know whether this is
                            // a stranger or a deal in negotiation before
                            // they read anything else on the row.
                            if (conversation.leadStageLabel.isNotEmpty)
                              Padding(
                                padding: const EdgeInsets.only(right: 4),
                                child: GestureDetector(
                                  key: Key(
                                      'conversation_lead_chip_${conversation.id}'),
                                  onTap: conversation.leadId.isEmpty
                                      ? null
                                      : () => context.go(
                                          '/funnel?lead=${conversation.leadId}'),
                                  child: Container(
                                    padding: const EdgeInsets.symmetric(
                                        horizontal: 6, vertical: 1),
                                    decoration: BoxDecoration(
                                      color: colors.bluePrimary
                                          .withValues(alpha: 0.15),
                                      borderRadius: BorderRadius.circular(4),
                                      border: Border.all(
                                          color: colors.bluePrimary
                                              .withValues(alpha: 0.35)),
                                    ),
                                    child: Text(
                                      conversation.leadStageLabel,
                                      style: TextStyle(
                                        color: colors.bluePrimary,
                                        fontSize: 10,
                                        fontWeight: FontWeight.w600,
                                      ),
                                    ),
                                  ),
                                ),
                              ),
                            ...conversation.tags.take(3).map((tag) {
                              final tagColor = _parseColor(tag.color);
                              return Padding(
                                padding: const EdgeInsets.only(right: 4),
                                child: Container(
                                  padding: const EdgeInsets.symmetric(
                                      horizontal: 6, vertical: 1),
                                  decoration: BoxDecoration(
                                    color: tagColor.withValues(alpha: 0.15),
                                    borderRadius: BorderRadius.circular(4),
                                  ),
                                  child: Text(
                                    tag.name,
                                    style: TextStyle(
                                      color: tagColor,
                                      fontSize: 10,
                                      fontWeight: FontWeight.w600,
                                    ),
                                  ),
                                ),
                              );
                            }),
                            if (conversation.assignedToName != null)
                              Padding(
                                padding: const EdgeInsets.only(left: 2),
                                child: Row(
                                  mainAxisSize: MainAxisSize.min,
                                  children: [
                                    CircleAvatar(
                                      radius: 7,
                                      backgroundColor: colors.bluePrimary
                                          .withValues(alpha: 0.2),
                                      child: Text(
                                        conversation.assignedToName!
                                                .trim()
                                                .isNotEmpty
                                            ? conversation.assignedToName!
                                                .trim()[0]
                                                .toUpperCase()
                                            : '?',
                                        style: TextStyle(
                                          color: colors.bluePrimary,
                                          fontSize: 8,
                                          fontWeight: FontWeight.w700,
                                        ),
                                      ),
                                    ),
                                    const SizedBox(width: 3),
                                    Text(
                                      _firstName(conversation.assignedToName!),
                                      style: TextStyle(
                                        color: colors.textMuted,
                                        fontSize: 10,
                                        fontWeight: FontWeight.w500,
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                          ],
                        ),
                      ],
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  /// Returns the first word of a display name — "Ana Costa" → "Ana".
  /// Compact because the tile already shows tags, time, and an avatar
  /// letter; showing a full "Ana Costa" would wrap on mobile.
  static String _firstName(String full) {
    final trimmed = full.trim();
    if (trimmed.isEmpty) return '';
    final space = trimmed.indexOf(' ');
    return space < 0 ? trimmed : trimmed.substring(0, space);
  }

  static Color _parseColor(String hex) {
    final clean = hex.replaceFirst('#', '');
    if (clean.length == 6) {
      return Color(int.parse('FF$clean', radix: 16));
    }
    return const Color(0xFF6366F1);
  }

  /// Tiny colored chip beside the contact name. Renders only for the
  /// non-default classes (negative / angry / positive); neutral and
  /// nil are hidden so the row stays uncluttered for the 70% of
  /// conversations that don't need attention.
  Widget? _sentimentBadge(String? sentiment) {
    if (sentiment == null || sentiment.isEmpty) return null;
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final (color, label, icon) = switch (sentiment.toLowerCase()) {
      'angry' => (
          colors.redAlert,
          t.sentiment.angry,
          Icons.local_fire_department
        ),
      'negative' => (
          colors.orangeAlert,
          t.sentiment.negative,
          Icons.sentiment_dissatisfied
        ),
      'positive' => (
          colors.whatsappGreen,
          t.sentiment.positive,
          Icons.sentiment_satisfied
        ),
      _ => (null, '', Icons.help_outline),
    };
    if (color == null) return null;
    return Tooltip(
      message: t.sentiment.detected_tooltip(label: label),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.18),
          borderRadius: BorderRadius.circular(4),
          border: Border.all(color: color.withValues(alpha: 0.4), width: 0.5),
        ),
        child: Icon(icon, size: 10, color: color),
      ),
    );
  }

  Color _avatarColor(String name) {
    final colors = [
      const Color(0xFF6366F1), // indigo
      const Color(0xFF8B5CF6), // violet
      const Color(0xFFEC4899), // pink
      const Color(0xFF14B8A6), // teal
      const Color(0xFFF59E0B), // amber
      const Color(0xFF3B82F6), // blue
      const Color(0xFF10B981), // emerald
      const Color(0xFFEF4444), // red
    ];
    final hash = name.codeUnits.fold<int>(0, (prev, c) => prev + c);
    return colors[hash % colors.length];
  }
}

bool _showPriority(String priority) =>
    priority == 'urgent' || priority == 'high' || priority == 'low';

class _PriorityBadge extends StatelessWidget {
  final String priority;
  const _PriorityBadge({required this.priority});

  @override
  Widget build(BuildContext context) {
    final color = switch (priority) {
      'urgent' => const Color(0xFFEF4444),
      'high' => const Color(0xFFF59E0B),
      'low' => const Color(0xFF6B7280),
      _ => const Color(0xFF9CA3AF),
    };
    final icon = switch (priority) {
      'urgent' => Icons.error,
      'high' => Icons.arrow_upward,
      'low' => Icons.arrow_downward,
      _ => Icons.circle,
    };
    final label = switch (priority) {
      'urgent' => t.conversation.priority_urgent,
      'high' => t.conversation.priority_high,
      'low' => t.conversation.priority_low,
      _ => priority,
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.18),
        borderRadius: BorderRadius.circular(4),
        border: Border.all(color: color.withValues(alpha: 0.55), width: 0.8),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 10, color: color),
          const SizedBox(width: 3),
          Text(
            label,
            style: TextStyle(
              color: color,
              fontSize: 10,
              fontWeight: FontWeight.w700,
              letterSpacing: 0.2,
            ),
          ),
        ],
      ),
    );
  }
}

class _TileAvatar extends StatelessWidget {
  final String name;
  final String? avatarUrl;
  final Color color;

  const _TileAvatar({
    required this.name,
    required this.avatarUrl,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    final initialAvatar = CircleAvatar(
      radius: 24,
      backgroundColor: color,
      child: Text(
        name.isNotEmpty ? name[0].toUpperCase() : '?',
        style: const TextStyle(
          color: Colors.white,
          fontSize: 18,
          fontWeight: FontWeight.w700,
        ),
      ),
    );

    final hasPhoto = avatarUrl != null && avatarUrl!.isNotEmpty;
    if (!hasPhoto) return initialAvatar;

    return AppNetworkImage(
      url: avatarUrl,
      width: 48,
      height: 48,
      renderWidth: 48,
      borderRadius: BorderRadius.circular(24),
      fallback: initialAvatar,
    );
  }
}

class _LastMessagePreview extends StatelessWidget {
  final String text;
  final String type;
  final bool unread;
  final AppColorsExtension colors;

  const _LastMessagePreview({
    required this.text,
    required this.type,
    required this.unread,
    required this.colors,
  });

  @override
  Widget build(BuildContext context) {
    final style = TextStyle(
      fontSize: 13,
      color: unread ? colors.textSecondary : colors.textMuted,
      fontWeight: unread ? FontWeight.w500 : FontWeight.w400,
    );
    final meta = _metaFor(type);
    if (meta == null) {
      return Text(text,
          style: style, maxLines: 1, overflow: TextOverflow.ellipsis);
    }
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(meta.icon, size: 14, color: style.color),
        const SizedBox(width: 4),
        Flexible(
          child: Text(
            text.trim().isEmpty ? meta.label : '${meta.label} · $text',
            style: style,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );
  }

  static _PreviewMeta? _metaFor(String type) {
    switch (type) {
      case 'voice':
      case 'audio':
        return _PreviewMeta(Icons.mic_none, t.media_preview.audio);
      case 'image':
        return _PreviewMeta(Icons.image_outlined, t.media_preview.image);
      case 'video':
        return _PreviewMeta(Icons.videocam_outlined, t.media_preview.video);
      case 'document':
        return _PreviewMeta(
            Icons.insert_drive_file_outlined, t.media_preview.document);
      case 'location':
        return _PreviewMeta(
            Icons.location_on_outlined, t.media_preview.location);
      case 'sticker':
        return _PreviewMeta(
            Icons.emoji_emotions_outlined, t.media_preview.sticker);
      default:
        return null;
    }
  }
}

class _PreviewMeta {
  final IconData icon;
  final String label;
  const _PreviewMeta(this.icon, this.label);
}

/// The categories worth a chip, and nothing else.
///
/// Only two of the seven earn one: quarantine, because a suspected scam must
/// not read as an ordinary invoice, and hot lead, because it is the thing
/// the product exists to surface. Labelling every conversation "cliente" or
/// "outros" would add noise to every row and information to none — a badge
/// that is always there stops being read.
({String label, bool danger})? _categoryChip(String category) =>
    switch (category) {
      'quarantine' => (label: t.dashboard.category_quarantine, danger: true),
      'hot_lead' => (label: t.dashboard.category_hot_lead, danger: false),
      _ => null,
    };
