import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';
import 'package:desktop_drop/desktop_drop.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/core/network/websocket_service.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/dashboard/domain/models/message_model.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/dashboard/presentation/widgets/reply_suggestions_bar.dart';
import 'package:app/features/dashboard/presentation/widgets/wa_send_template_dialog.dart';
import 'package:app/features/dashboard/presentation/widgets/audio_recorder_bar.dart';
import 'package:app/features/dashboard/presentation/widgets/message_bubble.dart';
import 'package:app/features/dashboard/presentation/widgets/source_badge.dart';
import 'package:app/features/dashboard/presentation/widgets/conversation_actions_bar.dart';
import 'package:app/features/dashboard/presentation/widgets/takeover_bar.dart';
import 'package:app/features/dashboard/presentation/widgets/contact_detail_sheet.dart';
import 'package:app/features/dashboard/presentation/widgets/media_send_helper.dart';
import 'package:app/features/settings/presentation/controllers/canned_response_controller.dart';
import 'package:app/features/dashboard/presentation/widgets/email_inbox_view.dart';
import 'package:app/features/dashboard/presentation/widgets/notes_panel.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/dashboard/presentation/widgets/ai_debounce_banner.dart';
import 'package:app/i18n/strings.g.dart';

class ConversationDetailPanel extends ConsumerStatefulWidget {
  final bool showBackButton;

  const ConversationDetailPanel({super.key, this.showBackButton = false});

  @override
  ConsumerState<ConversationDetailPanel> createState() =>
      _ConversationDetailPanelState();
}

