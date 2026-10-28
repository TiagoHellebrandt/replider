import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/settings/domain/models/claude_integration_models.dart';
import 'package:app/features/settings/presentation/controllers/claude_integration_controller.dart';
import 'package:app/i18n/strings.g.dart';

// Production fallback, same rule as channelWebhookUrl: a build missing the
// define must never print "localhost" into the copy box the customer pastes
// into `claude mcp add`.
const _apiBaseUrl = String.fromEnvironment(
  'API_BASE_URL',
  defaultValue: 'https://api.replider.com/v1',
);

/// The MCP endpoint is the API origin + /mcp (the /v1 suffix of the
/// REST base URL is not part of the MCP mount).
String get _mcpUrl {
  var origin = _apiBaseUrl;
  if (origin.endsWith('/v1')) {
    origin = origin.substring(0, origin.length - 3);
  }
  if (origin.endsWith('/')) {
    origin = origin.substring(0, origin.length - 1);
  }
  return '$origin/mcp';
}

/// Settings → Claude (MCP): connect instructions, personal access
/// tokens and active OAuth connections.
class ClaudeIntegrationScreen extends ConsumerWidget {
  const ClaudeIntegrationScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final state = ref.watch(claudeIntegrationProvider);

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        leading: IconButton(
          icon: Icon(Icons.arrow_back, color: colors.textPrimary),
          tooltip: t.claude_integration.back_tooltip,
          onPressed: () => context.go('/settings'),
        ),
        title: Text(
          t.claude_integration.title,
          style:
              TextStyle(color: colors.textPrimary, fontWeight: FontWeight.w600),
        ),
      ),
      body: state.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(
          child: Text(
            t.claude_integration.load_failed(error: parseApiError(e)),
            style: TextStyle(color: colors.textMuted),
          ),
        ),
        data: (data) => RefreshIndicator(
          onRefresh: () => ref.refresh(claudeIntegrationProvider.future),
          child: ListView(
            padding: const EdgeInsets.all(24),
            children: [
              Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 640),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      _ConnectCard(colors: colors),
                      const SizedBox(height: 24),
                      _ConnectionsCard(
                          colors: colors, connections: data.connections),
                      const SizedBox(height: 24),
                      _AdvancedSection(colors: colors, tokens: data.tokens),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ConnectCard extends StatelessWidget {
  final AppColorsExtension colors;
  const _ConnectCard({required this.colors});

  @override
  Widget build(BuildContext context) {
    return _card(
      colors: colors,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.claude_integration.connect_title,
            style: TextStyle(
                color: colors.textPrimary,
                fontWeight: FontWeight.w600,
                fontSize: 16),
          ),
          const SizedBox(height: 8),
          Text(
            t.claude_integration.connect_subtitle,
            style: TextStyle(color: colors.textMuted, fontSize: 13),
          ),
          const SizedBox(height: 16),
          // Passo 1: copiar o endereço.
          _StepLabel(
              colors: colors,
              number: 1,
              text: t.claude_integration.connect_step1),
          const SizedBox(height: 6),
          _copyRow(context, colors, _mcpUrl),
          const SizedBox(height: 14),
          // Passo 2: adicionar no Claude.
          _StepLabel(
              colors: colors,
              number: 2,
              text: t.claude_integration.connect_step2),
          const SizedBox(height: 14),
          // Passo 3: autorizar com email/senha.
          _StepLabel(
              colors: colors,
              number: 3,
              text: t.claude_integration.connect_step3),
          const SizedBox(height: 14),
          // Callout: no token on the non-technical path.
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
            decoration: BoxDecoration(
              color: colors.bluePrimary.withValues(alpha: 0.10),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(Icons.lock_outline, size: 16, color: colors.bluePrimary),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    t.claude_integration.connect_no_token_note,
                    style: TextStyle(color: colors.textSecondary, fontSize: 12),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Step number badge + label for the numbered connect flow.
class _StepLabel extends StatelessWidget {
  final AppColorsExtension colors;
  final int number;
  final String text;
  const _StepLabel(
      {required this.colors, required this.number, required this.text});

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: 22,
          height: 22,
          alignment: Alignment.center,
          decoration: BoxDecoration(
            color: colors.bluePrimary.withValues(alpha: 0.15),
            shape: BoxShape.circle,
          ),
          child: Text(
            '$number',
            style: TextStyle(
                color: colors.bluePrimary,
                fontWeight: FontWeight.w700,
                fontSize: 12),
          ),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Padding(
            padding: const EdgeInsets.only(top: 2),
            child: Text(
              text,
              style: TextStyle(color: colors.textPrimary, fontSize: 13),
            ),
          ),
        ),
      ],
    );
  }
}

/// Advanced section: Claude Code connection (needs a token). Hidden
/// below the lay-user claude.ai flow so non-technical users aren't
/// nudged into creating tokens they don't need.
class _AdvancedSection extends ConsumerWidget {
  final AppColorsExtension colors;
  final List<PatModel> tokens;
  const _AdvancedSection({required this.colors, required this.tokens});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Primary Claude Code path: no token — Claude Code opens the browser
    // for OAuth login and stores the token itself.
    final oauthCommand = 'claude mcp add --transport http replider $_mcpUrl';
    // Fallback for headless / CI where a browser login isn't possible.
    final tokenCommand =
        'claude mcp add --transport http replider $_mcpUrl --header "Authorization: Bearer SEU_TOKEN"';
    return _card(
      colors: colors,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.claude_integration.advanced_title,
            style: TextStyle(
                color: colors.textPrimary,
                fontWeight: FontWeight.w600,
                fontSize: 16),
          ),
          const SizedBox(height: 4),
          Text(
            t.claude_integration.advanced_subtitle,
            style: TextStyle(color: colors.textMuted, fontSize: 13),
          ),
          const SizedBox(height: 14),
          Text(
            t.claude_integration.cc_oauth_label,
            style: TextStyle(
                color: colors.textPrimary,
                fontWeight: FontWeight.w600,
                fontSize: 13),
          ),
          const SizedBox(height: 6),
          _copyRow(context, colors, oauthCommand),
          const SizedBox(height: 4),
          Text(
            t.claude_integration.cc_oauth_hint,
            style: TextStyle(color: colors.textMuted, fontSize: 12),
          ),
          const SizedBox(height: 20),
          Text(
            t.claude_integration.cc_token_label,
            style: TextStyle(
                color: colors.textPrimary,
                fontWeight: FontWeight.w600,
                fontSize: 13),
          ),
          const SizedBox(height: 6),
          _copyRow(context, colors, tokenCommand),
          const SizedBox(height: 4),
          Text(
            t.claude_integration.cc_token_hint,
            style: TextStyle(color: colors.textMuted, fontSize: 12),
          ),
          const SizedBox(height: 20),
          Row(
            children: [
              Expanded(
                child: Text(
                  t.claude_integration.tokens_title,
                  style: TextStyle(
                      color: colors.textPrimary,
                      fontWeight: FontWeight.w600,
                      fontSize: 14),
                ),
              ),
              TextButton.icon(
                onPressed: () => _createToken(context, ref),
                icon: const Icon(Icons.add, size: 18),
                label: Text(t.claude_integration.tokens_create),
              ),
            ],
          ),
          Text(
            t.claude_integration.tokens_subtitle,
            style: TextStyle(color: colors.textMuted, fontSize: 13),
          ),
          const SizedBox(height: 8),
          if (tokens.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 12),
              child: Text(
                t.claude_integration.tokens_empty,
                style: TextStyle(color: colors.textMuted, fontSize: 13),
              ),
            ),
          for (final token in tokens) ...[
            Divider(height: 1, color: colors.borderLight),
            ListTile(
              contentPadding: EdgeInsets.zero,
              title: Text(
                token.name,
                style: TextStyle(
                    color: colors.textPrimary, fontWeight: FontWeight.w500),
              ),
              subtitle: Text(
                '${token.tokenPrefix}…  ·  ${_lastUsedLabel(token)}',
                style: TextStyle(color: colors.textMuted, fontSize: 12),
              ),
              trailing: IconButton(
                icon: Icon(Icons.delete_outline, color: colors.textMuted),
                tooltip: t.claude_integration.tokens_revoke_tooltip,
                onPressed: () => _revokeToken(context, ref, token),
              ),
            ),
          ],
        ],
      ),
    );
  }

  String _lastUsedLabel(PatModel token) {
    final lastUsed = token.lastUsedAt;
    if (lastUsed == null) return t.claude_integration.tokens_never_used;
    return t.claude_integration
        .tokens_last_used(date: lastUsed.toLocal().toString().split('.').first);
  }

  Future<void> _createToken(BuildContext context, WidgetRef ref) async {
    final nameController = TextEditingController(text: 'Claude');
    final name = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(t.claude_integration.create_dialog_title),
        content: TextField(
          controller: nameController,
          autofocus: true,
          decoration: InputDecoration(
            labelText: t.claude_integration.create_dialog_name_label,
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: Text(t.common.cancel),
          ),
          FilledButton(
            onPressed: () =>
                Navigator.of(dialogContext).pop(nameController.text.trim()),
            child: Text(t.claude_integration.create_dialog_confirm),
          ),
        ],
      ),
    );
    if (name == null || !context.mounted) return;

    try {
      final created = await ref
          .read(claudeIntegrationProvider.notifier)
          .createToken(name.isEmpty ? 'Claude' : name);
      if (created?.token == null || !context.mounted) return;
      await _showTokenOnce(context, created!.token!);
    } catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(parseApiError(e))));
    }
  }

  Future<void> _showTokenOnce(BuildContext context, String token) {
    return showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => AlertDialog(
        title: Text(t.claude_integration.token_dialog_title),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(t.claude_integration.token_dialog_body),
            const SizedBox(height: 12),
            SelectableText(
              token,
              style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
            ),
          ],
        ),
        actions: [
          TextButton.icon(
            icon: const Icon(Icons.copy, size: 18),
            label: Text(t.claude_integration.token_dialog_copy),
            onPressed: () {
              Clipboard.setData(ClipboardData(text: token));
              ScaffoldMessenger.of(dialogContext).showSnackBar(
                SnackBar(content: Text(t.claude_integration.token_copied)),
              );
            },
          ),
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: Text(t.claude_integration.token_dialog_done),
          ),
        ],
      ),
    );
  }

  Future<void> _revokeToken(
      BuildContext context, WidgetRef ref, PatModel token) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(t.claude_integration.revoke_token_title),
        content: Text(t.claude_integration.revoke_token_body),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: Text(t.common.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: Text(t.claude_integration.revoke_confirm),
          ),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) return;
    try {
      await ref.read(claudeIntegrationProvider.notifier).revokeToken(token.id);
    } catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(parseApiError(e))));
    }
  }
}

