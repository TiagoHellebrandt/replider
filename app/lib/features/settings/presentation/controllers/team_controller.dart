import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/error/api_error_parser.dart';
import "package:app/i18n/strings.g.dart";
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/settings/data/datasources/team_remote_datasource.dart';
import 'package:app/features/settings/domain/models/team_member_model.dart';

part 'team_controller.g.dart';

// ── Datasource Provider ───────────────────────────────────────────────────

@riverpod
TeamRemoteDataSource teamRemoteDataSource(Ref ref) {
  final dio = ref.watch(dioProvider);
  return TeamRemoteDataSource(dio);
}

// ── Team Members Provider ─────────────────────────────────────────────────

@riverpod
class TeamMembers extends _$TeamMembers {
  @override
  Future<List<TeamMemberModel>> build() async {
    final datasource = ref.watch(teamRemoteDataSourceProvider);
    return await datasource.listMembers();
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(teamRemoteDataSourceProvider);
      return await datasource.listMembers();
    });
  }

  /// Invites a new team member. Returns (error, raw) so callers can
  /// route PLAN_LIMIT_EXCEEDED into the upgrade modal via
  /// handlePlanLimitError — same contract as createAgent/addChannel.
  Future<({String? error, Object? raw})> inviteMember(
      String name, String email, String role) async {
    final datasource = ref.read(teamRemoteDataSourceProvider);
    try {
      await datasource.inviteMember({
        'name': name,
        'email': email,
        'role': role,
      });
      await refresh();
      return (error: null, raw: null);
    } on DioException catch (e) {
      return (error: _extractErrorMessage(e), raw: e);
    } catch (e) {
      return (error: t.errors.UNKNOWN, raw: null);
    }
  }

  Future<String?> updateRole(String memberId, String role) async {
    final datasource = ref.read(teamRemoteDataSourceProvider);
    try {
      await datasource.updateRole(memberId, {'role': role});
      await refresh();
      return null;
    } on DioException catch (e) {
      return _extractErrorMessage(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }

  Future<String?> removeMember(String memberId) async {
    final datasource = ref.read(teamRemoteDataSourceProvider);
    try {
      await datasource.removeMember(memberId);
      await refresh();
      return null;
    } on DioException catch (e) {
      return _extractErrorMessage(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }

  /// Re-sends the invite email for a member that hasn't verified their
  /// email yet. Bumps users.invite_sent_at on the backend so the
  /// "Pending" badge timestamp refreshes.
  Future<String?> resendInvite(String memberId) async {
    final datasource = ref.read(teamRemoteDataSourceProvider);
    try {
      await datasource.resendInvite(memberId);
      await refresh();
      return null;
    } on DioException catch (e) {
      return _extractErrorMessage(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }

  String _extractErrorMessage(DioException e) => parseApiError(e);
}
