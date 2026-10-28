import 'dart:ui';

/// The locale tag sent in the `locale` field of signup requests. The
/// server derives the account's immutable billing currency from it —
/// only a real `en-US` device maps to USD; everything else (including a
/// bare `en`) defaults to BRL (US-017 review fixes).
///
/// Uses the DEVICE locale, not `LocaleSettings.currentLocale`: Slang
/// collapses every non-Portuguese device to its `en` base locale, which
/// used to force German/Spanish/French users into the USD ladder with
/// no way back.
///
/// [deviceLocale] is injectable for tests; production callers omit it.
String signupLocale([Locale? deviceLocale]) {
  final locale = deviceLocale ?? PlatformDispatcher.instance.locale;
  return locale.toLanguageTag();
}
