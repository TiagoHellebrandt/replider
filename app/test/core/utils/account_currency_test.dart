import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/utils/account_currency.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';

// Replider bills in USD as well as BRL, and ~30 screens passed a literal
// 'brl' into the formatter. A US customer read their own contract value,
// portfolio revenue and commission simulation prefixed with R$ — the app
// asserting a currency they are not charged in, over figures they entered.
void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));

  ProviderContainer containerFor(String? currency) {
    final container = ProviderContainer(overrides: [
      authControllerProvider.overrideWith(() => _FakeAuth(currency)),
    ]);
    addTearDown(container.dispose);
    return container;
  }

  Future<String> money(
    WidgetTester tester, {
    required String? accountCurrency,
    required int cents,
    String? explicit,
  }) async {
    late String out;
    await tester.pumpWidget(ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(() => _FakeAuth(accountCurrency)),
      ],
      child: Consumer(builder: (context, ref, _) {
        out = formatAccountMoney(ref, cents, explicit: explicit);
        return const SizedBox.shrink();
      }),
    ));
    await tester.pumpAndSettle();
    return out;
  }

  testWidgets('a USD account reads its own money in dollars', (tester) async {
    expect(
      await money(tester, accountCurrency: 'usd', cents: 123456),
      r'$1,234.56',
    );
  });

  testWidgets('a BRL account still reads in reais', (tester) async {
    expect(
      await money(tester, accountCurrency: 'brl', cents: 123456),
      'R\$ 1.234,56',
    );
  });

  // A contract signed in USD stays a USD contract even for an account billed
  // in BRL — the server sends `currency` on Contract and SignatureRequest.
  testWidgets("the document's own currency beats the account's",
      (tester) async {
    expect(
      await money(tester,
          accountCurrency: 'brl', cents: 100000, explicit: 'usd'),
      r'$1,000.00',
    );
  });

  testWidgets('a document with no stated currency falls back to the account',
      (tester) async {
    expect(
      await money(tester, accountCurrency: 'usd', cents: 100000, explicit: ''),
      r'$1,000.00',
    );
  });

  // The fallback exists for the window in which /me is still in flight — it is
  // a loading default, never a product decision.
  test('with /me unresolved the default is brl, not empty', () async {
    final container = containerFor(null);
    await container.read(authControllerProvider.future);

    expect(container.read(accountCurrencyProvider), 'brl');
  });

  test('an empty currency on /me does not become an empty string', () async {
    final container = containerFor('');
    await container.read(authControllerProvider.future);

    expect(container.read(accountCurrencyProvider), 'brl');
  });

  // /me takes a network round-trip, and until it answered every money figure
  // fell back to brl. A US customer watched R$ appear and then correct itself on
  // every cold load — the product contradicting itself about what it charges
  // them in, once per visit.
  group('cold load', () {
    setUp(() => AccountCurrencyCache.resetForTest());
    tearDown(() => AccountCurrencyCache.resetForTest());

    test('while /me is in flight, the last known currency answers', () async {
      AccountCurrencyCache.resetForTest('usd');
      final container = ProviderContainer(overrides: [
        authControllerProvider.overrideWith(_NeverResolvingAuth.new),
      ]);
      addTearDown(container.dispose);

      expect(container.read(accountCurrencyProvider), 'usd');
    });

    // A device that has never signed in has nothing better to offer.
    test('with nothing cached it is still brl', () async {
      final container = ProviderContainer(overrides: [
        authControllerProvider.overrideWith(_NeverResolvingAuth.new),
      ]);
      addTearDown(container.dispose);

      expect(container.read(accountCurrencyProvider), 'brl');
    });

    // /me always wins once it lands: the cache is a guess, not a source.
    test('a resolved /me overrides the cache', () async {
      AccountCurrencyCache.resetForTest('usd');
      final container = containerFor('brl');
      await container.read(authControllerProvider.future);

      expect(container.read(accountCurrencyProvider), 'brl');
    });

    test('remember keeps the value in memory for this session', () async {
      await AccountCurrencyCache.remember('usd');
      expect(AccountCurrencyCache.value, 'usd');
    });

    test('remember ignores an empty currency', () async {
      AccountCurrencyCache.resetForTest('usd');
      await AccountCurrencyCache.remember('   ');
      expect(AccountCurrencyCache.value, 'usd');
    });
  });

  test('the account currency reaches the provider', () async {
    final container = containerFor('usd');
    await container.read(authControllerProvider.future);

    expect(container.read(accountCurrencyProvider), 'usd');
  });
}

class _FakeAuth extends AuthController {
  _FakeAuth(this._currency);

  final String? _currency;

  @override
  FutureOr<UserModel?> build() async {
    if (_currency == null) return null;
    return UserModel(
      id: '01HZXK5T9GXW2M3N4P5Q6R7S8T',
      name: 'Test User',
      email: 'user@example.com',
      token: 'jwt',
      role: 'admin',
      accountCurrency: _currency,
    );
  }
}

/// An auth state that never resolves — the window the cache exists for.
class _NeverResolvingAuth extends AuthController {
  @override
  FutureOr<UserModel?> build() => Completer<UserModel?>().future;
}
