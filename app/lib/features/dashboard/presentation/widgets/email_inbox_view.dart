import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/dashboard/domain/models/message_model.dart';
import 'package:app/features/dashboard/domain/models/sender_authenticity.dart';
import 'package:app/features/dashboard/presentation/controllers/attachment_controller.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/dashboard/presentation/widgets/attachment_list.dart';
import 'package:app/features/dashboard/presentation/widgets/email_original_frame_stub.dart'
    if (dart.library.html) 'package:app/features/dashboard/presentation/widgets/email_original_frame_web.dart'
    as original_frame;
import 'package:app/features/dashboard/presentation/widgets/email_risk_banner.dart';
import 'package:app/features/dashboard/presentation/widgets/view_original_button.dart';
import 'package:app/i18n/strings.g.dart';

/// An email conversation as a TRADITIONAL INBOX, grouped by contact.
///
/// Two modes, like every mail client: the LIST (one row per email — subject,
/// sender, preview, date, attachment clip, anti-forgery verdict) and the
/// READER, which a click opens over the whole panel. The accordion this
/// replaces expanded rows in place — with a fixed-height iframe inside a
/// scrolling list it trapped the scroll wheel and read as neither a list nor
/// a message. The reader gives the formatted original the full panel height
/// instead.
///
/// Everything around it stays Replider: the composer replies into the same
/// thread, notes/tags/takeover are untouched.
///
/// The PANEL must mount this with `key: ValueKey(conversationId)`: the open
/// message and its frame are per-thread state.
class EmailInboxView extends ConsumerStatefulWidget {
  const EmailInboxView({
    super.key,
    required this.messages,
    required this.scrollController,
  });

  /// Chronological, oldest first (the order the panel already holds).
  final List<MessageModel> messages;
  final ScrollController scrollController;

  @override
  ConsumerState<EmailInboxView> createState() => _EmailInboxViewState();
}

class _EmailInboxViewState extends ConsumerState<EmailInboxView> {
  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // Newest first — an inbox, not a transcript.
    final ordered = widget.messages.reversed.toList();

    if (ordered.isEmpty) {
      return Center(
        child: Text(
          t.email_inbox.empty,
          key: const Key('email_inbox_empty'),
          style: TextStyle(color: colors.textMuted, fontSize: 13),
        ),
      );
    }

    // The open message lives in a provider (not local state) so the panel
    // above can hide the contact header + notes bar while a mail is read.
    final openId = ref.watch(emailReaderOpenProvider);
    MessageModel? open;
    if (openId != null) {
      for (final m in ordered) {
        if (m.id == openId) {
          open = m;
          break;
        }
      }
    }
    if (open != null) {
      return _EmailReader(
        key: ValueKey('reader_${open.id}'),
        message: open,
        onBack: () => ref.read(emailReaderOpenProvider.notifier).close(),
      );
    }

    return ListView.separated(
      key: const Key('email_inbox_view'),
      controller: widget.scrollController,
      padding: const EdgeInsets.symmetric(vertical: 8),
      itemCount: ordered.length,
      separatorBuilder: (_, __) =>
          Divider(height: 1, color: colors.borderMedium),
      itemBuilder: (context, index) {
        final msg = ordered[index];
        return _EmailRow(
          key: ValueKey(msg.id),
          message: msg,
          onOpen: () => ref.read(emailReaderOpenProvider.notifier).open(msg.id),
        );
      },
    );
  }
}

/// The subject an email leads with, or the explicit "(sem assunto)".
String emailSubjectOf(MessageModel m) {
  final s = m.metadata?['subject'];
  if (s is String && s.trim().isNotEmpty) return s.trim();
  return t.email_inbox.no_subject;
}

/// Who wrote it: the sender label for inbound, "Você" for our own replies.
String emailIdentityOf(MessageModel m) =>
    m.senderType == SenderType.customer ? m.senderLabel : t.email_inbox.you;

/// One row of the inbox list.
class _EmailRow extends StatelessWidget {
  const _EmailRow({super.key, required this.message, required this.onOpen});

  final MessageModel message;
  final VoidCallback onOpen;

  bool get _isInbound => message.senderType == SenderType.customer;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final verdict = _isInbound
        ? SenderAuthenticity.fromMetadata(message.metadata)
        : SenderAuthenticity.verified;
    final risky = message.risk != null;
    final preview = message.content.trim().split('\n').first.trim();

