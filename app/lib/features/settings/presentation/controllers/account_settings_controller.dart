import 'package:dio/dio.dart';
import 'package:app/core/auth/account_role.dart';
import 'package:flutter/foundation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/error/api_error_parser.dart';
import "package:app/i18n/strings.g.dart";
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';

part 'account_settings_controller.g.dart';

@immutable
class AccountSettingsState {
  final String accountName;
  final String userRole;
  final String? logoUrl;
  final String timezone;
  final bool replyReminderEnabled;
  final int replyReminderThresholdMinutes;

  const AccountSettingsState({
    required this.accountName,
    required this.userRole,
    required this.logoUrl,
    required this.timezone,
    required this.replyReminderEnabled,
    required this.replyReminderThresholdMinutes,
  });

  bool get isAdmin => roleIsAdmin(userRole);

  AccountSettingsState copyWith({
    String? accountName,
    String? userRole,
    Object? logoUrl = _sentinel,
    String? timezone,
    bool? replyReminderEnabled,
    int? replyReminderThresholdMinutes,
  }) =>
      AccountSettingsState(
        accountName: accountName ?? this.accountName,
        userRole: userRole ?? this.userRole,
        logoUrl:
            identical(logoUrl, _sentinel) ? this.logoUrl : logoUrl as String?,
        timezone: timezone ?? this.timezone,
        replyReminderEnabled: replyReminderEnabled ?? this.replyReminderEnabled,
        replyReminderThresholdMinutes:
            replyReminderThresholdMinutes ?? this.replyReminderThresholdMinutes,
      );

  static const _sentinel = Object();
}

/// Workspace-level settings: company name + danger-zone deletion.
/// Separate from ProfileController because Profile edits the authenticated
/// user (name/email/password) while this edits the Account row shared by
/// everyone in the workspace.
///
/// keepAlive: bouncing between settings tabs (account → channels → account)
/// would otherwise dispose this provider and refire GET /me each time.
/// Mutators (updateAccountName, updateBranding) patch
/// state in place, so keepAlive does not stale. Logout must invalidate
/// in auth_controller.dart.
@Riverpod(keepAlive: true)
class AccountSettingsController extends _$AccountSettingsController {
  @override
  Future<AccountSettingsState> build() async {
    final dio = ref.read(dioProvider);
    final res = await dio.get<dynamic>('/me');
    final data = res.data as Map<String, dynamic>;
    final account =
        (data['account'] as Map?)?.cast<String, dynamic>() ?? const {};
    final user = (data['user'] as Map?)?.cast<String, dynamic>() ?? const {};
    return AccountSettingsState(
      accountName: account['name'] as String? ?? '',
      userRole: user['role'] as String? ?? 'attendant',
      logoUrl: account['logo_url'] as String?,
      timezone: (account['timezone'] as String?)?.trim().isNotEmpty == true
          ? account['timezone'] as String
          : 'America/Sao_Paulo',
      // Default to enabled when the field is absent (older backends or the
      // brief window before migration 000043 runs).
      replyReminderEnabled: account['reply_reminder_enabled'] as bool? ?? false,
      replyReminderThresholdMinutes:
          account['reply_reminder_threshold_minutes'] as int? ?? 30,
    );
  }

