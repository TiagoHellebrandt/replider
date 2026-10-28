import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/custom_text_field.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/board_controller.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/features/sales/presentation/widgets/metadata_fields_form.dart';
import 'package:app/i18n/strings.g.dart';

/// Registers a lead by hand.
///
/// A lead in this product is a COMPANY with somebody in it: the hospital, the
/// person who buys there, how to reach them. Until 2026-08-19 the form asked
/// for "a name and a WhatsApp" — a consumer's shape — and a purchasing
/// manager reached by e-mail had no way in. Phone or e-mail, one of the two,
/// is what makes the lead reachable and therefore real; everything else is
/// optional and feeds the approach (the draft can name the person and the
/// city instead of writing around the gap).
Future<bool?> showNewLeadDialog(BuildContext context, String funnelId) {
  return showDialog<bool>(
    context: context,
    builder: (_) => _NewLeadDialog(funnelId: funnelId),
  );
}

class _NewLeadDialog extends ConsumerStatefulWidget {
  const _NewLeadDialog({required this.funnelId});

  final String funnelId;

  @override
  ConsumerState<_NewLeadDialog> createState() => _NewLeadDialogState();
}

class _NewLeadDialogState extends ConsumerState<_NewLeadDialog> {
  final _formKey = GlobalKey<FormState>();
  final _company = TextEditingController();
  final _name = TextEditingController();
  final _role = TextEditingController();
  final _email = TextEditingController();
  final _phone = TextEditingController();
  final _city = TextEditingController();

  /// What has been answered of the funnel's fields, by key.
  final _metadata = <String, String>{};
  bool _saving = false;
  String? _error;

  @override
  void dispose() {
    _company.dispose();
    _name.dispose();
    _role.dispose();
    _email.dispose();
    _phone.dispose();
    _city.dispose();
    super.dispose();
  }

  bool get _reachable =>
      _phone.text.trim().isNotEmpty || _email.text.trim().isNotEmpty;