    return InkWell(
      key: Key('email_row_${message.id}'),
      onTap: onOpen,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.only(top: 2),
              child: Icon(
                _isInbound ? Icons.call_received : Icons.call_made,
                size: 15,
                color: _isInbound ? colors.cyanGlow : colors.textMuted,
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      // A scam suspicion must be visible ON the row — a red
                      // flag only inside the opened mail warns after the fact.
                      if (risky) ...[
                        Icon(Icons.gpp_maybe,
                            key: Key('email_risk_mark_${message.id}'),
                            size: 14,
                            color: colors.redAlert),
                        const SizedBox(width: 4),
                      ],
                      Expanded(
                        child: Text(
                          emailSubjectOf(message),
                          key: Key('email_subject_${message.id}'),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            color: colors.textPrimary,
                            fontSize: 13.5,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                      if (message.attachments.any((a) => !a.isInline)) ...[
                        const SizedBox(width: 6),
                        Icon(Icons.attach_file,
                            size: 13, color: colors.textMuted),
                      ],
                      const SizedBox(width: 8),
                      Text(
                        DateFormat('dd/MM HH:mm').format(message.createdAt),
                        style: TextStyle(color: colors.textMuted, fontSize: 11),
                      ),
                    ],
                  ),
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Row(
                      children: [
                        Flexible(
                          child: Text(
                            emailIdentityOf(message),
                            key: Key('email_identity_${message.id}'),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                                color: colors.textMuted, fontSize: 11),
                          ),
                        ),
                        if (preview.isNotEmpty) ...[
                          Text(' · ',
                              style: TextStyle(
                                  color: colors.textMuted, fontSize: 11)),
                          Expanded(
                            child: Text(
                              preview,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                  color: colors.textSecondary, fontSize: 12),
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                  if (verdict.isWorthShowing)
                    SenderAuthBadgeInline(
                      verdict: verdict,
                      detail: SenderAuthDetail.fromMetadata(message.metadata),
                    ),
                  // A reply that never left is worse than no reply.
                  if (message.sendFailed)
                    Padding(
                      padding: const EdgeInsets.only(top: 2),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(Icons.error_outline,
                              key: Key('email_send_failed_${message.id}'),
                              size: 12,
                              color: colors.redAlert),
                          const SizedBox(width: 3),
                          Flexible(
                            child: Text(
                              message.failedReason?.isNotEmpty == true
                                  ? '${t.lead_timeline.not_delivered} — ${message.failedReason}'
                                  : t.lead_timeline.not_delivered,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                  color: colors.redAlert, fontSize: 11),
                            ),
                          ),
                        ],
                      ),
                    ),
                ],
              ),
            ),
            Icon(Icons.chevron_right, size: 16, color: colors.textMuted),
          ],
        ),
      ),
    );
  }
}

/// One open email, taking the whole panel — a mail reader, not a bubble.
///
/// The formatted original gets Expanded height (no fixed-height box trapping
/// the scroll); the plain text is the fallback and the opt-in.
class _EmailReader extends ConsumerStatefulWidget {
  const _EmailReader({super.key, required this.message, required this.onBack});

  final MessageModel message;
  final VoidCallback onBack;

  @override
  ConsumerState<_EmailReader> createState() => _EmailReaderState();
}

class _EmailReaderState extends ConsumerState<_EmailReader> {
  String? _originalUrl;
  bool _loadingOriginal = false;

  /// The iframe's load event. A heavy email (inlined photo → multi-MB data:
  /// URIs) takes seconds to arrive; until then the frame is a silent white
  /// void, which reads as broken. The overlay spinner stays up until the
  /// browser says the document landed.
  bool _frameLoaded = false;

  /// True once the operator chose the text view — the formatted original is
  /// the DEFAULT, text is the opt-out.
  bool _preferText = false;

  /// When the current signed URL was minted; ~2min token, re-mint after.
  DateTime? _mintedAt;

  MessageModel get msg => widget.message;

  bool get _canInline =>
      original_frame.inlineOriginalSupported &&
      ViewOriginalButton.isAvailable(msg.metadata);

  @override
  void initState() {
    super.initState();
    _loadOriginal();
  }

  Future<void> _loadOriginal() async {
    final fresh = _mintedAt != null &&
        DateTime.now().difference(_mintedAt!) < const Duration(seconds: 90);
    if (!_canInline ||
        _preferText ||
        (_originalUrl != null && fresh) ||
        _loadingOriginal) {
      return;
    }
    setState(() => _loadingOriginal = true);
    try {
      final url = await ref
          .read(attachmentActionsProvider)
          .originalViewUrl(msg.id, loadImages: false);
      if (mounted) {
        setState(() {
          // A new URL is a new document — the old load event doesn't count.
          if (url != _originalUrl) _frameLoaded = false;
          _originalUrl = url;
          _mintedAt = DateTime.now();
        });
      }
    } catch (_) {
      // The plain text renders as the fallback — same content, no snackbar.
    } finally {
      if (mounted) setState(() => _loadingOriginal = false);
    }
  }

