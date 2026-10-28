import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/primary_gradient_button.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/billing/domain/models/plan_model.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/owner/presentation/controllers/owner_controller.dart';
import 'package:app/features/owner/presentation/widgets/owner_2fa_step_up.dart';
import 'package:app/i18n/strings.g.dart';

class OwnerCreateContractScreen extends ConsumerStatefulWidget {
  const OwnerCreateContractScreen({super.key});

  @override
  ConsumerState<OwnerCreateContractScreen> createState() =>
      _OwnerCreateContractScreenState();
}

/// Whether the contract is being provisioned for a brand-new account
/// (company + admin email → createContract) or applied to an account
/// that already exists (pick account → applyContract).
enum _Mode { create, existing }

class _OwnerCreateContractScreenState
    extends ConsumerState<OwnerCreateContractScreen> {
  final _company = TextEditingController();
  final _email = TextEditingController();
  final _name = TextEditingController();
  final _agents = TextEditingController();
  final _channels = TextEditingController();
  final _convs = TextEditingController();
  final _tokens = TextEditingController();
  final _pick = TextEditingController();
  String? _planId;
  bool _trialBypass = false;
  bool _saving = false;
  List<PlanModel>? _plans;

  _Mode _mode = _Mode.create;
  String _pickSearch = '';
  String? _pickedId;
  String? _pickedName;

  @override
  void initState() {
    super.initState();
    _loadPlans();
  }

  Future<void> _loadPlans() async {
    try {
      // /owner/plans is the UNFILTERED catalogue — the public /plans hides the
      // SDR contract tiers (Vendedor/Equipe/Escala) a contract is actually for.
      // Keep only contract plans: a contract account isn't a self-serve
      // Stripe checkout, and this also drops the duplicate BRL/USD self-serve
      // "Solo"/"Pro" rows that made the picker ambiguous.
      final res =
          await ref.read(dioProvider).get<List<dynamic>>('/owner/plans');
      if (!mounted) return;
      final all = (res.data ?? const <dynamic>[])
          .whereType<Map<String, dynamic>>()
          .map(PlanModel.fromJson)
          .where((p) => p.isContractPlan)
          .toList();
      setState(() => _plans = all);
    } catch (_) {
      // Non-fatal — the user can still type a plan id manually if we
      // keep an "advanced" textbox in v2. For now we just leave the
      // dropdown empty and the form blocks the submit.
    }
  }

  @override
  void dispose() {
    for (final c in [
      _company,
      _email,
      _name,
      _agents,
      _channels,
      _convs,
      _tokens,
      _pick,
    ]) {
      c.dispose();
    }
    super.dispose();
  }

  int? _parse(String v) {
    final s = v.trim();
    if (s.isEmpty) return null;
    return int.tryParse(s);
  }

  Map<String, int?> _overrides() => {
        'max_agents': _parse(_agents.text),
        'max_channels': _parse(_channels.text),
        'max_conversations_per_month': _parse(_convs.text),
        'max_ai_tokens_per_month': _parse(_tokens.text),
      };

  Future<void> _submit() async {
    if (_planId == null) {
      _snack(t.owner.form_required);
      return;
    }
    if (_mode == _Mode.create &&
        (_company.text.trim().isEmpty || _email.text.trim().isEmpty)) {
      _snack(t.owner.form_required);
      return;
    }
    if (_mode == _Mode.existing && _pickedId == null) {
      _snack(t.owner.pick_account_required);
      return;
    }
    setState(() => _saving = true);
    try {
      final String accountId;
      if (_mode == _Mode.create) {
        // An expired 2FA proof asks for the code in place and retries — the
        // filled company/e-mail/override fields survive.
        final out = await withOwnerStepUp(
          context,
          ref,
          () => ref.read(ownerActionsProvider.notifier).createContract(
                companyName: _company.text.trim(),
                adminEmail: _email.text.trim(),
                adminName: _name.text.trim(),
                planId: _planId!,
                trialBypass: _trialBypass,
                overrides: _overrides(),
              ),
        );
        accountId = out.id;
      } else {
        await withOwnerStepUp(
          context,
          ref,
          () => ref.read(ownerActionsProvider.notifier).applyContract(
                _pickedId!,
                planId: _planId!,
                trialBypass: _trialBypass,
                overrides: _overrides(),
              ),
        );
        // applyContract returns void; the account id is the one we picked.
        accountId = _pickedId!;
      }
      if (!mounted) return;
      _snack(t.common.success);
      context.go('/owner/accounts/$accountId');
    } catch (e) {
      if (!mounted) return;
      _snack(parseApiError(e));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  void _snack(String msg) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        title: Text(
          t.owner.create_title,
          style: TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w700,
            color: colors.textPrimary,
          ),
        ),
        leading: IconButton(
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () =>
              context.canPop() ? context.pop() : context.go('/owner/accounts'),
        ),
      ),
      body: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          SegmentedButton<_Mode>(
            segments: [
              ButtonSegment(
                value: _Mode.create,
                label: Text(t.owner.mode_new_account),
                icon: const Icon(Icons.add_business_outlined, size: 18),
              ),
              ButtonSegment(
                value: _Mode.existing,
                label: Text(t.owner.mode_existing_account),
                icon: const Icon(Icons.manage_accounts_outlined, size: 18),
              ),
            ],
            selected: {_mode},
            onSelectionChanged: (s) => setState(() => _mode = s.first),
          ),
          const SizedBox(height: 20),
          if (_mode == _Mode.create) ...[
            _label(t.owner.create_company, typography, colors),
            TextField(controller: _company, decoration: _dec(colors)),
            const SizedBox(height: 16),
            _label(t.owner.create_admin_email, typography, colors),
            TextField(
              controller: _email,
              keyboardType: TextInputType.emailAddress,
              decoration: _dec(colors),
            ),
            const SizedBox(height: 16),
            _label(t.owner.create_admin_name, typography, colors),
            TextField(controller: _name, decoration: _dec(colors)),
          ] else
            _AccountPicker(
              controller: _pick,
              search: _pickSearch,
              pickedId: _pickedId,
              pickedName: _pickedName,
              onSearch: (v) => setState(() => _pickSearch = v.trim()),
              onPick: (id, name) => setState(() {
                _pickedId = id;
                _pickedName = name;
              }),
              onClear: () => setState(() {
                _pickedId = null;
                _pickedName = null;
                _pick.clear();
                _pickSearch = '';
              }),
            ),
          const SizedBox(height: 16),
          _label(t.owner.create_plan, typography, colors),
          DropdownButtonFormField<String>(
            initialValue: _planId,
            isExpanded: true,
            items: (_plans ?? const <PlanModel>[])
                .map((p) => DropdownMenuItem(
                    value: p.id,
                    child: Text('${p.name} · '
                        '${p.price <= 0 ? t.billing.plan_contract : p.formattedPrice}')))
                .toList(),
            onChanged: (v) => setState(() => _planId = v),
            decoration: _dec(colors),
          ),
          const SizedBox(height: 16),
          _label(t.owner.overrides_label, typography, colors),
          Row(children: [
            Expanded(
              child: TextField(
                controller: _agents,
                keyboardType: TextInputType.number,
                decoration: _dec(colors, label: t.owner.cap_max_agents),
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: TextField(
                controller: _channels,
                keyboardType: TextInputType.number,
                decoration: _dec(colors, label: t.owner.cap_max_channels),
              ),
            ),
          ]),
          const SizedBox(height: 8),
          Row(children: [
            Expanded(
              child: TextField(
                controller: _convs,
                keyboardType: TextInputType.number,
                decoration: _dec(colors, label: t.owner.cap_max_conversations),
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: TextField(
                controller: _tokens,
                keyboardType: TextInputType.number,
                decoration: _dec(colors, label: t.owner.cap_max_tokens),
              ),
            ),
          ]),
          const SizedBox(height: 16),
          SwitchListTile(
            value: _trialBypass,
            onChanged: (v) => setState(() => _trialBypass = v),
            title: Text(t.owner.create_trial_bypass),
            subtitle: Text(t.owner.create_trial_bypass_hint),
            contentPadding: EdgeInsets.zero,
          ),
          const SizedBox(height: 24),
          PrimaryGradientButton(
            text: _saving
                ? t.common.saving
                : (_mode == _Mode.create
                    ? t.owner.create_submit
                    : t.owner.apply_contract_submit),
            onPressed: _saving ? null : _submit,
          ),
        ],
      ),
    );
  }

  Widget _label(String text, AppTypographyExtension typography,
      AppColorsExtension colors) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Text(
        text,
        style: typography.body.copyWith(
          color: colors.textPrimary,
          fontSize: 12,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }

  /// Themed input decoration so the contract form fields match the
  /// rest of the app instead of using the bare Material outline.
  InputDecoration _dec(AppColorsExtension colors, {String? label}) {
    return InputDecoration(
      labelText: label,
      labelStyle: TextStyle(color: colors.textMuted),
    );
  }
}

