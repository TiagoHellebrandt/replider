import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:app/features/auth/data/repositories/auth_repository_impl.dart';
import 'package:app/features/auth/data/datasources/auth_remote_datasource.dart';
import 'package:app/features/auth/data/datasources/auth_local_datasource.dart';
import 'package:app/features/auth/domain/models/auth_dto.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/owner/data/impersonation_session.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class MockAuthRemoteDataSource extends Mock implements AuthRemoteDataSource {}

class MockAuthLocalDataSource extends Mock implements AuthLocalDataSource {}

class FakeAuthenticateUserDTO extends Fake implements AuthenticateUserDTO {}

class FakeVerifyMfaLoginDTO extends Fake implements VerifyMfaLoginDTO {}

void main() {
  late AuthRepositoryImpl repository;
  late MockAuthRemoteDataSource mockRemoteDataSource;
  late MockAuthLocalDataSource mockLocalDataSource;

  setUpAll(() {
    registerFallbackValue(FakeAuthenticateUserDTO());
    registerFallbackValue(FakeVerifyMfaLoginDTO());
    registerFallbackValue(const UserModel(
        id: '1', name: 'Test', email: 'test@test.com', token: 'token'));
  });

  setUp(() {
    mockRemoteDataSource = MockAuthRemoteDataSource();
    mockLocalDataSource = MockAuthLocalDataSource();
    repository = AuthRepositoryImpl(
      remoteDataSource: mockRemoteDataSource,
      localDataSource: mockLocalDataSource,
    );
  });

  group('AuthRepository Offline-First', () {
    const tToken = 'token123';
    const tUser =
        UserModel(id: '1', name: 'Test', email: 'test@test.com', token: tToken);

    final tAuthResponse = AuthenticateUserResponse(token: tToken);
    final tGetMeResponse = GetMeResponse(
      user: {
        'id': '1',
        'name': 'Test',
        'email': 'test@test.com',
      },
      account: {
        'id': 'acc-1',
        'name': 'Test Account',
        'is_trialing': false,
        'plan_name': 'Trial',
      },
    );

    test(
        'login should call remote datasource (login + me) and save to local datasource',
        () async {
      // arrange
      when(() => mockRemoteDataSource.login(any()))
          .thenAnswer((_) async => tAuthResponse);
      when(() => mockRemoteDataSource.getCurrentUser('Bearer $tToken'))
          .thenAnswer((_) async => tGetMeResponse);
      when(() => mockLocalDataSource.saveUser(any()))
          .thenAnswer((_) async => Future.value());

      // act
      final result = await repository.login('test@test.com', 'password');

      // assert
      expect(result, tUser);
      verify(() => mockRemoteDataSource.login(any())).called(1);
      verify(() => mockRemoteDataSource.getCurrentUser('Bearer $tToken'))
          .called(1);
      verify(() => mockLocalDataSource.saveUser(tUser)).called(1);
    });

    test('login persists the session even when rememberMe is false', () async {
      // Regression: the AuthInterceptor reads the token only from secure
      // storage, so a non-persisted session 401s on every request and a
      // reload logs the user out. rememberMe must NOT skip the write.
      when(() => mockRemoteDataSource.login(any()))
          .thenAnswer((_) async => tAuthResponse);
      when(() => mockRemoteDataSource.getCurrentUser('Bearer $tToken'))
          .thenAnswer((_) async => tGetMeResponse);
      when(() => mockLocalDataSource.saveUser(any())).thenAnswer((_) async {});
      when(() => mockLocalDataSource.clearUser()).thenAnswer((_) async {});

      await repository.login('test@test.com', 'password', rememberMe: false);

      verify(() => mockLocalDataSource.saveUser(tUser)).called(1);
      verifyNever(() => mockLocalDataSource.clearUser());
    });

    test('verifyMfaLogin persists the session', () async {
      when(() => mockRemoteDataSource.verifyMfaLogin(any()))
          .thenAnswer((_) async => const VerifyMfaLoginResponse(token: tToken));
      when(() => mockRemoteDataSource.getCurrentUser('Bearer $tToken'))
          .thenAnswer((_) async => tGetMeResponse);
      when(() => mockLocalDataSource.saveUser(any())).thenAnswer((_) async {});

      final result = await repository.verifyMfaLogin('mfa-session', '123456');

      expect(result, tUser);
      verify(() => mockLocalDataSource.saveUser(tUser)).called(1);
    });

    test('getCurrentUser should return refreshed data from backend', () async {
      // arrange: cached user exists and backend responds
      when(() => mockLocalDataSource.getUser()).thenAnswer((_) async => tUser);
      when(() => mockRemoteDataSource.getCurrentUser('Bearer $tToken'))
          .thenAnswer((_) async => tGetMeResponse);
      when(() => mockLocalDataSource.saveUser(any())).thenAnswer((_) async {});

      // act
      final result = await repository.getCurrentUser();

      // assert: refreshed from backend
      expect(result, isNotNull);
      expect(result!.id, '1');
      verify(() => mockLocalDataSource.getUser()).called(1);
      verify(() => mockRemoteDataSource.getCurrentUser('Bearer $tToken'))
          .called(1);
      verify(() => mockLocalDataSource.saveUser(any())).called(1);
    });

    test(
        'during impersonation /me uses the imp token + does NOT overwrite the operator',
        () async {
      // Regression: impersonation must make the whole app (settings/plan)
      // reflect the CLIENT, fetched on the imp token — but the operator's
      // own cached user must stay in storage so ending the session restores
      // them. So saveUser must NOT be called.
      FlutterSecureStorage.setMockInitialValues({});
      await ImpersonationStorage(const FlutterSecureStorage()).write(
        ImpersonationSession(
          token: 'imp-token',
          expiresAt: DateTime.now().add(const Duration(minutes: 15)),
          actor: 'op@replider.com',
          accountId: 'acc-client',
          accountName: 'Client',
          impersonatingUser: 'admin@client.com',
        ),
      );
      when(() => mockLocalDataSource.getUser()).thenAnswer((_) async => tUser);
      when(() => mockRemoteDataSource.getCurrentUser('Bearer imp-token'))
          .thenAnswer((_) async => GetMeResponse(
                user: {
                  'id': 'client-1',
                  'name': 'Client Admin',
                  'email': 'admin@client.com',
                },
                account: {'id': 'acc-client', 'plan_name': 'Enterprise'},
              ));

      final result = await repository.getCurrentUser();

      expect(result!.email, 'admin@client.com'); // the impersonated client
      verify(() => mockRemoteDataSource.getCurrentUser('Bearer imp-token'))
          .called(1);
      verifyNever(() => mockLocalDataSource.saveUser(any()));
      await ImpersonationStorage(const FlutterSecureStorage()).clear();
    });

    test('login maps totp_enabled from /me so the 2FA card reads enabled',
        () async {
      // Regression: the UserModel built from /me dropped totp_enabled, so the
      // profile 2FA badge always showed "DESATIVADA" after a reload even with
      // 2FA enrolled (it only looked on via the card's in-memory override).
      when(() => mockRemoteDataSource.login(any()))
          .thenAnswer((_) async => tAuthResponse);
      when(() => mockRemoteDataSource.getCurrentUser('Bearer $tToken'))
          .thenAnswer((_) async => GetMeResponse(
                user: {
                  'id': '1',
                  'name': 'Test',
                  'email': 'test@test.com',
                  'totp_enabled': true,
                },
                account: {'id': 'acc-1', 'name': 'Test Account'},
              ));
      when(() => mockLocalDataSource.saveUser(any())).thenAnswer((_) async {});

      final result = await repository.login('test@test.com', 'password');

      expect(result.totpEnabled, isTrue);
    });

    test('getCurrentUser maps totp_enabled from /me', () async {
      when(() => mockLocalDataSource.getUser()).thenAnswer((_) async => tUser);
      when(() => mockRemoteDataSource.getCurrentUser('Bearer $tToken'))
          .thenAnswer((_) async => GetMeResponse(
                user: {
                  'id': '1',
                  'name': 'Test',
                  'email': 'test@test.com',
                  'totp_enabled': true,
                },
                account: {'id': 'acc-1', 'name': 'Test Account'},
              ));
      when(() => mockLocalDataSource.saveUser(any())).thenAnswer((_) async {});

      final result = await repository.getCurrentUser();

      expect(result!.totpEnabled, isTrue);
    });

    test('getCurrentUser should fall back to cached data on network error',
        () async {
      // arrange: cached user exists but backend fails
      when(() => mockLocalDataSource.getUser()).thenAnswer((_) async => tUser);
      when(() => mockRemoteDataSource.getCurrentUser(any()))
          .thenThrow(Exception('Network error'));

      // act
      final result = await repository.getCurrentUser();

      // assert: returns cached data
      expect(result, tUser);
    });
  });
}
