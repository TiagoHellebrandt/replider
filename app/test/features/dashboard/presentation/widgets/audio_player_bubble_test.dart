import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/dashboard/presentation/widgets/audio_player_bubble.dart';
import 'package:app/i18n/strings.g.dart';

Widget _harness(Widget Function(AppColorsExtension colors) build) {
  return ProviderScope(
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: Builder(
          builder: (context) {
            final colors = Theme.of(context).extension<AppColorsExtension>()!;
            return build(colors);
          },
        ),
      ),
    ),
  );
}

void main() {
  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  final colors = AppTheme.darkTheme.extension<AppColorsExtension>()!;

  Future<void> pump(WidgetTester tester, {String? initialTranscript}) async {
    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: Scaffold(
            body: AudioPlayerBubble(
              messageId: 'msg-1',
              // null url keeps the test off just_audio's platform path.
              url: null,
              colors: colors,
              initialTranscript: initialTranscript,
            ),
          ),
        ),
      ),
    );
  }

  testWidgets('mounts and renders without throwing', (tester) async {
    await tester.pumpWidget(
      _harness((colors) => AudioPlayerBubble(
            messageId: 'msg_1',
            url: null,
            colors: colors,
          )),
    );
    await tester.pump();
    expect(find.byType(AudioPlayerBubble), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('disposes the durationStream subscription without leaking',
      (tester) async {
    await tester.pumpWidget(
      _harness((colors) => AudioPlayerBubble(
            messageId: 'msg_2',
            url: null,
            colors: colors,
          )),
    );
    await tester.pump();

    // Replace the subtree so the bubble's State.dispose() runs. With the
    // durationStream subscription cancelled, no late emission can fire
    // setState() on the disposed State — so no exception should surface.
    await tester.pumpWidget(
      _harness((_) => const SizedBox.shrink()),
    );
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.byType(AudioPlayerBubble), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('shows the existing transcript and hides the transcribe button',
      (tester) async {
    await pump(tester, initialTranscript: 'Olá, queria um orçamento');

    expect(find.text('Olá, queria um orçamento'), findsOneWidget);
    // Already transcribed — the request button must be gone.
    expect(find.byTooltip(t.audio_player.transcribe_tooltip), findsNothing);
  });

  testWidgets('shows the transcribe button when there is no transcript yet',
      (tester) async {
    await pump(tester, initialTranscript: null);

    expect(find.byTooltip(t.audio_player.transcribe_tooltip), findsOneWidget);
  });

  testWidgets('treats a blank transcript as none', (tester) async {
    await pump(tester, initialTranscript: '   ');

    expect(find.byTooltip(t.audio_player.transcribe_tooltip), findsOneWidget);
  });
}
