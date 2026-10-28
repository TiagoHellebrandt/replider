// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'auth_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(secureStorage)
final secureStorageProvider = SecureStorageProvider._();

final class SecureStorageProvider extends $FunctionalProvider<
    FlutterSecureStorage,
    FlutterSecureStorage,
    FlutterSecureStorage> with $Provider<FlutterSecureStorage> {
  SecureStorageProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'secureStorageProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$secureStorageHash();

  @$internal
  @override
  $ProviderElement<FlutterSecureStorage> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  FlutterSecureStorage create(Ref ref) {
    return secureStorage(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(FlutterSecureStorage value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<FlutterSecureStorage>(value),
    );
  }
}

String _$secureStorageHash() => r'273dc403a965c1f24962aaf4d40776611a26f8b8';

@ProviderFor(authLocalDataSource)
final authLocalDataSourceProvider = AuthLocalDataSourceProvider._();

final class AuthLocalDataSourceProvider extends $FunctionalProvider<
    AuthLocalDataSource,
    AuthLocalDataSource,
    AuthLocalDataSource> with $Provider<AuthLocalDataSource> {
  AuthLocalDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'authLocalDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$authLocalDataSourceHash();

  @$internal
  @override
  $ProviderElement<AuthLocalDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AuthLocalDataSource create(Ref ref) {
    return authLocalDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AuthLocalDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AuthLocalDataSource>(value),
    );
  }
}

String _$authLocalDataSourceHash() =>
    r'3cb19b6bc7c79b6ebc6da4d4f9dac6f5a289bc1d';

@ProviderFor(authRemoteDataSource)
final authRemoteDataSourceProvider = AuthRemoteDataSourceProvider._();

final class AuthRemoteDataSourceProvider extends $FunctionalProvider<
    AuthRemoteDataSource,
    AuthRemoteDataSource,
    AuthRemoteDataSource> with $Provider<AuthRemoteDataSource> {
  AuthRemoteDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'authRemoteDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$authRemoteDataSourceHash();

  @$internal
  @override
  $ProviderElement<AuthRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AuthRemoteDataSource create(Ref ref) {
    return authRemoteDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AuthRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AuthRemoteDataSource>(value),
    );
  }
}

String _$authRemoteDataSourceHash() =>
    r'08fbc5fbf94c86964791b7f20b39f2504c0e8a74';

@ProviderFor(authRepository)
final authRepositoryProvider = AuthRepositoryProvider._();

final class AuthRepositoryProvider
    extends $FunctionalProvider<AuthRepository, AuthRepository, AuthRepository>
    with $Provider<AuthRepository> {
  AuthRepositoryProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'authRepositoryProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$authRepositoryHash();

  @$internal
  @override
  $ProviderElement<AuthRepository> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AuthRepository create(Ref ref) {
    return authRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AuthRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AuthRepository>(value),
    );
  }
}

String _$authRepositoryHash() => r'3e368a5e330bf76b0c8446d25162c3c38495f87b';

/// Singleton GoogleSignIn instance.
///
/// CRITICAL: Must be a single instance per app lifetime. The
/// `google_sign_in_web` plugin internally calls
/// `google.accounts.id.initialize()` on construction, and Google's GSI library
/// logs a warning ("called multiple times") plus exhibits race conditions when
/// multiple instances exist concurrently. `keepAlive: true` ensures the
/// instance survives Riverpod's auto-dispose.
///
/// We also call `isSignedIn()` eagerly here. That method has no UI side
/// effects but it does internally invoke `_ensureInitialized()`, which is
/// what triggers the underlying platform plugin to load the GIS SDK and call
/// `id.initialize()` exactly once. Without this, the plugin only initializes
/// on the first user-triggered method call, which delays the first click and
/// can cause "Getting ready" placeholders to hang if any UI awaits the
/// `initialized` future.

@ProviderFor(googleSignIn)
final googleSignInProvider = GoogleSignInProvider._();

/// Singleton GoogleSignIn instance.
///
/// CRITICAL: Must be a single instance per app lifetime. The
/// `google_sign_in_web` plugin internally calls
/// `google.accounts.id.initialize()` on construction, and Google's GSI library
/// logs a warning ("called multiple times") plus exhibits race conditions when
/// multiple instances exist concurrently. `keepAlive: true` ensures the
/// instance survives Riverpod's auto-dispose.
///
/// We also call `isSignedIn()` eagerly here. That method has no UI side
/// effects but it does internally invoke `_ensureInitialized()`, which is
/// what triggers the underlying platform plugin to load the GIS SDK and call
/// `id.initialize()` exactly once. Without this, the plugin only initializes
/// on the first user-triggered method call, which delays the first click and
/// can cause "Getting ready" placeholders to hang if any UI awaits the
/// `initialized` future.

