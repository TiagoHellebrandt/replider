import 'package:flutter/material.dart';

import 'package:app/design_system/components/custom_text_field.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

/// Allowed metadata field types, mirroring the Go `MetadataFieldType` enum
/// (text/number/cpf/cnpj/date/bool/enum). Exposed so callers can validate.
const List<String> kMetadataFieldTypes = [
  'text',
  'number',
  'cpf',
  'cnpj',
  'date',
  'bool',
  'enum',
];

final RegExp _snakeCaseRe = RegExp(r'^[a-z][a-z0-9_]*$');

/// Plain UI value object for one schema field. Kept json-free so the editor
/// stays a reusable design-system component; the sales feature converts to/from
/// its `MetadataFieldModel`.
class MetadataFieldData {
  String key;
  String label;
  String type;
  bool required;
  String aiHint;
  List<String> enumOptions;

  MetadataFieldData({
    this.key = '',
    this.label = '',
    this.type = 'text',
    this.required = false,
    this.aiHint = '',
    List<String>? enumOptions,
  }) : enumOptions = enumOptions ?? [];
}

/// Returns a localised error if [fields] are not a valid schema (blank /
/// non-snake_case / duplicate keys, or an enum field without options), else
/// null. Shared by the editor (inline hints) and the parent form (save gate).
String? validateMetadataFields(List<MetadataFieldData> fields) {
  final seen = <String>{};
  for (final f in fields) {
    final key = f.key.trim();
    if (key.isEmpty) return t.sales.schema.error_key_required;
    if (!_snakeCaseRe.hasMatch(key)) return t.sales.schema.error_key_format;
    if (!seen.add(key)) return t.sales.schema.error_key_duplicate;
    if (f.type == 'enum' &&
        f.enumOptions.where((o) => o.trim().isNotEmpty).isEmpty) {
      return t.sales.schema.error_enum_required;
    }
  }
  return null;
}

/// Reusable editor for a funnel's dynamic metadata schema (US-009). Each row
/// declares a field the AI should invisibly collect: snake_case key, label,
/// type, required toggle and an AI hint (plus comma-separated options for enum
/// fields). Self-managing: it seeds from [initialFields] and emits the current
/// list via [onChanged] on every edit, so the parent only stores the value and
/// computes dirtiness. Inline validation flags blank/bad/duplicate keys.
class MetadataSchemaEditor extends StatefulWidget {
  final List<MetadataFieldData> initialFields;
  final ValueChanged<List<MetadataFieldData>> onChanged;
  final int maxFields;

  const MetadataSchemaEditor({
    super.key,
    required this.initialFields,
    required this.onChanged,
    this.maxFields = 30,
  });

  @override
  State<MetadataSchemaEditor> createState() => _MetadataSchemaEditorState();
}

class _MetadataSchemaEditorState extends State<MetadataSchemaEditor> {
  late final List<_RowControllers> _rows;

  @override
  void initState() {
    super.initState();
    _rows = widget.initialFields.map(_RowControllers.fromData).toList();
  }

  @override
  void dispose() {
    for (final r in _rows) {
      r.dispose();
    }
    super.dispose();
  }

  void _emit() => widget.onChanged(_rows.map((r) => r.toData()).toList());

  void _addRow() {
    setState(() => _rows.add(_RowControllers.fromData(MetadataFieldData())));
    _emit();
  }

  void _removeRow(int index) {
    setState(() {
      _rows.removeAt(index).dispose();
    });
    _emit();
  }

