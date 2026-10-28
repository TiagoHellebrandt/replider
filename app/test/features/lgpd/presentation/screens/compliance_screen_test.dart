import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/features/lgpd/domain/models/audit_log_model.dart';
import 'package:app/features/lgpd/presentation/controllers/lgpd_controller.dart';
import 'package:app/features/lgpd/presentation/screens/compliance_screen.dart';
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

  testWidgets('ComplianceScreen renders title and section headers',
      (WidgetTester tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          auditLogListProvider.overrideWith(() => _DataAuditLogList()),
        ],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: const ComplianceScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text(t.lgpd.screen_title), findsOneWidget);
    expect(find.text(t.lgpd.section_data_subject_rights, skipOffstage: false),
        findsOneWidget);
    expect(find.text(t.lgpd.section_audit_trail, skipOffstage: false),
        findsOneWidget);
  });

  testWidgets(
      'ComplianceScreen audit error state uses parseApiError, not raw toString',
      (WidgetTester tester) async {
    final error = Exception('raw-stack-detail-should-not-leak');

    // Tall viewport so the lazily-built ListView lays out the audit card
    // (the error branch lives in the last item, below an 800x600 fold).
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          auditLogListProvider.overrideWith(() => _ErrorAuditLogList(error)),
        ],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: const ComplianceScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text(parseApiError(error)), findsOneWidget);
    expect(
        find.textContaining('raw-stack-detail-should-not-leak'), findsNothing);
  });
}

class _DataAuditLogList extends AuditLogList {
  @override
  Future<AuditLogPage> build() async =>
      const AuditLogPage(total: 0, entries: []);

  @override
  Future<void> refresh() async {}

  @override
  Future<void> applyFilters(AuditLogFilter f) async {}
}

class _ErrorAuditLogList extends AuditLogList {
  final Object _error;
  _ErrorAuditLogList(this._error);

  @override
  Future<AuditLogPage> build() => Future.error(_error);

  @override
  Future<void> refresh() async {}

  @override
  Future<void> applyFilters(AuditLogFilter f) async {}
}
