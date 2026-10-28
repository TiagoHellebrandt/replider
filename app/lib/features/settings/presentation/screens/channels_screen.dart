import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:dio/dio.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:go_router/go_router.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/core/error/plan_limit_handler.dart';
import 'package:app/design_system/components/channel_tile.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/features/settings/presentation/controllers/channels_controller.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';
import 'package:app/features/settings/presentation/widgets/whatsapp_embedded_signup_button.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/features/settings/presentation/widgets/email_dns_card.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:app/features/settings/presentation/widgets/google_mailbox_connect_card.dart';

class ChannelsScreen extends ConsumerStatefulWidget {
  const ChannelsScreen({super.key});

  @override
  ConsumerState<ChannelsScreen> createState() => _ChannelsScreenState();
}

class _ChannelsScreenState extends ConsumerState<ChannelsScreen> {
  @override
  void initState() {
    super.initState();
    Future.microtask(() {
      ref.read(channelsListProvider.notifier).load();
    });
    // The Google connection finishes in another tab and lands back here, so its
    // outcome arrives as a query parameter rather than as the answer to
    // anything this screen asked for. Post-frame because a snackbar needs a
    // Scaffold that does not exist yet during initState.
    WidgetsBinding.instance.addPostFrameCallback((_) => _reportGmailResult());
  }