  /// PATCH /accounts {reply_reminder_enabled, reply_reminder_threshold_minutes}.
  /// One call updates both fields; callers pass the desired final state.
  /// Server clamps thresholdMinutes to [1, 1440].
  Future<String?> updateReplyReminder({
    required bool enabled,
    required int thresholdMinutes,
  }) async {
    final dio = ref.read(dioProvider);
    try {
      await dio.patch<dynamic>('/accounts', data: {
        'reply_reminder_enabled': enabled,
        'reply_reminder_threshold_minutes': thresholdMinutes,
      });
      final current = state.asData?.value;
      if (current != null) {
        state = AsyncValue.data(current.copyWith(
          replyReminderEnabled: enabled,
          replyReminderThresholdMinutes: thresholdMinutes,
        ));
      }
      return null;
    } on DioException catch (e) {
      return _extractError(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }

  /// PATCH /accounts {name}. Returns null on success, a readable error
  /// message otherwise (never throws — callers render the string in a
  /// snackbar).
  Future<String?> updateAccountName(String name) async {
    final trimmed = name.trim();
    if (trimmed.isEmpty) return t.errors.EMPTY_NAME;
    final dio = ref.read(dioProvider);
    try {
      await dio.patch<dynamic>('/accounts', data: {'name': trimmed});
      // Reflect the new value immediately so the screen rebuilds without
      // an extra /me round-trip.
      final current = state.asData?.value;
      if (current != null) {
        state = AsyncValue.data(current.copyWith(accountName: trimmed));
      }
      return null;
    } on DioException catch (e) {
      return _extractError(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }

  /// PATCH /accounts {logo_url, timezone}. Either can be nil to skip.
  /// Pass clearLogo=true to set logo_url = "" server-side (== remove logo).
  Future<String?> updateBranding({
    String? logoObjectPath,
    bool clearLogo = false,
    String? timezone,
  }) async {
    final body = <String, dynamic>{};
    if (clearLogo) {
      body['logo_url'] = '';
    } else if (logoObjectPath != null) {
      body['logo_url'] = logoObjectPath;
    }
    if (timezone != null && timezone.trim().isNotEmpty) {
      body['timezone'] = timezone.trim();
    }
    if (body.isEmpty) return null;

    final dio = ref.read(dioProvider);
    try {
      await dio.patch<dynamic>('/accounts', data: body);
      final current = state.asData?.value;
      if (current != null) {
        state = AsyncValue.data(current.copyWith(
          logoUrl: clearLogo ? null : (logoObjectPath ?? current.logoUrl),
          timezone: timezone ?? current.timezone,
        ));
      }
      return null;
    } on DioException catch (e) {
      return _extractError(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }

  /// POST /media/upload then PUT to GCS signed URL. Returns the persisted
  /// object_path on success (caller threads it into updateBranding), or a
  /// message string on failure.
  Future<({String? path, String? error})> uploadLogo(
    Uint8List bytes,
    String filename,
  ) async {
    final dio = ref.read(dioProvider);
    try {
      final lower = filename.toLowerCase();
      final contentType = lower.endsWith('.png') ? 'image/png' : 'image/jpeg';
      final up = await dio.post<dynamic>('/media/upload', data: {
        'filename': filename,
        'content_type': contentType,
      });
      final data = up.data as Map<String, dynamic>;
      final signedUrl = data['signed_url'] as String;
      final objectPath = data['object_path'] as String;

      final upload = Dio();
      await upload.put<void>(
        signedUrl,
        data: Stream.fromIterable([bytes]),
        options: Options(
          headers: {
            Headers.contentTypeHeader: contentType,
            'Content-Length': bytes.length,
          },
          followRedirects: false,
          validateStatus: (s) => s != null && s < 400,
        ),
      );
      return (path: objectPath, error: null);
    } on DioException catch (e) {
      return (path: null, error: _extractError(e));
    } catch (e) {
      return (path: null, error: t.errors.UNKNOWN);
    }
  }

  /// DELETE /accounts. Cascade deletes every workspace row server-side,
  /// then logs the current device out so the router bounces to /. Caller
  /// is responsible for the context.go('/') after this returns null.
  Future<String?> deleteAccount() async {
    final dio = ref.read(dioProvider);
    try {
      await dio.delete<dynamic>('/accounts');
      // Logout clears FlutterSecureStorage and flips AuthController to
      // AsyncData(null), which makes app_router.redirect bounce any
      // protected route back to '/'.
      await ref.read(authControllerProvider.notifier).logout();
      return null;
    } on DioException catch (e) {
      return _extractError(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }

  String _extractError(DioException e) => parseApiError(e);
}
