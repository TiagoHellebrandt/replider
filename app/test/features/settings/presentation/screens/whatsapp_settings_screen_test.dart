import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/billing/domain/models/usage_model.dart';
import 'package:app/features/billing/presentation/controllers/billing_controller.dart';
import 'package:app/features/settings/domain/models/whatsapp_template_model.dart';
import 'package:app/features/settings/presentation/controllers/whatsapp_config_controller.dart';
import 'package:app/features/settings/presentation/screens/whatsapp_settings_screen.dart';
import 'package:app/i18n/strings.g.dart';

class _FakeTemplates extends WhatsAppTemplates {
  _FakeTemplates(this.seed);
  final List<WhatsAppTemplateModel> seed;

  @override
  Future<List<WhatsAppTemplateModel>> build(String channelId) async => seed;
}

class _FakeUsage extends AccountUsage {
  @override
  Future<UsageModel> build() async => throw Exception('no-usage-in-test');
}

// Since 2026-09-01 the screen is the templates list and nothing else: the
// business-profile tab, the webhook diagnostics and the sibling phone
// numbers were v1 inbox surfaces. The lead panel's template picker is the
// reason the screen still exists.
void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  Future<void> pumpScreen(WidgetTester tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          whatsAppTemplatesProvider('ch_1')
              .overrideWith(() => _FakeTemplates(const [])),
          accountUsageProvider.overrideWith(() => _FakeUsage()),
        ],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: const WhatsAppSettingsScreen(channelId: 'ch_1'),
        ),
      ),
    );
    await tester.pump();
    await tester.pump();
  }

  testWidgets('is the templates list alone — no tabs, no profile',
      (WidgetTester tester) async {
    await pumpScreen(tester);

    expect(find.byType(TabBar), findsNothing);
    expect(find.text(t.settings.wa_settings_tab_profile), findsNothing);
    expect(find.text(t.settings.wa_settings_tab_templates), findsOneWidget);
  });

  testWidgets('keeps the way back to the channels list',
      (WidgetTester tester) async {
    await pumpScreen(tester);

    expect(find.byTooltip(t.settings.wa_settings_back_tooltip), findsOneWidget);
  });
}
