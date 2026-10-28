import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:google_sign_in/google_sign_in.dart';
import 'package:mocktail/mocktail.dart';
import 'package:app/features/auth/domain/repositories/auth_repository.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/data/datasources/auth_local_datasource.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/owner/data/datasources/owner_remote_datasource.dart';
import 'package:app/features/owner/data/impersonation_session.dart';
import 'package:app/features/owner/presentation/controllers/owner_controller.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class MockAuthRepository extends Mock implements AuthRepository {}

class MockOwnerDataSource extends Mock implements OwnerRemoteDataSource {}

class MockAuthLocalDataSource extends Mock implements AuthLocalDataSource {}

class MockGoogleSignIn extends Mock implements GoogleSignIn {}

class MockGoogleSignInAccount extends Mock implements GoogleSignInAccount {}

class MockGoogleSignInAuthentication extends Mock
    implements GoogleSignInAuthentication {}

class FakeUserModel extends Fake implements UserModel {}

void main() {
  late MockAuthRepository mockRepository;
  late MockAuthLocalDataSource mockLocalDataSource;
  late ProviderContainer container;

  setUpAll(() {
    registerFallbackValue(FakeUserModel());
  });

  const tUser = UserModel(
    id: 'user-1',
    name: 'Test User',
    email: 'test@example.com',
    token: 'jwt-token-123',
  );

  const tNewUser = UserModel(
    id: 'user-2',
    name: 'New User',
    email: 'new@example.com',
    token: 'jwt-token-456',
    isNew: true,
  );

  late MockGoogleSignIn mockGoogleSignIn;

  setUp(() {
    mockRepository = MockAuthRepository();
    mockLocalDataSource = MockAuthLocalDataSource();
    mockGoogleSignIn = MockGoogleSignIn();
  });

  tearDown(() {
    container.dispose();
  });

  ProviderContainer createContainer({UserModel? initialUser}) {
    // Mock getCurrentUser for the build() method
    when(() => mockRepository.getCurrentUser())
        .thenAnswer((_) async => initialUser);

    container = ProviderContainer(
      overrides: [
        authRepositoryProvider.overrideWithValue(mockRepository),
        authLocalDataSourceProvider.overrideWithValue(mockLocalDataSource),
        googleSignInProvider.overrideWithValue(mockGoogleSignIn),
      ],
    );
    return container;
  }

  group('AuthController', () {
    group('build', () {
      test('should return null when no user is stored locally', () async {
        createContainer(initialUser: null);

        // Wait for the async build to complete
        final state = await container.read(authControllerProvider.future);

        expect(state, isNull);
        verify(() => mockRepository.getCurrentUser()).called(1);
      });

      test('should return stored user when available', () async {
        createContainer(initialUser: tUser);

        final state = await container.read(authControllerProvider.future);

        expect(state, tUser);
        expect(state!.id, 'user-1');
        expect(state.name, 'Test User');
        expect(state.email, 'test@example.com');
      });
    });

    group('login', () {
      test('should set state to user on successful login', () async {
        createContainer();

        when(() => mockRepository.login('test@example.com', 'password123'))
            .thenAnswer((_) async => tUser);

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.login('test@example.com', 'password123');

        final state = container.read(authControllerProvider);
        expect(state.hasValue, isTrue);
        expect(state.value, tUser);
        verify(() => mockRepository.login('test@example.com', 'password123'))
            .called(1);
      });

      test('should set state to error on login failure', () async {
        createContainer();

        when(() => mockRepository.login(any(), any()))
            .thenThrow(Exception('Invalid credentials'));

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.login('bad@example.com', 'wrong');

        final state = container.read(authControllerProvider);
        expect(state.hasError, isTrue);
      });
    });

    group('register', () {
      test('should set state to user on successful registration', () async {
        createContainer();

        when(() => mockRepository.register(
              'Acme Corp',
              'Admin User',
              'admin@acme.com',
              'securepass',
            )).thenAnswer((_) async => tNewUser);

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.register(
            'Acme Corp', 'Admin User', 'admin@acme.com', 'securepass');

        final state = container.read(authControllerProvider);
        expect(state.hasValue, isTrue);
        expect(state.value, tNewUser);
      });

      test('should set state to error on registration failure', () async {
        createContainer();

        when(() => mockRepository.register(any(), any(), any(), any()))
            .thenThrow(Exception('Email already exists'));

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.register(
            'Acme Corp', 'Admin', 'dup@acme.com', 'password');

        final state = container.read(authControllerProvider);
        expect(state.hasError, isTrue);
      });
    });

    group('loginWithGoogle', () {
      late MockGoogleSignInAccount mockAccount;
      late MockGoogleSignInAuthentication mockAuth;

      setUp(() {
        mockAccount = MockGoogleSignInAccount();
        mockAuth = MockGoogleSignInAuthentication();
      });

      // NOTE: these tests run on the Dart VM where `kIsWeb == false`, so the
      // controller takes the mobile branch and calls `googleSignIn.signIn()`.
      // We do NOT mock `signOut()` because the controller no longer calls it
      // (signOut would disable FedCM auto-select on web and break the flow).

      test('should call signIn and forward id_token to repository on success',
          () async {
        createContainer();

        when(() => mockGoogleSignIn.signIn())
            .thenAnswer((_) async => mockAccount);
        when(() => mockAccount.email).thenReturn('user@example.com');
        when(() => mockAccount.authentication)
            .thenAnswer((_) async => mockAuth);
        when(() => mockAuth.idToken).thenReturn('valid-id-token');
        when(() => mockRepository.loginWithGoogle('valid-id-token'))
            .thenAnswer((_) async => tUser);

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.loginWithGoogle();

        final state = container.read(authControllerProvider);
        expect(state.hasValue, isTrue);
        expect(state.value, tUser);

        // signOut must NOT be called (would break FedCM on web)
        verifyNever(() => mockGoogleSignIn.signOut());
        verify(() => mockGoogleSignIn.signIn()).called(1);
        verify(() => mockRepository.loginWithGoogle('valid-id-token'))
            .called(1);
      });

      test(
          'should set state to error when user cancels Google sign-in (account is null)',
          () async {
        createContainer();

        when(() => mockGoogleSignIn.signIn()).thenAnswer((_) async => null);

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.loginWithGoogle();

        final state = container.read(authControllerProvider);
        expect(state.hasError, isTrue);
        // Repository must NOT be called when sign-in is cancelled
        verifyNever(() => mockRepository.loginWithGoogle(any()));
      });

      test('should set state to error when Google returns no id_token',
          () async {
        createContainer();

        when(() => mockGoogleSignIn.signIn())
            .thenAnswer((_) async => mockAccount);
        when(() => mockAccount.email).thenReturn('user@example.com');
        when(() => mockAccount.authentication)
            .thenAnswer((_) async => mockAuth);
        when(() => mockAuth.idToken).thenReturn(null); // missing token

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.loginWithGoogle();

        expect(container.read(authControllerProvider).hasError, isTrue);
        verifyNever(() => mockRepository.loginWithGoogle(any()));
      });

      test('should set state to error when repository call fails', () async {
        createContainer();

        when(() => mockGoogleSignIn.signIn())
            .thenAnswer((_) async => mockAccount);
        when(() => mockAccount.email).thenReturn('user@example.com');
        when(() => mockAccount.authentication)
            .thenAnswer((_) async => mockAuth);
        when(() => mockAuth.idToken).thenReturn('token');
        when(() => mockRepository.loginWithGoogle('token'))
            .thenThrow(Exception('Backend rejected token'));

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.loginWithGoogle();

        expect(container.read(authControllerProvider).hasError, isTrue);
      });

      test(
          'should set state to error when Google sign-in throws (e.g. CSP block, popup blocked)',
          () async {
        createContainer();

        when(() => mockGoogleSignIn.signIn())
            .thenThrow(Exception('popup_closed_by_user'));

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.loginWithGoogle();

        expect(container.read(authControllerProvider).hasError, isTrue);
        verifyNever(() => mockRepository.loginWithGoogle(any()));
      });

      test(
          'REGRESSION: googleSignInProvider must return the same instance across reads (no re-instantiation)',
          () async {
        // This test is critical: if a new GoogleSignIn is created on every
        // read, the GSI library logs "google.accounts.id.initialize() is
        // called multiple times" and the auth flow becomes flaky on web.
        // The fix was to make `googleSignInProvider` `keepAlive: true`.
        //
        // We deliberately do NOT override `googleSignInProvider` here, so the
        // real factory runs. We then assert that consecutive reads, and reads
        // after invalidating *unrelated* providers, return the exact same
        // GoogleSignIn instance.
        final realContainer = ProviderContainer(
          overrides: [
            authRepositoryProvider.overrideWithValue(mockRepository),
            authLocalDataSourceProvider.overrideWithValue(mockLocalDataSource),
          ],
        );
        addTearDown(realContainer.dispose);

        when(() => mockRepository.getCurrentUser())
            .thenAnswer((_) async => null);

        final first = realContainer.read(googleSignInProvider);
        final second = realContainer.read(googleSignInProvider);

        // Force the auth controller to rebuild — simulates a state change
        // that would otherwise re-trigger initialization in the buggy version.
        realContainer.invalidate(authControllerProvider);
        await realContainer.read(authControllerProvider.future);

        final third = realContainer.read(googleSignInProvider);

        expect(identical(first, second), isTrue,
            reason: 'consecutive reads must return same instance');
        expect(identical(second, third), isTrue,
            reason:
                'instance must survive unrelated provider rebuilds (keepAlive: true)');
      });
    });

    group('loginWithMeta', () {
      test('should set state to user on successful Meta login', () async {
        createContainer();

        when(() => mockRepository.loginWithMeta('meta-access-token'))
            .thenAnswer((_) async => tUser);

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.loginWithMeta('meta-access-token');

        final state = container.read(authControllerProvider);
        expect(state.hasValue, isTrue);
        expect(state.value, tUser);
      });

      test('should set state to error on Meta login failure', () async {
        createContainer();

        when(() => mockRepository.loginWithMeta(any()))
            .thenThrow(Exception('Invalid token'));

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.loginWithMeta('bad-token');

        final state = container.read(authControllerProvider);
        expect(state.hasError, isTrue);
      });
    });

    group('forgotPassword', () {
      test('should call repository forgotPassword without changing user state',
          () async {
        createContainer();

        when(() => mockRepository.forgotPassword('test@example.com'))
            .thenAnswer((_) async {});

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.forgotPassword('test@example.com');

        verify(() => mockRepository.forgotPassword('test@example.com'))
            .called(1);
      });

      test('should propagate error if forgotPassword fails', () async {
        createContainer();

        when(() => mockRepository.forgotPassword(any()))
            .thenThrow(Exception('User not found'));

        final notifier = container.read(authControllerProvider.notifier);

        expect(
          () => notifier.forgotPassword('missing@example.com'),
          throwsA(isA<Exception>()),
        );
      });
    });

    group('updateUserInState', () {
      test('should update user name and email in state and local storage',
          () async {
        createContainer(initialUser: tUser);
        // Wait for build to complete
        await container.read(authControllerProvider.future);

        when(() => mockLocalDataSource.saveUser(any()))
            .thenAnswer((_) async {});

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.updateUserInState('Updated Name', 'updated@example.com');

        final state = container.read(authControllerProvider);
        expect(state.hasValue, isTrue);
        expect(state.value!.name, 'Updated Name');
        expect(state.value!.email, 'updated@example.com');
        expect(state.value!.token, tUser.token); // token preserved
        expect(state.value!.id, tUser.id); // id preserved
        verify(() => mockLocalDataSource.saveUser(any())).called(1);
      });

      test('should do nothing if current user is null', () async {
        createContainer(initialUser: null);
        await container.read(authControllerProvider.future);

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.updateUserInState('Name', 'email@example.com');

        // saveUser should not be called
        verifyNever(() => mockLocalDataSource.saveUser(any()));
      });
    });

    group('logout', () {
      test('should set state to null on successful logout', () async {
        createContainer(initialUser: tUser);
        await container.read(authControllerProvider.future);

        when(() => mockRepository.logout()).thenAnswer((_) async {});

        final notifier = container.read(authControllerProvider.notifier);
        await notifier.logout();

        final state = container.read(authControllerProvider);
        expect(state.hasValue, isTrue);
        expect(state.value, isNull);
        verify(() => mockRepository.logout()).called(1);
      });

      test('invalidates owner-access probe so the next login re-probes',
          () async {
        // Regression: ownerAccessStatus is keepAlive — probed once and
        // cached. If logout doesn't invalidate it, a stale `forbidden`
        // (probed before the operator hit SUPER_ADMIN_EMAILS) survives a
        // logout→login on the same tab and the owner menu stays hidden.
        when(() => mockRepository.getCurrentUser())
            .thenAnswer((_) async => tUser);
        when(() => mockRepository.logout()).thenAnswer((_) async {});

        final mockOwner = MockOwnerDataSource();
        var listCalls = 0;
        when(() => mockOwner.list(limit: 1)).thenAnswer((_) async {
          listCalls++;
          return OwnerAccountsListResponse(items: const [], total: 0);
        });

        container = ProviderContainer(
          overrides: [
            authRepositoryProvider.overrideWithValue(mockRepository),
            authLocalDataSourceProvider.overrideWithValue(mockLocalDataSource),
            googleSignInProvider.overrideWithValue(mockGoogleSignIn),
            ownerDataSourceProvider.overrideWithValue(mockOwner),
          ],
        );
        await container.read(authControllerProvider.future);

        // Prime the keepAlive probe.
        await container.read(ownerAccessStatusProvider.future);
        expect(listCalls, 1);

        await container.read(authControllerProvider.notifier).logout();

        // Re-reading after logout must hit the datasource again — proves
        // the keepAlive cache was invalidated.
        await container.read(ownerAccessStatusProvider.future);
        expect(listCalls, 2);
      });
    });

    group('endImpersonation', () {
      ImpersonationSession session() {
        return ImpersonationSession(
          token: 'imp-token',
          expiresAt: DateTime.now().add(const Duration(minutes: 15)),
          actor: 'op@replider.com',
          accountId: 'acc-9',
          accountName: 'Acme',
          impersonatingUser: 'owner@acme.com',
        );
      }

      test('clears the slot but keeps the operator signed in (no logout)',
          () async {
        FlutterSecureStorage.setMockInitialValues({});
        await ImpersonationStorage(const FlutterSecureStorage())
            .write(session());

        createContainer(initialUser: tUser);
        await container.read(authControllerProvider.future);
        expect(await container.read(impersonationControllerProvider.future),
            isNotNull);

        await container
            .read(authControllerProvider.notifier)
            .endImpersonation();

        // Operator stays signed in — this is NOT a logout.
        expect(container.read(authControllerProvider).value, tUser);
        // Impersonation slot is gone.
        expect(await container.read(impersonationControllerProvider.future),
            isNull);
        // The backend logout endpoint must NOT be hit.
        verifyNever(() => mockRepository.logout());
      });

      test('re-probes owner access so the menu returns after ending', () async {
        // Regression: while impersonating, the keepAlive owner-access probe
        // can resolve to `forbidden` (the impersonated admin is not a
        // super-admin). endImpersonation must invalidate it so the
        // operator's owner menu comes back without a full reload.
        FlutterSecureStorage.setMockInitialValues({});
        await ImpersonationStorage(const FlutterSecureStorage())
            .write(session());
        when(() => mockRepository.getCurrentUser())
            .thenAnswer((_) async => tUser);

        final mockOwner = MockOwnerDataSource();
        var listCalls = 0;
        when(() => mockOwner.list(limit: 1)).thenAnswer((_) async {
          listCalls++;
          return OwnerAccountsListResponse(items: const [], total: 0);
        });
        container = ProviderContainer(
          overrides: [
            authRepositoryProvider.overrideWithValue(mockRepository),
            authLocalDataSourceProvider.overrideWithValue(mockLocalDataSource),
            googleSignInProvider.overrideWithValue(mockGoogleSignIn),
            ownerDataSourceProvider.overrideWithValue(mockOwner),
          ],
        );
        await container.read(authControllerProvider.future);
        await container.read(ownerAccessStatusProvider.future);
        expect(listCalls, 1);

        await container
            .read(authControllerProvider.notifier)
            .endImpersonation();

        await container.read(ownerAccessStatusProvider.future);
        expect(listCalls, 2, reason: 'owner access must be re-probed');
      });

      test(
          'refetches the conversations list so the messages screen drops the '
          'client threads', () async {
        // Regression: the dashboard stays mounted (we navigate to /home, not
        // /), so the autoDispose conversations provider survived the exit and
        // kept showing the impersonated client's threads. endImpersonation
        // must invalidate it explicitly.
        FlutterSecureStorage.setMockInitialValues({});
        await ImpersonationStorage(const FlutterSecureStorage())
            .write(session());
        when(() => mockRepository.getCurrentUser())
            .thenAnswer((_) async => tUser);

        var convBuilds = 0;
        container = ProviderContainer(
          overrides: [
            authRepositoryProvider.overrideWithValue(mockRepository),
            authLocalDataSourceProvider.overrideWithValue(mockLocalDataSource),
            googleSignInProvider.overrideWithValue(mockGoogleSignIn),
            conversationsProvider.overrideWith((ref) async {
              convBuilds++;
              return <ConversationModel>[];
            }),
          ],
        );
        await container.read(authControllerProvider.future);

        // Keep the list alive so invalidation forces a rebuild (mirrors the
        // still-mounted dashboard).
        final sub = container.listen(conversationsProvider, (_, __) {});
        await container.read(conversationsProvider.future);
        expect(convBuilds, 1);

        await container
            .read(authControllerProvider.notifier)
            .endImpersonation();
        await container.read(conversationsProvider.future);
        expect(convBuilds, 2);

        sub.close();
      });
    });
  });
}