class _ConnectionsCard extends ConsumerWidget {
  final AppColorsExtension colors;
  final List<ClaudeConnectionModel> connections;
  const _ConnectionsCard({required this.colors, required this.connections});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return _card(
      colors: colors,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.claude_integration.connections_title,
            style: TextStyle(
                color: colors.textPrimary,
                fontWeight: FontWeight.w600,
                fontSize: 16),
          ),
          Text(
            t.claude_integration.connections_subtitle,
            style: TextStyle(color: colors.textMuted, fontSize: 13),
          ),
          const SizedBox(height: 8),
          if (connections.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 12),
              child: Text(
                t.claude_integration.connections_empty,
                style: TextStyle(color: colors.textMuted, fontSize: 13),
              ),
            ),
          for (final connection in connections) ...[
            Divider(height: 1, color: colors.borderLight),
            ListTile(
              contentPadding: EdgeInsets.zero,
              leading: Icon(Icons.link, color: colors.textMuted),
              title: Text(
                connection.clientName.isEmpty
                    ? 'Claude'
                    : connection.clientName,
                style: TextStyle(
                    color: colors.textPrimary, fontWeight: FontWeight.w500),
              ),
              trailing: TextButton(
                onPressed: () => _revoke(context, ref, connection),
                child: Text(t.claude_integration.connections_revoke),
              ),
            ),
          ],
        ],
      ),
    );
  }

  Future<void> _revoke(BuildContext context, WidgetRef ref,
      ClaudeConnectionModel connection) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(t.claude_integration.revoke_connection_title),
        content: Text(t.claude_integration.revoke_connection_body),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: Text(t.common.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: Text(t.claude_integration.revoke_confirm),
          ),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) return;
    try {
      await ref
          .read(claudeIntegrationProvider.notifier)
          .revokeConnection(connection.clientId);
    } catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(parseApiError(e))));
    }
  }
}

Widget _card({required AppColorsExtension colors, required Widget child}) {
  return Container(
    padding: const EdgeInsets.all(16),
    decoration: BoxDecoration(
      color: colors.backgroundSecondary,
      borderRadius: BorderRadius.circular(12),
      border: Border.all(color: colors.borderLight),
    ),
    child: child,
  );
}

Widget _copyRow(BuildContext context, AppColorsExtension colors, String value) {
  return Container(
    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
    decoration: BoxDecoration(
      color: colors.backgroundPrimary,
      borderRadius: BorderRadius.circular(8),
      border: Border.all(color: colors.borderLight),
    ),
    child: Row(
      children: [
        Expanded(
          child: SelectableText(
            value,
            style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
          ),
        ),
        IconButton(
          icon: Icon(Icons.copy, size: 18, color: colors.textMuted),
          tooltip: t.claude_integration.copy_tooltip,
          onPressed: () {
            Clipboard.setData(ClipboardData(text: value));
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(content: Text(t.claude_integration.copied)),
            );
          },
        ),
      ],
    ),
  );
}
