import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/settings/domain/models/whatsapp_error.dart';
import 'package:app/features/settings/domain/models/whatsapp_template_model.dart';
import 'package:app/features/settings/presentation/controllers/channels_controller.dart';
import 'package:app/features/settings/presentation/controllers/whatsapp_config_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Sends an approved WhatsApp template to a specific recipient.
///
/// Redesigned 2026-08-28 (canvas "Enviar Template"): picking a template by
/// NAME alone was choosing blind — the dialog now renders the body as the
/// customer will receive it (WhatsApp-style bubble, variables applied live)
/// and asks for each {{n}} value below, {{1}} pre-filled with the contact's
/// name. A "create template" path opens [_CreateTemplateSheet]: Meta must
/// approve every template (minutes to a day), so it cannot serve THIS
/// moment — but suggestion chips make the next one a one-click setup.
class WaSendTemplateDialog extends ConsumerStatefulWidget {
  final String channelId;
  final String recipientPhone;
  final String? contactName;

  const WaSendTemplateDialog({
    super.key,
    required this.channelId,
    required this.recipientPhone,
    this.contactName,
  });

  @override
  ConsumerState<WaSendTemplateDialog> createState() =>
      _WaSendTemplateDialogState();
}

class _WaSendTemplateDialogState extends ConsumerState<WaSendTemplateDialog> {
  WhatsAppTemplateModel? _selected;
  bool _sending = false;
  String? _error;

  /// One controller per {{n}} of the selected template, index 0 = {{1}}.
  List<TextEditingController> _varCtrls = const [];

  @override
  void dispose() {
    for (final c in _varCtrls) {
      c.dispose();
    }
    super.dispose();
  }

  /// Highest {{n}} referenced by the template's body.
  int _varCount(WhatsAppTemplateModel tpl) {
    var max = 0;
    for (final m in RegExp(r'\{\{(\d+)\}\}').allMatches(tpl.bodyText)) {
      final n = int.tryParse(m.group(1) ?? '') ?? 0;
      if (n > max) max = n;
    }
    return max;
  }

  void _select(WhatsAppTemplateModel? tpl) {
    for (final c in _varCtrls) {
      c.dispose();
    }
    final count = tpl == null ? 0 : _varCount(tpl);
    final examples = tpl?.bodyExamples ?? const [];
    _varCtrls = List.generate(count, (i) {
      // {{1}} is almost always the person's name — pre-fill it; the rest
      // seed from Meta's stored examples so the preview never shows raw
      // braces.
      var seed = i < examples.length ? examples[i] : '';
      if (i == 0 && (widget.contactName?.trim().isNotEmpty ?? false)) {
        seed = widget.contactName!.trim();
      }
      final ctrl = TextEditingController(text: seed);
      ctrl.addListener(() => setState(() {}));
      return ctrl;
    });
    setState(() {
      _selected = tpl;
      _error = null;
    });
  }

  /// The body as the customer will receive it: typed values win, Meta's
  /// examples fill the gaps, raw {{n}} only when neither exists.
  String _previewBody(WhatsAppTemplateModel tpl) {
    return tpl.bodyText.replaceAllMapped(RegExp(r'\{\{(\d+)\}\}'), (m) {
      final idx = int.tryParse(m.group(1) ?? '');
      if (idx != null && idx >= 1 && idx <= _varCtrls.length) {
        final typed = _varCtrls[idx - 1].text.trim();
        if (typed.isNotEmpty) return typed;
      }
      return m.group(0)!;
    });
  }

  bool get _varsComplete => _varCtrls.every((c) => c.text.trim().isNotEmpty);

