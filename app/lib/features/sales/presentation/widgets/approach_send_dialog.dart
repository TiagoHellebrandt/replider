import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/widgets/wa_template_picker.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';
import 'package:app/features/settings/domain/models/whatsapp_template_model.dart';
import 'package:app/i18n/strings.g.dart';

/// What the approver chose alongside the "yes": the e-mail subject, or the
/// approved WhatsApp template the text is delivered through.
class ApproachSendSpec {
  const ApproachSendSpec({this.subject, this.template});

  final String? subject;
  final WhatsAppTemplateModel? template;
}

/// The send confirmation for the approval queue.
///
/// The queue used to confirm with only the text and the recipient, and then
/// POST just those two — so an e-mail approach went out on the server's
/// default subject and a WhatsApp approach went out with no template and was
/// refused by Meta with a generic error. The confirmation is where those
/// choices belong: the person is already looking at exactly what will be sent.
///
/// Returns null when dismissed; a spec when confirmed. For a WhatsApp lead the
/// confirm button stays disabled until a template is picked — sending without
/// one cannot succeed, and a button that fails on purpose teaches people the
/// queue is broken.
Future<ApproachSendSpec?> showApproachSendDialog(
  BuildContext context, {
  required String recipient,
  required String text,
  ChannelType? channelType,
  String? funnelId,
}) {
  return showDialog<ApproachSendSpec>(
    context: context,
    builder: (_) => _ApproachSendDialog(
      recipient: recipient,
      text: text,
      channelType: channelType,
      funnelId: funnelId,
    ),
  );
}

class _ApproachSendDialog extends ConsumerStatefulWidget {
  const _ApproachSendDialog({
    required this.recipient,
    required this.text,
    required this.channelType,
    required this.funnelId,
  });

  final String recipient;
  final String text;
  final ChannelType? channelType;
  final String? funnelId;

  @override
  ConsumerState<_ApproachSendDialog> createState() =>
      _ApproachSendDialogState();
}

class _ApproachSendDialogState extends ConsumerState<_ApproachSendDialog> {
  final _subject = TextEditingController();
  WhatsAppTemplateModel? _template;

  bool get _isEmail => widget.channelType == ChannelType.email;
  bool get _isWhatsApp => widget.channelType == ChannelType.whatsapp;

  /// WhatsApp cannot leave without a template; everything else can go.
  bool get _canSend => !_isWhatsApp || _template != null;

  @override
  void dispose() {
    _subject.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return AlertDialog(
      backgroundColor: colors.backgroundSecondary,
      title: Text(t.lead_detail.confirm_title,
          style: TextStyle(color: colors.textPrimary, fontSize: 18)),
      content: SizedBox(
        width: 460,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                '${t.lead_detail.confirm_to}: ${widget.recipient}',
                key: const Key('approach_send_recipient'),
                style: TextStyle(color: colors.textPrimary, fontSize: 13),
              ),
              if (widget.channelType != null) ...[
                const SizedBox(height: 4),
                // Which door it leaves through. A phone number can be
                // WhatsApp or SMS; the queue must not leave that a guess.
                Text(
                  '${t.lead_detail.send_channel}: ${widget.channelType!.label}',
                  key: const Key('approach_send_channel'),
                  style: TextStyle(color: colors.textSecondary, fontSize: 12),
                ),
              ],
              const SizedBox(height: 12),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: colors.glassMedium,
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Text(
                  widget.text,
                  key: const Key('approach_send_text'),
                  style: TextStyle(color: colors.textSecondary, fontSize: 13),
                ),
              ),
              if (_isEmail) ...[
                const SizedBox(height: 12),
                TextField(
                  key: const Key('approach_send_subject'),
                  controller: _subject,
                  style: TextStyle(color: colors.textPrimary, fontSize: 13),
                  decoration: InputDecoration(
                    labelText: t.lead_detail.subject,
                    hintText: t.lead_detail.subject_hint,
                  ),
                ),
              ],
              if (_isWhatsApp && widget.funnelId != null) ...[
                const SizedBox(height: 12),
                WaApprovedTemplatePicker(
                  funnelId: widget.funnelId!,
                  selected: _template,
                  onPicked: (tpl) => setState(() => _template = tpl),
                ),
              ],
              if (_isWhatsApp && widget.funnelId == null) ...[
                const SizedBox(height: 12),
                Text(
                  t.sales.leads.template_no_channel,
                  style: TextStyle(color: colors.orangeAlert, fontSize: 12),
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(t.common.cancel),
        ),
        FilledButton(
          key: const Key('approach_send_confirm'),
          onPressed: _canSend
              ? () => Navigator.of(context).pop(ApproachSendSpec(
                    subject: _isEmail ? _subject.text.trim() : null,
                    template: _isWhatsApp ? _template : null,
                  ))
              : null,
          child: Text(t.lead_detail.approve_send),
        ),
      ],
    );
  }
}
