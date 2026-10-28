import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/core/network/websocket_service.dart';
import 'package:app/features/dashboard/presentation/widgets/conversation_list_panel.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/home/domain/models/onboarding_progress_model.dart';
import 'package:app/features/home/presentation/controllers/onboarding_progress_controller.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/i18n/strings.g.dart';

void main() {
  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  Widget createPanel({required bool onboardingCompleted}) {
    return ProviderScope(
      overrides: [
        conversationsProvider.overrideWith((ref) async => const []),
        // No live WS handshake in tests.
        wsEventsProvider.overrideWith((ref) => const Stream<WSEvent>.empty()),
        onboardingProgressControllerProvider.overrideWith(
          () => _FakeOnboardingProgress(onboardingCompleted),
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: ConversationListPanel()),
      ),
    );
  }

  testWidgets(
      'empty inbox for a not-yet-onboarded account shows the connect-channel CTA',
      (WidgetTester tester) async {
    await tester.pumpWidget(createPanel(onboardingCompleted: false));
    await tester.pumpAndSettle();

    expect(find.text(t.dashboard.empty_inbox_cta_setup), findsOneWidget);
    expect(find.text(t.dashboard.empty_inbox_cta_share), findsNothing);
  });

  testWidgets('empty inbox for an onboarded account shows the share-link CTA',
      (WidgetTester tester) async {
    await tester.pumpWidget(createPanel(onboardingCompleted: true));
    await tester.pumpAndSettle();

    expect(find.text(t.dashboard.empty_inbox_cta_share), findsOneWidget);
    expect(find.text(t.dashboard.empty_inbox_cta_setup), findsNothing);
  });
}

class _FakeOnboardingProgress extends OnboardingProgressController {
  _FakeOnboardingProgress(this._completed);
  final bool _completed;

  @override
  Future<OnboardingProgressModel?> build() async {
    return OnboardingProgressModel(
      agentCreated: true,
      previewChatTested: _completed,
      channelConnected: _completed,
      knowledgeAdded: _completed,
      firstInboundReceived: false,
      onboardingCompleted: _completed,
      completionPercentage: _completed ? 100 : 25,
    );
  }
}