  Future<void> _send() async {
    final template = _selected;
    if (template == null) {
      setState(() => _error = t.whatsapp.choose_approved_template);
      return;
    }
    setState(() {
      _sending = true;
      _error = null;
    });
    try {
      final ds = ref.read(channelsRemoteDataSourceProvider);
      final body = <String, dynamic>{
        // Recipient is already in E.164 minus "+" (Replider stores
        // contact_id this way); the API accepts the bare digits.
        'to': widget.recipientPhone,
        'template_name': template.name,
        'language_code':
            template.language.isEmpty ? 'pt_BR' : template.language,
      };
      if (_varCtrls.isNotEmpty) {
        body['components'] = [
          {
            'type': 'body',
            'parameters': [
              for (final c in _varCtrls) {'type': 'text', 'text': c.text.trim()}
            ],
          }
        ];
      }
      await ds.sendTemplate(widget.channelId, body);
      if (!mounted) return;
      Navigator.of(context).pop();
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(
            t.whatsapp.template_sent_success(phone: widget.recipientPhone)),
        backgroundColor: Colors.green.shade700,
        behavior: SnackBarBehavior.floating,
      ));
    } on DioException catch (e) {
      if (!mounted) return;
      final parsed = WhatsAppError.fromDio(e);
      final message = (parsed != null && parsed.isKnown)
          ? parsed.humanize()
          : parseApiError(e);
      setState(() {
        _sending = false;
        _error = message;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final templatesAsync =
        ref.watch(whatsAppTemplatesProvider(widget.channelId));
    final approved = templatesAsync.maybeWhen(
      data: (list) => list
          .where((t) => t.status == WhatsAppTemplateStatus.approved)
          .toList(),
      orElse: () => <WhatsAppTemplateModel>[],
    );
    final selected = _selected;

    return AlertDialog(
      backgroundColor: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
      title: Row(
        children: [
          Expanded(
            child: Text(
              t.whatsapp.send_template_title,
              style: TextStyle(
                  color: colors.textPrimary, fontWeight: FontWeight.w700),
            ),
          ),
          Text(
            widget.contactName != null
                ? t.whatsapp.send_template_to_with_name(
                    name: widget.contactName!, phone: widget.recipientPhone)
                : t.whatsapp.send_template_to(phone: widget.recipientPhone),
            style: TextStyle(color: colors.textMuted, fontSize: 11.5),
          ),
        ],
      ),
      content: SizedBox(
        width: 460,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              templatesAsync.when(
                data: (_) => DropdownButtonFormField<WhatsAppTemplateModel>(
                  key: const Key('template_dropdown'),
                  initialValue: selected,
                  isExpanded: true,
                  dropdownColor: colors.backgroundSecondary,
                  style: TextStyle(color: colors.textPrimary, fontSize: 13),
                  decoration: InputDecoration(
                    labelText: t.whatsapp.approved_template_label,
                    labelStyle: TextStyle(color: colors.textMuted),
                  ),
                  items: approved
                      .map((tpl) => DropdownMenuItem(
                            value: tpl,
                            child: Text(
                              '${tpl.name} · ${tpl.languageLabel}'
                              '${tpl.category != null ? ' · ${tpl.category!.label}' : ''}',
                              overflow: TextOverflow.ellipsis,
                            ),
                          ))
                      .toList(),
                  onChanged: _select,
                ),
                loading: () => const LinearProgressIndicator(),
                error: (e, _) => Text(
                  t.whatsapp.loading_templates_error(error: '$e'),
                  style: TextStyle(color: colors.redAlert, fontSize: 12),
                ),
              ),
              if (approved.isEmpty &&
                  templatesAsync is AsyncData<List<WhatsAppTemplateModel>>) ...[
                const SizedBox(height: 8),
                Text(
                  t.whatsapp.no_approved_templates_hint,
                  style: TextStyle(color: colors.orangeAlert, fontSize: 11),
                ),
              ],
              // The preview IS the review: exactly what lands on the
              // customer's phone, WhatsApp-style, variables applied live.
              if (selected != null && selected.bodyText.isNotEmpty) ...[
                const SizedBox(height: 14),
                Text(
                  t.whatsapp.template_preview_label,
                  style: TextStyle(
                      color: colors.textMuted,
                      fontSize: 10.5,
                      fontWeight: FontWeight.w700,
                      letterSpacing: 0.4),
                ),
                const SizedBox(height: 6),
                Container(
                  key: const Key('template_preview'),
                  width: double.infinity,
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: const Color(0xFF0B141A),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Align(
                    alignment: Alignment.centerLeft,
                    child: Container(
                      constraints: const BoxConstraints(maxWidth: 340),
                      padding: const EdgeInsets.fromLTRB(12, 9, 12, 7),
                      decoration: const BoxDecoration(
                        color: Color(0xFF005C4B),
                        borderRadius: BorderRadius.only(
                          topLeft: Radius.circular(4),
                          topRight: Radius.circular(12),
                          bottomLeft: Radius.circular(12),
                          bottomRight: Radius.circular(12),
                        ),
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            _previewBody(selected),
                            style: const TextStyle(
                                color: Colors.white,
                                fontSize: 13,
                                height: 1.45),
                          ),
                          if (selected.footerText != null) ...[
                            const SizedBox(height: 4),
                            Text(
                              selected.footerText!,
                              style: TextStyle(
                                  color: Colors.white.withValues(alpha: 0.6),
                                  fontSize: 11),
                            ),
                          ],
                        ],
                      ),
                    ),
                  ),
                ),
                // One field per {{n}} — the send stays dead until every
                // variable has a value, so "{{1}}" never reaches a customer.
                if (_varCtrls.isNotEmpty) ...[
                  const SizedBox(height: 10),
                  Wrap(
                    spacing: 10,
                    runSpacing: 8,
                    children: [
                      for (var i = 0; i < _varCtrls.length; i++)
                        SizedBox(
                          width: 210,
                          child: TextField(
                            key: Key('template_var_${i + 1}'),
                            controller: _varCtrls[i],
                            style: TextStyle(
                                color: colors.textPrimary, fontSize: 13),
                            decoration: InputDecoration(
                              labelText: '{{${i + 1}}}',
                              labelStyle: TextStyle(
                                  color: colors.textMuted, fontSize: 12),
                              isDense: true,
                            ),
                          ),
                        ),
                    ],
                  ),
                ],
              ],
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(_error!,
                    style: TextStyle(color: colors.redAlert, fontSize: 12)),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const Key('template_create_new'),
          onPressed: _sending
              ? null
              : () => showDialog<void>(
                    context: context,
                    builder: (_) =>
                        _CreateTemplateSheet(channelId: widget.channelId),
                  ),
          child: Text(t.whatsapp.create_template_link,
              style: TextStyle(color: colors.bluePrimary)),
        ),
        Row(mainAxisSize: MainAxisSize.min, children: [
          TextButton(
            onPressed: _sending ? null : () => Navigator.of(context).pop(),
            child: Text(t.common.cancel,
                style: TextStyle(color: colors.textMuted)),
          ),
          ElevatedButton(
            key: const Key('template_send'),
            onPressed:
                (_sending || selected == null || !_varsComplete) ? null : _send,
            style: ElevatedButton.styleFrom(
              backgroundColor: colors.whatsappGreen,
              foregroundColor: colors.textInverted,
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10)),
            ),
            child: Text(_sending ? t.whatsapp.sending : t.whatsapp.send),
          ),
        ]),
      ],
      // AlertDialog actions render in an OverflowBar — no Spacer allowed;
      // spaceBetween over the two groups parks the create link left.
      actionsAlignment: MainAxisAlignment.spaceBetween,
    );
  }
}