  /// Inline per-row key error (blank/non-snake_case/duplicate within the list).
  String? _keyError(int index) {
    final key = _rows[index].keyCtrl.text.trim();
    if (key.isEmpty)
      return null; // empty handled at save time, no nag while typing
    if (!_snakeCaseRe.hasMatch(key)) return t.sales.schema.error_key_format;
    for (var i = 0; i < _rows.length; i++) {
      if (i != index && _rows[i].keyCtrl.text.trim() == key) {
        return t.sales.schema.error_key_duplicate;
      }
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final atLimit = _rows.length >= widget.maxFields;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (_rows.isEmpty)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 12),
            child: Text(
              t.sales.schema.no_fields,
              style: TextStyle(color: colors.textMuted, fontSize: 13),
            ),
          ),
        for (var i = 0; i < _rows.length; i++) ...[
          _fieldCard(colors, i),
          const SizedBox(height: 10),
        ],
        Align(
          alignment: Alignment.centerLeft,
          child: OutlinedButton.icon(
            onPressed: atLimit ? null : _addRow,
            icon: const Icon(Icons.add, size: 18),
            label: Text(t.sales.schema.add_field),
            style: OutlinedButton.styleFrom(
              foregroundColor: colors.bluePrimary,
              side: BorderSide(color: colors.borderLight),
            ),
          ),
        ),
      ],
    );
  }

  Widget _fieldCard(AppColorsExtension colors, int index) {
    final row = _rows[index];
    final keyError = _keyError(index);

    return Container(
      decoration: BoxDecoration(
        color: colors.backgroundPrimary,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: colors.borderLight),
      ),
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _label(colors, t.sales.schema.field_key),
                    CustomTextField(
                      hint: t.sales.schema.field_key_hint,
                      controller: row.keyCtrl,
                      onChanged: (_) {
                        setState(() {}); // refresh inline error
                        _emit();
                      },
                    ),
                    if (keyError != null)
                      Padding(
                        padding: const EdgeInsets.only(top: 4, left: 4),
                        child: Text(
                          keyError,
                          style:
                              TextStyle(color: colors.redAlert, fontSize: 11),
                        ),
                      ),
                  ],
                ),
              ),
              IconButton(
                tooltip: t.sales.schema.remove_field,
                onPressed: () => _removeRow(index),
                icon: Icon(Icons.delete_outline, color: colors.redAlert),
              ),
            ],
          ),
          const SizedBox(height: 10),
          _label(colors, t.sales.schema.field_label),
          CustomTextField(
            hint: t.sales.schema.field_label_hint,
            controller: row.labelCtrl,
            onChanged: (_) => _emit(),
          ),
          const SizedBox(height: 10),
          Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _label(colors, t.sales.schema.field_type),
                    _typeDropdown(colors, row),
                  ],
                ),
              ),
              const SizedBox(width: 12),
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _label(colors, t.sales.schema.field_required),
                  Switch(
                    value: row.required,
                    activeThumbColor: colors.whatsappGreen,
                    onChanged: (v) {
                      setState(() => row.required = v);
                      _emit();
                    },
                  ),
                ],
              ),
            ],
          ),
          if (row.type == 'enum') ...[
            const SizedBox(height: 10),
            _label(colors, t.sales.schema.field_enum_options),
            CustomTextField(
              hint: t.sales.schema.field_enum_options_hint,
              controller: row.enumCtrl,
              onChanged: (_) => _emit(),
            ),
          ],
          const SizedBox(height: 10),
          _label(colors, t.sales.schema.field_ai_hint),
          CustomTextField(
            hint: t.sales.schema.field_ai_hint_hint,
            controller: row.aiHintCtrl,
            onChanged: (_) => _emit(),
          ),
        ],
      ),
    );
  }

  Widget _typeDropdown(AppColorsExtension colors, _RowControllers row) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: colors.borderLight),
      ),
      child: DropdownButtonHideUnderline(
        child: DropdownButton<String>(
          value: row.type,
          isExpanded: true,
          dropdownColor: colors.backgroundSecondary,
          style: TextStyle(color: colors.textPrimary, fontSize: 14),
          items: [
            for (final type in kMetadataFieldTypes)
              DropdownMenuItem(value: type, child: Text(_typeLabel(type))),
          ],
          onChanged: (v) {
            if (v == null) return;
            setState(() => row.type = v);
            _emit();
          },
        ),
      ),
    );
  }

  String _typeLabel(String type) {
    final types = t.sales.schema.types;
    switch (type) {
      case 'text':
        return types.text;
      case 'number':
        return types.number;
      case 'cpf':
        return types.cpf;
      case 'cnpj':
        return types.cnpj;
      case 'date':
        return types.date;
      case 'bool':
        return types.bool;
      case 'enum':
        return types.kEnum;
      default:
        return type;
    }
  }

  Widget _label(AppColorsExtension colors, String text) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Text(
        text,
        style: TextStyle(
          color: colors.textSecondary,
          fontSize: 12,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}

/// Holds the live [TextEditingController]s + non-text state for one schema row.
class _RowControllers {
  final TextEditingController keyCtrl;
  final TextEditingController labelCtrl;
  final TextEditingController aiHintCtrl;
  final TextEditingController enumCtrl;
  String type;
  bool required;

  _RowControllers({
    required this.keyCtrl,
    required this.labelCtrl,
    required this.aiHintCtrl,
    required this.enumCtrl,
    required this.type,
    required this.required,
  });

  factory _RowControllers.fromData(MetadataFieldData d) => _RowControllers(
        keyCtrl: TextEditingController(text: d.key),
        labelCtrl: TextEditingController(text: d.label),
        aiHintCtrl: TextEditingController(text: d.aiHint),
        enumCtrl: TextEditingController(text: d.enumOptions.join(', ')),
        type: d.type,
        required: d.required,
      );

  MetadataFieldData toData() => MetadataFieldData(
        key: keyCtrl.text.trim(),
        label: labelCtrl.text.trim(),
        type: type,
        required: required,
        aiHint: aiHintCtrl.text.trim(),
        enumOptions: type == 'enum'
            ? enumCtrl.text
                .split(',')
                .map((o) => o.trim())
                .where((o) => o.isNotEmpty)
                .toList()
            : [],
      );

  void dispose() {
    keyCtrl.dispose();
    labelCtrl.dispose();
    aiHintCtrl.dispose();
    enumCtrl.dispose();
  }
}
