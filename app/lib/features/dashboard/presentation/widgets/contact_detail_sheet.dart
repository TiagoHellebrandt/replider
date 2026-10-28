import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/utils/phone_format.dart';
import 'package:app/design_system/components/custom_text_field.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/design_system/components/gradient_avatar.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/dashboard/domain/models/lead_profile_model.dart';
import 'package:app/features/dashboard/domain/models/message_model.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/dashboard/presentation/widgets/attachment_list.dart';
import 'package:app/features/dashboard/presentation/widgets/source_badge.dart';
import 'package:app/i18n/strings.g.dart';

/// Shows a bottom sheet with contact details for a conversation. Two
/// tabs: Info (identity + stats + status) and Arquivos (every media
/// the contact and bot exchanged in this conversation).
Future<void> showContactDetailSheet(
  BuildContext context,
  ConversationModel conversation,
) {
  return showModalBottomSheet<void>(
    context: context,
    backgroundColor: Colors.transparent,
    isScrollControlled: true,
    builder: (_) => ContactDetailSheet(conversation: conversation),
  );
}

class ContactDetailSheet extends ConsumerWidget {
  final ConversationModel conversation;

  const ContactDetailSheet({super.key, required this.conversation});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final size = MediaQuery.of(context).size;
    return SafeArea(
      child: Padding(
        padding: EdgeInsets.only(
          bottom: MediaQuery.of(context).viewInsets.bottom,
        ),
        child: ConstrainedBox(
          constraints: BoxConstraints(maxHeight: size.height * 0.85),
          child: DefaultTabController(
            length: 2,
            child: Container(
              decoration: BoxDecoration(
                color: colors.backgroundSecondary,
                borderRadius:
                    const BorderRadius.vertical(top: Radius.circular(20)),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const SizedBox(height: 14),
                  Center(
                    child: Container(
                      width: 36,
                      height: 4,
                      decoration: BoxDecoration(
                        color: colors.borderMedium,
                        borderRadius: BorderRadius.circular(2),
                      ),
                    ),
                  ),
                  const SizedBox(height: 16),
                  _Header(conversation: conversation),
                  const SizedBox(height: 12),
                  TabBar(
                    indicatorColor: colors.cyanGlow,
                    labelColor: colors.textPrimary,
                    unselectedLabelColor: colors.textMuted,
                    labelStyle: const TextStyle(
                        fontWeight: FontWeight.w700, fontSize: 13),
                    tabs: [
                      Tab(text: t.contact_sheet.tab_info),
                      Tab(text: t.contact_sheet.tab_files),
                    ],
                  ),
                  Flexible(
                    child: TabBarView(
                      children: [
                        _InfoTab(conversation: conversation),
                        _FilesTab(conversationId: conversation.id),
                      ],
                    ),
                  ),
                  Padding(
                    padding: const EdgeInsets.fromLTRB(24, 8, 24, 16),
                    child: SizedBox(
                      width: double.infinity,
                      child: OutlinedButton(
                        onPressed: () => Navigator.of(context).pop(),
                        style: OutlinedButton.styleFrom(
                          foregroundColor: colors.textPrimary,
                          side: BorderSide(color: colors.borderMedium),
                          padding: const EdgeInsets.symmetric(vertical: 12),
                        ),
                        child: Text(t.common.close),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _Header extends StatelessWidget {
  final ConversationModel conversation;
  const _Header({required this.conversation});

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final phone = parsePhone(conversation.contactExternalId);
    // Compact row, not a half-screen centerpiece: the sheet exists so a
    // SELLER can act on the lead, and every pixel the identity block eats
    // pushes the sales card further below the fold.
    final identityBits = <String>[
      if (conversation.source == ConversationSource.email &&
          conversation.contactExternalId.contains('@'))
        conversation.contactExternalId
      else if (phone.formatted.isNotEmpty)
        phone.formatted,
      _sourceLabel(conversation.source),
      if (conversation.contactUsername != null)
        '@${conversation.contactUsername}',
    ];
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 24),
      child: Row(
        children: [
          GradientAvatar(
            seed: conversation.contactExternalId.isNotEmpty
                ? conversation.contactExternalId
                : conversation.contactName,
            name: conversation.contactName,
            imageUrl: conversation.contactAvatarUrl,
            radius: 24,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  conversation.contactName,
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                    color: colors.textPrimary,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Row(
                  children: [
                    SourceBadge(source: conversation.source, size: 13),
                    const SizedBox(width: 5),
                    Expanded(
                      child: SelectableText(
                        identityBits.join(' · '),
                        key: const Key('contact_sheet_email'),
                        maxLines: 1,
                        style: TextStyle(
                            color: colors.textSecondary, fontSize: 12.5),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  String _sourceLabel(ConversationSource source) => switch (source) {
        ConversationSource.whatsapp => t.conversation.source_whatsapp,
        ConversationSource.email => t.conversation.source_email,
        ConversationSource.telegram => t.conversation.source_telegram,
        ConversationSource.instagram => t.conversation.source_instagram,
        ConversationSource.sms => t.conversation.source_sms,
        ConversationSource.internal => t.conversation.source_internal,
      };
}

class _InfoTab extends ConsumerWidget {
  final ConversationModel conversation;
  const _InfoTab({required this.conversation});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final statsAsync = ref.watch(contactStatsProvider(conversation.id));
    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(24, 12, 24, 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // SALES FIRST (pivot 2026-08): this product sells; support
          // bookkeeping moved to the footer line below.
          Container(
            key: const Key('contact_sales_card'),
            padding: const EdgeInsets.fromLTRB(14, 12, 14, 14),
            decoration: BoxDecoration(
              color: colors.bluePrimary.withValues(alpha: 0.06),
              borderRadius: BorderRadius.circular(14),
              border:
                  Border.all(color: colors.bluePrimary.withValues(alpha: 0.25)),
            ),
            child: _SalesProfileSection(conversationId: conversation.id),
          ),
          if (conversation.contactBio != null &&
              conversation.contactBio!.isNotEmpty)
            Container(
              margin: const EdgeInsets.only(top: 14),
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: colors.glassLight,
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: colors.borderLight),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    t.conversation.contact_bio,
                    style: TextStyle(
                      color: colors.textMuted,
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                      letterSpacing: 0.3,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    conversation.contactBio!,
                    style: TextStyle(
                      color: colors.textPrimary,
                      fontSize: 13,
                      height: 1.4,
                    ),
                  ),
                ],
              ),
            ),
          Divider(color: colors.borderLight, height: 24),
          Text(
            t.contact_sheet.conversation_section.toUpperCase(),
            style: TextStyle(
              color: colors.textMuted,
              fontSize: 11,
              fontWeight: FontWeight.w700,
              letterSpacing: 0.6,
            ),
          ),
          const SizedBox(height: 6),
          // One consolidated line instead of three rows of bookkeeping.
          _InfoRow(
            label: t.conversation.contact_status_label,
            value: [
              _statusLabel(conversation.status),
              if (conversation.assignedToName != null)
                t.conversation.contact_handled_by_inline(
                    name: conversation.assignedToName!),
              conversation.isBotPaused
                  ? t.conversation.contact_bot_paused_inline
                  : t.conversation.contact_bot_active_inline,
            ].join(' · '),
          ),
          _InfoRow(
            label: t.conversation.contact_last_message,
            value: _formatDate(conversation.lastMessageAt),
          ),
          if (conversation.tags.isNotEmpty) ...[
            const SizedBox(height: 8),
            Wrap(
              spacing: 6,
              runSpacing: 6,
              children: conversation.tags.map((tag) {
                final color = _parseColor(tag.color);
                return Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                  decoration: BoxDecoration(
                    color: color.withValues(alpha: 0.15),
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: Text(
                    tag.name,
                    style: TextStyle(
                      color: color,
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                );
              }).toList(),
            ),
          ],
          Divider(color: colors.borderLight, height: 24),
          // The old headline stats, demoted to a footer whisper — and
          // "primeiro contato", never "cliente desde": a lead is not a
          // customer yet, and this product knows the difference.
          statsAsync.when(
            data: (stats) => Text(
              t.contact_sheet.footer_stats(
                convs: '${stats.totalConversations}',
                msgs: '${stats.totalMessages}',
                since: stats.firstContactAt != null
                    ? _formatMonth(stats.firstContactAt!)
                    : '—',
              ),
              key: const Key('contact_footer_stats'),
              style: TextStyle(color: colors.textMuted, fontSize: 11.5),
            ),
            loading: () => const SizedBox.shrink(),
            error: (_, __) => const SizedBox.shrink(),
          ),
        ],
      ),
    );
  }

  String _formatMonth(DateTime d) {
    const months = [
      'jan',
      'fev',
      'mar',
      'abr',
      'mai',
      'jun',
      'jul',
      'ago',
      'set',
      'out',
      'nov',
      'dez'
    ];
    return "${months[d.month - 1]}/${d.year % 100}";
  }

  String _statusLabel(ConversationStatus status) => switch (status) {
        ConversationStatus.open => t.conversation.status_open,
        ConversationStatus.pendingHuman => t.conversation.status_pending,
        ConversationStatus.closed => t.dashboard.status_closed,
      };

  String _formatDate(DateTime dt) {
    final local = dt.toLocal();
    String pad(int n) => n.toString().padLeft(2, '0');
    return '${pad(local.day)}/${pad(local.month)}/${local.year} ${pad(local.hour)}:${pad(local.minute)}';
  }

  Color _parseColor(String hex) {
    final clean = hex.replaceFirst('#', '');
    if (clean.length == 6) {
      return Color(int.parse('FF$clean', radix: 16));
    }
    return const Color(0xFF6366F1);
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  const _InfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 150,
            child: Text(
              label,
              style: TextStyle(
                color: colors.textMuted,
                fontSize: 12,
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: TextStyle(
                color: colors.textPrimary,
                fontSize: 13,
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// "Perfil de vendas" — the lead's funnel memory: status, score, buy-intent
/// flag and the collected metadata fields (with origin badges). Empty when
/// the conversation isn't part of any funnel.
class _SalesProfileSection extends ConsumerWidget {
  final String conversationId;
  const _SalesProfileSection({required this.conversationId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final profileAsync = ref.watch(salesProfileProvider(conversationId));
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          t.sales.lead_profile.section_title.toUpperCase(),
          style: TextStyle(
            color: colors.textMuted,
            fontSize: 11,
            fontWeight: FontWeight.w700,
            letterSpacing: 0.6,
          ),
        ),
        const SizedBox(height: 10),
        profileAsync.when(
          data: (profile) => profile.inFunnel
              ? _SalesProfileBody(
                  profile: profile,
                  conversationId: conversationId,
                )
              : _emptyState(colors),
          loading: () => Padding(
            padding: const EdgeInsets.symmetric(vertical: 8),
            child: LinearProgressIndicator(
              color: colors.cyanGlow,
              backgroundColor: colors.borderLight,
              minHeight: 2,
            ),
          ),
          error: (_, __) => Text(
            t.sales.lead_profile.load_error,
            style: TextStyle(color: colors.textMuted, fontSize: 12),
          ),
        ),
      ],
    );
  }

  Widget _emptyState(AppColorsExtension colors) => Text(
        t.sales.lead_profile.empty,
        style: TextStyle(color: colors.textMuted, fontSize: 12, height: 1.4),
      );
}

class _SalesProfileBody extends StatelessWidget {
  final LeadProfileModel profile;
  final String conversationId;
  const _SalesProfileBody({
    required this.profile,
    required this.conversationId,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            if (profile.status != null && profile.status!.isNotEmpty)
              _Chip(
                label:
                    '${t.sales.lead_profile.status_label}: ${_statusLabel(profile.status!)}',
                color: colors.cyanGlow,
              ),
            _Chip(
              label: '${t.sales.lead_profile.score_label}: ${profile.score}',
              color: colors.textSecondary,
            ),
            if (profile.hasBuyIntent)
              _Chip(
                label: t.sales.lead_profile.buy_intent,
                color: colors.limeGlow,
                icon: Icons.local_fire_department,
              ),
          ],
        ),
        const SizedBox(height: 12),
        ...profile.fields.map(
          (f) => _LeadFieldRow(
            field: f,
            leadId: profile.leadId,
            conversationId: conversationId,
          ),
        ),
        if (profile.leadId != null && profile.leadId!.isNotEmpty) ...[
          const SizedBox(height: 10),
          Align(
            alignment: Alignment.centerLeft,
            child: ElevatedButton.icon(
              key: const Key('contact_new_proposal'),
              onPressed: () {
                Navigator.of(context).pop();
                context.go('/sales/proposals/new?lead=${profile.leadId}');
              },
              icon: const Icon(Icons.description_outlined, size: 14),
              label: Text(t.sales.lead_profile.new_proposal),
              style: ElevatedButton.styleFrom(
                backgroundColor: Theme.of(context)
                    .extension<AppColorsExtension>()!
                    .bluePrimary,
                foregroundColor: Colors.white,
                padding:
                    const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                textStyle: const TextStyle(
                    fontSize: 12.5, fontWeight: FontWeight.w600),
              ),
            ),
          ),
        ],
      ],
    );
  }

  String _statusLabel(String status) => switch (status) {
        'new' => t.sales.lead_profile.lead_status.kNew,
        'prospected' => t.sales.lead_profile.lead_status.prospected,
        'enriching' => t.sales.lead_profile.lead_status.enriching,
        'enriched' => t.sales.lead_profile.lead_status.enriched,
        'pending_approval' => t.sales.lead_profile.lead_status.pending_approval,
        'contacted' => t.sales.lead_profile.lead_status.contacted,
        'responded' => t.sales.lead_profile.lead_status.responded,
        'qualified' => t.sales.lead_profile.lead_status.qualified,
        'won' => t.sales.lead_profile.lead_status.won,
        'lost' => t.sales.lead_profile.lead_status.lost,
        _ => status,
      };
}

class _LeadFieldRow extends ConsumerWidget {
  final LeadFieldModel field;

  /// Lead this field belongs to. Null when the profile carries no lead id
  /// (then the field is read-only — there's nothing to PATCH).
  final String? leadId;
  final String conversationId;

  const _LeadFieldRow({
    required this.field,
    required this.leadId,
    required this.conversationId,
  });

  bool get _canEdit => leadId != null && leadId!.isNotEmpty;

  Future<void> _edit(BuildContext context, WidgetRef ref) async {
    final messenger = ScaffoldMessenger.of(context);
    final newValue = await showEditLeadFieldDialog(context, field);
    if (newValue == null) return; // cancelled
    try {
      await ref.read(conversationActionsProvider.notifier).updateLeadMetadata(
            conversationId,
            leadId!,
            field.key,
            newValue,
          );
    } catch (e) {
      messenger.showSnackBar(
        SnackBar(
          content: Text(
            t.sales.lead_profile.edit_error(error: parseApiError(e)),
          ),
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 130,
            child: Row(
              children: [
                Flexible(
                  child: Text(
                    field.label,
                    style: TextStyle(
                      color: colors.textMuted,
                      fontSize: 12,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ),
                if (field.required)
                  Text(
                    ' *',
                    style: TextStyle(color: colors.redAlert, fontSize: 12),
                  ),
              ],
            ),
          ),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (field.hasValue)
                  Text(
                    field.value,
                    style: TextStyle(
                      color: colors.textPrimary,
                      fontSize: 13,
                      fontWeight: FontWeight.w500,
                    ),
                  )
                else
                  _Chip(
                    label: field.isPending
                        ? t.sales.lead_profile.pending
                        : t.sales.lead_profile.no_value,
                    color:
                        field.isPending ? colors.orangeAlert : colors.textMuted,
                  ),
                if (field.source != LeadFieldSource.none) ...[
                  const SizedBox(height: 4),
                  _OriginBadge(source: field.source),
                ],
              ],
            ),
          ),
          if (_canEdit)
            IconButton(
              onPressed: () => _edit(context, ref),
              icon:
                  Icon(Icons.edit_outlined, size: 16, color: colors.textMuted),
              tooltip: t.sales.lead_profile.edit_field,
              visualDensity: VisualDensity.compact,
              padding: EdgeInsets.zero,
              constraints: const BoxConstraints(minWidth: 32, minHeight: 32),
            ),
        ],
      ),
    );
  }
}

/// Opens a type-aware editor for a single lead metadata field and returns
/// the new value as a string, or null if the attendant cancelled.
///
/// The input adapts to [LeadFieldModel.type]: `date` uses a date picker,
/// `bool` a yes/no toggle, `number` a numeric keyboard, everything else a
/// plain text field. The value is stored as a string regardless (the API's
/// `collected_metadata` keeps values as strings).
@visibleForTesting
Future<String?> showEditLeadFieldDialog(
  BuildContext context,
  LeadFieldModel field,
) {
  return showDialog<String>(
    context: context,
    builder: (_) => _EditLeadFieldDialog(field: field),
  );
}

class _EditLeadFieldDialog extends StatefulWidget {
  final LeadFieldModel field;
  const _EditLeadFieldDialog({required this.field});

  @override
  State<_EditLeadFieldDialog> createState() => _EditLeadFieldDialogState();
}

class _EditLeadFieldDialogState extends State<_EditLeadFieldDialog> {
  late final TextEditingController _controller =
      TextEditingController(text: widget.field.value);
  late bool _boolValue =
      widget.field.value == 'true' || widget.field.value == '1';

  bool get _isBool => widget.field.type == 'bool';
  bool get _isDate => widget.field.type == 'date';
  bool get _isNumber => widget.field.type == 'number';

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _pickDate(AppColorsExtension colors) async {
    final parsed = DateTime.tryParse(_controller.text);
    final picked = await showDatePicker(
      context: context,
      initialDate: parsed ?? DateTime.now(),
      firstDate: DateTime(2000),
      lastDate: DateTime(2100),
    );
    if (picked != null) {
      setState(() {
        _controller.text =
            '${picked.year.toString().padLeft(4, '0')}-${picked.month.toString().padLeft(2, '0')}-${picked.day.toString().padLeft(2, '0')}';
      });
    }
  }

  void _save() {
    Navigator.pop(context, _isBool ? _boolValue.toString() : _controller.text);
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return AlertDialog(
      backgroundColor: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      title: Text(
        widget.field.label,
        style: TextStyle(color: colors.textPrimary, fontSize: 16),
      ),
      content: _buildInput(colors),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child:
              Text(t.common.cancel, style: TextStyle(color: colors.textMuted)),
        ),
        ElevatedButton(
          onPressed: _save,
          style: ElevatedButton.styleFrom(
            backgroundColor: colors.bluePrimary,
            foregroundColor: Colors.white,
          ),
          child: Text(t.common.save),
        ),
      ],
    );
  }

  Widget _buildInput(AppColorsExtension colors) {
    if (_isBool) {
      return SwitchListTile(
        value: _boolValue,
        onChanged: (v) => setState(() => _boolValue = v),
        title: Text(
          _boolValue ? t.common.yes : t.common.no,
          style: TextStyle(color: colors.textPrimary),
        ),
        activeThumbColor: colors.cyanGlow,
        contentPadding: EdgeInsets.zero,
      );
    }
    if (_isDate) {
      return Row(
        children: [
          Expanded(
            child: CustomTextField(
              controller: _controller,
              hint: 'YYYY-MM-DD',
            ),
          ),
          IconButton(
            onPressed: () => _pickDate(colors),
            icon: Icon(Icons.calendar_today_outlined,
                size: 18, color: colors.cyanGlow),
            tooltip: t.sales.lead_profile.pick_date,
          ),
        ],
      );
    }
    return CustomTextField(
      controller: _controller,
      hint: widget.field.label,
      autofocus: true,
      keyboardType: _isNumber ? TextInputType.number : TextInputType.text,
    );
  }
}

/// Small "IA" / "Humano" badge marking a collected value's origin.
class _OriginBadge extends StatelessWidget {
  final LeadFieldSource source;
  const _OriginBadge({required this.source});

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final isHuman = source == LeadFieldSource.human;
    final color = isHuman ? colors.limeGlow : colors.cyanGlow;
    final label = isHuman
        ? t.sales.lead_profile.origin_human
        : t.sales.lead_profile.origin_ai;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            isHuman ? Icons.person : Icons.smart_toy_outlined,
            size: 11,
            color: color,
          ),
          const SizedBox(width: 3),
          Text(
            label,
            style: TextStyle(
              color: color,
              fontSize: 10,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }
}

class _Chip extends StatelessWidget {
  final String label;
  final Color color;
  final IconData? icon;
  const _Chip({required this.label, required this.color, this.icon});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[
            Icon(icon, size: 12, color: color),
            const SizedBox(width: 4),
          ],
          Text(
            label,
            style: TextStyle(
              color: color,
              fontSize: 11,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }
}

class _FilesTab extends ConsumerWidget {
  final String conversationId;
  const _FilesTab({required this.conversationId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final mediaAsync = ref.watch(conversationMediaProvider(conversationId));
    return mediaAsync.when(
      data: (messages) {
        if (messages.isEmpty) {
          return Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.folder_open, size: 36, color: colors.textMuted),
                  const SizedBox(height: 8),
                  Text(
                    t.contact_sheet.files_empty,
                    style: TextStyle(color: colors.textMuted, fontSize: 13),
                    textAlign: TextAlign.center,
                  ),
                ],
              ),
            ),
          );
        }
        final images = messages
            .where((m) =>
                m.messageType == MessageType.image ||
                m.messageType == MessageType.video ||
                m.messageType == MessageType.sticker)
            .toList();
        final docs = messages
            .where((m) =>
                m.messageType == MessageType.document ||
                m.messageType == MessageType.audio ||
                m.messageType == MessageType.voice)
            .toList();
        // Email files live in message_attachments, not media_url — a text
        // message carrying attachments IS a file entry here. Rendered via
        // AttachmentList so the antivirus verdict travels with each row.
        final withAttachments =
            messages.where((m) => m.attachments.isNotEmpty).toList();
        return SingleChildScrollView(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (images.isNotEmpty) ...[
                _section(colors, t.contact_sheet.files_section_images,
                    images.length),
                const SizedBox(height: 8),
                _ImageGrid(messages: images),
                const SizedBox(height: 16),
              ],
              if (docs.isNotEmpty) ...[
                _section(colors, t.contact_sheet.files_section_documents,
                    docs.length),
                const SizedBox(height: 8),
                ...docs.map((m) => _DocumentTile(message: m)),
                const SizedBox(height: 16),
              ],
              if (withAttachments.isNotEmpty) ...[
                _section(
                    colors,
                    t.contact_sheet.files_section_attachments,
                    withAttachments.fold<int>(
                        0, (n, m) => n + m.attachments.length)),
                // One group per email: the same invoice PDF arriving twice
                // is indistinguishable without the subject + date above it.
                ...withAttachments.map(
                  (m) => Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const SizedBox(height: 10),
                      Row(
                        key: Key('files_email_header_${m.id}'),
                        children: [
                          Icon(Icons.mail_outline,
                              size: 13, color: colors.textMuted),
                          const SizedBox(width: 5),
                          Expanded(
                            child: Text(
                              _emailSubject(m),
                              style: TextStyle(
                                color: colors.textSecondary,
                                fontSize: 12,
                                fontWeight: FontWeight.w600,
                              ),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                          const SizedBox(width: 8),
                          Text(
                            _formatDateTime(m.createdAt),
                            style: TextStyle(
                                color: colors.textMuted, fontSize: 11),
                          ),
                        ],
                      ),
                      AttachmentList(
                        key: Key('files_attachments_${m.id}'),
                        attachments: m.attachments,
                        includeLargeInline: true,
                      ),
                    ],
                  ),
                ),
              ],
            ],
          ),
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.error_outline, color: colors.redAlert, size: 32),
              const SizedBox(height: 8),
              Text(
                t.contact_sheet.files_error,
                style: TextStyle(color: colors.textMuted, fontSize: 12),
              ),
            ],
          ),
        ),
      ),
    );
  }

