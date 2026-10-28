import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/dashboard/presentation/widgets/source_badge.dart';

void main() {
  group('SourceBadge', () {
    Widget harness(ConversationSource source) => MaterialApp(
          home: Scaffold(body: SourceBadge(source: source)),
        );

    testWidgets('renders every channel source without throwing',
        (tester) async {
      for (final source in ConversationSource.values) {
        await tester.pumpWidget(harness(source));
        expect(tester.takeException(), isNull,
            reason: 'SourceBadge threw for $source');
        expect(find.byType(Icon), findsOneWidget);
      }
    });

    testWidgets('sms renders the sms icon, not the whatsapp fallback',
        (tester) async {
      await tester.pumpWidget(harness(ConversationSource.sms));
      expect(find.byIcon(Icons.sms), findsOneWidget);
      expect(find.byIcon(Icons.chat), findsNothing);
    });
  });
}
