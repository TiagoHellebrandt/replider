import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/dashboard/presentation/widgets/source_badge.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

// Email is the SDR product's main channel and the dashboard did not know it
// existed. An email conversation fell through _parseSource's whatsapp
// fallback, so it wore a WhatsApp badge AND inherited Meta's 24h service
// window — a rule that does not exist in email. These pin the fix.

void main() {
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  test('the source enum knows about email', () {
    expect(ConversationSource.values, contains(ConversationSource.email));
  });

  testWidgets('email gets its own badge, not the WhatsApp one', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(
        body: Column(
          children: [
            SourceBadge(source: ConversationSource.email),
            SourceBadge(source: ConversationSource.whatsapp),
          ],
        ),
      ),
    ));

    final icons = tester.widgetList<Icon>(find.byType(Icon)).toList();
    expect(icons.first.icon, Icons.alternate_email);
    expect(icons.first.color, isNot(icons.last.color),
        reason: 'a shared colour makes the two channels unreadable side by '
            'side in one inbox');
  });

  test('email has a label in both locales', () {
    LocaleSettings.setLocale(AppLocale.pt);
    expect(t.conversation.source_email, isNotEmpty);
    LocaleSettings.setLocale(AppLocale.en);
    expect(t.conversation.source_email, isNotEmpty);
  });
}