  String _emailSubject(MessageModel m) {
    final raw = m.metadata?['subject'];
    final subject = raw is String ? raw.trim() : '';
    return subject.isNotEmpty
        ? subject
        : t.contact_sheet.files_email_no_subject;
  }

  String _formatDateTime(DateTime dt) {
    final local = dt.toLocal();
    String pad(int n) => n.toString().padLeft(2, '0');
    return '${pad(local.day)}/${pad(local.month)}/${local.year} · '
        '${pad(local.hour)}:${pad(local.minute)}';
  }

  Widget _section(AppColorsExtension colors, String label, int count) {
    return Row(
      children: [
        Text(
          label.toUpperCase(),
          style: TextStyle(
            color: colors.textMuted,
            fontSize: 11,
            fontWeight: FontWeight.w700,
            letterSpacing: 0.6,
          ),
        ),
        const SizedBox(width: 6),
        Text(
          '($count)',
          style: TextStyle(color: colors.textMuted, fontSize: 11),
        ),
      ],
    );
  }
}

class _ImageGrid extends StatelessWidget {
  final List<MessageModel> messages;
  const _ImageGrid({required this.messages});

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 3,
        mainAxisSpacing: 6,
        crossAxisSpacing: 6,
      ),
      itemCount: messages.length,
      itemBuilder: (context, i) {
        final m = messages[i];
        final url = m.mediaUrl ?? '';
        return GestureDetector(
          onTap: url.isEmpty ? null : () => _open(url),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(6),
            // AppNetworkImage adds Flutter's ImageCache + a bounded cacheWidth
            // so re-opening the sheet or scrolling the gallery doesn't re-fetch
            // or re-decode every thumbnail at full resolution.
            child: AppNetworkImage(
              url: url,
              fit: BoxFit.cover,
              renderWidth: 200,
              fallback: Container(
                color: colors.glassLight,
                child:
                    Icon(Icons.broken_image, color: colors.textMuted, size: 20),
              ),
            ),
          ),
        );
      },
    );
  }

  Future<void> _open(String url) async {
    final uri = Uri.tryParse(url);
    if (uri == null) return;
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    }
  }
}

