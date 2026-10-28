import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/core/utils/currency_format.dart';
import 'package:app/features/owner/presentation/widgets/owner_2fa_step_up.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

/// "Convidar cliente" (fluxo sales-led, docs/fluxo-criacao-conta.md).
///
/// Emits a one-shot signup invite bound to the customer's e-mail and the
/// contracted plan. The resulting link is shown ONCE — the server keeps only
/// a hash — so this dialog's second page exists to make the person copy it
/// before anything else can happen to it.
class InviteCustomerDialog extends ConsumerStatefulWidget {
  const InviteCustomerDialog({super.key});

  @override
  ConsumerState<InviteCustomerDialog> createState() =>
      _InviteCustomerDialogState();
}

class _InviteCustomerDialogState extends ConsumerState<InviteCustomerDialog> {
  final _emailCtrl = TextEditingController();
  List<Map<String, dynamic>> _plans = const [];
  String? _planId;
  int _days = 7;
  bool _busy = false;
  String? _link;

  @override
  void initState() {
    super.initState();
    Future.microtask(_loadPlans);
  }

  @override
  void dispose() {
    _emailCtrl.dispose();
    super.dispose();
  }

  // Owner picker label: name + price so the contract tiers are legible and
  // any same-named rows are told apart. Price 0 (Enterprise/Partner) shows the
  // name alone.
  String _planLabel(Map<String, dynamic> p) {
    final name = p['name'] as String? ?? '?';
    final cents = (p['price'] as num?)?.toInt() ?? 0;
    if (cents <= 0) return '$name · ${t.billing.plan_contract}';
    final cur = (p['currency'] as String?)?.trim() ?? 'brl';
    return '$name · ${formatCurrencyCents(cents, cur)}';
  }

  Future<void> _loadPlans() async {
    try {
      // /owner/plans is the UNFILTERED catalogue — the public /plans hides
      // exactly the SDR contract tiers an invite points at.
      final res =
          await ref.read(dioProvider).get<List<dynamic>>('/owner/plans');
      if (!mounted) return;
      setState(() {
        _plans = (res.data ?? const <dynamic>[])
            .whereType<Map<String, dynamic>>()
            // Contract tiers only (no Stripe price): Enterprise + the SDR
            // plans. The retired v1 self-serve ladders (USD Solo/Core/Pro,
            // BRL Solo/Starter/Pro/Business/Scale) still exist in the table
            // for FK/invoice history but are never offered on a new invite.
            .where((p) {
          final sp = (p['stripe_price_id'] as String?)?.trim() ?? '';
          return sp.isEmpty;
        }).toList();
      });
    } catch (e) {
      if (!mounted) return;
      _snack(parseApiError(e));
    }
  }

  Future<void> _create() async {
    final email = _emailCtrl.text.trim();
    if (email.isEmpty || _planId == null) return;
    setState(() => _busy = true);
    try {
      // An expired 2FA proof asks for the code in place and retries — the
      // typed e-mail/plan/validity survive instead of being lost to a re-login.
      final res = await withOwnerStepUp(
        context,
        ref,
        () => ref.read(dioProvider).post<Map<String, dynamic>>(
          '/owner/invites',
          data: <String, dynamic>{
            'email': email,
            'plan_id': _planId,
            'expires_in_days': _days,
          },
        ),
      );
      final token = res.data?['token'] as String? ?? '';
      if (!mounted) return;
      // Web: the register link lives on this same origin. The fallback is
      // the production dashboard, for non-web sessions — and .origin THROWS
      // on non-http schemes (file:// under tests), so the scheme is checked
      // first, never the origin itself.
      final base = Uri.base;
      final origin = (base.scheme == 'http' || base.scheme == 'https')
          ? base.origin
          : 'https://app.replider.com';
      setState(() => _link = '$origin/register?invite=$token');
    } catch (e) {
      if (!mounted) return;
      _snack(parseApiError(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _snack(String msg) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(msg), behavior: SnackBarBehavior.floating),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return AlertDialog(
      backgroundColor: colors.backgroundSecondary,
      title: Text(
          _link == null ? t.owner.invite_title : t.owner.invite_link_title,
          style: TextStyle(color: colors.textPrimary, fontSize: 18)),
      content: SizedBox(
        width: 420,
        child: _link == null ? _form(colors) : _linkView(colors),
      ),
      actions: _link == null
          ? [
              TextButton(
                onPressed: () => Navigator.of(context).pop(),
                child: Text(t.common.cancel),
              ),
              FilledButton(
                key: const Key('invite_create_btn'),
                onPressed: _busy ? null : _create,
                child: Text(t.owner.invite_create),
              ),
            ]
          : [
              FilledButton(
                key: const Key('invite_done_btn'),
                onPressed: () => Navigator.of(context).pop(),
                child: Text(t.common.close),
              ),
            ],
    );
  }

  Widget _form(AppColorsExtension colors) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        TextField(
          key: const Key('invite_email_input'),
          controller: _emailCtrl,
          keyboardType: TextInputType.emailAddress,
          style: TextStyle(color: colors.textPrimary),
          decoration: InputDecoration(labelText: t.owner.invite_email),
        ),
        const SizedBox(height: 12),
        DropdownButtonFormField<String>(
          key: const Key('invite_plan_dropdown'),
          initialValue: _planId,
          items: [
            for (final p in _plans)
              DropdownMenuItem<String>(
                value: p['id'] as String? ?? '',
                child: Text(_planLabel(p)),
              ),
          ],
          onChanged: (v) => setState(() => _planId = v),
          decoration: InputDecoration(labelText: t.owner.invite_plan),
          dropdownColor: colors.backgroundSecondary,
          style: TextStyle(color: colors.textPrimary, fontSize: 14),
        ),
        const SizedBox(height: 12),
        DropdownButtonFormField<int>(
          initialValue: _days,
          items: [
            for (final d in const [3, 7, 14, 30])
              DropdownMenuItem<int>(value: d, child: Text('$d')),
          ],
          onChanged: (v) => setState(() => _days = v ?? 7),
          decoration: InputDecoration(labelText: t.owner.invite_days),
          dropdownColor: colors.backgroundSecondary,
          style: TextStyle(color: colors.textPrimary, fontSize: 14),
        ),
        const SizedBox(height: 8),
        Text(t.owner.invite_hint,
            style: TextStyle(color: colors.textMuted, fontSize: 12)),
      ],
    );
  }

  Widget _linkView(AppColorsExtension colors) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(t.owner.invite_link_hint,
            style: TextStyle(color: colors.orangeAlert, fontSize: 13)),
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.all(10),
          decoration: BoxDecoration(
            color: colors.glassMedium,
            borderRadius: BorderRadius.circular(8),
          ),
          child: SelectableText(
            _link!,
            key: const Key('invite_link_text'),
            style: TextStyle(color: colors.textPrimary, fontSize: 12),
          ),
        ),
        const SizedBox(height: 8),
        OutlinedButton.icon(
          key: const Key('invite_copy_btn'),
          onPressed: () async {
            await Clipboard.setData(ClipboardData(text: _link!));
            if (context.mounted) _snack(t.owner.invite_copied);
          },
          icon: const Icon(Icons.copy, size: 16),
          label: Text(t.owner.invite_copy),
        ),
      ],
    );
  }
}
