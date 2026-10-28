import 'package:app/features/auth/domain/models/user_model.dart';

abstract class AuthLocalDataSource {
  Future<UserModel?> getUser();
  Future<void> saveUser(UserModel user);
  Future<void> clearUser();
}
