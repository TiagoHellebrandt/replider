import 'package:flutter/foundation.dart';

/// Captures first-touch acquisition attribution (`utm_*`) from the landing
/// URL the user arrived on.
///
/// The marketing site (replider.com) appends `utm_source/medium/campaign`
/// to the register CTA, which lands on `app.replider.com/register?utm_*`.
/// We must read `Uri.base` once at startup — *before* GoRouter rewrites the
/// browser URL and drops the query string — and stash the values in memory
/// so the register flow can forward them to the API on signup.
///
/// Mobile (non-web) builds have no inbound UTM; [capture] is a no-op there.
class UtmBootstrap {
  UtmBootstrap._();

  static String? _source;
  static String? _medium;
  static String? _campaign;
  static String? _referralCode;

  static String? get source => _source;
  static String? get medium => _medium;
  static String? get campaign => _campaign;

  /// Referral code from a `?ref=CODE` link (US-174). Forwarded to the API
  /// on signup so the new account is attributed to its referrer.
  static String? get referralCode => _referralCode;

  /// Reads `utm_*` and `ref` from the initial browser URL and caches them.
  /// Call once, early in `main()`, ahead of any router-driven navigation.
  static void capture() {
    if (!kIsWeb) return;
    final q = Uri.base.queryParameters;
    _source = _clean(q['utm_source']);
    _medium = _clean(q['utm_medium']);
    _campaign = _clean(q['utm_campaign']);
    _referralCode = _clean(q['ref']);
  }

  /// Test/manual override seam.
  @visibleForTesting
  static void setForTest({
    String? source,
    String? medium,
    String? campaign,
    String? referralCode,
  }) {
    _source = _clean(source);
    _medium = _clean(medium);
    _campaign = _clean(campaign);
    _referralCode = _clean(referralCode);
  }

  @visibleForTesting
  static void reset() {
    _source = null;
    _medium = null;
    _campaign = null;
    _referralCode = null;
  }

  static String? _clean(String? v) {
    if (v == null) return null;
    final t = v.trim();
    return t.isEmpty ? null : t;
  }
}
