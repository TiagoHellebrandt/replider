import 'package:flutter/foundation.dart' show visibleForTesting;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'package:app/core/utils/currency_format.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';

/// The last currency this device saw, so a cold load does not flash the wrong
/// one.
///
/// `/me` takes a network round-trip to resolve, and until it does every money
/// figure fell back to `brl`. A US customer therefore watched `R$` appear and
/// then correct itself on every cold load — the product contradicting itself
/// about what it charges them in, once per visit.
///
/// Deliberately NOT awaited before `runApp`. A bounded await is still an await,
/// and a blank first frame is a worse failure than one wrong glyph; the storage
/// read finishes in single-digit milliseconds while `/me` takes hundreds, so the
/// cache is warm long before any money screen mounts.
class AccountCurrencyCache {
  AccountCurrencyCache._();

  static const _key = 'replider_last_account_currency';
  static const _storage = FlutterSecureStorage();

  /// Device-wide rather than per user: at the moment this is read there is no
  /// user yet. Two people from differently-billed accounts sharing one browser
  /// can see the other's currency for the milliseconds before `/me` lands —
  /// which is what the value is for, and strictly better than always guessing
  /// BRL.
  static String? _value;

  static String? get value => _value;

  /// Loads the cached currency. Fire-and-forget from `main()`.
  static Future<void> warmUp() async {
    try {
      final stored = await _storage.read(key: _key);
      if (stored != null && stored.trim().isNotEmpty) _value = stored.trim();
    } catch (_) {
      // Storage failure is non-fatal: the provider falls back as before.
    }
  }

  /// Records the currency `/me` reported, for the next cold load.
  static Future<void> remember(String currency) async {
    final value = currency.trim();
    if (value.isEmpty || value == _value) return;
    _value = value;
    try {
      await _storage.write(key: _key, value: value);
    } catch (_) {
      // Non-fatal: this session already has the right answer in memory.
    }
  }

  @visibleForTesting
  static void resetForTest([String? seed]) => _value = seed;
}

/// Keeps [AccountCurrencyCache] in step with `/me`.
///
/// Watched once in `main()`, next to the push-click router: a Provider that
/// merely reads must stay free of side effects, so the write lives here.
final accountCurrencyPersistenceProvider = Provider<void>((ref) {
  ref.listen(authControllerProvider, (_, next) {
    final currency = next.asData?.value?.accountCurrency ?? '';
    if (currency.isNotEmpty) AccountCurrencyCache.remember(currency);
  }, fireImmediately: true);
});

/// The currency the account's own money is rendered in.
///
/// Replider bills in USD as well as BRL (US-001/US-017), and roughly thirty
/// screens used to pass a literal `'brl'` into [formatCurrencyCents]. A US
/// customer therefore read their own contract value, portfolio revenue and
/// commission simulation prefixed with `R$` — the app asserting a currency
/// they are not charged in, over figures they entered themselves.
///
/// The answer was already on the client: `/me` carries `account.currency` and
/// [UserModel.accountCurrency] parses it. This is that value, in one place, so
/// a new money widget inherits the right currency instead of re-deciding it.
///
/// While `/me` is in flight it answers with the last currency this device saw
/// ([AccountCurrencyCache]), and only then with `brl` — a loading default for a
/// device that has never signed in, never a product decision.
final accountCurrencyProvider = Provider<String>((ref) {
  final currency =
      ref.watch(authControllerProvider).asData?.value?.accountCurrency ?? '';
  if (currency.isNotEmpty) return currency;
  return AccountCurrencyCache.value ?? 'brl';
});

/// Money that belongs to the ACCOUNT, formatted in the account's currency.
///
/// [explicit] wins when the payload carries its own currency: a contract signed
/// in USD stays a USD contract even for an account billed in BRL, and the
/// server already sends `currency` on [Contract] and [SignatureRequest].
String formatAccountMoney(
  WidgetRef ref,
  int cents, {
  String? explicit,
}) {
  final fromPayload = explicit?.trim() ?? '';
  if (fromPayload.isNotEmpty) return formatCurrencyCents(cents, fromPayload);
  return formatCurrencyCents(cents, ref.watch(accountCurrencyProvider));
}

/// The platform's own payroll currency.
///
/// Seller commissions and owner payouts are settled by Replider in BRL
/// regardless of what the customer is billed — `domain.CommissionLedgerCurrency`
/// on the server. Named rather than written as a bare `'brl'` at each call site
/// so the difference between "the platform pays this" and "the customer earns
/// this" stays legible: only the first is currency-fixed.
const platformPayrollCurrency = 'brl';
