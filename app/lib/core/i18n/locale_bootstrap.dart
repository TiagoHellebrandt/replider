import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:app/i18n/strings.g.dart';

/// Resolves and persists the app's preferred locale.
///
/// Resolution order on startup:
///   1. `?lang=` query param (from web→app handoff). Wins when present and
///      is also written to storage so subsequent visits stick.
///   2. Stored preference (last explicit user choice).
///   3. Device / browser locale (Slang's default).
class LocaleBootstrap {
  LocaleBootstrap._();

  static const _storageKey = 'replider_preferred_locale';
  static const _storage = FlutterSecureStorage();

  /// How long startup will wait on storage before giving up on the stored
  /// preference.
  ///
  /// `main()` awaits this before `runApp`, so a storage read that never
  /// completes does not degrade the app — it leaves a permanently BLANK
  /// page, which is the worst failure mode we can ship. The locale is a
  /// preference; the app rendering is not negotiable, so the read is
  /// bounded and falls back to the device locale.
  static const _storageTimeout = Duration(seconds: 3);

  static Future<void> apply() async {
    final fromQuery = _readQueryParam();
    if (fromQuery != null) {
      await _persist(fromQuery);
      return;
    }

    final stored = await _readStored();
    if (stored != null) {
      await LocaleSettings.setLocale(stored);
      return;
    }

    LocaleSettings.useDeviceLocale();
  }

  /// Persists a manual locale change made from the UI.
  static Future<void> persist(AppLocale locale) async {
    await _persist(locale);
  }

  static Future<void> _persist(AppLocale locale) async {
    await LocaleSettings.setLocale(locale);
    try {
      await _storage
          .write(key: _storageKey, value: locale.languageCode)
          .timeout(_storageTimeout);
    } catch (_) {
      // Storage failure is non-fatal: we still applied the locale for this
      // session.
    }
  }

  static AppLocale? _readQueryParam() {
    if (!kIsWeb) return null;
    final raw = Uri.base.queryParameters['lang'];
    if (raw == null || raw.isEmpty) return null;
    return _parse(raw);
  }

  static Future<AppLocale?> _readStored() async {
    try {
      final raw = await _storage
          .read(key: _storageKey)
          .timeout(_storageTimeout, onTimeout: () => null);
      if (raw == null || raw.isEmpty) return null;
      return _parse(raw);
    } catch (_) {
      return null;
    }
  }

  static AppLocale? _parse(String code) {
    final lower = code.toLowerCase().split(RegExp(r'[-_]')).first;
    for (final l in AppLocale.values) {
      if (l.languageCode == lower) return l;
    }
    return null;
  }
}