  void _reportGmailResult() {
    if (!mounted) return;
    // maybeOf, not of: GoRouterState.of throws when this screen is rendered
    // outside a route — which a widget test does, and which is not a reason for
    // a channels list to fail to appear.
    final message = gmailConnectMessage(
      GoRouter.maybeOf(context)?.state.uri.queryParameters['gmail'],
    );
    if (message == null) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        key: const Key('gmail_connect_result'),
        content: Text(message),
        behavior: SnackBarBehavior.floating,
      ),
    );
    // A freshly connected mailbox is a channel this list does not have yet.
    ref.read(channelsListProvider.notifier).load();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final channelsAsync = ref.watch(channelsListProvider);

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        leading: IconButton(
          tooltip: t.settings.channels_back_tooltip,
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () => context.go('/settings'),
        ),
        title: Text(
          t.settings.channels_screen_title,
          style: TextStyle(
            fontSize: 18,
            fontWeight: FontWeight.w700,
            color: colors.textPrimary,
          ),
        ),
        actions: [
          IconButton(
            icon: Icon(Icons.add, color: colors.bluePrimary),
            onPressed: () => _showChannelDialog(context, ref, colors),
            tooltip: t.settings.channels_add_tooltip,
          ),
        ],
      ),
      body: channelsAsync.when(
        data: (channels) {
          if (channels.isEmpty) {
            return _buildEmptyState(context, ref, colors);
          }
          return RefreshIndicator(
            onRefresh: () => ref.read(channelsListProvider.notifier).refresh(),
            child: ListView.separated(
              padding: const EdgeInsets.all(16),
              itemCount: channels.length,
              separatorBuilder: (_, __) => const SizedBox(height: 12),
              itemBuilder: (ctx, i) => _ChannelCard(
                channel: channels[i],
                onEdit: () => _showChannelDialog(
                  context,
                  ref,
                  colors,
                  existing: channels[i],
                ),
              ),
            ),
          );
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => RetryableError(
          error: err,
          onRetry: () => ref.read(channelsListProvider.notifier).refresh(),
        ),
      ),
    );
  }

  Widget _buildEmptyState(
    BuildContext context,
    WidgetRef ref,
    AppColorsExtension colors,
  ) {
    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 24),
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 440),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: 12),
              Center(
                child: Container(
                  width: 72,
                  height: 72,
                  decoration: BoxDecoration(
                    color: colors.bluePrimary.withValues(alpha: 0.12),
                    shape: BoxShape.circle,
                  ),
                  child: Icon(
                    Icons.chat_bubble_outline,
                    size: 36,
                    color: colors.bluePrimary,
                  ),
                ),
              ),
              const SizedBox(height: 20),
              Text(
                t.settings.no_channels,
                style: TextStyle(
                  color: colors.textPrimary,
                  fontSize: 22,
                  fontWeight: FontWeight.w700,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 8),
              Text(
                t.settings.channels_empty_subtitle,
                textAlign: TextAlign.center,
                style: TextStyle(
                    color: colors.textSecondary, fontSize: 14, height: 1.5),
              ),
              const SizedBox(height: 28),
              // The two channels a hospital-equipment seller reaches her
              // buyers on. Email first — the offers go out on it, and it has
              // a one-click path.
              Row(
                children: [
                  Expanded(
                    child: ChannelTile(
                      type: ChannelType.email,
                      isSelected: false,
                      onTap: () => _showChannelDialog(context, ref, colors,
                          preSelected: ChannelType.email),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: ChannelTile(
                      type: ChannelType.whatsapp,
                      isSelected: false,
                      onTap: () => _showChannelDialog(context, ref, colors,
                          preSelected: ChannelType.whatsapp),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 24),
              // Primary WhatsApp path: Meta's Embedded Signup popup.
              // Kept prominent — most users land here to hook up WA.
              const Center(child: WhatsAppEmbeddedSignupButton()),
            ],
          ),
        ),
      ),
    );
  }

  void _showChannelDialog(
    BuildContext context,
    WidgetRef ref,
    AppColorsExtension colors, {
    ChannelType? preSelected,
    ChannelModel? existing,
  }) {
    final isEditing = existing != null;
    // Email first: it is the channel the offers go out on, and the one with a
    // one-click path.
    ChannelType selectedType =
        existing?.type ?? preSelected ?? ChannelType.email;
    final nameController = TextEditingController(text: existing?.name ?? '');
    // The credential form (SMTP, or WhatsApp's manual ids) is the graduation
    // path, not the door: folded until asked for, so the one-click card is the
    // only thing on the tile for most people. Always open on edit — there is
    // no one-click path for changing a stored credential.
    bool showManual = isEditing;
    // Single-field controller (Telegram). Masked credentials (backend
    // returns `1234****` for bot_token) are not hydrated — saving the
    // mask back would look like a token rotation.
    final existingSingleValue =
        existing != null && selectedType.configKey != null
            ? (existing.config[selectedType.configKey!]?.toString() ?? '')
            : '';
    final configController = TextEditingController(
      text: existingSingleValue.contains('*') ? '' : existingSingleValue,
    );
    // Multi-field controllers (WhatsApp manual fallback, Instagram)
    final Map<String, TextEditingController> multiControllers = {};

    showDialog<void>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialogState) {
          // Rebuild multi-field controllers when type changes. Hydrate
          // from the existing channel config on first build so editing
          // an already-connected WhatsApp channel doesn't wipe the
          // other credentials (access_token, waba_id).
          for (final field in selectedType.multiFieldConfig) {
            multiControllers.putIfAbsent(field['key']!, () {
              final controller = TextEditingController();
              if (existing != null) {
                final v = existing.config[field['key']];
                // Skip masked values (backend returns `EAAx****` for
                // access_token). Hydrating them would let the user save
                // the mask back as if it were the real credential.
                if (v != null && !v.toString().contains('*')) {
                  controller.text = v.toString();
                }
              }
              return controller;
            });
          }

          final isMulti = selectedType.multiFieldConfig.isNotEmpty;
          // A mailbox connected by Google consent has no SMTP host, user or
          // password: rendering the SMTP form for it asked the person to
          // fill fields that do not exist, and submitting them re-validated
          // the channel as hand-typed SMTP — which refuses @gmail.com.
          final isOAuthMailbox = existing?.isGoogleOAuth ?? false;
          // Whether the credential form (and everything that only makes
          // sense with it) is on screen.
          final hasOneClick = !isEditing &&
              (selectedType == ChannelType.email ||
                  selectedType == ChannelType.whatsapp);
          final manualVisible = showManual || !hasOneClick;

          final screenWidth = MediaQuery.of(context).size.width;
          final dialogWidth = screenWidth.clamp(320.0, 480.0);
          return Dialog(
            backgroundColor: colors.backgroundSecondary,
            shape:
                RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
            insetPadding:
                const EdgeInsets.symmetric(horizontal: 16, vertical: 24),
            child: ConstrainedBox(
              constraints: BoxConstraints(
                maxWidth: dialogWidth,
                maxHeight: MediaQuery.of(context).size.height * 0.9,
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Hero header mirrors onboarding's step — circular
                  // accent + title + subtitle.
                  Padding(
                    padding: const EdgeInsets.fromLTRB(24, 24, 24, 0),
                    child: Column(
                      children: [
                        Container(
                          width: 56,
                          height: 56,
                          decoration: BoxDecoration(
                            color: colors.bluePrimary.withValues(alpha: 0.12),
                            shape: BoxShape.circle,
                          ),
                          child: Icon(
                            isEditing ? Icons.tune : Icons.chat_bubble_outline,
                            size: 28,
                            color: colors.bluePrimary,
                          ),
                        ),
                        const SizedBox(height: 12),
                        Text(
                          isEditing
                              ? t.settings.channel_dialog_edit_title
                              : t.settings.channel_dialog_add_title,
                          style: TextStyle(
                            color: colors.textPrimary,
                            fontSize: 20,
                            fontWeight: FontWeight.w700,
                          ),
                          textAlign: TextAlign.center,
                        ),
                        const SizedBox(height: 6),
                        Text(
                          isEditing
                              ? t.settings.channel_dialog_edit_subtitle
                              : t.settings.channel_dialog_add_subtitle,
                          style: TextStyle(
                            color: colors.textSecondary,
                            fontSize: 13,
                            height: 1.4,
                          ),
                          textAlign: TextAlign.center,
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 20),
                  Flexible(
                    child: SingleChildScrollView(
                      padding: const EdgeInsets.symmetric(horizontal: 24),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          // Channel type selector. Two tiles since 2026-08-18:
                          // a hospital-equipment seller reaches her buyers by
                          // email and WhatsApp, and five tiles in a 480px
                          // dialog truncated every label. Existing channels of
                          // the retired types still edit fine — on edit the
                          // one tile shown is the channel's own, locked (type
                          // isn't a safe thing to change under the hood).
                          Row(
                            children: [
                              for (final type in isEditing
                                  ? [existing.type]
                                  : const [
                                      ChannelType.email,
                                      ChannelType.whatsapp
                                    ])
                                Expanded(
                                  child: Padding(
                                    padding: const EdgeInsets.only(right: 12),
                                    child: ChannelTile(
                                      type: type,
                                      isSelected: selectedType == type,
                                      disabled: isEditing,
                                      onTap: () => setDialogState(() {
                                        selectedType = type;
                                        showManual = false;
                                        configController.clear();
                                        multiControllers.clear();
                                      }),
                                    ),
                                  ),
                                ),
                            ],
                          ),
                          const SizedBox(height: 20),

                          // The name is asked only on edit (a rename). On add
                          // it defaults to the channel's own label; nobody
                          // knows what to type in "channel name" before the
                          // channel exists.
                          if (isEditing) ...[
                            _inputField(
                              controller: nameController,
                              label: t.settings.channel_name_label,
                              hint: t.settings.channel_name_hint,
                              colors: colors,
                            ),
                            const SizedBox(height: 12),
                          ],

                          // Primary WhatsApp path: Meta's Embedded Signup
                          // (Tech Provider). Server-side creates the channel
                          // in one round-trip — no manual phone_number_id /
                          // waba_id / token to copy. Hidden on edit because
                          // rotation goes through the WhatsApp screen's
                          // reconnect dialog.
                          if (selectedType == ChannelType.whatsapp &&
                              !isEditing) ...[
                            Center(
                              child: WhatsAppEmbeddedSignupButton(
                                onConnected: () {
                                  Navigator.pop(ctx);
                                  ref
                                      .read(channelsListProvider.notifier)
                                      .refresh();
                                },
                              ),
                            ),
                            const SizedBox(height: 4),
                            TextButton.icon(
                              key: const Key('channel_manual_toggle'),
                              onPressed: () => setDialogState(
                                  () => showManual = !showManual),
                              icon: Icon(
                                showManual
                                    ? Icons.expand_less
                                    : Icons.expand_more,
                                size: 18,
                              ),
                              label: Text(t.settings.channel_or_manual),
                            ),
                            const SizedBox(height: 8),
                          ],

                          // Email offers a path with nothing to type at all.
                          // Above the fields, not beside them: for the person
                          // this is written for they are not two equivalent
                          // options — the fields below mean finding a hostname
                          // and generating an app password, and refuse a
                          // @gmail.com address outright.
                          if (selectedType == ChannelType.email &&
                              !isEditing) ...[
                            const GoogleMailboxConnectCard(),
                            const SizedBox(height: 4),
                            TextButton.icon(
                              key: const Key('channel_manual_toggle'),
                              onPressed: () => setDialogState(
                                  () => showManual = !showManual),
                              icon: Icon(
                                showManual
                                    ? Icons.expand_less
                                    : Icons.expand_more,
                                size: 18,
                              ),
                              label: Text(t.channels.gmail_or_smtp),
                            ),
                            const SizedBox(height: 8),
                          ],

                          // The credential form and the setup note are one
                          // block: the manual path. Shown when asked
                          // for (or on edit, or for a type with no one-click
                          // path). Webchat has no credential and no settings
                          // screen any more (cut 2026-09-01); the dialog
                          // refuses to create one anyway.
                          if (!manualVisible)
                            ...[]
                          else if (selectedType == ChannelType.webchat)
                            ...[]
                          else if (isOAuthMailbox) ...[
                            _GoogleMailboxEditBlock(
                              address: existing?.config['from_address']
                                      ?.toString() ??
                                  existing?.name ??
                                  '',
                              colors: colors,
                            ),
                            const SizedBox(height: 12),
                          ] else if (isMulti)
                            ...selectedType.multiFieldConfig.expand((field) => [
                                  _inputField(
                                    controller: multiControllers[field['key']]!,
                                    label: field['label']!,
                                    hint: field['hint']!,
                                    colors: colors,
                                    obscure: field['obscure'] == 'true',
                                  ),
                                  const SizedBox(height: 12),
                                ])
                          else ...[
                            _inputField(
                              controller: configController,
                              label: selectedType.configLabel,
                              hint: selectedType.configHint,
                              colors: colors,
                            ),
                            const SizedBox(height: 12),
                          ],

                          if (manualVisible) ...[
                            Container(
                              padding: const EdgeInsets.all(12),
                              decoration: BoxDecoration(
                                color: colors.glassMedium,
                                borderRadius: BorderRadius.circular(10),
                                border: Border.all(color: colors.borderLight),
                              ),
                              child: Row(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Icon(Icons.info_outline,
                                      size: 16, color: colors.textMuted),
                                  const SizedBox(width: 8),
                                  Expanded(
                                    child: Text(
                                      selectedType.setupDescription,
                                      style: TextStyle(
                                        color: colors.textSecondary,
                                        fontSize: 12,
                                        height: 1.4,
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                            ),
                            // The HARDEST form of the five (SMTP + ports +
                            // app passwords + SPF/DKIM) was the only one
                            // without a help link — kEmailSetupHelpUrl sat
                            // declared and unused.
                            if (selectedType == ChannelType.email)
                              Align(
                                alignment: Alignment.centerLeft,
                                child: TextButton.icon(
                                  key: const Key('channel_email_help'),
                                  onPressed: () => launchUrl(
                                    Uri.parse(kEmailSetupHelpUrl),
                                    mode: LaunchMode.externalApplication,
                                  ),
                                  icon: Icon(Icons.open_in_new,
                                      size: 14, color: colors.bluePrimary),
                                  label: Text(
                                    t.settings.channel_email_help_link,
                                    style: TextStyle(
                                      color: colors.bluePrimary,
                                      fontSize: 12,
                                    ),
                                  ),
                                ),
                              ),
                            if (selectedType == ChannelType.instagram)
                              Align(
                                alignment: Alignment.centerLeft,
                                child: TextButton.icon(
                                  onPressed: () => launchUrl(
                                    Uri.parse(kInstagramSetupHelpUrl),
                                    mode: LaunchMode.externalApplication,
                                  ),
                                  icon: Icon(Icons.open_in_new,
                                      size: 14, color: colors.bluePrimary),
                                  label: Text(
                                    t.settings.channel_instagram_help_link,
                                    style: TextStyle(
                                      color: colors.bluePrimary,
                                      fontSize: 12,
                                    ),
                                  ),
                                ),
                              ),
                            if (selectedType == ChannelType.sms)
                              Align(
                                alignment: Alignment.centerLeft,
                                child: TextButton.icon(
                                  onPressed: () => launchUrl(
                                    Uri.parse(kSmsSetupHelpUrl),
                                    mode: LaunchMode.externalApplication,
                                  ),
                                  icon: Icon(Icons.open_in_new,
                                      size: 14, color: colors.bluePrimary),
                                  label: Text(
                                    t.settings.channel_sms_help_link,
                                    style: TextStyle(
                                      color: colors.bluePrimary,
                                      fontSize: 12,
                                    ),
                                  ),
                                ),
                              ),
                            // The Twilio webhook URL only exists after the
                            // channel is created (it embeds the channel's
                            // webhook token), so it renders on edit — the
                            // create path points users here via setup_sms +
                            // the card's Webhook URL button.
                            if (selectedType == ChannelType.sms &&
                                isEditing &&
                                existing.webhookToken.isNotEmpty) ...[
                              const SizedBox(height: 12),
                              _SmsWebhookUrlBox(
                                  webhookToken: existing.webhookToken),
                            ],
                          ],
                          const SizedBox(height: 20),
                        ],
                      ),
                    ),
                  ),
                  // Sticky footer with CTAs. The primary button submits the
                  // manual form, so it only exists while that form is on
                  // screen — next to "Conectar com o Google" a second
                  // "Conectar" was a coin flip.
                  Container(
                    padding: const EdgeInsets.fromLTRB(24, 16, 24, 20),
                    decoration: BoxDecoration(
                      border: Border(
                        top: BorderSide(color: colors.borderLight),
                      ),
                    ),
                    child: Row(
                      children: [
                        Expanded(
                          child: TextButton(
                            onPressed: () => Navigator.pop(ctx),
                            style: TextButton.styleFrom(
                              padding: const EdgeInsets.symmetric(vertical: 14),
                              shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(12)),
                            ),
                            child: Text(
                              t.common.cancel,
                              style: TextStyle(
                                  color: colors.textMuted,
                                  fontWeight: FontWeight.w600),
                            ),
                          ),
                        ),
                        if (manualVisible) ...[
                          const SizedBox(width: 12),
                          Expanded(
                            flex: 2,
                            child: ElevatedButton(
                              key: const Key('channel_dialog_submit'),
                              onPressed: () async {
                                // On add the name is the type's own label; on
                                // edit whatever she typed.
                                final name = isEditing
                                    ? nameController.text.trim()
                                    : selectedType.label;
                                if (name.isEmpty) {
                                  // Never a silent return: a button that
                                  // swallows the click reads as broken.
                                  ScaffoldMessenger.of(context)
                                      .showSnackBar(SnackBar(
                                    content:
                                        Text(t.settings.channel_name_required),
                                    behavior: SnackBarBehavior.floating,
                                  ));
                                  return;
                                }

                                // Build config map.
                                //
                                // On EDIT: sensitive fields (auth_token,
                                // access_token…) come back masked and are
                                // skipped during hydration, so an empty one
                                // means "keep the stored credential" — the
                                // key is dropped and the backend preserves
                                // it. If NON-sensitive fields are empty too,
                                // we send an empty config (controller drops
                                // the key; backend keeps the whole blob).
                                final Map<String, String> config;
                                if (selectedType == ChannelType.webchat) {
                                  config = const {};
                                } else if (isOAuthMailbox) {
                                  // Nothing to post: the grant is written by
                                  // the OAuth callback only. An empty map
                                  // drops the key and the backend keeps the
                                  // stored config whole (auth_method included).
                                  config = const {};
                                } else if (isMulti) {
                                  final collected = {
                                    for (final e in multiControllers.entries)
                                      e.key: e.value.text.trim(),
                                  };
                                  // On edit, an empty credential field means
                                  // "keep the stored value" — it came back
                                  // masked and was skipped during hydration.
                                  // Drop the key; the backend preserves the
                                  // stored credential for omitted/masked
                                  // sensitive keys.
                                  if (isEditing) {
                                    collected.removeWhere((k, v) =>
                                        v.isEmpty && isSensitiveConfigKey(k));
                                  }
                                  // Only fields the config declares REQUIRED
                                  // gate the submit. The e-mail form's IMAP
                                  // pair is labelled "(opcional)" — requiring
                                  // it anyway made "Conectar" swallow the
                                  // click for anyone who filled just SMTP.
                                  final optionalKeys = {
                                    for (final f
                                        in selectedType.multiFieldConfig)
                                      if (f['optional'] == 'true') f['key'],
                                  };
                                  collected.removeWhere((k, v) =>
                                      v.isEmpty && optionalKeys.contains(k));
                                  final anyEmpty =
                                      collected.values.any((v) => v.isEmpty);
                                  final anyFilled =
                                      collected.values.any((v) => v.isNotEmpty);
                                  if (isEditing && anyEmpty) {
                                    // Partial credential rotation isn't supported —
                                    // backend replaces the whole config blob. If the
                                    // user filled some fields but left others blank,
                                    // warn them and abort so they don't silently lose
                                    // what they just typed.
                                    if (anyFilled) {
                                      ScaffoldMessenger.of(context)
                                          .showSnackBar(SnackBar(
                                        content: Text(t.settings
                                            .channel_partial_credentials),
                                        backgroundColor: Colors.orange.shade700,
                                        behavior: SnackBarBehavior.floating,
                                      ));
                                      return;
                                    }
                                    config = const {};
                                  } else {
                                    if (anyEmpty) {
                                      final missing = selectedType
                                          .multiFieldConfig
                                          .where((f) =>
                                              f['optional'] != 'true' &&
                                              (collected[f['key']] ?? '')
                                                  .isEmpty)
                                          .map((f) => f['label'])
                                          .join(', ');
                                      ScaffoldMessenger.of(context)
                                          .showSnackBar(SnackBar(
                                        key:
                                            const Key('channel_missing_fields'),
                                        content: Text(t.settings
                                            .channel_required_missing(
                                                fields: missing)),
                                        behavior: SnackBarBehavior.floating,
                                      ));
                                      return;
                                    }
                                    // WhatsApp-specific sanity checks. Meta's IDs are
                                    // strictly numeric; catch paste errors (extra
                                    // quotes, URL, prefix) before hitting the Graph
                                    // API and getting an opaque 400.
                                    if (selectedType == ChannelType.whatsapp) {
                                      final phoneRe = RegExp(r'^\d{12,16}$');
                                      final wabaRe = RegExp(r'^\d{12,18}$');
                                      final phoneId =
                                          collected['phone_number_id'] ?? '';
                                      final wabaId = collected['waba_id'] ?? '';
                                      // access_token may be absent on edit
                                      // (kept credential) — only validate a
                                      // value the user actually typed.
                                      final token = collected['access_token'];
                                      String? fieldError;
                                      if (!phoneRe.hasMatch(phoneId)) {
                                        fieldError =
                                            t.settings.channel_invalid_phone_id;
                                      } else if (!wabaRe.hasMatch(wabaId)) {
                                        fieldError =
                                            t.settings.channel_invalid_waba_id;
                                      } else if (token != null &&
                                          token.length < 20) {
                                        fieldError =
                                            t.settings.channel_invalid_token;
                                      }
                                      if (fieldError != null) {
                                        ScaffoldMessenger.of(context)
                                            .showSnackBar(SnackBar(
                                          content: Text(fieldError),
                                          backgroundColor: Colors.red.shade700,
                                          behavior: SnackBarBehavior.floating,
                                        ));
                                        return;
                                      }
                                    }
                                    // Twilio sanity checks. The SID has a fixed
                                    // shape and the number must be E.164 —
                                    // catch paste errors before Twilio rejects
                                    // the first send with an opaque 21xxx code.
                                    if (selectedType == ChannelType.sms) {
                                      final sidRe =
                                          RegExp(r'^AC[0-9a-fA-F]{32}$');
                                      final e164Re =
                                          RegExp(r'^\+[1-9]\d{7,14}$');
                                      final sid =
                                          collected['account_sid'] ?? '';
                                      final from =
                                          collected['from_number'] ?? '';
                                      String? fieldError;
                                      if (!sidRe.hasMatch(sid)) {
                                        fieldError = t.settings
                                            .channel_invalid_account_sid;
                                      } else if (!e164Re.hasMatch(from)) {
                                        fieldError = t.settings
                                            .channel_invalid_from_number;
                                      }
                                      if (fieldError != null) {
                                        ScaffoldMessenger.of(context)
                                            .showSnackBar(SnackBar(
                                          content: Text(fieldError),
                                          backgroundColor: Colors.red.shade700,
                                          behavior: SnackBarBehavior.floating,
                                        ));
                                        return;
                                      }
                                    }
                                    config = collected;
                                  }
                                } else {
                                  final val = configController.text.trim();
                                  if (val.isEmpty && !isEditing) {
                                    ScaffoldMessenger.of(context)
                                        .showSnackBar(SnackBar(
                                      content: Text(t.settings
                                          .channel_required_missing(
                                              fields: selectedType.label)),
                                      behavior: SnackBarBehavior.floating,
                                    ));
                                    return;
                                  }
                                  config = val.isEmpty
                                      ? const {}
                                      : {selectedType.configKey!: val};
                                }

                                Navigator.pop(ctx);

                                final notifier =
                                    ref.read(channelsListProvider.notifier);
                                String? error;
                                Object? rawError;
                                if (isEditing) {
                                  error = await notifier.updateChannel(
                                    id: existing.id,
                                    name: name,
                                    config: config,
                                  );
                                } else {
                                  final r = await notifier.addChannel(
                                    name: name,
                                    type: selectedType,
                                    config: config,
                                  );
                                  error = r.error;
                                  rawError = r.raw;
                                }
                                if (!context.mounted) return;
                                // PLAN_LIMIT_EXCEEDED path: the helper pops the upgrade
                                // modal + returns a resource-aware message; skip the
                                // snackbar dance for the create path when it fires.
                                final planMsg =
                                    handlePlanLimitError(context, rawError);
                                ScaffoldMessenger.of(context)
                                    .showSnackBar(SnackBar(
                                  content: Text(planMsg ??
                                      error ??
                                      (isEditing
                                          ? t.settings.channel_updated_success
                                          : t.settings
                                              .channel_created_success)),
                                  backgroundColor:
                                      (planMsg != null || error != null)
                                          ? Colors.red.shade700
                                          : null,
                                  behavior: SnackBarBehavior.floating,
                                ));
                              },
                              style: ElevatedButton.styleFrom(
                                backgroundColor: colors.bluePrimary,
                                foregroundColor: Colors.white,
                                padding:
                                    const EdgeInsets.symmetric(vertical: 14),
                                shape: RoundedRectangleBorder(
                                    borderRadius: BorderRadius.circular(12)),
                              ),
                              child: Text(
                                isEditing
                                    ? t.settings.channel_save_btn
                                    : t.settings.channel_connect_btn,
                                style: const TextStyle(
                                    fontSize: 14, fontWeight: FontWeight.w700),
                              ),
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    ).whenComplete(() {
      nameController.dispose();
      configController.dispose();
      for (final c in multiControllers.values) {
        c.dispose();
      }
    });
  }

  Widget _inputField({
    required TextEditingController controller,
    required String label,
    required String hint,
    required AppColorsExtension colors,
    bool autofocus = false,
    bool obscure = false,
  }) {
    return TextField(
      controller: controller,
      autofocus: autofocus,
      obscureText: obscure,
      style: TextStyle(color: colors.textPrimary),
      decoration: InputDecoration(
        labelText: label,
        labelStyle: TextStyle(color: colors.textMuted),
        hintText: hint,
        hintStyle: TextStyle(color: colors.textDisabled, fontSize: 12),
      ),
    );
  }
}

/// Individual channel card widget with test connection button.
class _ChannelCard extends ConsumerStatefulWidget {
  final ChannelModel channel;
  final VoidCallback onEdit;

  const _ChannelCard({required this.channel, required this.onEdit});

  @override
  ConsumerState<_ChannelCard> createState() => _ChannelCardState();
}

class _ChannelCardState extends ConsumerState<_ChannelCard> {
  bool _isTesting = false;

  (Color, IconData) _channelStyle(ChannelType type) {
    switch (type) {
      case ChannelType.whatsapp:
        return (Colors.green.shade600, Icons.chat);
      case ChannelType.telegram:
        return (Colors.blue.shade500, Icons.send);
      case ChannelType.instagram:
        return (Colors.purple.shade400, Icons.camera_alt);
      case ChannelType.webchat:
        return (const Color(0xFF4F46E5), Icons.language);
      case ChannelType.sms:
        return (ChannelType.sms.brandColor, Icons.sms_outlined);
      case ChannelType.email:
        return (ChannelType.email.brandColor, Icons.alternate_email);
    }
  }

  /// Brand-accurate icon for the channel type. Uses the SVG logos already
  /// shipped under assets/ (whatsapp_logo, telegram_logo, instagram_logo)
  /// instead of the generic Material glyphs — the old Icons.chat /
  /// Icons.send pair looked nothing like the actual apps and confused
  /// users scanning the channel list.
  Widget _channelIconWidget(ChannelType type, {double size = 22, Color? tint}) {
    final asset = switch (type) {
      ChannelType.whatsapp => 'assets/whatsapp_logo.svg',
      ChannelType.telegram => 'assets/telegram_logo.svg',
      ChannelType.instagram => 'assets/instagram_logo.svg',
      ChannelType.sms => 'assets/sms_logo.svg',
      ChannelType.webchat => null,
      ChannelType.email => null,
    };
    if (asset == null) {
      final (color, icon) = _channelStyle(type);
      return Icon(icon, color: tint ?? color, size: size);
    }
    return SvgPicture.asset(asset, width: size, height: size);
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final channel = widget.channel;
    final (color, icon) = _channelStyle(channel.type);

    return Container(
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header row
          Row(
            children: [
              Container(
                width: 42,
                height: 42,
                decoration: BoxDecoration(
                  color: color.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(10),
                ),
                alignment: Alignment.center,
                child: _channelIconWidget(channel.type, size: 22, tint: color),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      channel.name,
                      style: TextStyle(
                        color: colors.textPrimary,
                        fontWeight: FontWeight.w700,
                        fontSize: 15,
                      ),
                    ),
                    Row(
                      children: [
                        Container(
                          width: 6,
                          height: 6,
                          decoration: BoxDecoration(
                            color: channel.isActive
                                ? colors.whatsappGreen
                                : colors.textDisabled,
                            shape: BoxShape.circle,
                          ),
                        ),
                        const SizedBox(width: 5),
                        Text(
                          channel.isActive
                              ? t.settings.channel_status_connected
                              : t.settings.channel_status_disabled,
                          style: TextStyle(
                            color: channel.isActive
                                ? colors.whatsappGreen
                                : colors.textDisabled,
                            fontSize: 12,
                          ),
                        ),
                        // Incomplete badge: WhatsApp channel missing
                        // waba_id/access_token can't list templates or
                        // send messages. Tap to reopen the Conectar
                        // WhatsApp popup (Embedded Signup) or to
                        // re-enter credentials manually.
                        if (!channel.isFullyConfigured) ...[
                          const SizedBox(width: 8),
                          Container(
                            padding: const EdgeInsets.symmetric(
                                horizontal: 6, vertical: 2),
                            decoration: BoxDecoration(
                              color: colors.orangeAlert.withValues(alpha: 0.15),
                              borderRadius: BorderRadius.circular(4),
                              border: Border.all(
                                  color:
                                      colors.orangeAlert.withValues(alpha: 0.4),
                                  width: 0.5),
                            ),
                            child: Text(
                              t.settings.channel_status_incomplete,
                              style: TextStyle(
                                color: colors.orangeAlert,
                                fontSize: 10,
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                          ),
                        ],
                        // Reconnect badge: the IMAP poller is failing on
                        // this mailbox (revoked Google grant, changed
                        // password), so replies are not being read. The
                        // tooltip carries the provider's own words.
                        if (channel.needsReconnect) ...[
                          const SizedBox(width: 8),
                          Tooltip(
                            message: t.settings.channel_imap_failing(
                              count: channel.imapFailureCount.toString(),
                              error: channel.imapLastError,
                            ),
                            child: Container(
                              key: const Key('channel_imap_reconnect_badge'),
                              padding: const EdgeInsets.symmetric(
                                  horizontal: 6, vertical: 2),
                              decoration: BoxDecoration(
                                color: colors.redAlert.withValues(alpha: 0.15),
                                borderRadius: BorderRadius.circular(4),
                                border: Border.all(
                                    color:
                                        colors.redAlert.withValues(alpha: 0.4),
                                    width: 0.5),
                              ),
                              child: Text(
                                t.settings.channel_reconnect_action,
                                style: TextStyle(
                                  color: colors.redAlert,
                                  fontSize: 10,
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                            ),
                          ),
                        ],
                        // Webhook health — only polled for channels
                        // that have a credential (otherwise the
                        // endpoint short-circuits to false). Silent on
                        // loading/unknown, red "Webhook down" when
                        // unhealthy.
                        if (channel.isFullyConfigured)
                          _WebhookHealthBadge(channelId: channel.id),
                      ],
                    ),
                    // Email warmup (Item 1): the send gate fails closed on
                    // this state — so it cannot render as an 11px grey
                    // whisper. Blocked is RED (a total send block with an
                    // action to take); ramping is amber; done recedes.
                    if (channel.emailWarmup != null) ...[
                      const SizedBox(height: 4),
                      Builder(builder: (context) {
                        final w = channel.emailWarmup!;
                        final tone = !w.started
                            ? colors.redAlert
                            : (w.warming
                                ? colors.orangeAlert
                                : colors.textMuted);
                        return Row(
                          children: [
                            if (!w.started || w.warming) ...[
                              Icon(
                                  !w.started
                                      ? Icons.block
                                      : Icons.local_fire_department_outlined,
                                  size: 13,
                                  color: tone),
                              const SizedBox(width: 4),
                            ],
                            Expanded(
                              child: Text(
                                emailWarmupLabel(w),
                                key: Key('channel_warmup_${channel.id}'),
                                style: TextStyle(
                                    color: tone,
                                    fontSize: 11.5,
                                    fontWeight: !w.started
                                        ? FontWeight.w700
                                        : FontWeight.w400),
                              ),
                            ),
                          ],
                        );
                      }),
                    ],
                  ],
                ),
              ),
              Switch(
                value: channel.isActive,
                onChanged: (_) async {
                  final error = await ref
                      .read(channelsListProvider.notifier)
                      .toggleChannel(channel.id);
                  if (error != null && context.mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                      content: Text(error),
                      backgroundColor: Colors.red.shade700,
                      behavior: SnackBarBehavior.floating,
                    ));
                  }
                },
                activeColor: colors.whatsappGreen,
              ),
              IconButton(
                icon: Icon(Icons.edit_outlined,
                    color: colors.textMuted, size: 20),
                onPressed: widget.onEdit,
                splashRadius: 18,
                tooltip: t.settings.channel_edit_tooltip,
              ),
              IconButton(
                icon: Icon(Icons.delete_outline,
                    color: colors.redAlert, size: 20),
                onPressed: () => _confirmDelete(context, ref, colors, channel),
                splashRadius: 18,
              ),
            ],
          ),

          const SizedBox(height: 12),
          Divider(color: colors.borderLight, height: 1),
          const SizedBox(height: 12),

          // Actions row
          Row(
            children: [
              // Test connection button
              Expanded(
                child: OutlinedButton.icon(
                  onPressed: _isTesting ? null : () => _testConnection(colors),
                  icon: _isTesting
                      ? SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: colors.textMuted,
                          ),
                        )
                      : Icon(Icons.wifi_tethering, size: 16, color: color),
                  label: Text(
                    _isTesting
                        ? t.settings.channel_testing
                        : t.settings.channel_test_btn,
                    style: const TextStyle(fontSize: 12),
                  ),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: color,
                    side: BorderSide(color: colors.borderMedium),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(8),
                    ),
                    padding: const EdgeInsets.symmetric(vertical: 10),
                  ),
                ),
              ),
              if (channel.type == ChannelType.whatsapp) ...[
                const SizedBox(width: 8),
                Expanded(
                  child: OutlinedButton.icon(
                    onPressed: () =>
                        context.go('/settings/channels/${channel.id}/whatsapp'),
                    icon: Icon(Icons.settings_outlined,
                        size: 16, color: colors.whatsappGreen),
                    label: Text(
                      t.settings.channel_configure_btn,
                      style:
                          TextStyle(fontSize: 12, color: colors.whatsappGreen),
                    ),
                    style: OutlinedButton.styleFrom(
                      side: BorderSide(color: colors.borderMedium),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(8),
                      ),
                      padding: const EdgeInsets.symmetric(vertical: 10),
                    ),
                  ),
                ),
              ],
              // Twilio needs the channel's webhook URL pasted on the
              // number ("A message comes in") — one-tap copy from the
              // card so the admin doesn't have to reopen the dialog.
              if (channel.type == ChannelType.sms &&
                  channel.webhookToken.isNotEmpty) ...[
                const SizedBox(width: 8),
                Expanded(
                  child: OutlinedButton.icon(
                    onPressed: () async {
                      await Clipboard.setData(ClipboardData(
                          text: channelWebhookUrl(channel.webhookToken)));
                      if (!context.mounted) return;
                      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                        content: Text(t.settings.channel_webhook_url_copied),
                        behavior: SnackBarBehavior.floating,
                      ));
                    },
                    icon: Icon(Icons.link,
                        size: 16, color: ChannelType.sms.brandColor),
                    label: Text(
                      t.settings.channel_webhook_url_btn,
                      style: TextStyle(
                          fontSize: 12, color: ChannelType.sms.brandColor),
                    ),
                    style: OutlinedButton.styleFrom(
                      side: BorderSide(color: colors.borderMedium),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(8),
                      ),
                      padding: const EdgeInsets.symmetric(vertical: 10),
                    ),
                  ),
                ),
              ],
            ],
          ),
          // Authentication is what decides whether this channel delivers at
          // all, so it sits on the card rather than behind another screen.
          if (channel.type == ChannelType.email) ...[
            EmailDnsCard(channelId: channel.id),
            // The way in to "traz os clientes que você já tem". It lives on the
            // channel card because that is where the mailbox was connected, and
            // the mailbox is what the scan reads — putting it under a menu
            // elsewhere would make the connection between the two invisible.
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Align(
                alignment: Alignment.centerLeft,
                child: TextButton.icon(
                  key: Key('discover_customers_${channel.id}'),
                  onPressed: () => context.push('/funnel?view=inbox'),
                  icon: const Icon(Icons.group_add_outlined, size: 16),
                  // Navigation label, not the scan verb: this button OPENS the
                  // screen whose own button then scans — two identical labels
                  // made the first tap look like it did nothing.
                  label: Text(t.my_customers.open_inbox_customers),
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }

  Future<void> _testConnection(AppColorsExtension colors) async {
    setState(() => _isTesting = true);

    final result = await ref
        .read(channelsListProvider.notifier)
        .testChannel(widget.channel.id);

    if (!mounted) return;
    setState(() => _isTesting = false);

    final ok = result?['ok'] == true;
    final botName = result?['bot_name'] as String?;
    final displayNumber = result?['display_number'] as String?;
    final successLabel = botName ?? displayNumber;
    final emailMessage = ok ? emailTestMessage(result) : null;
    final errorMsg =
        result?['error'] as String? ?? t.settings.channel_test_unknown_error;
    // Identify "needs reconnect" errors so the snackbar can offer the
    // Embedded Signup flow inline instead of forcing the user back to
    // an empty channel list. Matches the pt-BR copy emitted by the
    // backend's MISSING_TOKEN / MISSING_PHONE_NUMBER_ID codes via
    // WhatsAppError.humanize().
    final needsReconnect = !ok &&
        widget.channel.type == ChannelType.whatsapp &&
        errorMsg.toLowerCase().contains('reconecte');

    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Row(
        children: [
          Icon(
            ok ? Icons.check_circle : Icons.error_outline,
            color: Colors.white,
            size: 18,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              ok
                  ? (emailMessage ??
                      (successLabel != null
                          ? t.settings
                              .channel_test_success_named(label: successLabel)
                          : t.settings.channel_test_success))
                  : t.settings.channel_test_failed(error: errorMsg),
            ),
          ),
        ],
      ),
      backgroundColor: ok ? Colors.green.shade700 : Colors.red.shade700,
      behavior: SnackBarBehavior.floating,
      duration: needsReconnect
          ? const Duration(seconds: 10)
          : const Duration(seconds: 4),
      action: needsReconnect
          ? SnackBarAction(
              label: t.settings.channel_reconnect_action,
              textColor: Colors.white,
              onPressed: () {
                // Route to the channels empty state flow — the big
                // Conectar WhatsApp via Meta button lives there. We
                // don't trigger the popup inline because a SnackBar
                // action runs outside the widget tree where the
                // signup helper expects a ref.
                if (mounted) context.go('/settings/channels');
              },
            )
          : null,
    ));
  }

  void _confirmDelete(
    BuildContext context,
    WidgetRef ref,
    AppColorsExtension colors,
    ChannelModel channel,
  ) {
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: colors.backgroundSecondary,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: Text(t.channels.delete_channel,
            style: TextStyle(color: colors.textPrimary)),
        content: Text(
          t.settings.channel_delete_confirm(name: channel.name),
          style: TextStyle(color: colors.textSecondary, fontSize: 14),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text(t.common.cancel,
                style: TextStyle(color: colors.textMuted)),
          ),
          ElevatedButton(
            onPressed: () async {
              Navigator.pop(ctx);
              final error = await ref
                  .read(channelsListProvider.notifier)
                  .deleteChannel(channel.id);
              if (context.mounted) {
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                  content: Text(error ?? t.settings.channel_deleted_success),
                  backgroundColor: error != null ? Colors.red.shade700 : null,
                  behavior: SnackBarBehavior.floating,
                ));
              }
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: colors.redAlert,
              foregroundColor: Colors.white,
            ),
            child: Text(t.common.delete),
          ),
        ],
      ),
    );
  }
}

