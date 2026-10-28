import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/sales/domain/models/board_model.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/features/sales/presentation/controllers/board_controller.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/features/sales/presentation/screens/funnel_branch_screen.dart';
import 'package:app/features/sales/presentation/screens/kanban_screen.dart';
import 'package:app/features/sales/presentation/widgets/discovered_contacts_body.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

// The three views of the customer list are three server surfaces with three
// role gates: the board admits a vendedor, /portfolio is attendant and above,
// /discovered-contacts is admin-only. Rendering a pill for a view the server
// refuses parked the person on a 403 with a Retry that could never succeed.

class _OneFunnel extends FunnelsList {
  @override
  Future<List<FunnelModel>> build() async =>
      [const FunnelModel(id: 'f1', name: 'Funil')];
}

class _FakeAuth extends AuthController {
  _FakeAuth(this.role);
  final String role;

  @override
  Future<UserModel?> build() async => UserModel(
        id: 'u1',
        name: 'Alguém',
        email: 'a@replider.com',
        token: 'tok',
        role: role,
      );
}

Board _board() => const Board(stages: [], cards: []);

Future<void> _pump(WidgetTester tester,
    {required String role, String location = '/funnel'}) async {
  tester.view.physicalSize = const Size(1400, 900);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  final router = GoRouter(
    initialLocation: location,
    routes: [
      GoRoute(
        path: '/funnel',
        builder: (context, state) => const FunnelBranchScreen(),
      ),
    ],
  );

  await tester.pumpWidget(ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuth(role)),
      funnelsListProvider.overrideWith(_OneFunnel.new),
      boardProvider('f1').overrideWith((ref) async => _board()),
      dioProvider.overrideWithValue(Dio()),
    ],
    child: MaterialApp.router(theme: AppTheme.darkTheme, routerConfig: router),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('a vendedor gets the pipeline only', (tester) async {
    await _pump(tester, role: 'vendedor');

    expect(find.byKey(const Key('funnel_view_pipeline')), findsOneWidget);
    expect(find.byKey(const Key('funnel_view_portfolio')), findsNothing);
    expect(find.byKey(const Key('funnel_view_inbox')), findsNothing);
  });

  testWidgets('a gestor gets the portfolio but not the Sent-folder scan',
      (tester) async {
    await _pump(tester, role: 'gestor');

    expect(find.byKey(const Key('funnel_view_pipeline')), findsOneWidget);
    expect(find.byKey(const Key('funnel_view_portfolio')), findsOneWidget);
    expect(find.byKey(const Key('funnel_view_inbox')), findsNothing);
  });

  testWidgets('an owner gets all three', (tester) async {
    await _pump(tester, role: 'owner');

    expect(find.byKey(const Key('funnel_view_pipeline')), findsOneWidget);
    expect(find.byKey(const Key('funnel_view_portfolio')), findsOneWidget);
    expect(find.byKey(const Key('funnel_view_inbox')), findsOneWidget);
  });

  // A forwarded link to a view she cannot open lands on the board, not on
  // a 403.
  testWidgets("a vendedor's ?view=inbox link degrades to the pipeline",
      (tester) async {
    await _pump(tester, role: 'vendedor', location: '/funnel?view=inbox');

    expect(find.byType(KanbanScreen), findsOneWidget);
    expect(find.byType(DiscoveredContactsBody), findsNothing);
  });
}
