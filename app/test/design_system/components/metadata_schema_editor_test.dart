import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/metadata_schema_editor.dart';
import 'package:app/i18n/strings.g.dart';

void main() {
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  group('validateMetadataFields', () {
    test('accepts a valid snake_case schema', () {
      final fields = [
        MetadataFieldData(key: 'cpf', type: 'cpf'),
        MetadataFieldData(key: 'plan_size', type: 'enum', enumOptions: ['s']),
      ];
      expect(validateMetadataFields(fields), isNull);
    });

    test('rejects a blank key', () {
      expect(
        validateMetadataFields([MetadataFieldData(key: '')]),
        t.sales.schema.error_key_required,
      );
    });

    test('rejects a non-snake_case key', () {
      expect(
        validateMetadataFields([MetadataFieldData(key: 'CamelCase')]),
        t.sales.schema.error_key_format,
      );
    });

    test('rejects duplicate keys', () {
      expect(
        validateMetadataFields([
          MetadataFieldData(key: 'cpf'),
          MetadataFieldData(key: 'cpf'),
        ]),
        t.sales.schema.error_key_duplicate,
      );
    });

    test('rejects an enum field without options', () {
      expect(
        validateMetadataFields([MetadataFieldData(key: 'size', type: 'enum')]),
        t.sales.schema.error_enum_required,
      );
    });
  });

  group('MetadataSchemaEditor widget', () {
    Future<void> pumpEditor(
      WidgetTester tester, {
      required List<MetadataFieldData> initial,
      required ValueChanged<List<MetadataFieldData>> onChanged,
    }) async {
      await tester.pumpWidget(MaterialApp(
        theme: AppTheme.darkTheme,
        home: Scaffold(
          body: SingleChildScrollView(
            child: MetadataSchemaEditor(
              initialFields: initial,
              onChanged: onChanged,
            ),
          ),
        ),
      ));
    }

    testWidgets('shows the empty hint with no fields', (tester) async {
      await pumpEditor(tester, initial: [], onChanged: (_) {});
      expect(find.text(t.sales.schema.no_fields), findsOneWidget);
    });

    testWidgets('Add field emits a new row', (tester) async {
      List<MetadataFieldData>? emitted;
      await pumpEditor(tester, initial: [], onChanged: (f) => emitted = f);

      await tester.tap(find.text(t.sales.schema.add_field));
      await tester.pump();

      expect(emitted, isNotNull);
      expect(emitted, hasLength(1));
      expect(find.text(t.sales.schema.field_key), findsOneWidget);
    });

    testWidgets('renders existing fields and removes one', (tester) async {
      List<MetadataFieldData>? emitted;
      await pumpEditor(
        tester,
        initial: [MetadataFieldData(key: 'cpf', label: 'CPF')],
        onChanged: (f) => emitted = f,
      );

      expect(find.text(t.sales.schema.field_key), findsOneWidget);

      await tester.tap(find.byIcon(Icons.delete_outline));
      await tester.pump();

      expect(emitted, isEmpty);
      expect(find.text(t.sales.schema.no_fields), findsOneWidget);
    });
  });
}