class _DocumentTile extends StatelessWidget {
  final MessageModel message;
  const _DocumentTile({required this.message});

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final url = message.mediaUrl ?? '';
    final icon = switch (message.messageType) {
      MessageType.audio || MessageType.voice => Icons.mic,
      MessageType.document => Icons.description_outlined,
      _ => Icons.attach_file,
    };
    final label = message.content.isNotEmpty
        ? message.content
        : _filenameFromUrl(url) ?? t.contact_sheet.files_unnamed;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: InkWell(
        borderRadius: BorderRadius.circular(8),
        onTap: url.isEmpty ? null : () => _open(url),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 10),
          decoration: BoxDecoration(
            color: colors.glassLight,
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: colors.borderLight),
          ),
          child: Row(
            children: [
              Icon(icon, color: colors.cyanGlow, size: 20),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      label,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        color: colors.textPrimary,
                        fontSize: 13,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                    Text(
                      _formatRelative(message.createdAt),
                      style: TextStyle(
                        color: colors.textMuted,
                        fontSize: 11,
                      ),
                    ),
                  ],
                ),
              ),
              Icon(Icons.download_outlined, color: colors.textMuted, size: 18),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _open(String url) async {
    final uri = Uri.tryParse(url);
    if (uri == null) return;
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    }
  }

  String? _filenameFromUrl(String url) {
    final uri = Uri.tryParse(url);
    if (uri == null || uri.pathSegments.isEmpty) return null;
    return uri.pathSegments.last;
  }

  String _formatRelative(DateTime dt) {
    final local = dt.toLocal();
    String pad(int n) => n.toString().padLeft(2, '0');
    return '${pad(local.day)}/${pad(local.month)}/${local.year}';
  }
}
