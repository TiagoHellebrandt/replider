import 'package:flutter/material.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/features/dashboard/presentation/widgets/image_viewer.dart';
import 'package:app/i18n/strings.g.dart';

Widget _harness({required String url}) {
  return MaterialApp(
    theme: AppTheme.darkTheme,
    home: ImageViewerScreen(
      url: url,
      filename: 'photo.jpg',
      caption: 'a caption',
      date: DateTime(2026, 1, 1, 10, 30),
    ),
  );
}

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  testWidgets('renders the photo through the cached AppNetworkImage',
      (tester) async {
    await tester.pumpWidget(_harness(url: 'https://cdn/photo.jpg'));
    await tester.pump(); // not pumpAndSettle — network load would hang

    expect(find.byType(AppNetworkImage), findsOneWidget);
    // Metadata footer still renders alongside the cached image.
    expect(find.text('a caption'), findsOneWidget);
  });

  testWidgets('AppNetworkImage still renders (fallback) when url is empty',
      (tester) async {
    await tester.pumpWidget(_harness(url: ''));
    await tester.pump();

    expect(find.byType(AppNetworkImage), findsOneWidget);
    expect(find.text('photo.jpg'), findsOneWidget);
  });
}