/// A ready-to-fill template suggestion. UTILITY on purpose: resuming an
/// existing subject needs no opt-out button; MARKETING does (anti-ban).
class _TemplateSuggestion {
  const _TemplateSuggestion(this.label, this.name, this.body);
  final String label;
  final String name;
  final String body;
}

/// Creates a template and submits it for Meta approval.
///
/// Deliberately smaller than the settings-screen builder (no header/footer/
/// buttons): this exists for the "window closed, nothing fits" moment, and
/// the suggestion chips make the common case one click. The full builder
/// stays in Configurações → WhatsApp.
class _CreateTemplateSheet extends ConsumerStatefulWidget {
  const _CreateTemplateSheet({required this.channelId});

  final String channelId;

  @override
  ConsumerState<_CreateTemplateSheet> createState() =>
      _CreateTemplateSheetState();
}

class _CreateTemplateSheetState extends ConsumerState<_CreateTemplateSheet> {
  final _nameCtrl = TextEditingController();
  final _bodyCtrl = TextEditingController();
  bool _submitting = false;
  String? _error;
  int _suggestion = 0;

  static final _suggestions = [
    _TemplateSuggestion(
      t.whatsapp.suggestion_reopen,
      'retomar_conversa',
      'Olá {{1}}! Tentamos falar com você sobre {{2}}. '
          'Pode responder por aqui quando puder?',
    ),
    _TemplateSuggestion(
      t.whatsapp.suggestion_followup,
      'follow_up_proposta',
      'Olá {{1}}! Sua proposta de {{2}} segue disponível. '
          'Ficou alguma dúvida que eu possa esclarecer?',
    ),
  ];

  @override
  void initState() {
    super.initState();
    _apply(0);
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _bodyCtrl.dispose();
    super.dispose();
  }

  void _apply(int index) {
    setState(() {
      _suggestion = index;
      if (index < _suggestions.length) {
        _nameCtrl.text = _suggestions[index].name;
        _bodyCtrl.text = _suggestions[index].body;
      } else {
        _nameCtrl.clear();
        _bodyCtrl.clear();
      }
      _error = null;
    });
  }

  /// Meta requires example values for every {{n}}; generic Portuguese
  /// stand-ins keep the submission self-contained.
  List<String> _examplesFor(String body) {
    const standIns = ['Maria', 'o orçamento', 'amanhã', 'nossa equipe'];
    var max = 0;
    for (final m in RegExp(r'\{\{(\d+)\}\}').allMatches(body)) {
      final n = int.tryParse(m.group(1) ?? '') ?? 0;
      if (n > max) max = n;
    }
    return List.generate(
        max, (i) => i < standIns.length ? standIns[i] : 'exemplo ${i + 1}');
  }

