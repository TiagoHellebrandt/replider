import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/design_system/components/gradient_avatar.dart';

void main() {
  Widget wrap(Widget child) =>
      MaterialApp(theme: AppTheme.darkTheme, home: Scaffold(body: child));

  testWidgets('renders single initial when name has one word', (tester) async {
    await tester.pumpWidget(wrap(
      const GradientAvatar(seed: '5511', name: 'Tiago'),
    ));
    expect(find.text('T'), findsOneWidget);
  });

  testWidgets('renders two initials when name has multiple words',
      (tester) async {
    await tester.pumpWidget(wrap(
      const GradientAvatar(seed: '5511', name: 'Tiago Hellebrandt'),
    ));
    expect(find.text('TH'), findsOneWidget);
  });

  testWidgets('renders ? for empty name', (tester) async {
    await tester.pumpWidget(wrap(
      const GradientAvatar(seed: '5511', name: ''),
    ));
    expect(find.text('?'), findsOneWidget);
  });

  testWidgets('routes avatar photo through cached AppNetworkImage',
      (tester) async {
    await tester.pumpWidget(wrap(
      const GradientAvatar(
        seed: '5511',
        name: 'Tiago',
        imageUrl: 'https://cdn/avatar.png',
      ),
    ));
    await tester.pump(); // avoid network fetch in pumpAndSettle
    expect(find.byType(AppNetworkImage), findsOneWidget);
  });

  testWidgets('no AppNetworkImage when imageUrl is empty', (tester) async {
    await tester.pumpWidget(wrap(
      const GradientAvatar(seed: '5511', name: 'Tiago', imageUrl: ''),
    ));
    await tester.pump();
    expect(find.byType(AppNetworkImage), findsNothing);
    expect(find.text('T'), findsOneWidget);
  });

  testWidgets('two avatars with the same seed paint with the same gradient',
      (tester) async {
    await tester.pumpWidget(wrap(Column(children: const [
      GradientAvatar(seed: 'phone-A', name: 'Foo Bar', key: ValueKey('a')),
      GradientAvatar(seed: 'phone-A', name: 'Other Name', key: ValueKey('b')),
    ])));
    final containers = tester.widgetList<Container>(find.descendant(
      of: find.byType(GradientAvatar),
      matching: find.byType(Container),
    ));
    final gradients = containers
        .map((c) => (c.decoration as BoxDecoration?)?.gradient)
        .whereType<LinearGradient>()
        .toList();
    expect(gradients.length, greaterThanOrEqualTo(2));
    expect(gradients[0].colors, equals(gradients[1].colors));
  });

  testWidgets('a batch of distinct seeds yields more than one gradient',
      (tester) async {
    // Sweep 30 random-ish seeds and assert we see at least 3 distinct
    // colour pairs — guards against the gradient picker collapsing
    // every input to one entry without making the test flaky on
    // unlucky 1-or-2 seed pairs.
    final children = <Widget>[
      for (var i = 0; i < 30; i++)
        GradientAvatar(seed: 'seed-$i-${i * 7}', name: 'X', radius: 8),
    ];
    await tester.pumpWidget(wrap(SingleChildScrollView(
      child: Column(children: children),
    )));
    final pairs = tester
        .widgetList<Container>(find.descendant(
          of: find.byType(GradientAvatar),
          matching: find.byType(Container),
        ))
        .map((c) => (c.decoration as BoxDecoration?)?.gradient)
        .whereType<LinearGradient>()
        .map((g) => g.colors.toString())
        .toSet();
    expect(pairs.length, greaterThan(2));
  });
}
