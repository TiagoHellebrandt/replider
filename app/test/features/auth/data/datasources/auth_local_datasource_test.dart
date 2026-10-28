import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:app/features/auth/data/datasources/auth_local_datasource_impl.dart';
import 'package:app/features/auth/domain/models/user_model.dart';

class MockFlutterSecureStorage extends Mock implements FlutterSecureStorage {}

void main() {
  late AuthLocalDataSourceImpl dataSource;
  late MockFlutterSecureStorage mockStorage;

  setUp(() {
    mockStorage = MockFlutterSecureStorage();
    dataSource = AuthLocalDataSourceImpl(mockStorage);
  });

  const tUser = UserModel(
    id: 'user-1',
    name: 'Alice',
    email: 'alice@example.com',
    token: 'jwt-abc',
  );

  group('saveUser', () {
    test('should write serialized user JSON to secure storage', () async {
      when(() => mockStorage.write(
          key: any(named: 'key'),
          value: any(named: 'value'))).thenAnswer((_) async {});

      await dataSource.saveUser(tUser);

      final captured = verify(
        () => mockStorage.write(
          key: 'cached_user_model',
          value: captureAny(named: 'value'),
        ),
      ).captured.single as String;

      final decoded = json.decode(captured) as Map<String, dynamic>;
      expect(decoded['id'], 'user-1');
      expect(decoded['name'], 'Alice');
      expect(decoded['email'], 'alice@example.com');
      expect(decoded['token'], 'jwt-abc');
    });
  });

  group('getUser', () {
    test('should return UserModel when stored JSON is valid', () async {
      when(() => mockStorage.read(key: 'cached_user_model'))
          .thenAnswer((_) async => json.encode(tUser.toJson()));

      final result = await dataSource.getUser();

      expect(result, tUser);
      verify(() => mockStorage.read(key: 'cached_user_model')).called(1);
    });

    test('should return null when no data is stored', () async {
      when(() => mockStorage.read(key: 'cached_user_model'))
          .thenAnswer((_) async => null);

      final result = await dataSource.getUser();

      expect(result, isNull);
    });

    test('should return null when stored JSON is corrupted', () async {
      when(() => mockStorage.read(key: 'cached_user_model'))
          .thenAnswer((_) async => '{invalid json}}}');

      final result = await dataSource.getUser();

      expect(result, isNull);
    });

    test('should return null when stored JSON is missing required fields',
        () async {
      when(() => mockStorage.read(key: 'cached_user_model'))
          .thenAnswer((_) async => json.encode({'id': '1'}));

      final result = await dataSource.getUser();

      // fromJson will throw due to missing required fields, caught by try/catch
      expect(result, isNull);
    });
  });

  group('clearUser', () {
    test('should delete the user key from secure storage', () async {
      when(() => mockStorage.delete(key: 'cached_user_model'))
          .thenAnswer((_) async {});

      await dataSource.clearUser();

      verify(() => mockStorage.delete(key: 'cached_user_model')).called(1);
    });
  });

  group('save then retrieve round-trip', () {
    test('should retrieve the same user that was saved', () async {
      String? storedValue;

      when(() => mockStorage.write(
          key: any(named: 'key'),
          value: any(named: 'value'))).thenAnswer((invocation) async {
        storedValue = invocation.namedArguments[#value] as String;
      });

      when(() => mockStorage.read(key: 'cached_user_model'))
          .thenAnswer((_) async => storedValue);

      await dataSource.saveUser(tUser);
      final result = await dataSource.getUser();

      expect(result, tUser);
    });
  });

  group('clear then retrieve', () {
    test('should return null after clearing', () async {
      String? storedValue = json.encode(tUser.toJson());

      when(() => mockStorage.read(key: 'cached_user_model'))
          .thenAnswer((_) async => storedValue);

      when(() => mockStorage.delete(key: 'cached_user_model'))
          .thenAnswer((_) async {
        storedValue = null;
      });

      // Verify user exists before clear.
      final before = await dataSource.getUser();
      expect(before, tUser);

      await dataSource.clearUser();

      final after = await dataSource.getUser();
      expect(after, isNull);
    });
  });
}
