import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// "Sua chave de IA" — the BYOK step, shown only to contract accounts.
///
/// A contract account runs the AI on ITS OWN key (`AIFactory`: the platform key
/// is for self-serve plans only). Nothing in the wizard used to ask for one, so
/// the very next step — reading a catalogue PDF — refused with an error that
/// blamed the environment for a key nobody had been asked for. This is that
/// question, put before the step that needs it.
///
/// Skippable on purpose: the price list still imports as CSV, which uses no
/// model at all, so a person without a key today is not stopped from finishing.
class StepAIKey extends ConsumerStatefulWidget {
  const StepAIKey({super.key});

  @override
  ConsumerState<StepAIKey> createState() => _StepAIKeyState();
}

class _StepAIKeyState extends ConsumerState<StepAIKey> {
  final _ctrl = TextEditingController();
  bool _busy = false;

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    final key = _ctrl.text.trim();
    if (key.isEmpty) return;
    setState(() => _busy = true);
    try {
      // Only the field being set travels: the handler reads pointers, so an
      // omitted provider keeps whatever it had. (The field is `gemini_key` —
      // a wrong name is accepted and silently saves nothing.)
      await ref.read(dioProvider).patch<void>(
        '/me/api-keys',
        data: <String, dynamic>{'gemini_key': key},
      );
      // The masked key rides on /me, and the next step reads it to decide
      // whether the PDF path is open — so the refresh is what makes this
      // step's effect visible one screen later.
      await ref.read(authControllerProvider.notifier).refreshMe();
      if (!mounted) return;
      _ctrl.clear();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(parseApiError(e)),
          behavior: SnackBarBehavior.floating,
        ),
      );
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final user = ref.watch(authControllerProvider).asData?.value;
    final masked = user?.geminiKeyMasked ?? '';
    final hasKey = masked.isNotEmpty;

    return SingleChildScrollView(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.onboarding.aikey_title,
            style: TextStyle(
              fontSize: 26,
              fontWeight: FontWeight.w700,
              color: colors.textPrimary,
            ),
          ),
          const SizedBox(height: 10),
          Text(
            t.onboarding.aikey_subtitle,
            style: TextStyle(color: colors.textSecondary, fontSize: 15),
          ),
          const SizedBox(height: 24),
          if (hasKey)
            Container(
              key: const Key('onboarding_aikey_configured'),
              padding: const EdgeInsets.all(14),
              decoration: BoxDecoration(
                color: colors.glassLight,
                borderRadius: BorderRadius.circular(12),
              ),
              child: Row(
                children: [
                  Icon(Icons.check_circle_outline,
                      size: 18, color: colors.whatsappGreen),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          '${t.onboarding.aikey_saved}  ·  $masked',
                          style: TextStyle(
                            color: colors.textPrimary,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          t.onboarding.aikey_saved_hint,
                          style:
                              TextStyle(color: colors.textMuted, fontSize: 12),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            )
          else ...[
            TextField(
              key: const Key('onboarding_aikey_input'),
              controller: _ctrl,
              enabled: !_busy,
              obscureText: true,
              onChanged: (_) => setState(() {}),
              style: TextStyle(color: colors.textPrimary),
              decoration: InputDecoration(labelText: t.onboarding.aikey_field),
            ),
            const SizedBox(height: 8),
            Text(
              t.onboarding.aikey_hint,
              style: TextStyle(color: colors.textMuted, fontSize: 12),
            ),
            const SizedBox(height: 16),
            FilledButton.icon(
              key: const Key('onboarding_aikey_save'),
              // Gated on the same emptiness the handler would have bailed on,
              // so the button never accepts a click that does nothing.
              onPressed: _busy || _ctrl.text.trim().isEmpty ? null : _save,
              icon: const Icon(Icons.key_outlined, size: 18),
              label: Text(t.onboarding.aikey_save),
            ),
            const SizedBox(height: 20),
            Text(
              t.onboarding.aikey_skip_note,
              key: const Key('onboarding_aikey_skip_note'),
              style: TextStyle(color: colors.textMuted, fontSize: 12),
            ),
          ],
          if (_busy) ...[
            const SizedBox(height: 16),
            const Center(child: CircularProgressIndicator()),
          ],
        ],
      ),
    );
  }
}