  Future<void> _submit() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;
    if (!_reachable) {
      setState(() => _error = t.board.new_lead_reach_required);
      return;
    }
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      final created = await ref.read(boardServiceProvider).createLead(
        funnelId: widget.funnelId,
        company: _company.text.trim(),
        name: _name.text.trim(),
        role: _role.text.trim(),
        email: _email.text.trim(),
        phone: _phone.text.trim(),
        city: _city.text.trim(),
        metadata: {
          for (final e in _metadata.entries)
            if (e.value.trim().isNotEmpty) e.key: e.value.trim(),
        },
      );
      if (!mounted) return;
      // The server is idempotent, so a repeat submission is not an error —
      // but the caller must be told it matched instead of created, or the
      // user will keep looking for a second card that will never appear.
      Navigator.of(context).pop(created);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _saving = false;
        _error = parseApiError(e);
      });
    }
  }

  /// The target funnel's schema. Empty when the funnel asks for nothing beyond
  /// the basics — or when the list has not loaded yet, in which case the simple
  /// create form keeps working instead of blocking.
  List<MetadataFieldModel> get _schema =>
      ref
          .watch(funnelsListProvider)
          .asData
          ?.value
          .where((f) => f.id == widget.funnelId)
          .firstOrNull
          ?.metadataSchema ??
      const [];

  /// Two fields side by side when there is room, stacked on a phone: the
  /// dialog is where an SDR types a lead in between appointments, and a row
  /// that overflows there is a form nobody finishes.
  Widget _pair(BuildContext context, Widget left, Widget right,
      {int leftFlex = 1, int rightFlex = 1}) {
    final narrow = MediaQuery.sizeOf(context).width < 480;
    if (narrow) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [left, const SizedBox(height: 14), right],
      );
    }
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(flex: leftFlex, child: left),
        const SizedBox(width: 12),
        Expanded(flex: rightFlex, child: right),
      ],
    );
  }

  Widget _field(AppColorsExtension colors,
          {required String label,
          required String hint,
          required TextEditingController controller,
          required Key key,
          TextInputType keyboardType = TextInputType.text}) =>
      Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _label(label, colors),
          CustomTextField(
            key: key,
            hint: hint,
            controller: controller,
            keyboardType: keyboardType,
          ),
        ],
      );

  Widget _label(String text, AppColorsExtension colors) => Padding(
        padding: const EdgeInsets.only(bottom: 6),
        child: Text(text,
            style: TextStyle(color: colors.textSecondary, fontSize: 12)),
      );

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Dialog(
      backgroundColor: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      insetPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 24),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 460),
        child: Form(
          key: _formKey,
          child: SingleChildScrollView(
            padding: const EdgeInsets.fromLTRB(24, 22, 24, 20),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  t.board.new_lead,
                  style: TextStyle(
                      color: colors.textPrimary,
                      fontSize: 20,
                      fontWeight: FontWeight.w700),
                ),
                const SizedBox(height: 4),
                Text(
                  t.board.new_lead_subtitle,
                  style: TextStyle(color: colors.textSecondary, fontSize: 13),
                ),
                const SizedBox(height: 20),

                // THE COMPANY. Required: it is what the card is named after
                // and what the draft is written about.
                _label(t.board.new_lead_company, colors),
                CustomTextField(
                  key: const Key('new_lead_company'),
                  hint: t.board.new_lead_company_hint,
                  controller: _company,
                  autofocus: true,
                  validator: (v) => (v == null || v.trim().isEmpty)
                      ? t.board.new_lead_company_required
                      : null,
                ),
                const SizedBox(height: 14),

                // THE PERSON. Optional; when given, the draft addresses
                // somebody instead of "a quem interessar".
                _pair(
                  context,
                  _field(colors,
                      key: const Key('new_lead_name'),
                      label: t.board.new_lead_name,
                      hint: t.board.new_lead_name_hint,
                      controller: _name),
                  _field(colors,
                      key: const Key('new_lead_role'),
                      label: t.board.new_lead_role,
                      hint: t.board.new_lead_role_hint,
                      controller: _role),
                  leftFlex: 3,
                  rightFlex: 2,
                ),
                const SizedBox(height: 14),

                // HOW TO REACH THEM. One of the two is required — checked on
                // submit, because "required" on both would be wrong and on
                // neither would let a note onto the board.
                _field(colors,
                    key: const Key('new_lead_email'),
                    label: t.board.new_lead_email,
                    hint: t.board.new_lead_email_hint,
                    controller: _email,
                    keyboardType: TextInputType.emailAddress),
                const SizedBox(height: 14),
                _pair(
                  context,
                  _field(colors,
                      key: const Key('new_lead_phone'),
                      label: t.board.new_lead_phone,
                      hint: t.board.new_lead_phone_hint,
                      controller: _phone,
                      keyboardType: TextInputType.phone),
                  _field(colors,
                      key: const Key('new_lead_city'),
                      label: t.board.new_lead_city,
                      hint: t.board.new_lead_city_hint,
                      controller: _city),
                ),
                const SizedBox(height: 6),
                Text(
                  t.board.new_lead_reach_help,
                  style: TextStyle(color: colors.textMuted, fontSize: 11),
                ),

                // The fields THIS funnel asks for. They were declared in its
                // configuration and reached no form: the create dialog asked
                // for a name and a phone, and the required ones — which are
                // what makes a lead qualified — had nowhere to be filled in.
                if (_schema.isNotEmpty) ...[
                  const SizedBox(height: 18),
                  Divider(color: colors.borderLight, height: 1),
                  const SizedBox(height: 14),
                  Text(t.board.funnel_fields,
                      style: TextStyle(
                          color: colors.textSecondary,
                          fontSize: 12,
                          fontWeight: FontWeight.w600)),
                  const SizedBox(height: 10),
                  MetadataFieldsForm(
                    schema: _schema,
                    values: _metadata,
                    onChanged: (key, value) => _metadata[key] = value,
                  ),
                ],
                if (_error != null) ...[
                  const SizedBox(height: 12),
                  Text(
                    _error!,
                    key: const Key('new_lead_error'),
                    style: TextStyle(color: colors.redAlert, fontSize: 12),
                  ),
                ],
                const SizedBox(height: 22),
                // Side by side, right-aligned, the primary no wider than its
                // label: a full-width Save with Cancel floating above it
                // read as a form that had not finished laying out.
                Wrap(
                  alignment: WrapAlignment.end,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    TextButton(
                      onPressed:
                          _saving ? null : () => Navigator.of(context).pop(),
                      child: Text(t.common.cancel,
                          style: TextStyle(color: colors.textSecondary)),
                    ),
                    FilledButton(
                      key: const Key('new_lead_submit'),
                      onPressed: _saving ? null : _submit,
                      style: FilledButton.styleFrom(
                        backgroundColor: colors.bluePrimary,
                        foregroundColor: Colors.white,
                        padding: const EdgeInsets.symmetric(
                            horizontal: 22, vertical: 14),
                        shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(100)),
                      ),
                      child: _saving
                          ? const SizedBox(
                              width: 18,
                              height: 18,
                              child: CircularProgressIndicator(
                                  strokeWidth: 2, color: Colors.white),
                            )
                          : Text(t.board.new_lead_submit),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
