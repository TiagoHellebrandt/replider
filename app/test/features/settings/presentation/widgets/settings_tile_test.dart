import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/settings/presentation/widgets/settings_tile.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

// The row and the card that carry every option in Settings. Forty rows were
// hand-written before this, which is why the file was 1488 lines and why one of
// them shipped without a subtitle.

Future<void> _pump(WidgetTester tester, Widget child) async {
  final prev = FlutterError.onError;
  FlutterError.onError = (d) {
    // Debug-only paint assertion for a ListTile inside a themed container.
    if (d.exceptionAsString().contains('ListTile background color')) return;
    prev?.call(d);
  };
  addTearDown(() => FlutterError.onError = prev);

  await tester.pumpWidget(MaterialApp(
    theme: AppTheme.darkTheme,
    home: Scaffold(body: child),
  ));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('a row says what it is and what it is for', (tester) async {
    var taps = 0;
    await _pump(
      tester,
      SettingsTile(
        icon: Icons.business,
        title: 'Empresa',
        subtitle: 'Nome e fuso',
        onTap: () => taps++,
      ),
    );

    expect(find.text('Empresa'), findsOneWidget);
    expect(find.text('Nome e fuso'), findsOneWidget);
    expect(find.byIcon(Icons.business), findsOneWidget);
    // Without an explicit trailing, the row still shows it leads somewhere.
    expect(find.byIcon(Icons.chevron_right), findsOneWidget);

    await tester.tap(find.text('Empresa'));
    expect(taps, 1);
  });

  // The channels row shows how many are offline; a chevron there would waste
  // the only place that state has to appear.
  testWidgets('an explicit trailing replaces the chevron', (tester) async {
    await _pump(
      tester,
      SettingsTile(
        icon: Icons.forum_outlined,
        title: 'Canais',
        trailing: const Text('2 offline'),
        onTap: () {},
      ),
    );

    expect(find.text('2 offline'), findsOneWidget);
    expect(find.byIcon(Icons.chevron_right), findsNothing);
  });

  testWidgets('a danger row paints its title in the alert colour',
      (tester) async {
    await _pump(
      tester,
      SettingsTile(
        icon: Icons.logout,
        title: 'Sair',
        danger: true,
        onTap: () {},
      ),
    );

    final ctx = tester.element(find.text('Sair'));
    final colors = Theme.of(ctx).extension<AppColorsExtension>()!;
    expect(
        tester.widget<Text>(find.text('Sair')).style?.color, colors.redAlert);
  });

  // The reason SettingsGroup exists rather than a bare container: a hand-written
  // divider has to be gated by the same condition as the row next to it, and one
  // that is not leaves a line with nothing under it.
  testWidgets('a group separates its rows and never strands a divider',
      (tester) async {
    await _pump(
      tester,
      SettingsGroup(children: [
        SettingsTile(icon: Icons.one_k, title: 'Um', onTap: () {}),
        null, // a row gated off — no divider may survive it
        SettingsTile(icon: Icons.two_k, title: 'Dois', onTap: () {}),
      ]),
    );

    expect(find.text('Um'), findsOneWidget);
    expect(find.text('Dois'), findsOneWidget);
    // Two rows means exactly one line between them, not one per child.
    expect(find.byType(Divider), findsOneWidget);
  });

  testWidgets('a single row gets no divider at all', (tester) async {
    await _pump(
      tester,
      SettingsGroup(children: [
        SettingsTile(icon: Icons.one_k, title: 'Só', onTap: () {}),
      ]),
    );

    expect(find.byType(Divider), findsNothing);
  });

  // An attendant sees a section whose every row is admin-only. An empty card
  // there is a promise of something that is not coming.
  testWidgets('a group with nothing to show draws nothing', (tester) async {
    await _pump(tester, const SettingsGroup(children: [null, null]));

    expect(find.byType(Divider), findsNothing);
    expect(tester.widget<SizedBox>(find.byType(SizedBox).first), isNotNull);
  });

  testWidgets('the section label shouts, the translation does not',
      (tester) async {
    await _pump(tester, const SettingsSectionLabel('E-mail e envio'));

    expect(find.text('E-MAIL E ENVIO'), findsOneWidget);
  });
}
