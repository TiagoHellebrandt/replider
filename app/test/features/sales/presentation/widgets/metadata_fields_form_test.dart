import 'package:flutter/material.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/features/sales/presentation/widgets/metadata_fields_form.dart';
import 'package:app/i18n/strings.g.dart';

// The funnel declares what it needs to know about a lead — key, label, type,
// whether it is required — and none of it reached a screen. One component,
// used by the create form and the panel, because the alternative is two
// screens disagreeing about what is required.
void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  Future<Map<String, String>> pump(
    WidgetTester tester, {
    required List<MetadataFieldModel> schema,
    Map<String, String> values = const {},
  }) async {
    final captured = <String, String>{};
    await tester.pumpWidget(MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: SingleChildScrollView(
          child: MetadataFieldsForm(
            schema: schema,
            values: values,
            onChanged: (k, v) => captured[k] = v,
          ),
        ),
      ),
    ));
    return captured;
  }

  testWidgets('the funnel label shows, not the column key', (tester) async {
    await pump(tester, schema: const [
      MetadataFieldModel(key: 'n_equipamentos', label: 'Nº de equipamentos'),
    ]);

    expect(find.text('Nº de equipamentos'), findsOneWidget);
    expect(find.text('n_equipamentos'), findsNothing);
  });

  // With no label configured the key is the last resort — worse than the right
  // question, better than an unnamed field.
  testWidgets('with no label, it falls back to the key', (tester) async {
    await pump(tester,
        schema: const [MetadataFieldModel(key: 'cnpj', label: '')]);

    expect(find.text('cnpj'), findsOneWidget);
  });

  testWidgets('a required field is marked', (tester) async {
    await pump(tester, schema: const [
      MetadataFieldModel(key: 'a', label: 'Campo A', isRequired: true),
      MetadataFieldModel(key: 'b', label: 'Campo B'),
    ]);

    expect(find.text(' *'), findsOneWidget);
  });

  // The type is not decoration: it is what stops a "yes" written three ways
  // from becoming three values in a report.
  testWidgets('an enum becomes a list, not a text box', (tester) async {
    await pump(tester, schema: const [
      MetadataFieldModel(
        key: 'convenio',
        label: 'Convênio',
        type: 'enum',
        enumOptions: ['Unimed', 'Bradesco'],
      ),
    ]);

    expect(find.byType(DropdownButtonFormField<String>), findsOneWidget);
  });

  testWidgets('a bool becomes a switch and returns true/false', (tester) async {
    final captured = await pump(tester, schema: const [
      MetadataFieldModel(key: 'ativo', label: 'Ativo', type: 'bool'),
    ]);

    await tester.tap(find.byType(Switch));
    await tester.pump();

    expect(captured['ativo'], 'true');
  });

  testWidgets('an already-collected value starts in the field', (tester) async {
    await pump(
      tester,
      schema: const [MetadataFieldModel(key: 'cidade', label: 'Cidade')],
      values: const {'cidade': 'Sorocaba'},
    );

    expect(find.text('Sorocaba'), findsOneWidget);
  });

  testWidgets('typing notifies whoever is listening', (tester) async {
    final captured = await pump(tester,
        schema: const [MetadataFieldModel(key: 'cidade', label: 'Cidade')]);

    await tester.enterText(
        find.byKey(const Key('metadata_field_cidade')), 'Campinas');

    expect(captured['cidade'], 'Campinas');
  });

  // A funnel that asks for nothing must not render an empty titled section.
  testWidgets('an empty schema draws nothing', (tester) async {
    await pump(tester, schema: const []);

    expect(find.byType(TextField), findsNothing);
    expect(find.byType(Switch), findsNothing);
  });
}
