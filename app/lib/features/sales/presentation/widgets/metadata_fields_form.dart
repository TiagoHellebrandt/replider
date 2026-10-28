import 'package:flutter/material.dart';

import 'package:app/design_system/components/custom_text_field.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/i18n/strings.g.dart';

/// The funnel's collection fields, rendered from its own definition.
///
/// A funnel declares what it needs to know about a lead — key, label, type,
/// whether it is required — and none of that reached a screen: the create form
/// asked for a name and a phone, and the panel showed collected values with a
/// humanised key in place of the label somebody wrote. Whoever configured the
/// funnel did not recognise their own funnel.
///
/// One component, used by the create form and the panel, because the
/// alternative is two screens disagreeing about what is required.
class MetadataFieldsForm extends StatefulWidget {
  const MetadataFieldsForm({
    super.key,
    required this.schema,
    required this.values,
    required this.onChanged,
  });

  /// The funnel's field definitions, in the order they were configured.
  final List<MetadataFieldModel> schema;

  /// Current value per key. Absent or empty means not collected.
  final Map<String, String> values;

  final void Function(String key, String value) onChanged;

  @override
  State<MetadataFieldsForm> createState() => _MetadataFieldsFormState();
}

class _MetadataFieldsFormState extends State<MetadataFieldsForm> {
  /// One controller per text field, created once.
  ///
  /// Rebuilding the field on every keystroke with a fresh initialValue would
  /// send the caret back to the start of the line — the classic defect of a
  /// form driven by external state.
  final _controllers = <String, TextEditingController>{};

  @override
  void dispose() {
    for (final c in _controllers.values) {
      c.dispose();
    }
    super.dispose();
  }

  TextEditingController _controllerFor(String key, String value) =>
      _controllers.putIfAbsent(key, () => TextEditingController(text: value));

  @override
  Widget build(BuildContext context) {
    if (widget.schema.isEmpty) return const SizedBox.shrink();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (final field in widget.schema) ...[
          _FieldLabel(field: field),
          const SizedBox(height: 6),
          _MetadataField(
            field: field,
            value: widget.values[field.key] ?? '',
            controller:
                _controllerFor(field.key, widget.values[field.key] ?? ''),
            onChanged: (v) => widget.onChanged(field.key, v),
          ),
          const SizedBox(height: 14),
        ],
      ],
    );
  }
}

class _FieldLabel extends StatelessWidget {
  const _FieldLabel({required this.field});

  final MetadataFieldModel field;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // The label is what the person wrote when configuring the funnel. Falling
    // back to the key is the last resort: `n_equipamentos` is a column name,
    // not a question.
    final label = field.label.trim().isEmpty ? field.key : field.label;

    return Row(
      children: [
        Text(label,
            style: TextStyle(color: colors.textSecondary, fontSize: 12)),
        if (field.isRequired)
          Text(' *', style: TextStyle(color: colors.orangeAlert, fontSize: 12)),
      ],
    );
  }
}

/// One field, in the shape its type calls for.
///
/// The type is not decoration: it is what separates typing a date from picking
/// one, and what stops a "yes" written three different ways from becoming
/// three distinct values in a report.
class _MetadataField extends StatelessWidget {
  const _MetadataField({
    required this.field,
    required this.value,
    required this.controller,
    required this.onChanged,
  });

  final MetadataFieldModel field;
  final String value;
  final TextEditingController controller;
  final ValueChanged<String> onChanged;

  String? _validate(String? v) {
    if (!field.isRequired) return null;
    return (v == null || v.trim().isEmpty) ? t.board.field_required : null;
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final key = Key('metadata_field_${field.key}');

    switch (field.type) {
      case 'bool':
        return Row(
          key: key,
          children: [
            Switch(
              value: value == 'true',
              onChanged: (on) => onChanged(on ? 'true' : 'false'),
            ),
            const SizedBox(width: 8),
            Text(
              value == 'true' ? t.common.yes : t.common.no,
              style: TextStyle(color: colors.textPrimary, fontSize: 13),
            ),
          ],
        );

      case 'enum':
        final options = field.enumOptions ?? const <String>[];
        return DropdownButtonFormField<String>(
          key: key,
          initialValue: options.contains(value) ? value : null,
          dropdownColor: colors.backgroundSecondary,
          style: TextStyle(color: colors.textPrimary, fontSize: 14),
          decoration: InputDecoration(
            filled: true,
            fillColor: colors.backgroundTertiary,
            isDense: true,
          ),
          items: [
            for (final option in options)
              DropdownMenuItem(value: option, child: Text(option)),
          ],
          onChanged: (v) => onChanged(v ?? ''),
          validator: _validate,
        );

      case 'date':
        // Pick, don't type: a typed date arrives in five formats and no
        // report can add them up.
        return _DateField(
          fieldKey: key,
          value: value,
          onChanged: onChanged,
          validate: _validate,
        );

      default:
        return CustomTextField(
          key: key,
          hint: field.aiHint.trim(),
          controller: controller,
          keyboardType: switch (field.type) {
            'number' || 'cpf' || 'cnpj' => TextInputType.number,
            _ => TextInputType.text,
          },
          onChanged: onChanged,
          validator: _validate,
        );
    }
  }
}

class _DateField extends StatelessWidget {
  const _DateField({
    required this.fieldKey,
    required this.value,
    required this.onChanged,
    required this.validate,
  });

  final Key fieldKey;
  final String value;
  final ValueChanged<String> onChanged;
  final String? Function(String?) validate;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return FormField<String>(
      key: fieldKey,
      initialValue: value,
      validator: validate,
      builder: (state) => InkWell(
        onTap: () async {
          final now = DateTime.now();
          final picked = await showDatePicker(
            context: context,
            initialDate: DateTime.tryParse(value) ?? now,
            firstDate: DateTime(now.year - 10),
            lastDate: DateTime(now.year + 10),
          );
          if (picked == null) return;
          // ISO on the wire: the server stores a string, and a sortable date
          // is worth more than a pretty one.
          final iso = picked.toIso8601String().split('T').first;
          state.didChange(iso);
          onChanged(iso);
        },
        child: InputDecorator(
          decoration: InputDecoration(
            filled: true,
            fillColor: colors.backgroundTertiary,
            isDense: true,
            errorText: state.errorText,
            suffixIcon:
                Icon(Icons.calendar_today, size: 16, color: colors.textMuted),
          ),
          child: Text(
            state.value?.isNotEmpty == true ? state.value! : '—',
            style: TextStyle(
              color: state.value?.isNotEmpty == true
                  ? colors.textPrimary
                  : colors.textMuted,
              fontSize: 14,
            ),
          ),
        ),
      ),
    );
  }
}
