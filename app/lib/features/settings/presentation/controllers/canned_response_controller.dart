import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/settings/data/datasources/canned_response_remote_datasource.dart';
import 'package:app/features/settings/domain/models/canned_response_model.dart';

part 'canned_response_controller.g.dart';

@riverpod
CannedResponseRemoteDataSource cannedResponseDataSource(Ref ref) {
  final dio = ref.watch(dioProvider);
  return CannedResponseRemoteDataSource(dio);
}

@riverpod
class CannedResponseController extends _$CannedResponseController {
  @override
  FutureOr<List<CannedResponseModel>> build() async {
    final ds = ref.watch(cannedResponseDataSourceProvider);
    return ds.list();
  }

  Future<void> create({
    required String shortcut,
    required String title,
    required String content,
    String category = 'general',
    // 'account' (admin-only) or 'personal' (owner-only) — US-018.
    // Immutable after creation, so update() doesn't take it.
    String visibility = 'account',
  }) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final ds = ref.read(cannedResponseDataSourceProvider);
      await ds.create({
        'shortcut': shortcut,
        'title': title,
        'content': content,
        'category': category,
        'visibility': visibility,
      });
      return ds.list();
    });
  }

  Future<void> delete(String id) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final ds = ref.read(cannedResponseDataSourceProvider);
      await ds.delete(id);
      return ds.list();
    });
  }
}
