import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/websocket_service.dart';
import 'package:app/core/notifications/push_notification_controller.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/support/data/support_repository.dart';
import 'package:app/features/support/domain/support_message.dart';
import 'package:app/features/support/presentation/screens/support_screen.dart';
import 'package:app/i18n/strings.g.dart';

class _FakeAuth extends AuthController {
  _FakeAuth(this.user);
  final UserModel? user;

  @override
  FutureOr<UserModel?> build() async => user;
}

class _FakeSupportRepository implements SupportRepository {
  @override
  Future<SupportThread> fetchThread() async => const SupportThread();

  @override
  Future<SupportSendResult> send(String content) async {
    return SupportSendResult(
      conversationId: 'conv-1',
      message: SupportMessage(id: 'sent-1', text: content, fromMe: true),
    );
  }
}

/// Overrides the real controller so tests control the push status without
/// a browser: build() skips the kIsWeb/service checks entirely.
class _FakePushController extends PushNotificationController {
  _FakePushController(this.initial);
  final PushSubscriptionStatus initial;
  int enableCalls = 0;

  @override
  Future<PushState> build() async => PushState(status: initial);

  @override
  Future<void> enable() async {
    enableCalls++;
    state = const AsyncValue.data(
      PushState(status: PushSubscriptionStatus.subscribed),
    );
  }
}

const _user = UserModel(
  id: 'user-1',
  name: 'Cliente',
  email: 'c@x.com',
  token: 't',
  role: 'admin',
);

void main() {
  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
    FlutterSecureStorage.setMockInitialValues({});
  });

  Widget wrap(_FakePushController push) {
    return ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(() => _FakeAuth(_user)),
        supportRepositoryProvider.overrideWithValue(_FakeSupportRepository()),
        wsEventsProvider.overrideWith((ref) => const Stream.empty()),
        pushNotificationControllerProvider.overrideWith(() => push),
      ],
      child:
          MaterialApp(theme: AppTheme.darkTheme, home: const SupportScreen()),
    );
  }

  testWidgets('shows the opt-in prompt on first use when not subscribed',
      (tester) async {
    final push = _FakePushController(PushSubscriptionStatus.notSubscribed);
    await tester.pumpWidget(wrap(push));
    await tester.pumpAndSettle();

    expect(find.text(t.support.push_prompt_title), findsOneWidget);
    expect(find.text(t.support.push_prompt_enable), findsOneWidget);
  });

  testWidgets('stays hidden when already subscribed', (tester) async {
    final push = _FakePushController(PushSubscriptionStatus.subscribed);
    await tester.pumpWidget(wrap(push));
    await tester.pumpAndSettle();

    expect(find.text(t.support.push_prompt_title), findsNothing);
  });

  testWidgets('stays hidden when the browser permission is denied',
      (tester) async {
    final push = _FakePushController(PushSubscriptionStatus.denied);
    await tester.pumpWidget(wrap(push));
    await tester.pumpAndSettle();

    expect(find.text(t.support.push_prompt_title), findsNothing);
  });

  testWidgets('does not re-ask after a persisted dismissal', (tester) async {
    FlutterSecureStorage.setMockInitialValues(
        {'support_push_prompt_dismissed': '1'});
    final push = _FakePushController(PushSubscriptionStatus.notSubscribed);
    await tester.pumpWidget(wrap(push));
    await tester.pumpAndSettle();

    expect(find.text(t.support.push_prompt_title), findsNothing);
  });

  testWidgets('enable subscribes, hides the prompt and persists the choice',
      (tester) async {
    final push = _FakePushController(PushSubscriptionStatus.notSubscribed);
    await tester.pumpWidget(wrap(push));
    await tester.pumpAndSettle();

    await tester.tap(find.text(t.support.push_prompt_enable));
    await tester.pumpAndSettle();

    expect(push.enableCalls, 1);
    expect(find.text(t.support.push_prompt_title), findsNothing);
    expect(
      await const FlutterSecureStorage()
          .read(key: 'support_push_prompt_dismissed'),
      isNotNull,
    );
  });

  testWidgets('"not now" hides the prompt and persists the choice',
      (tester) async {
    final push = _FakePushController(PushSubscriptionStatus.notSubscribed);
    await tester.pumpWidget(wrap(push));
    await tester.pumpAndSettle();

    await tester.tap(find.text(t.support.push_prompt_later));
    await tester.pumpAndSettle();

    expect(push.enableCalls, 0);
    expect(find.text(t.support.push_prompt_title), findsNothing);
    expect(
      await const FlutterSecureStorage()
          .read(key: 'support_push_prompt_dismissed'),
      isNotNull,
    );
  });
}
