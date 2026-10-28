import 'dart:convert';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/data/datasources/auth_local_datasource.dart';

class AuthLocalDataSourceImpl implements AuthLocalDataSource {
  final FlutterSecureStorage _storage;

  AuthLocalDataSourceImpl(this._storage);

  static const _userKey = 'cached_user_model';

  @override
  Future<UserModel?> getUser() async {
    final userJsonStr = await _storage.read(key: _userKey);
    if (userJsonStr != null) {
      try {
        final jsonMap = json.decode(userJsonStr) as Map<String, dynamic>;
        return UserModel.fromJson(jsonMap);
      } catch (e) {
        // If parsing fails for some reason (e.g. model changed), return null
        return null;
      }
    }
    return null;
  }

  @override
  Future<void> saveUser(UserModel user) async {
    final userJsonStr = json.encode(user.toJson());
    await _storage.write(key: _userKey, value: userJsonStr);
  }

  @override
  Future<void> clearUser() async {
    await _storage.delete(key: _userKey);
  }
}