/// Read-only webhook URL with a copy button, shown when editing an SMS
/// channel. The admin pastes this into the Twilio console as the
/// "A message comes in" webhook for their number — inbound texts don't
/// flow until they do.
/// What the edit dialog shows in place of the SMTP form for a mailbox
/// connected by Google consent: the address, why there is nothing to type,
/// and the one action that fixes a dead grant — consenting again. The
/// reconnect goes through the same endpoint the connect card uses; the
/// callback merges the new grant into THIS channel (same address).
class _GoogleMailboxEditBlock extends ConsumerStatefulWidget {
  const _GoogleMailboxEditBlock({required this.address, required this.colors});
  final String address;
  final AppColorsExtension colors;

  @override
  ConsumerState<_GoogleMailboxEditBlock> createState() =>
      _GoogleMailboxEditBlockState();
}

class _GoogleMailboxEditBlockState
    extends ConsumerState<_GoogleMailboxEditBlock> {
  bool _busy = false;

  Future<void> _reconnect() async {
    setState(() => _busy = true);
    try {
      final res = await ref.read(dioProvider).post<Map<String, dynamic>>(
            '/channels/email/google/connect',
          );
      final url = res.data?['auth_url'] as String?;
      if (url == null || url.isEmpty) {
        throw DioException(
          requestOptions:
              RequestOptions(path: '/channels/email/google/connect'),
          message: 'no auth_url',
        );
      }
      await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(parseApiError(e)),
        backgroundColor: Colors.red.shade700,
        behavior: SnackBarBehavior.floating,
      ));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = widget.colors;
    return Container(
      key: const Key('channel_google_edit_block'),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: colors.glassMedium,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.check_circle_outline,
                  size: 18, color: colors.whatsappGreen),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  t.settings.channel_google_connected(address: widget.address),
                  style: TextStyle(
                      color: colors.textPrimary, fontWeight: FontWeight.w600),
                ),
              ),
            ],
          ),
          const SizedBox(height: 6),
          Text(
            t.settings.channel_google_edit_hint,
            style: TextStyle(color: colors.textMuted, fontSize: 12.5),
          ),
          const SizedBox(height: 12),
          OutlinedButton.icon(
            key: const Key('channel_google_reconnect'),
            onPressed: _busy ? null : _reconnect,
            icon: _busy
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.refresh, size: 16),
            label: Text(t.settings.channel_google_reconnect),
          ),
        ],
      ),
    );
  }
}

