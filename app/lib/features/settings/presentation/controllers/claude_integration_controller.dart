import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/settings/data/datasources/claude_integration_remote_datasource.dart';
import 'package:app/features/settings/domain/models/claude_integration_models.dart';

part 'claude_integration_controller.g.dart';

@riverpod
ClaudeIntegrationRemoteDataSource claudeIntegrationRemoteDataSource(Ref ref) {
  final dio = ref.watch(dioProvider);
  return ClaudeIntegrationRemoteDataSource(dio);
}

/// Combined screen state: PATs + active OAuth connections.
class ClaudeIntegrationState {
  final List<PatModel> tokens;
  final List<ClaudeConnectionModel> connections;

  const ClaudeIntegrationState({
    required this.tokens,
    required this.connections,
  });
}

@riverpod
class ClaudeIntegration extends _$ClaudeIntegration {
  @override
  Future<ClaudeIntegrationState> build() async {
    final ds = ref.watch(claudeIntegrationRemoteDataSourceProvider);
    final tokensRes = await ds.listTokens();
    final connectionsRes = await ds.listConnections();
    return ClaudeIntegrationState(
      tokens: _parseList(tokensRes.data, PatModel.fromJson),
      connections:
          _parseList(connectionsRes.data, ClaudeConnectionModel.fromJson),
    );
  }

  static List<T> _parseList<T>(
    dynamic body,
    T Function(Map<String, dynamic>) fromJson,
  ) {
    if (body is List) {
      return body
          .whereType<Map<String, dynamic>>()
          .map(fromJson)
          .toList(growable: false);
    }
    return const [];
  }

  /// Creates a PAT and returns it WITH the one-time plaintext token so
  /// the screen can pop the "save this now" dialog.
  Future<PatModel?> createToken(String name) async {
    final ds = ref.read(claudeIntegrationRemoteDataSourceProvider);
    final res = await ds.createToken({'name': name});
    final body = res.data;
    ref.invalidateSelf();
    if (body is! Map<String, dynamic>) return null;
    return PatModel.fromJson(body);
  }

  Future<void> revokeToken(String id) async {
    final ds = ref.read(claudeIntegrationRemoteDataSourceProvider);
    await ds.revokeToken(id);
    ref.invalidateSelf();
  }

  Future<void> revokeConnection(String clientId) async {
    final ds = ref.read(claudeIntegrationRemoteDataSourceProvider);
    await ds.revokeConnection(clientId);
    ref.invalidateSelf();
  }
}
