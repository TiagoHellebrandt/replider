import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/settings/presentation/controllers/tag_controller.dart';
import 'package:app/features/settings/presentation/controllers/team_controller.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

/// Runs one bar action and SAYS SO when it fails.
///
/// The chips used to fire-and-forget into the shared notifier, whose only
/// listener reacts to message sends — a resolve that 500'd changed nothing
/// on screen and said nothing, so the attendant believed it worked.
Future<void> _run(
  BuildContext context,
  WidgetRef ref,
  Future<void> Function() action,
) async {
  final messenger = ScaffoldMessenger.of(context);
  await action();
  final state = ref.read(conversationActionsProvider);
  if (state.hasError) {
    messenger.showSnackBar(SnackBar(
      key: const Key('conversation_action_error'),
      content: Text(parseApiError(state.error!)),
    ));
  }
}

class ConversationActionsBar extends ConsumerWidget {
  final ConversationModel conversation;

  const ConversationActionsBar({super.key, required this.conversation});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        border: Border(
          bottom: BorderSide(color: colors.borderLight, width: 0.5),
        ),
      ),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Row(
          children: [
            // Priority
            _PriorityChip(
              priority: conversation.priority,
              colors: colors,
              onChanged: (priority) => _run(
                context,
                ref,
                () => ref
                    .read(conversationActionsProvider.notifier)
                    .setPriority(conversation.id, priority),
              ),
            ),
            const SizedBox(width: 8),

            // Assign
            _AssignChip(
              assignedToName: conversation.assignedToName,
              colors: colors,
              onAssign: (userId) => _run(
                context,
                ref,
                () => ref
                    .read(conversationActionsProvider.notifier)
                    .assignConversation(conversation.id, userId),
              ),
            ),
            const SizedBox(width: 8),

            // Tags
            _TagsChip(
              conversationId: conversation.id,
              tags: conversation.tags,
              colors: colors,
            ),
            const SizedBox(width: 8),

            // Resolve. Adding a confirmation dialog because the tap
            // target sits next to Pause/Assign and a misfire closes the
            // conversation — previously you had to reopen via the list
            // filter and lose context. A single yes/no prompt prevents
            // the accident without adding real friction.
            if (conversation.status != ConversationStatus.closed)
              _ActionButton(
                icon: Icons.check_circle_outline,
                label: t.conversation.resolve_action,
                color: colors.whatsappGreen,
                colors: colors,
                onPressed: () async {
                  final ok = await showConfirmDialog(
                    context,
                    title: t.conversation.resolve_confirm_title,
                    message: t.conversation.resolve_confirm_body,
                    confirmText: t.conversation.resolve_action,
                  );
                  if (!ok) return;
                  if (!context.mounted) return;
                  await _run(
                    context,
                    ref,
                    () => ref
                        .read(conversationActionsProvider.notifier)
                        .resolveConversation(conversation.id),
                  );
                },
              ),

            // Reopen. A resolved thread used to be reachable only through
            // the "Encerradas" filter, and the customer's next reply opened
            // a SECOND conversation with no history; the server now
            // reopens on reply, and this is the attendant's own way back.
            if (conversation.status == ConversationStatus.closed)
              _ActionButton(
                key: const Key('conversation_reopen_button'),
                icon: Icons.replay_outlined,
                label: t.conversation.reopen_action,
                color: colors.whatsappGreen,
                colors: colors,
                onPressed: () => _run(
                  context,
                  ref,
                  () => ref
                      .read(conversationActionsProvider.notifier)
                      .reopenConversation(conversation.id),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _PriorityChip extends StatelessWidget {
  final String priority;
  final AppColorsExtension colors;
  final ValueChanged<String> onChanged;

  const _PriorityChip({
    required this.priority,
    required this.colors,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return PopupMenuButton<String>(
      onSelected: onChanged,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
        decoration: BoxDecoration(
          color: _priorityColor(priority).withValues(alpha: 0.15),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
              color: _priorityColor(priority).withValues(alpha: 0.3)),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(_priorityIcon(priority),
                size: 14, color: _priorityColor(priority)),
            const SizedBox(width: 4),
            Text(
              _priorityLabel(priority),
              style: TextStyle(
                color: _priorityColor(priority),
                fontSize: 12,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ),
      ),
      itemBuilder: (context) => [
        _priorityItem('low', colors),
        _priorityItem('normal', colors),
        _priorityItem('high', colors),
        _priorityItem('urgent', colors),
      ],
    );
  }

  PopupMenuItem<String> _priorityItem(String value, AppColorsExtension colors) {
    return PopupMenuItem(
      value: value,
      child: Row(
        children: [
          Icon(_priorityIcon(value), size: 16, color: _priorityColor(value)),
          const SizedBox(width: 8),
          Text(
            _priorityLabel(value),
            style: TextStyle(color: colors.textPrimary, fontSize: 13),
          ),
        ],
      ),
    );
  }

  static Color _priorityColor(String priority) {
    return switch (priority) {
      'urgent' => const Color(0xFFEF4444),
      'high' => const Color(0xFFF59E0B),
      'normal' => const Color(0xFF3B82F6),
      'low' => const Color(0xFF6B7280),
      _ => const Color(0xFF3B82F6),
    };
  }

  static IconData _priorityIcon(String priority) {
    return switch (priority) {
      'urgent' => Icons.error,
      'high' => Icons.arrow_upward,
      'normal' => Icons.remove,
      'low' => Icons.arrow_downward,
      _ => Icons.remove,
    };
  }

  static String _priorityLabel(String priority) {
    return switch (priority) {
      'urgent' => t.conversation.priority_urgent,
      'high' => t.conversation.priority_high,
      'normal' => t.conversation.priority_normal,
      'low' => t.conversation.priority_low,
      _ => t.conversation.priority_normal,
    };
  }
}

class _AssignChip extends ConsumerWidget {
  final String? assignedToName;
  final AppColorsExtension colors;
  final ValueChanged<String?> onAssign;

  const _AssignChip({
    required this.assignedToName,
    required this.colors,
    required this.onAssign,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final membersAsync = ref.watch(teamMembersProvider);

    return PopupMenuButton<String?>(
      onSelected: onAssign,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
        decoration: BoxDecoration(
          color: colors.glassMedium,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: colors.borderLight),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.person_outline, size: 14, color: colors.textMuted),
            const SizedBox(width: 4),
            Text(
              assignedToName ?? t.conversation.assign_label,
              style: TextStyle(
                color: assignedToName != null
                    ? colors.textPrimary
                    : colors.textMuted,
                fontSize: 12,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ),
      ),
      itemBuilder: (context) {
        final members = membersAsync.asData?.value ?? [];
        return [
          PopupMenuItem<String?>(
            value: '',
            child: Row(
              children: [
                Icon(Icons.person_off_outlined,
                    size: 16, color: colors.textMuted),
                const SizedBox(width: 8),
                Text(t.conversation.nobody,
                    style: TextStyle(color: colors.textMuted, fontSize: 13)),
              ],
            ),
          ),
          ...members.map((m) => PopupMenuItem<String?>(
                value: m.id,
                child: Row(
                  children: [
                    CircleAvatar(
                      radius: 10,
                      backgroundColor:
                          colors.bluePrimary.withValues(alpha: 0.15),
                      child: Text(
                        m.name.isNotEmpty ? m.name[0].toUpperCase() : '?',
                        style: TextStyle(
                            fontSize: 9,
                            color: colors.bluePrimary,
                            fontWeight: FontWeight.w700),
                      ),
                    ),
                    const SizedBox(width: 8),
                    Text(m.name,
                        style:
                            TextStyle(color: colors.textPrimary, fontSize: 13)),
                  ],
                ),
              )),
        ];
      },
    );
  }
}

class _TagsChip extends ConsumerWidget {
  final String conversationId;
  final List<ConversationTag> tags;
  final AppColorsExtension colors;

  const _TagsChip({
    required this.conversationId,
    required this.tags,
    required this.colors,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final allTagsAsync = ref.watch(tagControllerProvider);

    return GestureDetector(
      onTap: () => _showTagPicker(context, ref, allTagsAsync),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
        decoration: BoxDecoration(
          color: colors.glassMedium,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: colors.borderLight),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.label_outline, size: 14, color: colors.textMuted),
            const SizedBox(width: 4),
            if (tags.isEmpty)
              Text(
                t.conversation.tags_label,
                style: TextStyle(
                    color: colors.textMuted,
                    fontSize: 12,
                    fontWeight: FontWeight.w600),
              )
            else
              Row(
                mainAxisSize: MainAxisSize.min,
                children: tags.take(3).map((tag) {
                  return Padding(
                    padding: const EdgeInsets.only(right: 4),
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 6, vertical: 2),
                      decoration: BoxDecoration(
                        color: _parseColor(tag.color).withValues(alpha: 0.2),
                        borderRadius: BorderRadius.circular(4),
                      ),
                      child: Text(
                        tag.name,
                        style: TextStyle(
                          color: _parseColor(tag.color),
                          fontSize: 10,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                  );
                }).toList(),
              ),
            if (tags.length > 3)
              Text(
                '+${tags.length - 3}',
                style: TextStyle(color: colors.textMuted, fontSize: 10),
              ),
          ],
        ),
      ),
    );
  }

  void _showTagPicker(
      BuildContext context, WidgetRef ref, AsyncValue<dynamic> allTagsAsync) {
    final currentTagIds = tags.map((t) => t.id).toSet();

    showModalBottomSheet<void>(
      context: context,
      backgroundColor: colors.backgroundSecondary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      isScrollControlled: true,
      builder: (ctx) {
        String filter = '';
        return Padding(
          padding: EdgeInsets.only(
            bottom: MediaQuery.of(ctx).viewInsets.bottom,
          ),
          child: StatefulBuilder(
            builder: (ctx, setState) {
              // Re-read the tag list on every rebuild so a newly-created
              // tag shows up immediately after the in-modal create flow
              // invalidates tagControllerProvider.
              return Consumer(
                builder: (ctx, cref, _) {
                  final allTags =
                      cref.watch(tagControllerProvider).asData?.value ??
                          const [];
                  final visible = filter.isEmpty
                      ? allTags
                      : allTags
                          .where((t) => t.name.toLowerCase().contains(filter))
                          .toList();
                  return Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Expanded(
                              child: Text(
                                t.conversation.tag_picker_title,
                                style: TextStyle(
                                  color: colors.textPrimary,
                                  fontSize: 18,
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                            ),
                            TextButton.icon(
                              onPressed: () => _showCreateTagDialog(ctx, cref),
                              icon: const Icon(Icons.add, size: 18),
                              label: Text(t.conversation.tag_picker_new),
                              style: TextButton.styleFrom(
                                foregroundColor: colors.bluePrimary,
                                padding: const EdgeInsets.symmetric(
                                    horizontal: 10, vertical: 6),
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 12),
                        if (allTags.length > 8)
                          TextField(
                            autofocus: true,
                            style: TextStyle(
                                color: colors.textPrimary, fontSize: 14),
                            decoration: InputDecoration(
                              prefixIcon: Icon(Icons.search,
                                  size: 18, color: colors.textMuted),
                              hintText: t.conversation.tag_picker_search,
                              hintStyle: TextStyle(
                                  color: colors.textMuted, fontSize: 13),
                              isDense: true,
                            ),
                            onChanged: (v) =>
                                setState(() => filter = v.trim().toLowerCase()),
                          ),
                        const SizedBox(height: 12),
                        if (allTags.isEmpty)
                          Padding(
                            padding: const EdgeInsets.symmetric(vertical: 24),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.center,
                              children: [
                                Icon(Icons.label_outline,
                                    size: 36, color: colors.textMuted),
                                const SizedBox(height: 8),
                                Text(
                                  t.conversation.tag_picker_empty_title,
                                  style: TextStyle(
                                      color: colors.textSecondary,
                                      fontSize: 14,
                                      fontWeight: FontWeight.w600),
                                ),
                                const SizedBox(height: 4),
                                Text(
                                  t.conversation.tag_picker_empty_body,
                                  style: TextStyle(
                                      color: colors.textMuted, fontSize: 12),
                                ),
                              ],
                            ),
                          )
                        else if (visible.isEmpty)
                          Padding(
                            padding: const EdgeInsets.all(16),
                            child: Text(
                              t.conversation.tag_picker_no_match(query: filter),
                              style: TextStyle(
                                  color: colors.textMuted, fontSize: 13),
                            ),
                          )
                        else
                          Wrap(
                            spacing: 8,
                            runSpacing: 8,
                            children: visible.map<Widget>((tag) {
                              final isActive = currentTagIds.contains(tag.id);
                              final tagColor = _parseColor(tag.color);
                              return FilterChip(
                                selected: isActive,
                                label: Text(
                                  tag.name,
                                  style: TextStyle(
                                    color: isActive ? Colors.white : tagColor,
                                    fontSize: 13,
                                    fontWeight: FontWeight.w600,
                                  ),
                                ),
                                backgroundColor:
                                    tagColor.withValues(alpha: 0.15),
                                selectedColor: tagColor,
                                checkmarkColor: Colors.white,
                                side: BorderSide(
                                    color: tagColor.withValues(alpha: 0.3)),
                                onSelected: (selected) async {
                                  // Optimistic UI: flip the chip first so
                                  // the user gets instant feedback, roll
                                  // back on error. The old flow awaited
                                  // the network call before setState,
                                  // which made taps feel broken on slow
                                  // connections.
                                  setState(() {
                                    if (selected) {
                                      currentTagIds.add(tag.id);
                                    } else {
                                      currentTagIds.remove(tag.id);
                                    }
                                  });
                                  final ds = cref.read(tagDataSourceProvider);
                                  try {
                                    if (selected) {
                                      await ds.addToConversation(
                                          conversationId, {'tag_id': tag.id});
                                    } else {
                                      await ds.removeFromConversation(
                                          conversationId, tag.id);
                                    }
                                    cref.invalidate(conversationsProvider);
                                  } catch (e) {
                                    setState(() {
                                      if (selected) {
                                        currentTagIds.remove(tag.id);
                                      } else {
                                        currentTagIds.add(tag.id);
                                      }
                                    });
                                    if (ctx.mounted) {
                                      ScaffoldMessenger.of(ctx).showSnackBar(
                                        SnackBar(
                                          content: Text(t.conversation
                                              .tag_update_error(
                                                  error: parseApiError(e))),
                                          backgroundColor: Colors.red.shade700,
                                        ),
                                      );
                                    }
                                  }
                                },
                              );
                            }).toList(),
                          ),
                        const SizedBox(height: 16),
                      ],
                    ),
                  );
                },
              );
            },
          ),
        );
      },
    );
  }

  /// Inline tag-creation dialog. Saves through TagController.create so
  /// the open bottom sheet re-renders via its Consumer + picks up the
  /// new tag without the user leaving the conversation. The color
  /// palette mirrors the one on Settings → Tags for consistency.
  Future<void> _showCreateTagDialog(BuildContext parent, WidgetRef ref) async {
    final controller = TextEditingController();
    try {
      String selectedColor = _tagPalette.first;
      final created = await showDialog<bool>(
        context: parent,
        builder: (ctx) {
          return StatefulBuilder(
            builder: (ctx, setState) {
              return AlertDialog(
                title: Text(t.conversation.tag_dialog_title),
                content: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    TextField(
                      controller: controller,
                      autofocus: true,
                      decoration: InputDecoration(
                        labelText: t.conversation.tag_name_label,
                        hintText: t.conversation.tag_name_hint,
                        isDense: true,
                      ),
                      textInputAction: TextInputAction.done,
                      // The button follows what is typed: without this it took
                      // the click, closed the dialog and created no tag at
                      // all.
                      onChanged: (_) => setState(() {}),
                      onSubmitted: (_) => Navigator.of(ctx).pop(true),
                    ),
                    const SizedBox(height: 14),
                    Text(t.conversation.tag_color_label,
                        style: const TextStyle(fontSize: 12)),
                    const SizedBox(height: 6),
                    Wrap(
                      spacing: 6,
                      runSpacing: 6,
                      children: _tagPalette.map((hex) {
                        final picked = hex == selectedColor;
                        return GestureDetector(
                          onTap: () => setState(() => selectedColor = hex),
                          child: Container(
                            width: 28,
                            height: 28,
                            decoration: BoxDecoration(
                              color: _parseColor(hex),
                              shape: BoxShape.circle,
                              border: Border.all(
                                color:
                                    picked ? Colors.white : Colors.transparent,
                                width: 2,
                              ),
                              boxShadow: picked
                                  ? [
                                      BoxShadow(
                                          color: _parseColor(hex),
                                          blurRadius: 6)
                                    ]
                                  : null,
                            ),
                          ),
                        );
                      }).toList(),
                    ),
                  ],
                ),
                actions: [
                  TextButton(
                    onPressed: () => Navigator.of(ctx).pop(false),
                    child: Text(t.common.cancel),
                  ),
                  FilledButton(
                    onPressed: controller.text.trim().isEmpty
                        ? null
                        : () => Navigator.of(ctx).pop(true),
                    child: Text(t.conversation.tag_dialog_create),
                  ),
                ],
              );
            },
          );
        },
      );
      if (created != true) return;
      final name = controller.text.trim();
      if (name.isEmpty) return;
      try {
        await ref
            .read(tagControllerProvider.notifier)
            .create(name: name, color: selectedColor);
      } catch (e) {
        if (parent.mounted) {
          ScaffoldMessenger.of(parent).showSnackBar(
            SnackBar(
              content: Text(
                  t.conversation.tag_create_error(error: parseApiError(e))),
              backgroundColor: Colors.red.shade700,
            ),
          );
        }
      }
    } finally {
      controller.dispose();
    }
  }

  /// Curated tag palette — same set surfaced on Settings → Tags, kept
  /// in sync so tags created here don't look out of place there.
  static const List<String> _tagPalette = [
    '#6366F1', // indigo
    '#8B5CF6', // violet
    '#EC4899', // pink
    '#EF4444', // red
    '#F59E0B', // amber
    '#10B981', // emerald
    '#14B8A6', // teal
    '#3B82F6', // blue
    '#6B7280', // gray
  ];

  static Color _parseColor(String hex) {
    final clean = hex.replaceFirst('#', '');
    if (clean.length == 6) {
      return Color(int.parse('FF$clean', radix: 16));
    }
    return const Color(0xFF6366F1);
  }
}

class _ActionButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color color;
  final AppColorsExtension colors;
  final VoidCallback onPressed;

  const _ActionButton({
    super.key,
    required this.icon,
    required this.label,
    required this.color,
    required this.colors,
    required this.onPressed,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onPressed,
      borderRadius: BorderRadius.circular(8),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.15),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: color.withValues(alpha: 0.3)),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 14, color: color),
            const SizedBox(width: 4),
            Text(
              label,
              style: TextStyle(
                color: color,
                fontSize: 12,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
