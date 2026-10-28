import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/websocket_service.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/support/presentation/widgets/support_nav_button.dart';
import 'package:app/i18n/strings.g.dart';

class _FakeAuth extends AuthController {
  _FakeAuth(this.user);
  final UserModel? user;

  @override
  FutureOr<UserModel?> build() async => user;
}

const _customerUser = UserModel(
  id: 'user-1',
  name: 'Cliente',
  email: 'c@x.com',
  token: 't',
  role: 'attendant',
);

const _platformUser = UserModel(
  id: 'plat-1',
  name: 'Vendedor',
  email: 'v@replider.com',
  token: 't',
  role: 'attendant',
  isPlatformAccount: true,
);

WSEvent _platformReply({required String threadOwnerId, required String id}) {
  return WSEvent.fromJson({
    'event': 'support_message',
    'internal_thread': true,
    'user_id': threadOwnerId,
    'conversation_id': 'conv-1',
    'message': {
      'id': id,
      'content': 'resposta',
      'direction': 'outbound',
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

  Widget wrap(UserModel? user, Stream<WSEvent> wsStream) {
    return ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(() => _FakeAuth(user)),
        wsEventsProvider.overrideWith((ref) => wsStream),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: SupportNavButton()),
      ),
    );
  }

  testWidgets('hidden for platform-account users', (tester) async {
    await tester.pumpWidget(wrap(_platformUser, const Stream.empty()));
    await tester.pumpAndSettle();

    expect(find.byIcon(Icons.support_agent), findsNothing);
  });

  testWidgets('visible for customer users, no badge while read',
      (tester) async {
    await tester.pumpWidget(wrap(_customerUser, const Stream.empty()));
    await tester.pumpAndSettle();

    expect(find.byIcon(Icons.support_agent), findsOneWidget);
    expect(find.text('1'), findsNothing);
  });

  testWidgets('badge counts platform replies for my thread only',
      (tester) async {
    final ws = StreamController<WSEvent>.broadcast();
    addTearDown(ws.close);

    await tester.pumpWidget(wrap(_customerUser, ws.stream));
    await tester.pumpAndSettle();

    // Another user's thread must not badge me (per-user isolation).
    ws.add(_platformReply(threadOwnerId: 'other-user', id: 'm-1'));
    await tester.pumpAndSettle();
    expect(find.text('1'), findsNothing);

    ws.add(_platformReply(threadOwnerId: 'user-1', id: 'm-2'));
    await tester.pumpAndSettle();
    expect(find.text('1'), findsOneWidget);

    ws.add(_platformReply(threadOwnerId: 'user-1', id: 'm-3'));
    await tester.pumpAndSettle();
    expect(find.text('2'), findsOneWidget);
  });
}