class _ConversationDetailPanelState
    extends ConsumerState<ConversationDetailPanel> {
  bool _isSearching = false;
  bool _showCannedOverlay = false;
  String _cannedFilter = '';
  int _searchMatchIndex = 0;
  final _searchController = TextEditingController();
  final _searchFocusNode = FocusNode();
  final _messageController = TextEditingController();
  // Email composer only: the subject line of the outgoing reply.
  final _subjectController = TextEditingController();
  // Which conversation the subject was last prefilled for — "Re: <newest
  // inbound subject>" seeds once per thread and then belongs to the user.
  String? _subjectSeededFor;

  /// Whether the email composer is expanded. Collapsed (a single "Reply"
  /// chip) is the default — an email reply is an occasional deliberate act,
  /// and a permanently open subject+body+button block taxed every read with
  /// ~180px of dead height. Reopened automatically when a draft, a co-pilot
  /// prefill or a failed-send restore puts text back in the body.
  bool _emailComposerOpen = false;

  /// Explicit FocusNode on the message input. Kept in state so we can
  /// re-focus after the widget rebuilds (e.g. when a new message
  /// arrives via WS and the messages list invalidates, the TextField
  /// was losing focus mid-typing — annoying on a busy inbox).
  final _messageFocusNode = FocusNode();

  /// Last send attempt's text. Cleared on success, used to restore the
  /// input when the notifier lands in error state. Without this the
  /// previous fire-and-forget send() dropped the user's message on
  /// network failure with no way to recover.
  String? _pendingText;
  final _scrollController = ScrollController();

  /// True while an external drag is hovering over the panel. Drives
  /// the "drop to send" overlay so the user gets feedback that the
  /// surface accepts files.
  bool _isDraggingFile = false;

  /// When true the composer swaps the text field + attachment icons for
  /// the inline AudioRecorderBar. Toggled by the mic icon; cleared when
  /// the recorder completes (sent) or is canceled.
  bool _recording = false;

  /// Per-conversation composer drafts. The controller is panel state and
  /// used to SURVIVE a conversation switch: type for customer A, click
  /// customer B, press send — A's message went to B. Stashed on switch,
  /// restored on return.
  final Map<String, String> _drafts = {};

  @override
  void initState() {
    super.initState();
    _messageController.addListener(_onMessageChanged);
  }

  void _onMessageChanged() {
    final text = _messageController.text;
    if (text.startsWith('/')) {
      final filter = text.substring(1).toLowerCase();
      if (!_showCannedOverlay || _cannedFilter != filter) {
        setState(() {
          _showCannedOverlay = true;
          _cannedFilter = filter;
        });
      }
    } else if (_showCannedOverlay) {
      setState(() {
        _showCannedOverlay = false;
        _cannedFilter = '';
      });
    }
  }

  @override
  void dispose() {
    _messageController.removeListener(_onMessageChanged);
    _messageFocusNode.dispose();
    _searchController.dispose();
    _searchFocusNode.dispose();
    _messageController.dispose();
    _subjectController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  void _scrollToBottom() {
    // Schedule up to three frames. On slow networks the first post-
    // frame callback fires before the rebuilt ListView has measured
    // its new content; retrying once or twice catches the common case
    // without spinning forever. Also guards against unattached
    // controllers and unmounted state across each attempt.
    void attempt(int remaining) {
      if (!mounted || remaining == 0) return;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        if (!_scrollController.hasClients) {
          attempt(remaining - 1);
          return;
        }
        _scrollController.animateTo(
          0,
          duration: const Duration(milliseconds: 300),
          curve: Curves.easeOut,
        );
      });
    }

    attempt(3);
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final selectedId = ref.watch(selectedConversationProvider);

    // Switching conversations stashes the draft, restores the target's,
    // and resets everything that is about ONE thread: a half-typed
    // message, a running recording, an open search bar and a pending
    // retry must never follow the operator to the next customer.
    ref.listen<String?>(selectedConversationProvider, (prev, next) {
      if (prev == next) return;
      if (prev != null) _drafts[prev] = _messageController.text;
      _messageController.text = next == null ? '' : (_drafts[next] ?? '');
      _messageController.selection =
          TextSelection.collapsed(offset: _messageController.text.length);
      // The email reader is per-thread; the next conversation opens on its
      // own inbox list, never on the previous thread's open mail.
      ref.read(emailReaderOpenProvider.notifier).close();
      setState(() {
        _pendingText = null;
        // A stashed draft reopens the composer; otherwise it starts folded.
        _emailComposerOpen = _messageController.text.trim().isNotEmpty;
        _recording = false;
        _isSearching = false;
        _searchMatchIndex = 0;
      });
      _searchController.clear();
      ref.read(messageSearchProvider.notifier).clear();
    });

    if (selectedId == null) {
      return _buildEmptyState(colors);
    }

    // One-shot composer prefill (US-015 co-pilot templates). Applied
    // after the frame so we never mutate providers during build; text
    // already typed is preserved, the template is appended below it.
    final prefill = ref.watch(composerPrefillProvider);
    if (prefill != null && prefill.isNotEmpty) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        ref.read(composerPrefillProvider.notifier).set(null);
        final existing = _messageController.text;
        _messageController.text =
            existing.isEmpty ? prefill : '$existing\n$prefill';
        _messageController.selection =
            TextSelection.collapsed(offset: _messageController.text.length);
        // A prefilled template must be visible — unfold the email composer.
        if (!_emailComposerOpen) setState(() => _emailComposerOpen = true);
        _messageFocusNode.requestFocus();
      });
    }

    // Direct WS listener for targeted, immediate message refresh
    ref.listen(wsEventsProvider, (_, next) {
      next.whenData((event) {
        if (event.conversationId != selectedId) return;
        if (event.event == 'new_message') {
          ref.invalidate(messagesProvider(selectedId));
          _scrollToBottom();
          // Re-focus the composer if it had focus before this rebuild.
          // Invalidating messagesProvider triggers a widget refresh that
          // occasionally steals focus mid-typing — hand it back after the
          // next frame so the user can keep typing without tapping again.
          if (_messageFocusNode.hasFocus) {
            WidgetsBinding.instance.addPostFrameCallback((_) {
              if (mounted && !_messageFocusNode.hasFocus) {
                _messageFocusNode.requestFocus();
              }
            });
          }
        }
      });
    });

    // Recover pending text when a reply fails — the notifier already
    // tracks the error, but the original flow cleared the input
    // optimistically, leaving the user with a silent failure and no
    // message to retry. Here we restore the text + show a snackbar
    // with a Retry action.
    ref.listen(conversationActionsProvider, (prev, next) {
      if (next is AsyncError && _pendingText != null) {
        final text = _pendingText!;
        _pendingText = null;
        // Always restore the failed text. If the user has already
        // started typing the next message, prepend the failed text +
        // newline so neither is lost — the earlier "only when empty"
        // guard silently dropped the failed message in that case.
        final existing = _messageController.text;
        final restored = existing.isEmpty ? text : '$text\n$existing';
        _messageController.text = restored;
        _messageController.selection =
            TextSelection.collapsed(offset: text.length);
        // The restored text must be visible — unfold the email composer.
        if (!_emailComposerOpen) setState(() => _emailComposerOpen = true);
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(t.conversation.send_failed_recovered),
          behavior: SnackBarBehavior.floating,
          action: SnackBarAction(
            label: t.dashboard.retry_send,
            onPressed: () => _sendMessage(selectedId),
          ),
        ));
      } else if (next is AsyncData && prev is AsyncLoading) {
        // Successful send; drop the pending buffer. Also re-fetch the
        // messages list — if the WebSocket was momentarily disconnected
        // during the send (3s reconnect window), the "new_message"
        // event would have been missed and the bubble would never show
        // up locally. The REST refresh closes that gap.
        _pendingText = null;
        ref.invalidate(messagesProvider(selectedId));
      }
    });

    final conversationsAsync = ref.watch(conversationsProvider);

    return conversationsAsync.when(
      data: (conversations) {
        final conversation =
            conversations.where((c) => c.id == selectedId).firstOrNull;
        if (conversation == null) {
          return _buildEmptyState(colors);
        }

        final messagesAsync = ref.watch(messagesProvider(selectedId));
        final isTyping = ref.watch(
          typingStateProvider.select((m) => m[selectedId] ?? false),
        );

        // Drop mirrors the paperclip's gates: no media on email (the MIME
        // builder only carries text — the upload would succeed and the file
        // silently never reach the recipient) and no media while the bot
        // owns the conversation (the server refuses AFTER the GCS upload).
        final dropEnabled = conversation.source != ConversationSource.email &&
            conversation.isBotPaused;

        return DropTarget(
          enable: dropEnabled,
          onDragEntered: (_) {
            if (!_isDraggingFile) {
              setState(() => _isDraggingFile = true);
            }
          },
          onDragExited: (_) {
            if (_isDraggingFile) {
              setState(() => _isDraggingFile = false);
            }
          },
          onDragDone: (detail) async {
            setState(() => _isDraggingFile = false);
            await _handleDroppedFiles(selectedId, detail);
          },
          child: Stack(
            children: [
              Column(
                children: [
                  // While an EMAIL is open in the reader, the contact header
                  // and notes bar yield: the reader's own header already
                  // names sender + date, and the ~215px of stacked chrome
                  // was burying the message (design A, 2026-08-27). The
                  // actions bar stays — resolve/assign/tags apply while
                  // reading too.
                  if (conversation.source != ConversationSource.email ||
                      ref.watch(emailReaderOpenProvider) == null) ...[
                    _buildHeader(context, colors, conversation),
                    ConversationActionsBar(conversation: conversation),
                    NotesPanel(conversationId: conversation.id),
                  ] else
                    ConversationActionsBar(conversation: conversation),
                  Expanded(
                    child: Container(
                      decoration:
                          BoxDecoration(color: colors.backgroundPrimary),
                      child: messagesAsync.when(
                        data: (latest) {
                          // Older pages loaded on demand ride in front of
                          // the newest page (see MessageHistory).
                          final history =
                              ref.watch(messageHistoryProvider(selectedId));
                          final messages =
                              mergeMessageHistory(history.messages, latest);
                          // More to load while the newest page is full
                          // and the server has not answered a short page.
                          final canLoadOlder = !history.exhausted &&
                              latest.length >= messagePageSize;
                          if (messages.isEmpty && !isTyping) {
                            return Center(
                              child: Text(
                                t.settings.no_messages,
                                style: TextStyle(
                                    color: colors.textMuted, fontSize: 14),
                              ),
                            );
                          }
                          // Email reads as an INBOX, not a chat: subject
                          // first, expand-in-place, original formatting in a
                          // sandboxed frame. The composer below still replies
                          // into the same thread.
                          if (conversation.source == ConversationSource.email) {
                            return EmailInboxView(
                              // Keyed by CONVERSATION: expansion state and
                              // open original frames are per-thread; a
                              // reused State leaks thread A's layout onto B.
                              key: ValueKey('email_inbox_$selectedId'),
                              messages: messages,
                              scrollController: _scrollController,
                            );
                          }
                          final searchQuery = ref.watch(messageSearchProvider);
                          final matchedIndices = searchQuery.isEmpty
                              ? <int>[]
                              : ref.watch(
                                  matchedMessageIndicesProvider(selectedId));
                          // Reversed list: the "load earlier" row is the
                          // LAST index, i.e. the top of the transcript.
                          final itemCount = messages.length +
                              (isTyping ? 1 : 0) +
                              (canLoadOlder ? 1 : 0);
                          // SelectionArea, not SelectableText per bubble: the
                          // transcript is selectable with the mouse across
                          // messages, the bubble keeps its own long press (it
                          // is the deeper recognizer, so it wins the arena),
                          // and nothing paints a stray selection handle into a
                          // bubble when no selection exists.
                          return SelectionArea(
                              child: ListView.builder(
                            controller: _scrollController,
                            reverse: true,
                            padding: const EdgeInsets.symmetric(vertical: 16),
                            itemCount: itemCount,
                            itemBuilder: (context, index) {
                              // Typing indicator is the "newest" item (index 0 in reversed list)
                              if (isTyping && index == 0) {
                                return _buildTypingIndicator(colors);
                              }
                              if (canLoadOlder && index == itemCount - 1) {
                                return _LoadOlderRow(
                                  loading: history.loading,
                                  onPressed: () => ref
                                      .read(messageHistoryProvider(selectedId)
                                          .notifier)
                                      .loadOlder(messages.first.createdAt),
                                );
                              }
                              final msgIndex = messages.length -
                                  1 -
                                  (isTyping ? index - 1 : index);
                              final msg = messages[msgIndex];
                              final prev =
                                  msgIndex > 0 ? messages[msgIndex - 1] : null;
                              final showSeparator = prev == null ||
                                  !_sameDay(prev.createdAt, msg.createdAt);
                              final isCurrentMatch =
                                  matchedIndices.isNotEmpty &&
                                      matchedIndices[_searchMatchIndex %
                                              matchedIndices.length] ==
                                          msgIndex;
                              return Column(
                                children: [
                                  if (showSeparator)
                                    _daySeparator(colors, msg.createdAt),
                                  MessageBubble(
                                    message: msg,
                                    searchQuery: searchQuery.isEmpty
                                        ? null
                                        : searchQuery,
                                    isCurrentMatch: isCurrentMatch,
                                    source: conversation.source,
                                  ),
                                ],
                              );
                            },
                          ));
                        },
                        loading: () =>
                            const Center(child: CircularProgressIndicator()),
                        error: (err, _) => Center(
                          child: Text(parseApiError(err),
                              style: TextStyle(color: colors.textMuted)),
                        ),
                      ),
                    ),
                  ),
                  _buildInputArea(colors, selectedId),
                ],
              ),
              if (_isDraggingFile)
                Positioned.fill(child: _buildDropOverlay(colors)),
            ],
          ),
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => Center(
        child:
            Text(parseApiError(err), style: TextStyle(color: colors.redAlert)),
      ),
    );
  }

  /// Thin pill with the day label ("Hoje", "Ontem" or an absolute
  /// date). Rendered between messages whenever the calendar day
  /// rolls over. We format in PT-BR because the dashboard audience
  /// is effectively pt_BR-only — localizing this to the app locale
  /// would be strictly better but isn't a blocker.
  Widget _daySeparator(AppColorsExtension colors, DateTime when) {
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    final d = DateTime(when.year, when.month, when.day);
    final diffDays = today.difference(d).inDays;
    final label = diffDays == 0
        ? t.dashboard.day_today
        : diffDays == 1
            ? t.dashboard.day_yesterday
            : '${d.day.toString().padLeft(2, '0')}/${d.month.toString().padLeft(2, '0')}/${d.year}';
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Center(
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
          decoration: BoxDecoration(
            color: colors.glassMedium,
            borderRadius: BorderRadius.circular(10),
          ),
          child: Text(
            label,
            style: TextStyle(
              color: colors.textMuted,
              fontSize: 11,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
      ),
    );
  }

  void _scrollToMatch(int msgIndex, int totalMessages) {
    if (!_scrollController.hasClients) return;
    if (totalMessages <= 1) return;
    // List is reverse:true — msgIndex 0 = oldest = top = maxScrollExtent.
    // reversed list position of msgIndex = totalMessages - 1 - msgIndex.
    final reversedPos = totalMessages - 1 - msgIndex;
    final fraction = reversedPos / (totalMessages - 1);
    final target = fraction * _scrollController.position.maxScrollExtent;
    _scrollController.animateTo(
      target,
      duration: const Duration(milliseconds: 300),
      curve: Curves.easeOut,
    );
  }

  bool _sameDay(DateTime a, DateTime b) =>
      a.year == b.year && a.month == b.month && a.day == b.day;

  /// Renders `text` with the first `needle` substring painted in
  /// [matchColor]. Used by the canned-responses overlay so the user
  /// can tell at a glance which part of a title the filter matched —
  /// otherwise a dozen results all look identical and the filter
  /// feels like magic.
  Widget _highlightMatch(
    String text,
    String needle, {
    required TextStyle baseStyle,
    required Color matchColor,
  }) {
    if (needle.isEmpty) {
      return Text(text, style: baseStyle);
    }
    final lower = text.toLowerCase();
    final i = lower.indexOf(needle);
    if (i < 0) return Text(text, style: baseStyle);
    return RichText(
      text: TextSpan(style: baseStyle, children: [
        TextSpan(text: text.substring(0, i)),
        TextSpan(
          text: text.substring(i, i + needle.length),
          style: baseStyle.copyWith(
              color: matchColor, fontWeight: FontWeight.w800),
        ),
        TextSpan(text: text.substring(i + needle.length)),
      ]),
    );
  }

  Widget _buildTypingIndicator(AppColorsExtension colors) {
    // Fade-in so the indicator doesn't pop into existence — feels
    // more alive and signals to the attendant that the bot just
    // started composing, rather than the dots having been there the
    // whole time. TweenAnimationBuilder starts the animation on first
    // build and stays at 1.0 afterwards; no controller needed.
    return TweenAnimationBuilder<double>(
      tween: Tween(begin: 0.0, end: 1.0),
      duration: const Duration(milliseconds: 250),
      builder: (_, value, child) => Opacity(opacity: value, child: child),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 3),
        child: Align(
          alignment: Alignment.centerRight,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Padding(
                padding: const EdgeInsets.only(bottom: 2),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(Icons.smart_toy, size: 12, color: colors.cyanGlow),
                    const SizedBox(width: 3),
                    Text(
                      t.settings.typing,
                      style: TextStyle(
                        fontSize: 10,
                        fontWeight: FontWeight.w600,
                        color: colors.cyanGlow,
                      ),
                    ),
                  ],
                ),
              ),
              Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                decoration: BoxDecoration(
                  color: const Color(0xFF1A3A5C),
                  borderRadius: const BorderRadius.only(
                    topLeft: Radius.circular(16),
                    topRight: Radius.circular(16),
                    bottomLeft: Radius.circular(16),
                  ),
                  border: Border.all(color: colors.borderLight, width: 0.5),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    _TypingDot(colors: colors, delay: 0),
                    const SizedBox(width: 4),
                    _TypingDot(colors: colors, delay: 200),
                    const SizedBox(width: 4),
                    _TypingDot(colors: colors, delay: 400),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildHeader(
    BuildContext context,
    AppColorsExtension colors,
    ConversationModel conversation,
  ) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        border: Border(
          bottom: BorderSide(color: colors.borderLight, width: 0.5),
        ),
      ),
      child: Row(
        children: [
          if (widget.showBackButton)
            IconButton(
              icon:
                  Icon(Icons.arrow_back, color: colors.textSecondary, size: 22),
              onPressed: () {
                ref.read(selectedConversationProvider.notifier).select(null);
              },
              splashRadius: 20,
              padding: EdgeInsets.zero,
              constraints: const BoxConstraints(minWidth: 36, minHeight: 36),
            ),
          InkWell(
            onTap: () => showContactDetailSheet(context, conversation),
            borderRadius: BorderRadius.circular(20),
            child: _HeaderAvatar(
              name: conversation.contactName,
              avatarUrl: conversation.contactAvatarUrl,
              color: _avatarColor(conversation.contactName),
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: InkWell(
              onTap: () => showContactDetailSheet(context, conversation),
              borderRadius: BorderRadius.circular(6),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Flexible(
                        child: Text(
                          conversation.contactName,
                          style: TextStyle(
                            fontSize: 15,
                            fontWeight: FontWeight.w600,
                            color: colors.textPrimary,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      const SizedBox(width: 6),
                      SourceBadge(source: conversation.source, size: 12),
                      // Bot-paused pill. High-contrast so the attendant
                      // can tell at a glance whether the AI is replying
                      // on its own. Previously this state only showed in
                      // small grey copy at the bottom of the header and
                      // attendants kept sending duplicate messages
                      // because they assumed the bot was off.
                      //
                      // `pendingHuman` is the lifecycle state the backend
                      // flips to when a human takes over; we use it as the
                      // proxy for "bot is paused" here so the client model
                      // doesn't need the extra field.
                      if (conversation.status ==
                          ConversationStatus.pendingHuman) ...[
                        const SizedBox(width: 6),
                        Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 6, vertical: 2),
                          decoration: BoxDecoration(
                            color: colors.orangeAlert.withValues(alpha: 0.15),
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Icon(Icons.pause_circle_filled,
                                  size: 10, color: colors.orangeAlert),
                              const SizedBox(width: 3),
                              Text(
                                t.dashboard.bot_paused_pill,
                                style: TextStyle(
                                  color: colors.orangeAlert,
                                  fontSize: 10,
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ],
                  ),
                  const SizedBox(height: 2),
                  Row(
                    children: [
                      if (conversation.contactUsername != null) ...[
                        Text(
                          '@${conversation.contactUsername}',
                          style: TextStyle(
                            fontSize: 11,
                            color: colors.cyanGlow,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                        const SizedBox(width: 6),
                      ],
                      // On email the ADDRESS is the identity — a display
                      // name alone ("Tiago Silva") hides who actually wrote,
                      // which on a channel where anyone can claim any name
                      // is exactly the wrong thing to hide.
                      if (conversation.source == ConversationSource.email &&
                          conversation.contactExternalId.contains('@')) ...[
                        Flexible(
                          child: Text(
                            conversation.contactExternalId,
                            key: const Key('conversation_sender_address'),
                            style: TextStyle(
                              fontSize: 11,
                              color: colors.textSecondary,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                        const SizedBox(width: 6),
                      ],
                      Text(
                        _statusLabel(
                            conversation.status, conversation.isBotPaused),
                        style: TextStyle(
                          fontSize: 11,
                          color: _statusColor(conversation.status,
                              conversation.isBotPaused, colors),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
          // Inline summary affordance. Tap fires the LLM endpoint;
          // result lands as a snackbar so the operator reads it
          // without a modal blocking the chat.
          IconButton(
            tooltip: t.dashboard.summary_tooltip,
            icon: Icon(Icons.summarize, color: colors.bluePrimary, size: 20),
            onPressed: () => _showSummary(conversation.id),
            splashRadius: 18,
            padding: EdgeInsets.zero,
            constraints: const BoxConstraints(minWidth: 36, minHeight: 36),
          ),
          if (_isSearching) ...[
            const SizedBox(width: 6),
            Expanded(
              child: Container(
                height: 32,
                decoration: BoxDecoration(
                  color: colors.glassLight,
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(color: colors.borderMedium),
                ),
                child: TextField(
                  controller: _searchController,
                  focusNode: _searchFocusNode,
                  onChanged: (v) {
                    ref.read(messageSearchProvider.notifier).update(v);
                    setState(() => _searchMatchIndex = 0);
                  },
                  style: TextStyle(fontSize: 13, color: colors.textPrimary),
                  decoration: InputDecoration(
                    hintText: t.dashboard.search_messages_hint,
                    hintStyle: TextStyle(color: colors.textMuted, fontSize: 13),
                    border: InputBorder.none,
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: 10,
                      vertical: 6,
                    ),
                    isDense: true,
                  ),
                ),
              ),
            ),
            // Match counter + navigation
            Builder(builder: (_) {
              final matched =
                  ref.watch(matchedMessageIndicesProvider(conversation.id));
              final total = matched.length;
              final current = total == 0 ? 0 : _searchMatchIndex + 1;
              return Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (total > 0)
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 4),
                      child: Text(
                        '$current/$total',
                        style: TextStyle(
                          fontSize: 11,
                          color: colors.textMuted,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                  IconButton(
                    icon: Icon(Icons.keyboard_arrow_up,
                        size: 18,
                        color: total > 0
                            ? colors.textSecondary
                            : colors.textMuted),
                    padding: EdgeInsets.zero,
                    constraints:
                        const BoxConstraints(minWidth: 28, minHeight: 28),
                    splashRadius: 14,
                    onPressed: total == 0
                        ? null
                        : () {
                            setState(() {
                              _searchMatchIndex =
                                  (_searchMatchIndex + 1) % total;
                            });
                            final msgs = ref
                                .read(messagesProvider(conversation.id))
                                .asData
                                ?.value;
                            if (msgs != null && msgs.isNotEmpty) {
                              _scrollToMatch(
                                  matched[_searchMatchIndex], msgs.length);
                            }
                          },
                  ),
                  IconButton(
                    icon: Icon(Icons.keyboard_arrow_down,
                        size: 18,
                        color: total > 0
                            ? colors.textSecondary
                            : colors.textMuted),
                    padding: EdgeInsets.zero,
                    constraints:
                        const BoxConstraints(minWidth: 28, minHeight: 28),
                    splashRadius: 14,
                    onPressed: total == 0
                        ? null
                        : () {
                            setState(() {
                              _searchMatchIndex =
                                  (_searchMatchIndex - 1 + total) % total;
                            });
                            final msgs = ref
                                .read(messagesProvider(conversation.id))
                                .asData
                                ?.value;
                            if (msgs != null && msgs.isNotEmpty) {
                              _scrollToMatch(
                                  matched[_searchMatchIndex], msgs.length);
                            }
                          },
                  ),
                ],
              );
            }),
            const SizedBox(width: 4),
          ],
          // Hidden on email: the inbox view has no highlight/scroll wiring,
          // so the box would count matches and visibly do nothing — a
          // control that lies is worse than one that's absent.
          if (conversation.source != ConversationSource.email)
            IconButton(
              icon: Icon(
                _isSearching ? Icons.close : Icons.search,
                color: colors.textSecondary,
                size: 20,
              ),
              onPressed: () {
                setState(() {
                  _isSearching = !_isSearching;
                  _searchMatchIndex = 0;
                  if (!_isSearching) {
                    _searchController.clear();
                    ref.read(messageSearchProvider.notifier).clear();
                  } else {
                    Future.microtask(() => _searchFocusNode.requestFocus());
                  }
                });
              },
              splashRadius: 20,
            ),
          // Only while the bot is actually paused: with it active this menu
          // item was a silent no-op, and it competed with the TakeoverBar's
          // own "Devolver ao bot" without the bar's ownership rules.
          if (ref.watch(selectedConversationProvider) != null &&
              conversation.isBotPaused)
            PopupMenuButton<String>(
              icon:
                  Icon(Icons.more_vert, color: colors.textSecondary, size: 20),
              onSelected: (value) async {
                if (value == 'resume_bot') {
                  final id = ref.read(selectedConversationProvider);
                  if (id == null) return;
                  // Taking the thread from ANOTHER attendant deserves a
                  // pause: they may be mid-reply.
                  final other = conversation.assignedTo != null &&
                      conversation.assignedTo !=
                          ref.read(authControllerProvider).asData?.value?.id;
                  if (other) {
                    final ok = await showConfirmDialog(
                      context,
                      title: t.conversation.resume_bot,
                      message: t.conversation.resume_bot_other_confirm(
                          name: conversation.assignedToName ?? ''),
                      confirmText: t.conversation.resume_bot,
                    );
                    if (!ok) return;
                  }
                  await ref
                      .read(conversationActionsProvider.notifier)
                      .resumeBot(id);
                }
              },
              itemBuilder: (context) => [
                PopupMenuItem(
                  value: 'resume_bot',
                  child: Row(
                    children: [
                      const Icon(Icons.smart_toy, size: 18),
                      const SizedBox(width: 8),
                      Text(t.conversation.resume_bot),
                    ],
                  ),
                ),
              ],
            ),
        ],
      ),
    );
  }

  Widget _buildCannedOverlay(AppColorsExtension colors, String conversationId) {
    final cannedAsync = ref.watch(cannedResponseControllerProvider);
    final responses = cannedAsync.asData?.value ?? [];
    final filtered = _cannedFilter.isEmpty
        ? responses
        : responses
            .where((r) =>
                r.shortcut.toLowerCase().contains(_cannedFilter) ||
                r.title.toLowerCase().contains(_cannedFilter))
            .toList();

    if (filtered.isEmpty) return const SizedBox.shrink();

    // Bumped the cap from 200 → 280 so the typical 4-5 responses are
    // fully visible without the user needing to realize the overlay
    // scrolls. For accounts with 20+ responses the Scrollbar makes the
    // affordance obvious — the old layout clipped silently past item 4.
    return Container(
      constraints: const BoxConstraints(maxHeight: 280),
      margin: const EdgeInsets.symmetric(horizontal: 16),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderMedium),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.3),
            blurRadius: 8,
            offset: const Offset(0, -2),
          ),
        ],
      ),
      child: Scrollbar(
        thumbVisibility: filtered.length > 5,
        child: ListView.builder(
          padding: const EdgeInsets.symmetric(vertical: 4),
          itemCount: filtered.length,
          itemBuilder: (context, index) {
            final r = filtered[index];
            return InkWell(
              onTap: () {
                _messageController.text = r.content;
                _messageController.selection = TextSelection.collapsed(
                  offset: r.content.length,
                );
                setState(() {
                  _showCannedOverlay = false;
                  _cannedFilter = '';
                });
              },
              child: Padding(
                padding:
                    const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                child: Row(
                  children: [
                    Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 6, vertical: 2),
                      decoration: BoxDecoration(
                        color: colors.bluePrimary.withValues(alpha: 0.15),
                        borderRadius: BorderRadius.circular(4),
                      ),
                      child: Text(
                        '/${r.shortcut}',
                        style: TextStyle(
                          color: colors.bluePrimary,
                          fontSize: 11,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          // Highlight the matching substring in the title
                          // so the user can quickly tell WHY a row is
                          // showing up — especially useful when they typed
                          // "sup" and the chip pill shortcut is "/suporte"
                          // but the title is "Política de suporte".
                          _highlightMatch(
                            r.title,
                            _cannedFilter,
                            baseStyle: TextStyle(
                              color: colors.textPrimary,
                              fontSize: 13,
                              fontWeight: FontWeight.w600,
                            ),
                            matchColor: colors.bluePrimary,
                          ),
                          Text(
                            r.content,
                            style: TextStyle(
                                color: colors.textMuted, fontSize: 11),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  Widget _buildInputArea(AppColorsExtension colors, String conversationId) {
    final actionsState = ref.watch(conversationActionsProvider);
    final isSending = actionsState.isLoading;
    final myUserId = ref.watch(authControllerProvider).asData?.value?.id;

    // 24h service window awareness. WhatsApp rejects free-form text
    // outside the 24h-since-last-inbound window — the only way back in
    // is a pre-approved template. Detect this locally by looking at
    // the most recent customer message; if it's > 24h old (or there
    // isn't one yet), show a banner in place of the composer.
    final conversationsAsync = ref.watch(conversationsProvider);
    final conv = conversationsAsync.maybeWhen(
      data: (list) => list.where((c) => c.id == conversationId).firstOrNull,
      orElse: () => null,
    );
    // EMAIL gets its own composer and none of the chat furniture. There is
    // no bot to take over from (the server never auto-replies on email —
    // B2B hospital sales, high ticket, the AI only drafts in the approval
    // queue), so the takeover bar, the AI-waiting banner and the reply
    // suggestions would all be noise. The composer is a mail composer:
    // subject line, tall body, an explicit "send email" button.
    if (conv?.source == ConversationSource.email) {
      return _buildEmailComposer(colors, conversationId, isSending);
    }
    // Takeover flags: the composer is gated on a human having assumed
    // the conversation so the bot and a human don't both reply.
    final isBotPaused = conv?.isBotPaused ?? false;
    final assignedToMe =
        isBotPaused && conv?.assignedTo != null && conv?.assignedTo == myUserId;
    final assignedToOther =
        isBotPaused && conv?.assignedTo != null && conv?.assignedTo != myUserId;
    final messagesAsync = ref.watch(messagesProvider(conversationId));
    final lastInbound = messagesAsync.maybeWhen(
      data: (msgs) => msgs
          .where((m) => m.senderType == SenderType.customer)
          .map((m) => m.createdAt)
          .fold<DateTime?>(null,
              (prev, cur) => prev == null || cur.isAfter(prev) ? cur : prev),
      orElse: () => null,
    );
    // Meta's 24h customer-service window applies to WhatsApp AND Instagram
    // — an old IG thread accepted free-form text that then died at the
    // adapter. And the window is only computed once the transcript has
    // LOADED: during loading `lastInbound` is null, which used to flash
    // the orange banner and lock the composer on every conversation open.
    final metaWindowed = conv?.source == ConversationSource.whatsapp ||
        conv?.source == ConversationSource.instagram;
    final windowClosed = metaWindowed &&
        messagesAsync.hasValue &&
        (lastInbound == null ||
            DateTime.now().difference(lastInbound) > const Duration(hours: 24));

    // Composer is disabled when:
    //   - bot still owns the conversation (must take over first), or
    //   - a send is in flight, or
    //   - WhatsApp 24h window expired (free-form text rejected by Meta;
    //     the only legal outbound is a template, surfaced inline below).
    final inputEnabled = isBotPaused && !isSending && !windowClosed;

    // Window closed AND nobody has assumed: typing is useless and taking
    // over unlocks nothing — the old stack said so three times (orange
    // banner + takeover bar + a dead composer whose hint repeated the bar).
    // One block with the two actions that actually work replaces all three.
    if (windowClosed && !isBotPaused) {
      return _buildWindowClosedFooter(colors, conv, conversationId, isSending);
    }

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (_showCannedOverlay) _buildCannedOverlay(colors, conversationId),
        if (windowClosed) _build24hBanner(colors, conv),
        // K1: the AI is waiting for the customer to finish typing. Sits right
        // above the composer because that is where an attendant looks when they
        // are deciding whether to answer themselves — which is exactly the
        // decision the banner exists to inform. Renders nothing when no reply is
        // pending.
        AiDebounceBanner(conversationId: conversationId),
        if (inputEnabled)
          ReplySuggestionsBar(
            conversationId: conversationId,
            onPick: (text) {
              _messageController.text = text;
              _messageController.selection =
                  TextSelection.fromPosition(TextPosition(offset: text.length));
              _messageFocusNode.requestFocus();
            },
          ),
        TakeoverBar(
          conversationId: conversationId,
          isBotPaused: isBotPaused,
          assignedToMe: assignedToMe,
          assignedToOther: assignedToOther,
          assignedToName: conv?.assignedToName,
          isSending: isSending,
        ),
        if (_recording)
          AudioRecorderBar(
            conversationId: conversationId,
            onClose: () => setState(() => _recording = false),
          )
        else
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
            decoration: BoxDecoration(
              color: colors.backgroundSecondary,
              border: Border(
                top: BorderSide(color: colors.borderLight, width: 0.5),
              ),
            ),
            child: Row(
              children: [
                // Attachment + mic: hidden on EMAIL, where the server-side
                // MIME builder only carries text — a "sent" file or voice
                // note would silently never reach the recipient. Reinstate
                // when real MIME attachments exist on the outbound path.
                if (conv?.source != ConversationSource.email) ...[
                  SizedBox(
                    width: 36,
                    height: 36,
                    child: IconButton(
                      icon: Icon(Icons.attach_file,
                          color: colors.textSecondary, size: 20),
                      onPressed: inputEnabled
                          ? () => _pickAndSendMedia(conversationId)
                          : null,
                      padding: EdgeInsets.zero,
                      splashRadius: 18,
                      tooltip: t.dashboard.send_file_tooltip,
                    ),
                  ),
                  SizedBox(
                    width: 36,
                    height: 36,
                    child: IconButton(
                      icon: Icon(Icons.mic_none,
                          color: colors.textSecondary, size: 20),
                      onPressed: inputEnabled
                          ? () => setState(() => _recording = true)
                          : null,
                      padding: EdgeInsets.zero,
                      splashRadius: 18,
                      tooltip: t.dashboard.record_audio_tooltip,
                    ),
                  ),
                ],
                // Insert-signature affordance (US-018): only when the
                // attendant has one configured in their profile. Appends
                // to the MANUAL composer — AI replies never get it.
                if ((ref
                            .watch(authControllerProvider)
                            .asData
                            ?.value
                            ?.signature ??
                        '')
                    .isNotEmpty)
                  SizedBox(
                    width: 36,
                    height: 36,
                    child: IconButton(
                      icon: Icon(Icons.history_edu_outlined,
                          color: colors.textSecondary, size: 20),
                      onPressed: inputEnabled ? _insertSignature : null,
                      padding: EdgeInsets.zero,
                      splashRadius: 18,
                      tooltip: t.dashboard.insert_signature_tooltip,
                    ),
                  ),
                const SizedBox(width: 4),
                Expanded(
                  // Composer grows up to 4 lines before an internal scroll
                  // kicks in. The old fixed-40px height hid everything past
                  // the first line — users were blindly typing into a
                  // clipping box. constraints preserve the pill shape when
                  // the field is empty.
                  child: Tooltip(
                    message: inputEnabled
                        ? ''
                        : t.conversation.input_disabled_tooltip,
                    child: Container(
                      constraints:
                          const BoxConstraints(minHeight: 40, maxHeight: 120),
                      decoration: BoxDecoration(
                        color: colors.glassLight,
                        borderRadius: BorderRadius.circular(20),
                        border: Border.all(color: colors.borderMedium),
                      ),
                      // Enter sends, Shift+Enter breaks the line. A
                      // multiline field never fires onSubmitted on
                      // web/desktop, so the day's most-used action
                      // required a mouse click.
                      child: CallbackShortcuts(
                        bindings: {
                          const SingleActivator(LogicalKeyboardKey.enter): () =>
                              _sendMessage(conversationId),
                        },
                        child: TextField(
                          controller: _messageController,
                          focusNode: _messageFocusNode,
                          enabled: inputEnabled,
                          minLines: 1,
                          maxLines: 4,
                          keyboardType: TextInputType.multiline,
                          textInputAction: TextInputAction.newline,
                          style: TextStyle(
                              fontSize: 14, color: colors.textPrimary),
                          onSubmitted: (_) => _sendMessage(conversationId),
                          decoration: InputDecoration(
                            hintText: inputEnabled
                                ? t.dashboard.type_a_message
                                : t.conversation.input_disabled_tooltip,
                            hintStyle: TextStyle(
                                color: colors.textMuted, fontSize: 14),
                            border: InputBorder.none,
                            contentPadding: const EdgeInsets.symmetric(
                              horizontal: 16,
                              vertical: 10,
                            ),
                            isDense: true,
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    gradient: LinearGradient(
                      colors: [colors.blueDark, colors.indigoDark],
                    ),
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: isSending
                      ? const Padding(
                          padding: EdgeInsets.all(10),
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        )
                      // The most-used button in the product. Live over an empty
                      // box it swallowed the click: `_sendMessage` returns on its
                      // first line and says nothing. A ValueListenable around
                      // just this one — rebuilding the whole panel on every
                      // keystroke would be expensive, and this is the screen
                      // people type on.
                      : ValueListenableBuilder<TextEditingValue>(
                          valueListenable: _messageController,
                          builder: (context, value, _) => IconButton(
                            icon: const Icon(Icons.send,
                                color: Colors.white, size: 18),
                            onPressed:
                                inputEnabled && value.text.trim().isNotEmpty
                                    ? () => _sendMessage(conversationId)
                                    : null,
                            padding: EdgeInsets.zero,
                          ),
                        ),
                ),
              ],
            ),
          ),
      ],
    );
  }

  /// The email reply composer — a mail client's, not a chat's.
  ///
  /// Subject prefilled as "Re: <newest inbound subject>", a body that starts
  /// three lines tall, and a labeled "send email" button: an email is a
  /// deliberate document, not a chat ping. Enter breaks the line;
  /// Ctrl/Cmd+Enter sends. Always enabled — email has no bot, so there is
  /// nothing to "assume" before writing.
  Widget _buildEmailComposer(
      AppColorsExtension colors, String conversationId, bool isSending) {
    final messagesAsync = ref.watch(messagesProvider(conversationId));
    if (_subjectSeededFor != conversationId && messagesAsync.hasValue) {
      _subjectSeededFor = conversationId;
      MessageModel? newestInbound;
      for (final m in messagesAsync.value!) {
        if (m.senderType == SenderType.customer) newestInbound = m;
      }
      var seed = '';
      final s = newestInbound?.metadata?['subject'];
      if (s is String && s.trim().isNotEmpty) {
        final trimmed = s.trim();
        seed = RegExp(r'^re:', caseSensitive: false).hasMatch(trimmed)
            ? trimmed
            : 'Re: $trimmed';
      }
      _subjectController.text = seed;
    }
    final enabled = !isSending;
    final hasSignature =
        (ref.watch(authControllerProvider).asData?.value?.signature ?? '')
            .isNotEmpty;

    // Folded: a single "Reply" chip. The full composer only earns its
    // ~180px while an email is actually being written.
    if (!_emailComposerOpen) {
      return Container(
        padding: const EdgeInsets.fromLTRB(16, 10, 16, 12),
        decoration: BoxDecoration(
          color: colors.backgroundSecondary,
          border: Border(
            top: BorderSide(color: colors.borderLight, width: 0.5),
          ),
        ),
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            key: const Key('email_composer_reply_chip'),
            borderRadius: BorderRadius.circular(22),
            onTap: () {
              setState(() => _emailComposerOpen = true);
              WidgetsBinding.instance.addPostFrameCallback((_) {
                if (mounted) _messageFocusNode.requestFocus();
              });
            },
            child: Container(
              height: 42,
              padding: const EdgeInsets.symmetric(horizontal: 16),
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(22),
                border: Border.all(color: colors.borderMedium),
                color: colors.glassLight,
              ),
              child: Row(
                children: [
                  Icon(Icons.reply, size: 16, color: colors.textSecondary),
                  const SizedBox(width: 8),
                  Text(
                    t.email_composer.reply,
                    style: TextStyle(color: colors.textSecondary, fontSize: 13),
                  ),
                ],
              ),
            ),
          ),
        ),
      );
    }

    final gradients = Theme.of(context).extension<AppGradientsExtension>()!;
    // The theme paints an outline on EVERY TextField via enabledBorder /
    // focusedBorder — InputBorder.none on `border` alone does not remove
    // them, which is exactly the double-box mess this composer had. Zero
    // all four states so the card's own surface is the only chrome.
    const noBorder = InputDecoration(
      border: InputBorder.none,
      enabledBorder: InputBorder.none,
      focusedBorder: InputBorder.none,
      disabledBorder: InputBorder.none,
      isDense: true,
    );

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (_showCannedOverlay) _buildCannedOverlay(colors, conversationId),
        // A floating card over the thread — no full-width strip, no top
        // border: the elevation and shadow are what separate "writing an
        // email" from "reading the thread".
        Padding(
          padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
          child: Container(
            padding: const EdgeInsets.fromLTRB(16, 2, 10, 12),
            decoration: BoxDecoration(
              color: colors.backgroundTertiary,
              borderRadius: BorderRadius.circular(14),
              border: Border.all(color: colors.borderLight),
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withValues(alpha: 0.5),
                  blurRadius: 32,
                  offset: const Offset(0, 8),
                ),
              ],
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        key: const Key('email_composer_subject'),
                        controller: _subjectController,
                        enabled: enabled,
                        maxLines: 1,
                        style: TextStyle(
                          fontSize: 13.5,
                          color: colors.textPrimary,
                          fontWeight: FontWeight.w600,
                        ),
                        decoration: noBorder.copyWith(
                          hintText: t.email_composer.subject_hint,
                          hintStyle:
                              TextStyle(color: colors.textMuted, fontSize: 13),
                          contentPadding:
                              const EdgeInsets.symmetric(vertical: 10),
                        ),
                      ),
                    ),
                    IconButton(
                      key: const Key('email_composer_collapse'),
                      icon: Icon(Icons.keyboard_arrow_down,
                          size: 20, color: colors.textMuted),
                      onPressed: () =>
                          setState(() => _emailComposerOpen = false),
                      padding: EdgeInsets.zero,
                      splashRadius: 16,
                      tooltip: t.common.close,
                    ),
                  ],
                ),
                Divider(height: 1, color: colors.borderLight),
                const SizedBox(height: 6),
                Padding(
                  padding: const EdgeInsets.only(right: 6),
                  child: CallbackShortcuts(
                    bindings: {
                      const SingleActivator(LogicalKeyboardKey.enter,
                          control: true): () => _sendEmail(conversationId),
                      const SingleActivator(LogicalKeyboardKey.enter,
                          meta: true): () => _sendEmail(conversationId),
                    },
                    child: TextField(
                      key: const Key('email_composer_body'),
                      controller: _messageController,
                      focusNode: _messageFocusNode,
                      enabled: enabled,
                      minLines: 3,
                      maxLines: 10,
                      keyboardType: TextInputType.multiline,
                      textInputAction: TextInputAction.newline,
                      style: TextStyle(
                          fontSize: 13.5,
                          color: colors.textPrimary,
                          height: 1.4),
                      decoration: noBorder.copyWith(
                        hintText: t.email_composer.body_hint,
                        hintStyle:
                            TextStyle(color: colors.textMuted, fontSize: 13),
                        contentPadding: const EdgeInsets.symmetric(vertical: 4),
                      ),
                    ),
                  ),
                ),
                const SizedBox(height: 8),
                Row(
                  children: [
                    // Send lives on the LEFT (mail-client convention) — which
                    // also keeps it clear of the floating onboarding launcher
                    // parked in the bottom-right corner of the dashboard.
                    ValueListenableBuilder<TextEditingValue>(
                      valueListenable: _messageController,
                      builder: (context, value, _) {
                        final canSend = enabled && value.text.trim().isNotEmpty;
                        final fg =
                            canSend ? Colors.white : colors.textSecondary;
                        return Material(
                          color: Colors.transparent,
                          borderRadius: BorderRadius.circular(999),
                          child: InkWell(
                            key: const Key('email_composer_send'),
                            borderRadius: BorderRadius.circular(999),
                            onTap: canSend
                                ? () => _sendEmail(conversationId)
                                : null,
                            child: Ink(
                              decoration: BoxDecoration(
                                gradient:
                                    canSend ? gradients.primaryButton : null,
                                color: canSend ? null : colors.borderMedium,
                                borderRadius: BorderRadius.circular(999),
                              ),
                              child: Padding(
                                padding: const EdgeInsets.symmetric(
                                    horizontal: 18, vertical: 9),
                                child: Row(
                                  mainAxisSize: MainAxisSize.min,
                                  children: [
                                    Text(
                                      t.email_composer.send,
                                      style: TextStyle(
                                        color: fg,
                                        fontSize: 13,
                                        fontWeight: FontWeight.w600,
                                      ),
                                    ),
                                    const SizedBox(width: 8),
                                    isSending
                                        ? const SizedBox(
                                            width: 13,
                                            height: 13,
                                            child: CircularProgressIndicator(
                                                strokeWidth: 2,
                                                color: Colors.white),
                                          )
                                        : Icon(Icons.send, size: 13, color: fg),
                                  ],
                                ),
                              ),
                            ),
                          ),
                        );
                      },
                    ),
                    const SizedBox(width: 12),
                    Text(
                      Theme.of(context).platform == TargetPlatform.macOS
                          ? '⌘↵'
                          : 'Ctrl+↵',
                      style:
                          TextStyle(color: colors.textDisabled, fontSize: 11),
                    ),
                    const Spacer(),
                    if (hasSignature)
                      IconButton(
                        icon: Icon(Icons.history_edu_outlined,
                            color: colors.textSecondary, size: 20),
                        onPressed: enabled ? _insertSignature : null,
                        padding: EdgeInsets.zero,
                        splashRadius: 18,
                        tooltip: t.dashboard.insert_signature_tooltip,
                      ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  void _sendEmail(String conversationId) {
    final text = _messageController.text.trim();
    if (text.isEmpty) return;
    _pendingText = text;
    ref.read(conversationActionsProvider.notifier).sendHumanMessage(
        conversationId, text,
        subject: _subjectController.text);
    _messageController.clear();
  }

  /// The whole footer while the 24h window is closed and nobody assumed the
  /// conversation: one explanation line and the two actions that actually
  /// work — a template (WhatsApp's only legal outbound) and taking over for
  /// when the customer answers. No dead composer.
  Widget _buildWindowClosedFooter(AppColorsExtension colors,
      ConversationModel? conv, String conversationId, bool isSending) {
    return Container(
      key: const Key('window_closed_footer'),
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 14),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        border: Border(
          top: BorderSide(color: colors.borderLight, width: 0.5),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.timer_off_outlined,
                  size: 16, color: colors.orangeAlert),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  t.dashboard.wa_24h_window_expired,
                  style: TextStyle(
                    color: colors.textSecondary,
                    fontSize: 12.5,
                    height: 1.4,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              // Template CTA is WhatsApp's escape hatch only — Instagram
              // shares the window but has no template mechanism.
              if (conv != null &&
                  conv.source == ConversationSource.whatsapp) ...[
                ElevatedButton.icon(
                  key: const Key('window_closed_template'),
                  onPressed: () => _openTemplateSendDialog(conv),
                  icon: const Icon(Icons.send, size: 14),
                  label: Text(t.dashboard.wa_send_template),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: colors.orangeAlert,
                    foregroundColor: colors.textInverted,
                    padding:
                        const EdgeInsets.symmetric(horizontal: 14, vertical: 9),
                    shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(8)),
                    textStyle: const TextStyle(
                        fontSize: 12.5, fontWeight: FontWeight.w700),
                  ),
                ),
                const SizedBox(width: 10),
              ],
              OutlinedButton(
                key: const Key('window_closed_assume'),
                onPressed: isSending
                    ? null
                    : () => ref
                        .read(conversationActionsProvider.notifier)
                        .assumeConversation(conversationId),
                style: OutlinedButton.styleFrom(
                  foregroundColor: colors.textSecondary,
                  side: BorderSide(color: colors.borderMedium),
                  padding:
                      const EdgeInsets.symmetric(horizontal: 14, vertical: 9),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(8)),
                  textStyle: const TextStyle(
                      fontSize: 12.5, fontWeight: FontWeight.w500),
                ),
                child: Text(t.conversation.assume),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  t.dashboard.wa_window_composer_back,
                  style: TextStyle(color: colors.textDisabled, fontSize: 11.5),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _build24hBanner(AppColorsExtension colors, ConversationModel? conv) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      color: colors.orangeAlert.withValues(alpha: 0.15),
      child: Row(
        children: [
          Icon(Icons.timer_off_outlined, size: 16, color: colors.orangeAlert),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              t.dashboard.wa_24h_window_expired,
              style: TextStyle(
                color: colors.orangeAlert,
                fontSize: 12,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
          // Template CTA is WhatsApp's escape hatch only — Instagram has
          // the same window but no template mechanism; offering the picker
          // there would send a WhatsApp payload to an IG thread.
          if (conv != null && conv.source == ConversationSource.whatsapp) ...[
            const SizedBox(width: 8),
            ElevatedButton.icon(
              onPressed: () => _openTemplateSendDialog(conv),
              icon: const Icon(Icons.send, size: 14),
              label: Text(t.dashboard.wa_send_template),
              style: ElevatedButton.styleFrom(
                backgroundColor: colors.orangeAlert,
                foregroundColor: colors.textInverted,
                padding:
                    const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8)),
                textStyle:
                    const TextStyle(fontSize: 12, fontWeight: FontWeight.w700),
              ),
            ),
          ],
        ],
      ),
    );
  }

  /// Pops the template-send dialog scoped to the current conversation.
  /// Pre-fills the recipient phone (E.164 minus the leading "+") so the
  /// attendant just picks an approved template and hits send.
  /// Fires the /summary endpoint and surfaces the result. We pop a
  /// modal (not a snackbar) so long summaries stay legible and the
  /// operator can copy without scrolling them off-screen.
  /// The conversation brief: agreed / pending / next step, from the cached
  /// server-side summary. Replaced the old one-sentence POST endpoint
  /// (2026-08-28) whose weak language hint answered a short thread in
  /// Spanish — the brief is pt-BR by contract, structured, and cached.
  Future<void> _showSummary(String conversationId) async {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    await showDialog<void>(
      context: context,
      builder: (ctx) => Consumer(
        builder: (ctx, ref, _) {
          final state = ref.watch(conversationBriefProvider(conversationId));
          return AlertDialog(
            backgroundColor: colors.backgroundSecondary,
            shape:
                RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
            title: Row(
              children: [
                Icon(Icons.auto_awesome, color: colors.bluePrimary, size: 18),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(t.dashboard.summary_title,
                      style:
                          TextStyle(color: colors.textPrimary, fontSize: 16)),
                ),
                if (state.asData?.value?.generatedAt != null)
                  Text(
                    _briefAge(state.asData!.value!.generatedAt!),
                    style: TextStyle(color: colors.textDisabled, fontSize: 11),
                  ),
              ],
            ),
            content: SizedBox(
              width: 460,
              child: state.when(
                loading: () => const SizedBox(
                  height: 80,
                  child: Center(child: CircularProgressIndicator()),
                ),
                error: (e, _) => Text(
                    t.dashboard.summary_failed(error: parseApiError(e)),
                    style: TextStyle(color: colors.redAlert, fontSize: 13)),
                data: (brief) {
                  if (brief == null) {
                    return Text(
                      t.dashboard.summary_too_short,
                      key: const Key('brief_too_short'),
                      style: TextStyle(color: colors.textMuted, fontSize: 13),
                    );
                  }
                  if (!brief.isStructured) {
                    // Pre-migration rows: flowing text, still useful.
                    return Text(
                      brief.text.isEmpty
                          ? t.dashboard.summary_empty
                          : brief.text,
                      style: TextStyle(
                          color: colors.textPrimary, fontSize: 14, height: 1.5),
                    );
                  }
                  return Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (brief.agreed.isNotEmpty)
                        _briefRow(
                          colors,
                          key: const Key('brief_agreed'),
                          icon: Icons.check,
                          color: colors.whatsappGreen,
                          label: t.dashboard.brief_agreed,
                          text: brief.agreed,
                        ),
                      if (brief.pending.isNotEmpty)
                        _briefRow(
                          colors,
                          key: const Key('brief_pending'),
                          icon: Icons.schedule,
                          color: colors.orangeAlert,
                          label: t.dashboard.brief_pending,
                          text: brief.pending,
                        ),
                      if (brief.nextStep.isNotEmpty)
                        _briefRow(
                          colors,
                          key: const Key('brief_next_step'),
                          icon: Icons.arrow_forward,
                          color: colors.bluePrimary,
                          label: t.dashboard.brief_next_step,
                          text: brief.nextStep,
                        ),
                    ],
                  );
                },
              ),
            ),
            actions: [
              TextButton.icon(
                key: const Key('brief_copy'),
                onPressed: state.asData?.value == null
                    ? null
                    : () {
                        Clipboard.setData(
                            ClipboardData(text: state.asData!.value!.text));
                        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                          content: Text(t.dashboard.brief_copied),
                          behavior: SnackBarBehavior.floating,
                          duration: const Duration(milliseconds: 1600),
                        ));
                      },
                icon: const Icon(Icons.copy_outlined, size: 14),
                label: Text(t.common.copy),
              ),
              TextButton.icon(
                key: const Key('brief_refresh'),
                onPressed: () => ref
                    .read(conversationBriefProvider(conversationId).notifier)
                    .refresh(),
                icon: const Icon(Icons.refresh, size: 14),
                label: Text(t.dashboard.summary_refresh),
              ),
              ElevatedButton(
                onPressed: () => Navigator.pop(ctx),
                style: ElevatedButton.styleFrom(
                    backgroundColor: colors.bluePrimary,
                    foregroundColor: Colors.white),
                child: Text(t.common.close),
              ),
            ],
          );
        },
      ),
    );
  }

  Widget _briefRow(
    AppColorsExtension colors, {
    required Key key,
    required IconData icon,
    required Color color,
    required String label,
    required String text,
  }) {
    return Padding(
      key: key,
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 26,
            height: 26,
            decoration: BoxDecoration(
              color: color.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Icon(icon, size: 14, color: color),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  label,
                  style: TextStyle(
                    color: colors.textMuted,
                    fontSize: 10.5,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 0.4,
                  ),
                ),
                const SizedBox(height: 2),
                SelectableText(
                  text,
                  style: TextStyle(
                      color: colors.textSecondary, fontSize: 13, height: 1.45),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  String _briefAge(DateTime generatedAt) {
    final d = DateTime.now().difference(generatedAt);
    // A zero/absurd timestamp (server bug, clock skew) must never render
    // as "há 739855d" — an age that cannot be true reads as "just now".
    if (d.isNegative || generatedAt.year < 2020) {
      return t.dashboard.brief_generated_now;
    }
    if (d.inMinutes < 2) return t.dashboard.brief_generated_now;
    if (d.inHours < 1) {
      return t.dashboard.brief_generated_ago(when: '${d.inMinutes}min');
    }
    if (d.inDays < 1) {
      return t.dashboard.brief_generated_ago(when: '${d.inHours}h');
    }
    return t.dashboard.brief_generated_ago(when: '${d.inDays}d');
  }

  void _openTemplateSendDialog(ConversationModel conv) {
    showDialog<void>(
      context: context,
      builder: (_) => WaSendTemplateDialog(
        channelId: conv.channelId,
        recipientPhone: conv.contactExternalId,
        contactName: conv.contactName,
      ),
    );
  }

  /// Appends the attendant's personal signature (US-018) to the manual
  /// composer text. Never auto-sends and never touches AI replies —
  /// the signature only exists client-side until the user hits send.
  void _insertSignature() {
    final signature = ref.read(authControllerProvider).asData?.value?.signature;
    if (signature == null || signature.isEmpty) return;
    final existing = _messageController.text;
    // Don't stack duplicates on repeated taps.
    if (existing.trimRight().endsWith(signature)) return;
    _messageController.text =
        existing.trimRight().isEmpty ? signature : '$existing\n\n$signature';
    _messageController.selection =
        TextSelection.collapsed(offset: _messageController.text.length);
    _messageFocusNode.requestFocus();
  }

  void _sendMessage(String conversationId) {
    final text = _messageController.text.trim();
    if (text.isEmpty) return;

    // Stash the text BEFORE we clear the input so the error listener
    // above can put it back on failure.
    _pendingText = text;
    // Under takeover we go through the dedicated send-message endpoint
    // (which enforces CONV_NOT_ASSUMED server-side). The input gate
    // above already blocks sends when the bot isn't paused, but this
    // keeps the bot/human path cleanly separated server-side too.
    ref
        .read(conversationActionsProvider.notifier)
        .sendHumanMessage(conversationId, text);
    _messageController.clear();
  }

  Future<void> _pickAndSendMedia(String conversationId) async {
    final result = await FilePicker.platform.pickFiles(
      withData: true,
      type: FileType.any,
    );
    if (result == null || result.files.isEmpty) return;

    final file = result.files.first;
    final bytes = file.bytes;
    if (bytes == null) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(t.conversation.file_read_error)),
        );
      }
      return;
    }
    if (!mounted) return;
    await sendMediaToConversation(
      ref: ref,
      context: context,
      conversationId: conversationId,
      bytes: bytes,
      filename: file.name,
    );
  }

  Widget _buildDropOverlay(AppColorsExtension colors) {
    return IgnorePointer(
      child: Container(
        color: colors.bluePrimary.withValues(alpha: 0.18),
        child: Center(
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 20),
            decoration: BoxDecoration(
              color: colors.backgroundSecondary,
              borderRadius: BorderRadius.circular(16),
              border: Border.all(
                color: colors.bluePrimary,
                width: 2,
                style: BorderStyle.solid,
              ),
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.file_upload_outlined,
                    size: 48, color: colors.bluePrimary),
                const SizedBox(height: 8),
                Text(
                  t.dashboard.drop_to_send,
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                    color: colors.textPrimary,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _handleDroppedFiles(
      String conversationId, DropDoneDetails detail) async {
    if (detail.files.isEmpty) return;
    final file = detail.files.first;
    final bytes = await file.readAsBytes();
    if (!mounted) return;
    await sendMediaToConversation(
      ref: ref,
      context: context,
      conversationId: conversationId,
      bytes: bytes,
      filename: file.name,
    );
  }

  Widget _buildEmptyState(AppColorsExtension colors) {
    return Container(
      color: colors.backgroundPrimary,
      child: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              width: 80,
              height: 80,
              decoration: BoxDecoration(
                color: colors.glassMedium,
                borderRadius: BorderRadius.circular(40),
              ),
              child: Icon(
                Icons.forum_outlined,
                size: 36,
                color: colors.textDisabled,
              ),
            ),
            const SizedBox(height: 20),
            Text(
              t.dashboard.select_conversation_title,
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.w600,
                color: colors.textSecondary,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              t.dashboard.select_conversation_subtitle,
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 14,
                color: colors.textMuted,
                height: 1.5,
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _statusLabel(ConversationStatus status, bool isBotPaused) {
    if (status == ConversationStatus.closed) return t.dashboard.status_closed;
    // A human who takes over only flips is_bot_paused — status stays
    // "open" on the server. Checking isBotPaused first keeps the
    // header honest when the takeover bar is visible.
    if (isBotPaused) return t.dashboard.human_handling;
    return switch (status) {
      ConversationStatus.open => t.dashboard.status_open_bot,
      ConversationStatus.pendingHuman => t.dashboard.status_pending_human,
      ConversationStatus.closed => t.dashboard.status_closed,
    };
  }

  Color _statusColor(
      ConversationStatus status, bool isBotPaused, AppColorsExtension colors) {
    if (status == ConversationStatus.closed) return colors.textMuted;
    if (isBotPaused) return colors.orangeAlert;
    return switch (status) {
      ConversationStatus.open => colors.whatsappGreen,
      ConversationStatus.pendingHuman => colors.orangeAlert,
      ConversationStatus.closed => colors.textMuted,
    };
  }

  Color _avatarColor(String name) {
    final avatarColors = [
      const Color(0xFF6366F1),
      const Color(0xFF8B5CF6),
      const Color(0xFFEC4899),
      const Color(0xFF14B8A6),
      const Color(0xFFF59E0B),
      const Color(0xFF3B82F6),
      const Color(0xFF10B981),
      const Color(0xFFEF4444),
    ];
    final hash = name.codeUnits.fold<int>(0, (prev, c) => prev + c);
    return avatarColors[hash % avatarColors.length];
  }
}

/// Animated dot for the typing indicator.
class _TypingDot extends StatefulWidget {
  final AppColorsExtension colors;
  final int delay;

  const _TypingDot({required this.colors, required this.delay});

  @override
  State<_TypingDot> createState() => _TypingDotState();
}

class _TypingDotState extends State<_TypingDot>
    with SingleTickerProviderStateMixin {
  late AnimationController _controller;
  late Animation<double> _animation;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 600),
    );
    _animation = Tween<double>(begin: 0, end: -6).animate(
      CurvedAnimation(parent: _controller, curve: Curves.easeInOut),
    );
    Future.delayed(Duration(milliseconds: widget.delay), () {
      if (mounted) _controller.repeat(reverse: true);
    });
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _animation,
      builder: (_, __) => Transform.translate(
        offset: Offset(0, _animation.value),
        child: Container(
          width: 7,
          height: 7,
          decoration: BoxDecoration(
            color: widget.colors.cyanGlow.withValues(alpha: 0.8),
            shape: BoxShape.circle,
          ),
        ),
      ),
    );
  }
}

/// Small avatar used in the conversation header. Renders the remote
/// profile picture when available (Telegram) and falls back to the
/// first letter of the contact's name otherwise.
class _HeaderAvatar extends StatelessWidget {
  final String name;
  final String? avatarUrl;
  final Color color;

  const _HeaderAvatar({
    required this.name,
    required this.avatarUrl,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    final initialAvatar = CircleAvatar(
      radius: 18,
      backgroundColor: color,
      child: Text(
        name.isNotEmpty ? name[0].toUpperCase() : '?',
        style: const TextStyle(
          color: Colors.white,
          fontSize: 14,
          fontWeight: FontWeight.w700,
        ),
      ),
    );

    final hasPhoto = avatarUrl != null && avatarUrl!.isNotEmpty;
    if (!hasPhoto) return initialAvatar;

    return AppNetworkImage(
      url: avatarUrl,
      width: 36,
      height: 36,
      renderWidth: 36,
      borderRadius: BorderRadius.circular(18),
      fallback: initialAvatar,
    );
  }
}

/// "Carregar mensagens anteriores" — the top row of a transcript whose
/// newest page is full. Sits inside the reversed ListView so it scrolls
/// with the history it extends.
class _LoadOlderRow extends StatelessWidget {
  const _LoadOlderRow({required this.loading, required this.onPressed});

  final bool loading;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 8),
        child: loading
            ? const SizedBox(
                width: 18,
                height: 18,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            : TextButton.icon(
                key: const Key('load_older_messages'),
                onPressed: onPressed,
                icon: Icon(Icons.history, size: 16, color: colors.textMuted),
                label: Text(
                  t.conversation.load_older_messages,
                  style: TextStyle(color: colors.textMuted, fontSize: 12),
                ),
              ),
      ),
    );
  }
}
