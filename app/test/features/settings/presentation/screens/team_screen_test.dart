import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/features/settings/domain/models/team_member_model.dart';
import 'package:app/features/settings/presentation/controllers/team_controller.dart';
import 'package:app/features/settings/presentation/screens/team_screen.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  testWidgets('TeamScreen error state uses parseApiError, not raw toString',
      (WidgetTester tester) async {
    final error = Exception('raw-stack-detail-should-not-leak');

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          teamMembersProvider.overrideWith(() => _ErrorTeamMembers(error)),
        ],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: const TeamScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text(parseApiError(error)), findsOneWidget);
    expect(
        find.textContaining('raw-stack-detail-should-not-leak'), findsNothing);
  });
}

class _ErrorTeamMembers extends TeamMembers {
  final Object _error;
  _ErrorTeamMembers(this._error);

  @override
  Future<List<TeamMemberModel>> build() => Future.error(_error);

  @override
  Future<void> refresh() async {}
}