  Future<void> _submit() async {
    final name = _nameCtrl.text.trim();
    final body = _bodyCtrl.text.trim();
    if (name.isEmpty || body.isEmpty) {
      setState(() => _error = t.whatsapp.create_template_missing_fields);
      return;
    }
    setState(() {
      _submitting = true;
      _error = null;
    });

    final bodyComp = <String, dynamic>{'type': 'BODY', 'text': body};
    final examples = _examplesFor(body);
    if (examples.isNotEmpty) {
      bodyComp['example'] = {
        'body_text': [examples]
      };
    }
    final result = await ref
        .read(whatsAppTemplatesProvider(widget.channelId).notifier)
        .create(
      name: name,
      category: WhatsAppTemplateCategory.utility,
      language: 'pt_BR',
      components: [bodyComp],
    );

    if (!mounted) return;
    if (result == null) {
      Navigator.of(context).pop();
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(t.whatsapp.create_template_submitted),
        backgroundColor: Colors.green.shade700,
        behavior: SnackBarBehavior.floating,
      ));
    } else {
      setState(() {
        _submitting = false;
        _error = result;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return AlertDialog(
      backgroundColor: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
      title: Text(
        t.whatsapp.create_template_title,
        style:
            TextStyle(color: colors.textPrimary, fontWeight: FontWeight.w700),
      ),
      content: SizedBox(
        width: 460,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Honesty first: this cannot serve the CURRENT conversation.
              Container(
                key: const Key('create_template_notice'),
                padding:
                    const EdgeInsets.symmetric(horizontal: 12, vertical: 9),
                decoration: BoxDecoration(
                  color: colors.bluePrimary.withValues(alpha: 0.10),
                  borderRadius: BorderRadius.circular(10),
                  border: Border.all(
                      color: colors.bluePrimary.withValues(alpha: 0.35)),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(Icons.info_outline,
                        size: 15, color: colors.bluePrimary),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        t.whatsapp.create_template_approval_notice,
                        style: TextStyle(
                            color: colors.textSecondary,
                            fontSize: 12,
                            height: 1.4),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 14),
              Wrap(
                spacing: 8,
                children: [
                  for (var i = 0; i <= _suggestions.length; i++)
                    ChoiceChip(
                      key: Key('template_suggestion_$i'),
                      label: Text(i < _suggestions.length
                          ? _suggestions[i].label
                          : t.whatsapp.suggestion_blank),
                      selected: _suggestion == i,
                      onSelected: (_) => _apply(i),
                      labelStyle: TextStyle(
                        fontSize: 12.5,
                        color: _suggestion == i
                            ? colors.textPrimary
                            : colors.textSecondary,
                      ),
                      selectedColor: colors.bluePrimary.withValues(alpha: 0.2),
                      backgroundColor: Colors.transparent,
                      side: BorderSide(
                          color: _suggestion == i
                              ? colors.bluePrimary
                              : colors.borderMedium),
                    ),
                ],
              ),
              const SizedBox(height: 12),
              TextField(
                key: const Key('create_template_name'),
                controller: _nameCtrl,
                style: TextStyle(color: colors.textPrimary, fontSize: 13),
                decoration: InputDecoration(
                  labelText: t.whatsapp.create_template_name_label,
                  helperText: t.whatsapp.create_template_name_helper,
                  helperStyle:
                      TextStyle(color: colors.textDisabled, fontSize: 10.5),
                  labelStyle: TextStyle(color: colors.textMuted),
                  isDense: true,
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                key: const Key('create_template_body'),
                controller: _bodyCtrl,
                minLines: 3,
                maxLines: 6,
                style: TextStyle(
                    color: colors.textPrimary, fontSize: 13, height: 1.4),
                decoration: InputDecoration(
                  labelText: t.whatsapp.create_template_body_label,
                  helperText: t.whatsapp.create_template_body_helper,
                  helperStyle:
                      TextStyle(color: colors.textDisabled, fontSize: 10.5),
                  helperMaxLines: 2,
                  labelStyle: TextStyle(color: colors.textMuted),
                  alignLabelWithHint: true,
                ),
              ),
              if (_error != null) ...[
                const SizedBox(height: 10),
                Text(_error!,
                    style: TextStyle(color: colors.redAlert, fontSize: 12)),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _submitting ? null : () => Navigator.of(context).pop(),
          child:
              Text(t.common.cancel, style: TextStyle(color: colors.textMuted)),
        ),
        ElevatedButton(
          key: const Key('create_template_submit'),
          onPressed: _submitting ? null : _submit,
          style: ElevatedButton.styleFrom(
            backgroundColor: colors.bluePrimary,
            foregroundColor: Colors.white,
            shape:
                RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
          ),
          child: Text(_submitting
              ? t.whatsapp.sending
              : t.whatsapp.create_template_submit),
        ),
      ],
    );
  }
}
