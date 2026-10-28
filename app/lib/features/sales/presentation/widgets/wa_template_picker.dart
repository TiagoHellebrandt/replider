import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/features/settings/domain/models/whatsapp_template_model.dart';
import 'package:app/features/settings/presentation/controllers/whatsapp_config_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Dropdown of the funnel channel's APPROVED WhatsApp templates.
///
/// Cold first-contact on the WhatsApp Cloud API is only deliverable through an
/// approved template — a free-form text is rejected outside the 24h window a
/// prospected lead never opened. Every surface that approves a WhatsApp send
/// (leads screen, approval queue, lead panel) needs this same picker; one
/// widget so they can never disagree about what "approved" means.
///
/// Resolves funnel → its channel → that channel's templates.
class WaApprovedTemplatePicker extends ConsumerWidget {
  const WaApprovedTemplatePicker({
    super.key,
    required this.funnelId,
    required this.selected,
    required this.onPicked,
    this.enabled = true,
  });

  final String funnelId;
  final WhatsAppTemplateModel? selected;
  final ValueChanged<WhatsAppTemplateModel> onPicked;
  final bool enabled;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final muted = TextStyle(color: colors.textMuted, fontSize: 12);
    return ref.watch(funnelProvider(funnelId)).when(
          loading: () => const LinearProgressIndicator(minHeight: 2),
          error: (_, __) =>
              Text(t.sales.leads.template_load_error, style: muted),
          data: (funnel) {
            final channelId = funnel.channelId;
            if (channelId == null || channelId.isEmpty) {
              return Text(t.sales.leads.template_no_channel, style: muted);
            }
            return ref.watch(whatsAppTemplatesProvider(channelId)).when(
                  loading: () => const LinearProgressIndicator(minHeight: 2),
                  error: (_, __) =>
                      Text(t.sales.leads.template_load_error, style: muted),
                  data: (all) {
                    final approved = all
                        .where((tpl) =>
                            tpl.status == WhatsAppTemplateStatus.approved)
                        .toList();
                    if (approved.isEmpty) {
                      return Text(t.sales.leads.template_none, style: muted);
                    }
                    // Keep the selection valid against the current list.
                    final value =
                        approved.any((tpl) => tpl.name == selected?.name)
                            ? selected!.name
                            : null;
                    return DropdownButtonFormField<String>(
                      key: const Key('wa_template_picker'),
                      initialValue: value,
                      isExpanded: true,
                      decoration: InputDecoration(
                        labelText: t.sales.leads.template_label,
                        filled: true,
                        fillColor: colors.backgroundPrimary,
                      ),
                      hint: Text(t.sales.leads.template_select, style: muted),
                      items: approved
                          .map((tpl) => DropdownMenuItem(
                                value: tpl.name,
                                child: Text(
                                  '${tpl.name} · ${tpl.languageLabel}',
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                      color: colors.textPrimary, fontSize: 13),
                                ),
                              ))
                          .toList(),
                      onChanged: enabled
                          ? (name) => onPicked(
                              approved.firstWhere((tpl) => tpl.name == name))
                          : null,
                    );
                  },
                );
          },
        );
  }
}

/// The template's declared buttons in the shape approve-send expects, so the
/// API's anti-ban guard can see the opt-out quick-reply (a marketing template
/// without one is rejected before any network call).
List<Map<String, dynamic>> waTemplateButtons(WhatsAppTemplateModel tpl) =>
    tpl.buttons
        .map((b) => <String, dynamic>{
              'sub_type': (b['type'] ?? '').toString().toLowerCase(),
              'text': (b['text'] ?? '').toString(),
              'payload': (b['text'] ?? '').toString(),
            })
        .toList();
