import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/components/status_badge.dart';

void main() {
  testWidgets('renders the label in the supplied color', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: StatusBadge(label: 'ACTIVE', color: Color(0xFF00FF00)),
        ),
      ),
    );

    expect(find.text('ACTIVE'), findsOneWidget);
    final text = tester.widget<Text>(find.text('ACTIVE'));
    expect(text.style?.color, const Color(0xFF00FF00));
  });
}
