import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/core/network/websocket_service.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/lead_timeline.dart';
import 'package:app/features/sales/presentation/controllers/lead_timeline_controller.dart';
import 'package:app/features/sales/presentation/widgets/lead_timeline_block.dart';
import 'package:app/i18n/strings.g.dart';

// The reply that arrives while the lead panel is open. Before this the
// timeline was read once and never again: the vendedor sat looking at a
// stale thread while the customer's answer was already in the database.

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('a new_message WebSocket event re-reads the timeline',
      (tester) async {
    final events = StreamController<WSEvent>.broadcast();
    addTearDown(events.close);
    var reads = 0;

    await tester.pumpWidget(ProviderScope(
      overrides: [
        dioProvider.overrideWithValue(Dio()),
        wsEventsProvider.overrideWith((ref) => events.stream),
        leadTimelineProvider('lead-1').overrideWith((ref) async {
          reads++;
          return LeadTimeline(messages: [
            TimelineMessage(
              id: 'm$reads',
              author: TimelineAuthor.customer,
              text: 'resposta $reads',
              authorName: '',
              kind: 'text',
              sentAt: DateTime(2026, 9, 1),
            ),
          ]);
        }),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: LeadTimelineBlock(leadId: 'lead-1')),
      ),
    ));
    await tester.pumpAndSettle();
    expect(reads, 1);
    expect(find.text('resposta 1'), findsOneWidget);

    events.add(WSEvent.fromJson({
      'event': 'new_message',
      'conversation_id': 'conv-1',
    }));
    await tester.pumpAndSettle();

    expect(reads, 2, reason: 'the inbound must re-fetch the timeline');
    expect(find.text('resposta 2'), findsOneWidget);

    // Unrelated events leave it alone.
    events.add(WSEvent.fromJson({'event': 'typing', 'conversation_id': 'x'}));
    await tester.pumpAndSettle();
    expect(reads, 2);
  });
}
