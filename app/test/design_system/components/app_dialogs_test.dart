import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pointer_interceptor/pointer_interceptor.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/i18n/strings.g.dart';

void main() {
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  Future<String?> openInputDialog(
    WidgetTester tester, {
    String? initialValue,
  }) async {
    String? result;
    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.darkTheme,
        home: Scaffold(
          body: Builder(
            builder: (context) => Center(
              child: ElevatedButton(
                onPressed: () async {
                  result = await showInputDialog(
                    context,
                    title: 'Reason',
                    hint: 'Why?',
                    initialValue: initialValue,
                  );
                },
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    return result;
  }

  testWidgets('showInputDialog returns the trimmed text on confirm',
      (tester) async {
    await openInputDialog(tester);
    expect(find.text('Reason'), findsOneWidget);

    await tester.enterText(find.byType(TextField), '  abuse  ');
    await tester.tap(find.text(t.common.confirm));
    await tester.pumpAndSettle();

    // The dialog is gone and the value was trimmed.
    expect(find.text('Reason'), findsNothing);
  });

  testWidgets('showInputDialog returns null on cancel', (tester) async {
    await openInputDialog(tester, initialValue: 'seed');
    // Seeded initial value is shown.
    expect(find.text('seed'), findsOneWidget);

    await tester.tap(find.text(t.common.cancel));
    await tester.pumpAndSettle();
    expect(find.text('Reason'), findsNothing);
  });

  // Regression: on web an HtmlElementView (the email reader's iframe) is a
  // real DOM element above Flutter's canvas — a dialog floating over it was
  // visible but unclickable, because the pointer landed in the iframe. The
  // PointerInterceptor under each dialog is what routes the click to the
  // buttons; if it disappears, the "Ver original com imagens" confirm goes
  // dead again.
  Future<bool?> openConfirmDialog(WidgetTester tester) async {
    bool? result;
    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.darkTheme,
        home: Scaffold(
          body: Builder(
            builder: (context) => Center(
              child: ElevatedButton(
                onPressed: () async {
                  result = await showConfirmDialog(
                    context,
                    title: 'Load images?',
                    message: 'Tells the sender you opened it.',
                    confirmText: 'Load',
                  );
                },
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    return result;
  }

  testWidgets('showConfirmDialog sits behind a PointerInterceptor',
      (tester) async {
    await openConfirmDialog(tester);
    expect(
      find.ancestor(
        of: find.byType(AlertDialog),
        matching: find.byType(PointerInterceptor),
      ),
      findsOneWidget,
    );

    await tester.tap(find.text('Load'));
    await tester.pumpAndSettle();
    expect(find.text('Load images?'), findsNothing);
  });

  testWidgets('showInputDialog sits behind a PointerInterceptor',
      (tester) async {
    await openInputDialog(tester);
    expect(
      find.ancestor(
        of: find.byType(AlertDialog),
        matching: find.byType(PointerInterceptor),
      ),
      findsOneWidget,
    );
  });
}