class _SmsWebhookUrlBox extends StatelessWidget {
  final String webhookToken;
  const _SmsWebhookUrlBox({required this.webhookToken});

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final url = channelWebhookUrl(webhookToken);
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.glassMedium,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: colors.borderLight),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.settings.channel_webhook_url_label,
            style: TextStyle(
              color: colors.textSecondary,
              fontSize: 12,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 6),
          Row(
            children: [
              Expanded(
                child: SelectableText(
                  url,
                  style: TextStyle(
                    color: colors.textPrimary,
                    fontSize: 12,
                    fontFamily: 'monospace',
                  ),
                ),
              ),
              IconButton(
                icon: Icon(Icons.copy, size: 16, color: colors.bluePrimary),
                tooltip: t.common.copy,
                splashRadius: 16,
                onPressed: () async {
                  await Clipboard.setData(ClipboardData(text: url));
                  if (!context.mounted) return;
                  ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                    content: Text(t.settings.channel_webhook_url_copied),
                    behavior: SnackBarBehavior.floating,
                  ));
                },
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            t.settings.channel_webhook_url_help,
            style: TextStyle(
              color: colors.textMuted,
              fontSize: 11,
              height: 1.4,
            ),
          ),
        ],
      ),
    );
  }
}

/// Tiny inline badge that flips the channel row from "just connected"
/// to "webhook is broken, attention needed" — without the attendant
/// having to open the channel's detail screen. Polls the existing
/// GET /channels/:id/webhook-status via channelWebhookHealthProvider.
/// Webhook health indicator for the channel card. When unhealthy,
/// the badge is tappable: it fires POST /channels/:id/webhook-register
/// to re-register Telegram's setWebhook (or Meta's subscribed_apps for
/// WhatsApp) so the user can self-serve the fix without copy/pasting
/// curl from support docs.
class _WebhookHealthBadge extends ConsumerStatefulWidget {
  final String channelId;
  const _WebhookHealthBadge({required this.channelId});

