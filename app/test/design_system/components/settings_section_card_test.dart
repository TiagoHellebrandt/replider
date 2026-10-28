import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/settings_section_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Widget harness(Widget child) =>
      MaterialApp(theme: AppTheme.darkTheme, home: Scaffold(body: child));

  testWidgets('renders title, icon, subtitle and body', (tester) async {
    await tester.pumpWidget(harness(
      const SettingsSectionCard(
        title: 'Section',
        icon: Icons.business,
        subtitle: 'What this section is for',
        subtitleKey: Key('card_subtitle'),
        child: Text('body'),
      ),
    ));

    expect(find.text('Section'), findsOneWidget);
    expect(find.byIcon(Icons.business), findsOneWidget);
    expect(find.byKey(const Key('card_subtitle')), findsOneWidget);
    expect(find.text('body'), findsOneWidget);
  });

  testWidgets('accent tints the title', (tester) async {
    await tester.pumpWidget(harness(
      const SettingsSectionCard(
        title: 'Danger',
        icon: Icons.block,
        accent: Colors.red,
        child: SizedBox.shrink(),
      ),
    ));

    final title = tester.widget<Text>(find.text('Danger'));
    expect(title.style?.color, Colors.red);
    final icon = tester.widget<Icon>(find.byIcon(Icons.block));
    expect(icon.color, Colors.red);
  });

  testWidgets('SettingsPageBody holds content to the 640px column',
      (tester) async {
    tester.view.physicalSize = const Size(1600, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(harness(
      const SettingsPageBody(
        children: [
          SizedBox(key: Key('content'), height: 10),
        ],
      ),
    ));

    // The child stretches to the column, and the column stops at 640
    // even with 1600px available.
    expect(tester.getSize(find.byKey(const Key('content'))).width, 640);
  });
}
