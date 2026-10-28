import 'dart:convert';
import 'dart:typed_data';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/owner/presentation/widgets/invite_customer_dialog.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// The invite dialog's contract: plans come from the UNFILTERED owner
// catalogue, and the link is shown once with a warning that closing loses it.

class _RouteAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    if (options.path.contains('/owner/plans')) {
      return ResponseBody.fromString(
        jsonEncode([
          {'id': 'plan-sdr', 'name': 'Vendedor'},
        ]),
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    if (options.path.contains('/owner/invites')) {
      return ResponseBody.fromString(
        jsonEncode({
          'token': 'sekret-once',
          'invite': {'id': 'inv-1'},
        }),
        201,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    return ResponseBody.fromString('{}', 404);
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  Widget harness() {
    final dio = Dio(BaseOptions(baseUrl: 'https://api.test/v1'));
    dio.httpClientAdapter = _RouteAdapter();
    return ProviderScope(
      overrides: [dioProvider.overrideWithValue(dio)],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: InviteCustomerDialog()),
      ),
    );
  }

  testWidgets(
      'emits an invite and shows the link exactly once, with the copy warning',
      (tester) async {
    await tester.pumpWidget(harness());
    await tester.pumpAndSettle();

    // Plans arrived from /owner/plans (the unfiltered catalogue).
    await tester.tap(find.byKey(const Key('invite_plan_dropdown')));
    await tester.pumpAndSettle();
    await tester.tap(find.textContaining('Vendedor').last);
    await tester.pumpAndSettle();

    await tester.enterText(
        find.byKey(const Key('invite_email_input')), 'compras@clinica.com.br');
    await tester.tap(find.byKey(const Key('invite_create_btn')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('invite_link_text')), findsOneWidget);
    expect(find.textContaining('register?invite=sekret-once'), findsOneWidget);
    expect(find.text(t.owner.invite_link_hint), findsOneWidget,
        reason: "o aviso de 'aparece uma vez' é parte do contrato");
    expect(find.byKey(const Key('invite_copy_btn')), findsOneWidget);
  });
}
