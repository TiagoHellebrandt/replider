import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// BYOK API-keys card for contract plans (Partner / Enterprise).
///
/// Renders a row per provider (OpenAI / Gemini) with the masked
/// preview of whatever the server has on file plus an "edit" button
/// that opens an inline form. Submissions PATCH /v1/me/api-keys with
/// the full `{openai_key, gemini_key}` payload — the server overwrites
/// both, so we always send the current value of the untouched field
/// alongside the edited one.
///
/// Hidden entirely when the account is on a self-serve plan; the API
/// rejects writes there too (`BYOK_NOT_AVAILABLE`), but server-side
/// enforcement plus a hidden surface is one error path the user
/// won't ever see.
class ApiKeysSettingsCard extends ConsumerStatefulWidget {
  const ApiKeysSettingsCard({super.key});

  @override
  ConsumerState<ApiKeysSettingsCard> createState() =>
      _ApiKeysSettingsCardState();
}

class _ApiKeysSettingsCardState extends ConsumerState<ApiKeysSettingsCard> {
  bool _saving = false;

  @override
  Widget build(BuildContext context) {
    final auth = ref.watch(authControllerProvider);
    final me = auth.value;
    if (me == null || !me.isContractPlan) {
      return const SizedBox.shrink();
    }
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

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
          Row(
            children: [
              Icon(Icons.key_outlined, color: colors.bluePrimary),
              const SizedBox(width: 8),
              Text(
                t.api_keys.title,
                style: TextStyle(
                  color: colors.textPrimary,
                  fontWeight: FontWeight.w700,
                  fontSize: 15,
                ),
              ),
            ],
          ),
          const SizedBox(height: 6),
          Text(
            t.api_keys.subtitle,
            style: TextStyle(color: colors.textMuted, fontSize: 13),
          ),
          const SizedBox(height: 16),
          _ProviderRow(
            colors: colors,
            label: 'OpenAI',
            masked: me.openAIKeyMasked,
            saving: _saving,
            onSave: (value) => _save(openAI: value),
          ),
          const SizedBox(height: 12),
          _ProviderRow(
            colors: colors,
            label: 'Google Gemini',
            masked: me.geminiKeyMasked,
            saving: _saving,
            onSave: (value) => _save(gemini: value),
          ),
        ],
      ),
    );
  }

  Future<void> _save({String? openAI, String? gemini}) async {
    setState(() => _saving = true);
    final dio = ref.read(dioProvider);
    final body = <String, dynamic>{};
    if (openAI != null) body['openai_key'] = openAI;
    if (gemini != null) body['gemini_key'] = gemini;
    try {
      // No validateStatus override: it swallowed 400/403 (rejected key,
      // BYOK unavailable) and the user read "salvo" over a key the server
      // refused — the agent stayed dead with a green confirmation.
      await dio.patch<void>('/me/api-keys', data: body);
      // Refresh /me so the new mask renders.
      await ref.read(authControllerProvider.notifier).refreshMe();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(t.api_keys.saved)),
      );
    } on DioException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          backgroundColor: Colors.red.shade700,
          content: Text(parseApiError(e)),
        ),
      );
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }
}

class _ProviderRow extends StatefulWidget {
  const _ProviderRow({
    required this.colors,
    required this.label,
    required this.masked,
    required this.saving,
    required this.onSave,
  });

  final AppColorsExtension colors;
  final String label;
  final String masked;
  final bool saving;
  final Future<void> Function(String value) onSave;

  @override
  State<_ProviderRow> createState() => _ProviderRowState();
}

class _ProviderRowState extends State<_ProviderRow> {
  bool _editing = false;
  final _ctrl = TextEditingController();

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = widget.colors;
    final hasKey = widget.masked.isNotEmpty;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    widget.label,
                    style: TextStyle(
                      color: colors.textPrimary,
                      fontWeight: FontWeight.w600,
                      fontSize: 14,
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    hasKey ? widget.masked : t.api_keys.not_configured,
                    style: TextStyle(
                      color: hasKey ? colors.textSecondary : colors.textMuted,
                      fontSize: 13,
                      fontFeatures:
                          hasKey ? const [FontFeature.tabularFigures()] : null,
                    ),
                  ),
                ],
              ),
            ),
            if (!_editing)
              TextButton.icon(
                onPressed: widget.saving
                    ? null
                    : () => setState(() => _editing = true),
                icon: Icon(hasKey ? Icons.edit_outlined : Icons.add, size: 16),
                label: Text(hasKey ? t.api_keys.edit : t.api_keys.add),
              ),
          ],
        ),
        if (_editing) ...[
          const SizedBox(height: 8),
          TextField(
            controller: _ctrl,
            obscureText: true,
            autofocus: true,
            // Rebuild on every keystroke/paste so the Save button's
            // `_ctrl.text.trim().isEmpty` gate re-evaluates — without this
            // the button kept its initial disabled state (the field was
            // empty when the row first built) even after a key was pasted.
            onChanged: (_) => setState(() {}),
            decoration: InputDecoration(
              isDense: true,
              hintText: t.api_keys.input_hint,
            ),
          ),
          const SizedBox(height: 8),
          Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              if (hasKey)
                TextButton(
                  onPressed: widget.saving
                      ? null
                      : () async {
                          await widget.onSave('');
                          if (mounted) {
                            setState(() {
                              _editing = false;
                              _ctrl.clear();
                            });
                          }
                        },
                  child: Text(
                    t.api_keys.clear,
                    style: TextStyle(color: colors.redAlert),
                  ),
                ),
              TextButton(
                onPressed: widget.saving
                    ? null
                    : () => setState(() {
                          _editing = false;
                          _ctrl.clear();
                        }),
                child: Text(t.common.cancel),
              ),
              const SizedBox(width: 4),
              FilledButton(
                onPressed: widget.saving || _ctrl.text.trim().isEmpty
                    ? null
                    : () async {
                        await widget.onSave(_ctrl.text.trim());
                        if (mounted) {
                          setState(() {
                            _editing = false;
                            _ctrl.clear();
                          });
                        }
                      },
                child: Text(t.api_keys.save),
              ),
            ],
          ),
        ],
      ],
    );
  }
}
