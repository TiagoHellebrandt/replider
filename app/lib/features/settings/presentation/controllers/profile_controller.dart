import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/i18n/strings.g.dart';

part 'profile_controller.g.dart';

@riverpod
class ProfileController extends _$ProfileController {
  @override
  FutureOr<void> build() {}

  /// Updates the user name, email and signature via PUT /me.
  /// [signature]: null = don't touch, '' = clear, else set (US-018).
  Future<String?> updateProfile({
    required String name,
    required String email,
    String? signature,
  }) async {
    final dio = ref.read(dioProvider);
    try {
      await dio.put<dynamic>('/me', data: {
        'name': name,
        'email': email,
        if (signature != null) 'signature': signature,
      });
      // Immediately update Riverpod state so UI rebuilds without re-login
      await ref
          .read(authControllerProvider.notifier)
          .updateUserInState(name, email, signature: signature);
      return null;
    } on DioException catch (e) {
      if (e.response?.statusCode == 404 || e.response?.statusCode == 405) {
        return t.profile.profile_update_unavailable;
      }
      return _extractError(e);
    } catch (e) {
      return '${t.profile.unexpected_error}: $e';
    }
  }

  /// Changes password via PATCH /me/password.
  Future<String?> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async {
    final dio = ref.read(dioProvider);
    try {
      await dio.patch<dynamic>('/me/password', data: {
        'current_password': currentPassword,
        'new_password': newPassword,
      });
      return null;
    } on DioException catch (e) {
      if (e.response?.statusCode == 404 || e.response?.statusCode == 405) {
        return t.profile.password_change_unavailable;
      }
      if (e.response?.statusCode == 401) {
        return t.profile.incorrect_current_password;
      }
      return _extractError(e);
    } catch (e) {
      return '${t.profile.unexpected_error}: $e';
    }
  }

  String _extractError(DioException e) {
    final data = e.response?.data;
    if (data is Map<String, dynamic>) {
      return data['message'] as String? ??
          data['error'] as String? ??
          '${t.profile.error_prefix}: ${e.response?.statusCode}';
    }
    return '${t.profile.network_error}: ${e.message}';
  }
}

/// Simple provider that wraps the Ref for ProfileController
class ProfileControllerRef {
  final Ref ref;
  const ProfileControllerRef(this.ref);
}