/// Search-and-select an existing account to apply a contract to. Once an
/// account is picked it collapses to a removable chip so the rest of the
/// form (plan + caps) stays the focus.
class _AccountPicker extends ConsumerWidget {
  final TextEditingController controller;
  final String search;
  final String? pickedId;
  final String? pickedName;
  final ValueChanged<String> onSearch;
  final void Function(String id, String name) onPick;
  final VoidCallback onClear;

  const _AccountPicker({
    required this.controller,
    required this.search,
    required this.pickedId,
    required this.pickedName,
    required this.onSearch,
    required this.onPick,
    required this.onClear,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    Widget label(String text) => Padding(
          padding: const EdgeInsets.only(bottom: 6),
          child: Text(
            text,
            style: typography.body.copyWith(
              color: colors.textPrimary,
              fontSize: 12,
              fontWeight: FontWeight.w600,
            ),
          ),
        );

    if (pickedId != null) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          label(t.owner.pick_account_label),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
            decoration: BoxDecoration(
              color: colors.bluePrimary.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(10),
              border:
                  Border.all(color: colors.bluePrimary.withValues(alpha: 0.4)),
            ),
            child: Row(
              children: [
                Icon(Icons.business, size: 18, color: colors.bluePrimary),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    pickedName ?? pickedId!,
                    style: typography.body.copyWith(
                      color: colors.textPrimary,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                IconButton(
                  icon:
                      Icon(Icons.close, size: 18, color: colors.textSecondary),
                  onPressed: onClear,
                ),
              ],
            ),
          ),
        ],
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        label(t.owner.pick_account_label),
        TextField(
          controller: controller,
          onChanged: onSearch,
          style: TextStyle(color: colors.textPrimary),
          decoration: InputDecoration(
            isDense: true,
            hintText: t.owner.pick_account_hint,
            hintStyle: TextStyle(color: colors.textMuted),
            prefixIcon:
                Icon(Icons.search, color: colors.textSecondary, size: 20),
          ),
        ),
        if (search.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: _results(ref, colors, typography),
          ),
      ],
    );
  }

  Widget _results(
    WidgetRef ref,
    AppColorsExtension colors,
    AppTypographyExtension typography,
  ) {
    final async = ref.watch(ownerAccountsProvider(search: search, status: ''));
    return async.maybeWhen(
      data: (data) {
        if (data.items.isEmpty) {
          return Padding(
            padding: const EdgeInsets.all(12),
            child: Text(
              t.owner.empty_title,
              style: typography.body
                  .copyWith(color: colors.textMuted, fontSize: 13),
            ),
          );
        }
        final shown = data.items.take(8).toList();
        return Container(
          decoration: BoxDecoration(
            color: colors.glassMedium.withValues(alpha: 0.5),
            borderRadius: BorderRadius.circular(10),
            border: Border.all(color: colors.borderMedium),
          ),
          child: Column(
            children: [
              for (final it in shown)
                ListTile(
                  dense: true,
                  title: Text(
                    it.account.name,
                    style: typography.body.copyWith(
                      color: colors.textPrimary,
                      fontSize: 14,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  subtitle: it.ownerEmail.isEmpty
                      ? null
                      : Text(
                          it.ownerEmail,
                          style: typography.body.copyWith(
                            color: colors.textSecondary,
                            fontSize: 12,
                          ),
                        ),
                  onTap: () => onPick(it.account.id, it.account.name),
                ),
            ],
          ),
        );
      },
      orElse: () => const Padding(
        padding: EdgeInsets.all(12),
        child: Center(child: CircularProgressIndicator()),
      ),
    );
  }
}
