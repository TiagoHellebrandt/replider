import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/router/account_menu.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/i18n/strings.g.dart';

/// Reported from the dashboard: the account menu opened and then trapped the
/// person — clicking the dimmed area around it did nothing, so the only way out
/// was to pick an entry and navigate somewhere they had not asked to go.
///
/// Cause: `showGeneralDialog` defaults `barrierDismissible` to FALSE, unlike
/// `showDialog` and unlike the bottom sheet this same menu becomes on a narrow
/// window. The dimming is what promises the click works.

Widget _harness({required Size size}) {
  return ProviderScope(
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: MediaQuery(
        data: MediaQueryData(size: size),
        child: Builder(
          builder: (context) => Scaffold(
            body: Center(
              child: ElevatedButton(
                onPressed: () => showAccountMenu(context),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    ),
  );
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('on a wide window, clicking outside closes the menu',
      (tester) async {
    tester.view.physicalSize = const Size(1400, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_harness(size: const Size(1400, 900)));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    expect(find.text(t.account_menu.logout), findsOneWidget,
        reason: 'the menu must be open before the dismissal means anything');

    // The menu is anchored bottom-LEFT, so the far top-right is barrier and
    // nothing else.
    await tester.tapAt(const Offset(1300, 60));
    await tester.pumpAndSettle();

    expect(
      find.text(t.account_menu.logout),
      findsNothing,
      reason: 'clicking the dimmed area must close the menu — otherwise the '
          'only way out is to navigate somewhere unasked',
    );
  });

  testWidgets('the barrier names the dismissal, not a menu entry',
      (tester) async {
    tester.view.physicalSize = const Size(1400, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_harness(size: const Size(1400, 900)));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    // What a screen reader announces for "tap outside". Labelling it with a
    // menu ENTRY told the user that tapping away would open that entry.
    final route = ModalRoute.of(tester.element(find.byType(Scaffold)))!;
    expect(route.navigator!.widget.pages, isEmpty,
        reason: 'sanity: this app pushes dialogs imperatively, not via pages');
    expect(find.bySemanticsLabel(t.common.close), findsWidgets);
  });

  testWidgets('on a narrow window it is a sheet, which already dismissed',
      (tester) async {
    tester.view.physicalSize = const Size(420, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_harness(size: const Size(420, 900)));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    expect(find.text(t.account_menu.logout), findsOneWidget);

    // Guards the fix from being "fixed" by making both branches dialogs: the
    // phone width must stay a bottom sheet, anchored to the thumb.
    expect(find.byType(BottomSheet), findsOneWidget);
  });
}
