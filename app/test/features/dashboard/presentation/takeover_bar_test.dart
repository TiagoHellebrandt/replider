import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/dashboard/presentation/widgets/takeover_bar.dart';
import 'package:app/i18n/strings.g.dart';

// Widget-level tests for the Human Takeover action bar. The widget is
// a pure function of its inputs (no Dio, no WS), so we can assert the
// state machine without stubbing the whole dashboard provider graph.

void main() {
  setUpAll(() {
    LocaleSettings.setLocale(AppLocale.pt);
  });

  Widget mountBar({
    required bool isBotPaused,
    required bool assignedToMe,
    required bool assignedToOther,
    String? assignedToName,
  }) {
    return ProviderScope(
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: Scaffold(
          body: TakeoverBar(
            conversationId: 'conv-1',
            isBotPaused: isBotPaused,
            assignedToMe: assignedToMe,
            assignedToOther: assignedToOther,
            assignedToName: assignedToName,
            isSending: false,
          ),
        ),
      ),
    );
  }

  testWidgets('shows "Assumir conversa" when bot is running', (tester) async {
    await tester.pumpWidget(mountBar(
      isBotPaused: false,
      assignedToMe: false,
      assignedToOther: false,
    ));
    await tester.pump();

    expect(find.text('Assumir conversa'), findsOneWidget);
    expect(find.text('Devolver ao bot'), findsNothing);
    // Static "take-over-to-reply" hint is shown when the input is gated.
    expect(find.textContaining('Assuma a conversa'), findsOneWidget);
  });

  testWidgets('shows "Devolver ao bot" and takeover badge when assigned to me',
      (tester) async {
    await tester.pumpWidget(mountBar(
      isBotPaused: true,
      assignedToMe: true,
      assignedToOther: false,
      assignedToName: 'Alice',
    ));
    await tester.pump();

    expect(find.text('Devolver ao bot'), findsOneWidget);
    expect(find.text('Assumir conversa'), findsNothing);
    expect(find.text('Em atendimento por Alice'), findsOneWidget);
  });

  testWidgets(
      'shows "Assumir conversa" and other-attendant badge when taken by teammate',
      (tester) async {
    await tester.pumpWidget(mountBar(
      isBotPaused: true,
      assignedToMe: false,
      assignedToOther: true,
      assignedToName: 'Bob',
    ));
    await tester.pump();

    expect(find.text('Assumir conversa'), findsOneWidget);
    expect(find.text('Em atendimento por Bob'), findsOneWidget);
  });
}
