import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/dashboard/domain/models/note_model.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/i18n/strings.g.dart';

part 'notes_panel.g.dart';

@riverpod
Future<List<NoteModel>> conversationNotes(
    Ref ref, String conversationId) async {
  final dio = ref.watch(dioProvider);
  final response =
      await dio.get<List<dynamic>>('/conversations/$conversationId/notes');
  return (response.data ?? [])
      .map((e) => NoteModel.fromJson(e as Map<String, dynamic>))
      .toList();
}

class NotesPanel extends ConsumerStatefulWidget {
  final String conversationId;

  const NotesPanel({super.key, required this.conversationId});

  @override
  ConsumerState<NotesPanel> createState() => _NotesPanelState();
}

class _NotesPanelState extends ConsumerState<NotesPanel> {
  bool _isExpanded = false;
  final _noteController = TextEditingController();
  bool _isSending = false;

  @override
  void dispose() {
    _noteController.dispose();
    super.dispose();
  }

  /// Intercepts ESC on the note TextField so the dashboard's global
  /// shortcut can clear the selected conversation. Web TextFields don't
  /// reliably bubble ESC when focused, so we handle it locally: unfocus
  /// to let any subsequent keys reach the root Shortcuts, then run the
  /// same action directly.
  KeyEventResult _handleKey(FocusNode node, KeyEvent event) {
    if (event is KeyDownEvent &&
        event.logicalKey == LogicalKeyboardKey.escape) {
      // ESC with a half-typed note first CLEARS the note; only an empty
      // field closes the conversation. Closing immediately threw away the
      // text the person was in the middle of writing.
      if (_noteController.text.isNotEmpty) {
        _noteController.clear();
        return KeyEventResult.handled;
      }
      FocusScope.of(context).unfocus();
      ref.read(selectedConversationProvider.notifier).select(null);
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  Future<void> _addNote() async {
    final content = _noteController.text.trim();
    if (content.isEmpty) return;

    FocusScope.of(context).unfocus();
    setState(() => _isSending = true);
    try {
      final dio = ref.read(dioProvider);
      await dio.post<dynamic>('/conversations/${widget.conversationId}/notes',
          data: {'content': content});
      _noteController.clear();
      ref.invalidate(conversationNotesProvider(widget.conversationId));
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(t.notes.add_error)),
        );
      }
    } finally {
      if (mounted) setState(() => _isSending = false);
    }
  }

  Future<void> _deleteNote(String noteId) async {
    try {
      final dio = ref.read(dioProvider);
      await dio.delete<dynamic>(
          '/conversations/${widget.conversationId}/notes/$noteId');
      ref.invalidate(conversationNotesProvider(widget.conversationId));
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(t.notes.delete_error)),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final notesAsync =
        ref.watch(conversationNotesProvider(widget.conversationId));

    return Container(
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        border:
            Border(bottom: BorderSide(color: colors.borderLight, width: 0.5)),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Toggle header
          InkWell(
            onTap: () => setState(() => _isExpanded = !_isExpanded),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              child: Row(
                children: [
                  Icon(Icons.sticky_note_2_outlined,
                      size: 16, color: colors.textMuted),
                  const SizedBox(width: 6),
                  Text(
                    t.notes.internal_notes,
                    style: TextStyle(
                        color: colors.textMuted,
                        fontSize: 12,
                        fontWeight: FontWeight.w600),
                  ),
                  const Spacer(),
                  notesAsync.whenOrNull(
                        data: (notes) => notes.isNotEmpty
                            ? Container(
                                padding: const EdgeInsets.symmetric(
                                    horizontal: 6, vertical: 1),
                                decoration: BoxDecoration(
                                  color: colors.bluePrimary
                                      .withValues(alpha: 0.15),
                                  borderRadius: BorderRadius.circular(8),
                                ),
                                child: Text(
                                  '${notes.length}',
                                  style: TextStyle(
                                      color: colors.bluePrimary,
                                      fontSize: 11,
                                      fontWeight: FontWeight.w700),
                                ),
                              )
                            : null,
                      ) ??
                      const SizedBox.shrink(),
                  const SizedBox(width: 4),
                  Icon(
                    _isExpanded ? Icons.expand_less : Icons.expand_more,
                    size: 18,
                    color: colors.textMuted,
                  ),
                ],
              ),
            ),
          ),
          // Expandable content
          if (_isExpanded) ...[
            // Notes list
            notesAsync.when(
              data: (notes) {
                if (notes.isEmpty) {
                  return Padding(
                    padding: const EdgeInsets.all(12),
                    child: Text(t.notes.no_notes,
                        style: TextStyle(
                            color: colors.textDisabled, fontSize: 12)),
                  );
                }
                return ConstrainedBox(
                  constraints: const BoxConstraints(maxHeight: 200),
                  child: ListView.builder(
                    shrinkWrap: true,
                    padding: const EdgeInsets.symmetric(horizontal: 12),
                    itemCount: notes.length,
                    itemBuilder: (ctx, i) {
                      final note = notes[i];
                      // Internal-note styling: amber tint + left
                      // accent bar + sticky-note icon on the header.
                      // Previously notes used the same glassLight
                      // background as messages, so scanning a thread it
                      // was easy to mistake a supervisor's internal
                      // comment for a customer reply. The accent makes
                      // the distinction readable in <1 second.
                      return Container(
                        margin: const EdgeInsets.only(bottom: 6),
                        padding: const EdgeInsets.fromLTRB(10, 8, 10, 10),
                        decoration: BoxDecoration(
                          color: colors.orangeAlert.withValues(alpha: 0.08),
                          borderRadius: BorderRadius.circular(8),
                          border: Border(
                            left:
                                BorderSide(color: colors.orangeAlert, width: 3),
                            top: BorderSide(
                                color:
                                    colors.orangeAlert.withValues(alpha: 0.2)),
                            right: BorderSide(
                                color:
                                    colors.orangeAlert.withValues(alpha: 0.2)),
                            bottom: BorderSide(
                                color:
                                    colors.orangeAlert.withValues(alpha: 0.2)),
                          ),
                        ),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Row(
                              children: [
                                Icon(Icons.sticky_note_2_outlined,
                                    size: 12, color: colors.orangeAlert),
                                const SizedBox(width: 4),
                                Text(t.notes.internal_note,
                                    style: TextStyle(
                                      color: colors.orangeAlert,
                                      fontSize: 10,
                                      fontWeight: FontWeight.w700,
                                      letterSpacing: 0.3,
                                    )),
                                const SizedBox(width: 8),
                                Text(note.userName,
                                    style: TextStyle(
                                        color: colors.textSecondary,
                                        fontSize: 11,
                                        fontWeight: FontWeight.w600)),
                                const Spacer(),
                                Text(_formatTime(note.createdAt),
                                    style: TextStyle(
                                        color: colors.textDisabled,
                                        fontSize: 10)),
                                const SizedBox(width: 4),
                                InkWell(
                                  onTap: () async {
                                    // Deleting is the one destructive act on
                                    // this panel and had NO confirmation —
                                    // resolve, far less destructive, has one.
                                    final ok = await showConfirmDialog(
                                      context,
                                      title: t.notes.delete_confirm_title,
                                      message: t.notes.delete_confirm_body,
                                      confirmText: t.common.delete,
                                    );
                                    if (ok) _deleteNote(note.id);
                                  },
                                  child: Icon(Icons.close,
                                      size: 14, color: colors.textDisabled),
                                ),
                              ],
                            ),
                            const SizedBox(height: 4),
                            Text(note.content,
                                style: TextStyle(
                                    color: colors.textPrimary, fontSize: 13)),
                          ],
                        ),
                      );
                    },
                  ),
                );
              },
              loading: () => const Padding(
                padding: EdgeInsets.all(12),
                child: SizedBox(
                    height: 20,
                    width: 20,
                    child: CircularProgressIndicator(strokeWidth: 2)),
              ),
              error: (_, __) => Padding(
                padding: const EdgeInsets.all(12),
                child: Text(t.notes.load_error,
                    style: TextStyle(color: colors.redAlert, fontSize: 12)),
              ),
            ),
            // Input
            Padding(
              padding: const EdgeInsets.fromLTRB(12, 0, 12, 8),
              child: Row(
                children: [
                  Expanded(
                    child: Container(
                      height: 34,
                      decoration: BoxDecoration(
                        color: colors.glassLight,
                        borderRadius: BorderRadius.circular(8),
                        border: Border.all(color: colors.borderMedium),
                      ),
                      child: Focus(
                        onKeyEvent: _handleKey,
                        child: TextField(
                          controller: _noteController,
                          // The button follows what is typed.
                          onChanged: (_) => setState(() {}),
                          enabled: !_isSending,
                          style: TextStyle(
                              fontSize: 13, color: colors.textPrimary),
                          onSubmitted: (_) => _addNote(),
                          decoration: InputDecoration(
                            hintText: t.notes.add_hint,
                            hintStyle: TextStyle(
                                color: colors.textMuted, fontSize: 13),
                            border: InputBorder.none,
                            contentPadding: const EdgeInsets.symmetric(
                                horizontal: 10, vertical: 7),
                            isDense: true,
                          ),
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  SizedBox(
                    width: 34,
                    height: 34,
                    child: _isSending
                        ? const Padding(
                            padding: EdgeInsets.all(8),
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : IconButton(
                            // An empty note is not a note: `_addNote` returns
                            // without writing anything.
                            onPressed: _noteController.text.trim().isEmpty
                                ? null
                                : _addNote,
                            icon: Icon(Icons.send,
                                size: 16, color: colors.bluePrimary),
                            padding: EdgeInsets.zero,
                          ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }

  String _formatTime(DateTime dt) {
    final now = DateTime.now();
    final diff = now.difference(dt);
    if (diff.inMinutes < 60) return '${diff.inMinutes}min';
    if (diff.inHours < 24) return '${diff.inHours}h';
    return '${dt.day}/${dt.month}';
  }
}