  Future<void> _toggleText() async {
    if (!_preferText && _originalUrl != null) {
      setState(() => _preferText = true);
      return;
    }
    setState(() => _preferText = false);
    await _loadOriginal();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final verdict = msg.senderType == SenderType.customer
        ? SenderAuthenticity.fromMetadata(msg.metadata)
        : SenderAuthenticity.verified;
    final showFrame = _originalUrl != null && !_preferText;

    return Column(
      key: Key('email_reader_${msg.id}'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Reader header: back to the list, subject, who/when, view toggle.
        Container(
          padding: const EdgeInsets.fromLTRB(8, 8, 12, 8),
          decoration: BoxDecoration(
            border: Border(bottom: BorderSide(color: colors.borderMedium)),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              IconButton(
                key: const Key('email_reader_back'),
                tooltip: t.email_inbox.back_to_list,
                onPressed: widget.onBack,
                icon: Icon(Icons.arrow_back,
                    size: 18, color: colors.textSecondary),
              ),
              const SizedBox(width: 4),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      emailSubjectOf(msg),
                      key: const Key('email_reader_subject'),
                      style: TextStyle(
                        color: colors.textPrimary,
                        fontSize: 14.5,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      '${emailIdentityOf(msg)} · '
                      '${DateFormat('dd/MM/yyyy HH:mm').format(msg.createdAt)}',
                      style: TextStyle(color: colors.textMuted, fontSize: 11.5),
                    ),
                    if (verdict.isWorthShowing)
                      SenderAuthBadgeInline(
                        verdict: verdict,
                        detail: SenderAuthDetail.fromMetadata(msg.metadata),
                      ),
                  ],
                ),
              ),
              if (_canInline) _viewToggle(colors, showFrame),
              if (ViewOriginalButton.isAvailable(msg.metadata)) ...[
                const SizedBox(width: 4),
                ViewOriginalButton(
                    messageId: msg.id,
                    imagesOnly: _canInline,
                    compact: _canInline),
              ],
            ],
          ),
        ),
        if (msg.risk != null)
          EmailRiskBanner(
            risk: msg.risk!,
            messageId: msg.id,
            conversationId: msg.conversationId,
          ),
        // The message body takes the REST of the panel — the whole point of
        // the reader over the accordion: the iframe gets real height and its
        // own scroll instead of a fixed box fighting the list's wheel.
        Expanded(child: _buildBody(colors, showFrame)),
        // Attachments join the same 720px reading column as the document
        // card, as their own dark card — not a full-width strip.
        if (msg.attachments
            .any((a) => !a.isInline || (!showFrame && a.isLargeInlineImage)))
          Container(
            color: colors.backgroundPrimary,
            alignment: Alignment.topCenter,
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 10),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 720),
              child: Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                decoration: BoxDecoration(
                  color: colors.backgroundSecondary,
                  borderRadius: BorderRadius.circular(12),
                  border: Border.all(color: colors.borderLight),
                ),
                child: AttachmentList(
                  attachments: msg.attachments,
                  // The text mode cannot render the pasted photo in place —
                  // give it a row that opens in a new tab instead. The
                  // formatted view already shows it, so no duplicate there.
                  includeLargeInline: !showFrame,
                ),
              ),
            ),
          ),
      ],
    );
  }

  /// Texto | Original — one segmented control instead of a text link that
  /// renamed itself per state ("Voltar ao texto" / "Mostrar original aqui").
  Widget _viewToggle(AppColorsExtension colors, bool showFrame) {
    Widget seg(String label, bool active, Key key, {bool loading = false}) {
      return InkWell(
        key: key,
        onTap: active || _loadingOriginal ? null : _toggleText,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
          color: active ? colors.glassMedium : Colors.transparent,
          child: loading
              ? const SizedBox(
                  width: 12,
                  height: 12,
                  child: CircularProgressIndicator(strokeWidth: 2))
              : Text(
                  label,
                  style: TextStyle(
                    color: active ? colors.textPrimary : colors.textMuted,
                    fontSize: 12,
                    fontWeight: active ? FontWeight.w600 : FontWeight.w400,
                  ),
                ),
        ),
      );
    }

    return Container(
      margin: const EdgeInsets.only(top: 4),
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        border: Border.all(color: colors.borderMedium),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          seg(t.email_inbox.view_text, !showFrame,
              const Key('email_view_text')),
          Container(width: 1, height: 26, color: colors.borderMedium),
          seg(t.email_inbox.view_original, showFrame,
              const Key('email_view_original'),
              loading: _loadingOriginal && !showFrame),
        ],
      ),
    );
  }

  /// The reader body in its three states: minting the signed URL (spinner —
  /// showing the plain text here would flash and then be replaced), frame up
  /// but document still loading (frame + spinner overlay), and the plain
  /// text (preference or fallback).
  ///
  /// All three render as a white DOCUMENT CARD centered in a 720px reading
  /// column over the dark panel (design A) — an email is a document, and
  /// letterboxing it stops the white from bleeding edge to edge against the
  /// dark theme. Paper colors are deliberate literals, not theme tokens:
  /// this surface mimics the email's own background, not the app's.
  Widget _buildBody(AppColorsExtension colors, bool showFrame) {
    Widget card(Widget child) {
      return Container(
        color: colors.backgroundPrimary,
        alignment: Alignment.topCenter,
        padding: const EdgeInsets.fromLTRB(16, 14, 16, 10),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 720),
          child: Container(
            height: double.infinity,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(12),
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withValues(alpha: 0.4),
                  blurRadius: 24,
                  offset: const Offset(0, 4),
                ),
              ],
            ),
            child: ClipRRect(
              borderRadius: BorderRadius.circular(12),
              child: ColoredBox(color: Colors.white, child: child),
            ),
          ),
        ),
      );
    }

    if (showFrame) {
      return card(Stack(
        fit: StackFit.expand,
        children: [
          original_frame.buildEmailOriginalFrame(
            _originalUrl!,
            onLoad: () {
              if (mounted) setState(() => _frameLoaded = true);
            },
          ),
          if (!_frameLoaded)
            _loadingPane(key: const Key('email_frame_loading')),
        ],
      ));
    }
    if (_loadingOriginal && !_preferText && _canInline) {
      return card(_loadingPane(key: const Key('email_original_minting')));
    }
    // Plain text gets a CONTENT-SIZED card, full column width — the fill
    // card above exists for the iframe, and wrapping three lines of text in
    // it painted a huge empty white slab shrunk to the text's own width.
    return Container(
      color: colors.backgroundPrimary,
      alignment: Alignment.topCenter,
      padding: const EdgeInsets.fromLTRB(16, 14, 16, 10),
      child: SingleChildScrollView(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 720),
          child: Container(
            width: double.infinity,
            padding: const EdgeInsets.all(20),
            decoration: BoxDecoration(
              color: Colors.white,
              borderRadius: BorderRadius.circular(12),
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withValues(alpha: 0.4),
                  blurRadius: 24,
                  offset: const Offset(0, 4),
                ),
              ],
            ),
            child: SelectableText(
              msg.content,
              style: const TextStyle(
                  color: Color(0xFF1F1F1F), fontSize: 13.5, height: 1.5),
            ),
          ),
        ),
      ),
    );
  }

  Widget _loadingPane({Key? key}) {
    return Container(
      key: key,
      color: Colors.white,
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(
              width: 22,
              height: 22,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
            const SizedBox(height: 10),
            Text(
              t.email_inbox.loading_email,
              style: const TextStyle(color: Color(0xFF6B7280), fontSize: 12),
            ),
          ],
        ),
      ),
    );
  }
}

