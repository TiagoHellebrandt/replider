import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/websocket_service.dart';
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
  _FakeSupportRepository({SupportThread? initial})
      : initial = initial ?? const SupportThread();

  final SupportThread initial;
  final List<String> sent = [];

  @override
  Future<SupportThread> fetchThread() async => initial;

  @override
  Future<SupportSendResult> send(String content) async {
    sent.add(content);
    return SupportSendResult(
      conversationId: 'conv-1',
      message: SupportMessage(
        id: 'sent-${sent.length}',
        text: content,
        fromMe: true,
      ),
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

WSEvent _supportEvent({
  required String threadOwnerId,
  required String messageId,
  required String content,
  String direction = 'outbound',
}) {
  return WSEvent.fromJson({
    'event': 'support_message',
    'internal_thread': true,
    'user_id': threadOwnerId,
    'conversation_id': 'conv-1',
    'message': {
      'id': messageId,
      'content': content,
      'direction': direction,
      'sender': 'bot',
      'created_at': '2026-07-10T12:00:00Z',
    },
  });
}

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  Widget wrap({
    required _FakeSupportRepository repo,
    required Stream<WSEvent> wsStream,
  }) {
    return ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(() => _FakeAuth(_user)),
        supportRepositoryProvider.overrideWithValue(repo),
        wsEventsProvider.overrideWith((ref) => wsStream),
      ],
      child:
          MaterialApp(theme: AppTheme.darkTheme, home: const SupportScreen()),
    );
  }

  testWidgets('shows welcome empty state before first contact', (tester) async {
    final repo = _FakeSupportRepository();
    await tester.pumpWidget(wrap(repo: repo, wsStream: const Stream.empty()));
    await tester.pumpAndSettle();

    expect(find.text(t.support.empty_title), findsOneWidget);
  });

  testWidgets('sending a message posts to the API and renders the bubble',
      (tester) async {
    final repo = _FakeSupportRepository();
    await tester.pumpWidget(wrap(repo: repo, wsStream: const Stream.empty()));
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextField), 'preciso de ajuda');
    await tester.tap(find.byIcon(Icons.send));
    await tester.pumpAndSettle();

    expect(repo.sent, ['preciso de ajuda']);
    expect(find.text('preciso de ajuda'), findsOneWidget);
  });

  testWidgets('renders an incoming platform reply from the WS event',
      (tester) async {
    final repo = _FakeSupportRepository();
    final ws = StreamController<WSEvent>.broadcast();
    addTearDown(ws.close);

    await tester.pumpWidget(wrap(repo: repo, wsStream: ws.stream));
    await tester.pumpAndSettle();

    ws.add(_supportEvent(
      threadOwnerId: 'user-1',
      messageId: 'm-9',
      content: 'Oi! Como posso ajudar?',
    ));
    await tester.pumpAndSettle();

    expect(find.text('Oi! Como posso ajudar?'), findsOneWidget);
  });

  testWidgets("ignores another user's thread event (per-user isolation)",
      (tester) async {
    final repo = _FakeSupportRepository();
    final ws = StreamController<WSEvent>.broadcast();
    addTearDown(ws.close);

    await tester.pumpWidget(wrap(repo: repo, wsStream: ws.stream));
    await tester.pumpAndSettle();

    ws.add(_supportEvent(
      threadOwnerId: 'other-user',
      messageId: 'm-10',
      content: 'mensagem de outro user',
    ));
    await tester.pumpAndSettle();

    expect(find.text('mensagem de outro user'), findsNothing);
    expect(find.text(t.support.empty_title), findsOneWidget);
  });

  testWidgets('does not duplicate own message when the WS echo arrives',
      (tester) async {
    final repo = _FakeSupportRepository();
    final ws = StreamController<WSEvent>.broadcast();
    addTearDown(ws.close);

    await tester.pumpWidget(wrap(repo: repo, wsStream: ws.stream));
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextField), 'oi');
    await tester.tap(find.byIcon(Icons.send));
    await tester.pumpAndSettle();

    // The WS echo of the message just persisted (same id as the POST
    // response) must not render twice.
    ws.add(_supportEvent(
      threadOwnerId: 'user-1',
      messageId: 'sent-1',
      content: 'oi',
      direction: 'inbound',
    ));
    await tester.pumpAndSettle();

    expect(find.text('oi'), findsOneWidget);
  });
}
