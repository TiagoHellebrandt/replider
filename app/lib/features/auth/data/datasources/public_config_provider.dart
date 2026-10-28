import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';

/// Unauthenticated bootstrap flags served by `GET /v1/config` (US-F001).
class PublicConfig {
  /// Mirrors the API's `REGISTRATION_ENABLED`. When false the register
  /// screen hides its form and shows the "em breve" state — the API
  /// refuses the signup anyway (503 `REGISTRATION_PAUSED`).
  final bool registrationEnabled;

  const PublicConfig({required this.registrationEnabled});

  factory PublicConfig.fromJson(Map<String, dynamic> json) => PublicConfig(
        registrationEnabled: json['registration_enabled'] as bool? ?? true,
      );
}

/// Fetches the public config once per app session.
///
/// Deliberately **fails open**: any transport/parse error resolves to
/// `registrationEnabled: true`. A flaky config call must never be the
/// reason a working signup form is hidden — the server is the authority
/// and still answers 503 when registration really is paused.
///
/// Hand-written (no `@riverpod` codegen) so adding a single public flag
/// doesn't require a full build_runner pass over the app.
final publicConfigProvider = FutureProvider<PublicConfig>((ref) async {
  final dio = ref.watch(dioProvider);
  try {
    final res = await dio.get<Map<String, dynamic>>('/config');
    final data = res.data;
    if (data == null) return const PublicConfig(registrationEnabled: true);
    return PublicConfig.fromJson(data);
  } catch (_) {
    return const PublicConfig(registrationEnabled: true);
  }
});