/// The three-state verdict, in row form.
///
/// Same rules as the bubble badge: silent when verified (marking the normal
/// case turns the one that matters into wallpaper), grey question for
/// unchecked, red alarm for an active failure — with the explainer dialog,
/// which carries the raw SPF/DKIM/DMARC line the reader forwards to IT.
class SenderAuthBadgeInline extends StatelessWidget {
  const SenderAuthBadgeInline({super.key, required this.verdict, this.detail});

  final SenderAuthenticity verdict;
  final SenderAuthDetail? detail;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final failed = verdict == SenderAuthenticity.failed;
    final color = failed ? colors.redAlert : colors.textMuted;
    return Padding(
      padding: const EdgeInsets.only(top: 2),
      child: InkWell(
        key: Key(failed ? 'email_auth_failed' : 'email_auth_unknown'),
        onTap: () {
          final body =
              failed ? t.sender_auth.body_failed : t.sender_auth.body_unknown;
          showConfirmDialog(
            context,
            title: failed
                ? t.sender_auth.title_failed
                : t.sender_auth.title_unknown,
            message: detail == null
                ? body
                : '$body\n\n${t.sender_auth.checks(checks: detail.toString())}',
            confirmText: t.common.close,
            cancelText: '',
          );
        },
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(failed ? Icons.gpp_maybe_outlined : Icons.help_outline,
                size: 12, color: color),
            const SizedBox(width: 3),
            Text(
              failed ? t.sender_auth.badge_failed : t.sender_auth.badge_unknown,
              style: TextStyle(color: color, fontSize: 11),
            ),
          ],
        ),
      ),
    );
  }
}
