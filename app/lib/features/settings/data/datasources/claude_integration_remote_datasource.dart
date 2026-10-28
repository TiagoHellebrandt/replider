import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';

part 'claude_integration_remote_datasource.g.dart';

/// MCP integration management: personal access tokens (pasted into
/// Claude Code) and active OAuth connections (claude.ai connector).
/// The PAT create response carries the plaintext token exactly once.
@RestApi()
abstract class ClaudeIntegrationRemoteDataSource {
  factory ClaudeIntegrationRemoteDataSource(Dio dio) =
      _ClaudeIntegrationRemoteDataSource;

  @GET('/me/mcp-tokens')
  Future<HttpResponse<dynamic>> listTokens();

  @POST('/me/mcp-tokens')
  Future<HttpResponse<dynamic>> createToken(@Body() Map<String, dynamic> body);

  @DELETE('/me/mcp-tokens/{id}')
  Future<HttpResponse<dynamic>> revokeToken(@Path('id') String id);

  @GET('/me/mcp-connections')
  Future<HttpResponse<dynamic>> listConnections();

  @DELETE('/me/mcp-connections/{clientId}')
  Future<HttpResponse<dynamic>> revokeConnection(
    @Path('clientId') String clientId,
  );
}