final class GoogleSignInProvider
    extends $FunctionalProvider<GoogleSignIn, GoogleSignIn, GoogleSignIn>
    with $Provider<GoogleSignIn> {
  /// Singleton GoogleSignIn instance.
  ///
  /// CRITICAL: Must be a single instance per app lifetime. The
  /// `google_sign_in_web` plugin internally calls
  /// `google.accounts.id.initialize()` on construction, and Google's GSI library
  /// logs a warning ("called multiple times") plus exhibits race conditions when
  /// multiple instances exist concurrently. `keepAlive: true` ensures the
  /// instance survives Riverpod's auto-dispose.
  ///
  /// We also call `isSignedIn()` eagerly here. That method has no UI side
  /// effects but it does internally invoke `_ensureInitialized()`, which is
  /// what triggers the underlying platform plugin to load the GIS SDK and call
  /// `id.initialize()` exactly once. Without this, the plugin only initializes
  /// on the first user-triggered method call, which delays the first click and
  /// can cause "Getting ready" placeholders to hang if any UI awaits the
  /// `initialized` future.
  GoogleSignInProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'googleSignInProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$googleSignInHash();

  @$internal
  @override
  $ProviderElement<GoogleSignIn> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  GoogleSignIn create(Ref ref) {
    return googleSignIn(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(GoogleSignIn value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<GoogleSignIn>(value),
    );
  }
}

String _$googleSignInHash() => r'c6811a6fb7b047667bfc8285c57ef51bfd95c78f';

/// A long-lived stream of [GoogleSignInUserData] emitted by the web plugin
/// whenever a credential is delivered (One Tap, FedCM, or the rendered GIS
/// button). The relevant field is `idToken`, which the backend exchanges for
/// a session JWT.
///
/// Why a stream and not a `Future`: the rendered GIS button is asynchronous —
/// the user clicks, picks an account in the Google popup, consents, and only
/// then does the credential arrive. There's no caller-side `await` — we have
/// to listen.
///
/// Empty on non-web platforms (mobile uses the [loginWithGoogle] method
/// directly).

@ProviderFor(googleUserDataEvents)
final googleUserDataEventsProvider = GoogleUserDataEventsProvider._();

/// A long-lived stream of [GoogleSignInUserData] emitted by the web plugin
/// whenever a credential is delivered (One Tap, FedCM, or the rendered GIS
/// button). The relevant field is `idToken`, which the backend exchanges for
/// a session JWT.
///
/// Why a stream and not a `Future`: the rendered GIS button is asynchronous —
/// the user clicks, picks an account in the Google popup, consents, and only
/// then does the credential arrive. There's no caller-side `await` — we have
/// to listen.
///
/// Empty on non-web platforms (mobile uses the [loginWithGoogle] method
/// directly).

final class GoogleUserDataEventsProvider extends $FunctionalProvider<
        AsyncValue<GoogleSignInUserData?>,
        GoogleSignInUserData?,
        Stream<GoogleSignInUserData?>>
    with
        $FutureModifier<GoogleSignInUserData?>,
        $StreamProvider<GoogleSignInUserData?> {
  /// A long-lived stream of [GoogleSignInUserData] emitted by the web plugin
  /// whenever a credential is delivered (One Tap, FedCM, or the rendered GIS
  /// button). The relevant field is `idToken`, which the backend exchanges for
  /// a session JWT.
  ///
  /// Why a stream and not a `Future`: the rendered GIS button is asynchronous —
  /// the user clicks, picks an account in the Google popup, consents, and only
  /// then does the credential arrive. There's no caller-side `await` — we have
  /// to listen.
  ///
  /// Empty on non-web platforms (mobile uses the [loginWithGoogle] method
  /// directly).
  GoogleUserDataEventsProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'googleUserDataEventsProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$googleUserDataEventsHash();

  @$internal
  @override
  $StreamProviderElement<GoogleSignInUserData?> $createElement(
          $ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<GoogleSignInUserData?> create(Ref ref) {
    return googleUserDataEvents(ref);
  }
}

String _$googleUserDataEventsHash() =>
    r'7971ca65c89378e9d586d51506a6b07f3a3788d4';

@ProviderFor(AuthController)
final authControllerProvider = AuthControllerProvider._();

final class AuthControllerProvider
    extends $AsyncNotifierProvider<AuthController, UserModel?> {
  AuthControllerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'authControllerProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$authControllerHash();

  @$internal
  @override
  AuthController create() => AuthController();
}

String _$authControllerHash() => r'cf8bf548024a51c5fb2081d0ebb5fac8df537e13';

abstract class _$AuthController extends $AsyncNotifier<UserModel?> {
  FutureOr<UserModel?> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<UserModel?>, UserModel?>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<UserModel?>, UserModel?>,
        AsyncValue<UserModel?>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