  @override
  ConsumerState<_WebhookHealthBadge> createState() =>
      _WebhookHealthBadgeState();
}

class _WebhookHealthBadgeState extends ConsumerState<_WebhookHealthBadge> {
  bool _registering = false;

  Future<void> _registerWebhook() async {
    if (_registering) return;
    setState(() => _registering = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final dio = ref.read(dioProvider);
      await dio.post<dynamic>('/channels/${widget.channelId}/webhook-register');
      // Nudge the health check to re-run now that the server-side
      // webhook was rewritten.
      ref.invalidate(channelWebhookHealthProvider(widget.channelId));
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(
        content: Text(t.settings.channel_webhook_re_registered),
        behavior: SnackBarBehavior.floating,
      ));
    } catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(
        content: Text(t.settings
            .channel_webhook_register_failed(error: parseApiError(e))),
        backgroundColor: Colors.red.shade700,
        behavior: SnackBarBehavior.floating,
      ));
    } finally {
      if (mounted) setState(() => _registering = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final async = ref.watch(channelWebhookHealthProvider(widget.channelId));
    return async.when(
      loading: () => const SizedBox.shrink(),
      // A failed health check is NOT a healthy channel. Hiding the badge on
      // error made "the webhook is fine" and "I couldn't find out" render
      // identically — on the one indicator that warns the account its WhatsApp
      // number is about to stop receiving messages.
      error: (_, __) => _UnknownHealthBadge(
        colors: colors,
        onRetry: () =>
            ref.invalidate(channelWebhookHealthProvider(widget.channelId)),
      ),
      data: (healthy) {
        // Same for a null: the backend declining to answer is not a pass.
        if (healthy == null) {
          return _UnknownHealthBadge(
            colors: colors,
            onRetry: () =>
                ref.invalidate(channelWebhookHealthProvider(widget.channelId)),
          );
        }
        if (healthy) return const SizedBox.shrink();
        return Padding(
          padding: const EdgeInsets.only(left: 8),
          child: InkWell(
            onTap: _registering ? null : _registerWebhook,
            borderRadius: BorderRadius.circular(4),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
              decoration: BoxDecoration(
                color: colors.redAlert.withValues(alpha: 0.15),
                borderRadius: BorderRadius.circular(4),
                border: Border.all(
                    color: colors.redAlert.withValues(alpha: 0.4), width: 0.5),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (_registering)
                    SizedBox(
                      width: 10,
                      height: 10,
                      child: CircularProgressIndicator(
                        strokeWidth: 1.5,
                        color: colors.redAlert,
                      ),
                    )
                  else
                    Icon(Icons.refresh, size: 11, color: colors.redAlert),
                  const SizedBox(width: 4),
                  Text(
                    t.channels.webhook_down,
                    style: TextStyle(
                      color: colors.redAlert,
                      fontSize: 10,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}

/// "I could not check this" — deliberately not the same as "this is fine".
///
/// Amber rather than red: the channel may well be healthy. What is certain is
/// that nobody knows, and the person responsible for the number is entitled to
/// see the difference. Tapping asks again.
class _UnknownHealthBadge extends StatelessWidget {
  const _UnknownHealthBadge({required this.colors, required this.onRetry});

  final AppColorsExtension colors;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(left: 8),
      child: InkWell(
        onTap: onRetry,
        borderRadius: BorderRadius.circular(4),
        child: Container(
          key: const Key('webhook_health_unknown'),
          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
          decoration: BoxDecoration(
            color: colors.orangeAlert.withValues(alpha: 0.15),
            borderRadius: BorderRadius.circular(4),
            border: Border.all(
                color: colors.orangeAlert.withValues(alpha: 0.4), width: 0.5),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.help_outline, size: 11, color: colors.orangeAlert),
              const SizedBox(width: 4),
              Text(
                t.channels.webhook_health_unknown,
                style: TextStyle(
                  color: colors.orangeAlert,
                  fontSize: 10,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// What to tell the operator after an email channel's connection test.
///
/// Email says more than "ok", because for a channel sending through the
/// customer's own provider the test IS the authentication: it reports where the
/// probe landed (their own inbox — nobody outside the account was contacted)
/// and how many approaches a day it just unlocked. A bare checkmark would leave
/// them guessing whether they may send.
///
/// Returns null for a response that carries no email fields, so the caller
/// falls back to the generic copy and other channel types are untouched.
String? emailTestMessage(Map<String, dynamic>? result) {
  final sentTo = result?['sent_to'] as String?;
  if (sentTo == null || sentTo.isEmpty) return null;

  final cap = (result?['daily_cap'] as num?)?.toInt() ?? 0;
  final unlocked = result?['cold_send_unlocked'] == true;
  // The send worked and the release did not register: a different situation
  // from success. Reporting it as success would promise sending that the next
  // approach refuses, with nothing on screen to explain the contradiction.
  if (!unlocked || cap <= 0) {
    return t.settings.channel_test_email_not_unlocked(to: sentTo);
  }
  return t.settings.channel_test_email_success(to: sentTo, cap: cap.toString());
}

/// One line of warmup truth for an email channel, from the same numbers the
/// send gate applies. Top-level so the approval queue's banner and any test
/// can reuse it.
String emailWarmupLabel(EmailWarmupStatus w) {
  if (!w.started) {
    return t.settings.email_warmup_not_started;
  }
  if (w.warming) {
    // Both halves of the day: how far the ramp is AND how much of today's
    // allowance was used. "restam 8" without "de quantos" could not say
    // whether the day was productive or the ramp is tiny.
    return t.settings.email_warmup_progress(
      day: '${w.day}',
      total: '${w.rampDays}',
      sent: '${w.sentToday}',
      cap: '${w.dailyCap}',
    );
  }
  return t.settings.email_warmup_done(cap: '${w.dailyCap}');
}
