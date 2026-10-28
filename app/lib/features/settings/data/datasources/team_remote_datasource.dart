import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';
import 'package:app/features/settings/domain/models/team_member_model.dart';

part 'team_remote_datasource.g.dart';

@RestApi()
abstract class TeamRemoteDataSource {
  factory TeamRemoteDataSource(Dio dio) = _TeamRemoteDataSource;

  @GET('/team')
  Future<List<TeamMemberModel>> listMembers();

  @POST('/team')
  Future<TeamMemberModel> inviteMember(@Body() Map<String, dynamic> body);

  @PATCH('/team/{id}/role')
  Future<void> updateRole(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  @DELETE('/team/{id}')
  Future<void> removeMember(@Path('id') String id);

  @POST('/team/{id}/resend-invite')
  Future<void> resendInvite(@Path('id') String id);
}
