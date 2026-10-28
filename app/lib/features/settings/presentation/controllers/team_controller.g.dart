// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'team_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(teamRemoteDataSource)
final teamRemoteDataSourceProvider = TeamRemoteDataSourceProvider._();

final class TeamRemoteDataSourceProvider extends $FunctionalProvider<
    TeamRemoteDataSource,
    TeamRemoteDataSource,
    TeamRemoteDataSource> with $Provider<TeamRemoteDataSource> {
  TeamRemoteDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'teamRemoteDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$teamRemoteDataSourceHash();

  @$internal
  @override
  $ProviderElement<TeamRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  TeamRemoteDataSource create(Ref ref) {
    return teamRemoteDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(TeamRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<TeamRemoteDataSource>(value),
    );
  }
}

String _$teamRemoteDataSourceHash() =>
    r'e67ce871e3254c56eb58a6702f1b911143763cbd';

@ProviderFor(TeamMembers)
final teamMembersProvider = TeamMembersProvider._();

final class TeamMembersProvider
    extends $AsyncNotifierProvider<TeamMembers, List<TeamMemberModel>> {
  TeamMembersProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'teamMembersProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$teamMembersHash();

  @$internal
  @override
  TeamMembers create() => TeamMembers();
}

String _$teamMembersHash() => r'f6a0f978d81338137a960fc0662e80a9c88c98ba';

abstract class _$TeamMembers extends $AsyncNotifier<List<TeamMemberModel>> {
  FutureOr<List<TeamMemberModel>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref
        as $Ref<AsyncValue<List<TeamMemberModel>>, List<TeamMemberModel>>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<List<TeamMemberModel>>, List<TeamMemberModel>>,
        AsyncValue<List<TeamMemberModel>>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
