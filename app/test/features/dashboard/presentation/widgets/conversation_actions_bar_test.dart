import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/dashboard/presentation/widgets/conversation_actions_bar.dart';
import 'package:app/features/settings/domain/models/tag_model.dart';
import 'package:app/features/settings/domain/models/team_member_model.dart';
import 'package:app/features/settings/presentation/controllers/tag_controller.dart';
import 'package:app/features/settings/presentation/controllers/team_controller.dart';
import 'package:app/i18n/strings.g.dart';

class _FakeTeam extends TeamMembers {
  @override
  Future<List<TeamMemberModel>> build() async => const [];
}

class _FakeTags extends TagController {
  @override
  Future<List<TagModel>> build() async => const [];
}

ConversationModel _conversation({
  ConversationStatus status = ConversationStatus.open,
}) =>
    ConversationModel(
      id: 'conv-1',
      channelId: 'chan-1',
      contactExternalId: '5511999999999',
      contactName: 'Lead',
      lastMessage: 'oi',
      lastMessageAt: DateTime(2026, 1, 1),
      source: ConversationSource.whatsapp,
      status: status,
    );

Widget _harness({ConversationStatus status = ConversationStatus.open}) =>
    ProviderScope(
      overrides: [
        teamMembersProvider.overrideWith(() => _FakeTeam()),
        tagControllerProvider.overrideWith(() => _FakeTags()),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: Scaffold(
          body: ConversationActionsBar(
              conversation: _conversation(status: status)),
        ),
      ),
    );

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  // A resolved thread offers its way back; an open one offers Resolve. The
  // server also reopens on a customer reply — this is the attendant's side.
  testWidgets('a resolved conversation shows Reopen instead of Resolve',
      (tester) async {
    await tester.pumpWidget(_harness(status: ConversationStatus.closed));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('conversation_reopen_button')), findsOneWidget);
    expect(find.text(t.conversation.reopen_action), findsOneWidget);
    expect(find.text(t.conversation.resolve_action), findsNothing);
  });

  testWidgets('an open conversation shows Resolve, not Reopen', (tester) async {
    await tester.pumpWidget(_harness());
    await tester.pumpAndSettle();

    expect(find.text(t.conversation.resolve_action), findsOneWidget);
    expect(find.byKey(const Key('conversation_reopen_button')), findsNothing);
  });
}
