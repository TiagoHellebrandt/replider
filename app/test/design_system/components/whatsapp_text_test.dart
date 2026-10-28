import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/whatsapp_text.dart';

Widget _harness(Widget child) => MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(body: child),
    );

void main() {
  const style = TextStyle(fontSize: 14);

  // The support thread and the template preview read as documents: a reader
  // copying a paragraph out of them has no other affordance, so the default
  // must stay selectable. Only a caller whose ancestor claims the long press
  // opts out.
  testWidgets('selection is on by default', (tester) async {
    await tester
        .pumpWidget(_harness(const WhatsAppText('bom dia', baseStyle: style)));
    expect(find.byType(SelectableText), findsOneWidget);
  });

  testWidgets('selectable: false renders plain text instead', (tester) async {
    await tester.pumpWidget(_harness(
        const WhatsAppText('bom dia', baseStyle: style, selectable: false)));
    expect(find.byType(SelectableText), findsNothing);
    expect(find.textContaining('bom dia'), findsOneWidget);
  });

  // Markup must survive the switch — the non-selectable branch parses the same
  // spans, it does not fall back to a raw string.
  testWidgets('markup is still parsed when not selectable', (tester) async {
    await tester.pumpWidget(_harness(const WhatsAppText('um *forte* abraco',
        baseStyle: style, selectable: false)));
    final widget = tester.widget<Text>(find.byType(Text).first);
    final span = widget.textSpan! as TextSpan;
    expect(span.children!.length, greaterThan(1),
        reason: 'the bold run must still be its own span, not literal text');
  });
}
