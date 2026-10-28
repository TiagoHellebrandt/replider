import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/settings/data/datasources/tag_remote_datasource.dart';
import 'package:app/features/settings/domain/models/tag_model.dart';

part 'tag_controller.g.dart';

@riverpod
TagRemoteDataSource tagDataSource(Ref ref) {
  final dio = ref.watch(dioProvider);
  return TagRemoteDataSource(dio);
}

@riverpod
class TagController extends _$TagController {
  @override
  FutureOr<List<TagModel>> build() async {
    final ds = ref.watch(tagDataSourceProvider);
    return ds.list();
  }

  Future<void> create({required String name, String color = '#6366f1'}) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final ds = ref.read(tagDataSourceProvider);
      await ds.create({'name': name, 'color': color});
      return ds.list();
    });
  }

  Future<void> delete(String id) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final ds = ref.read(tagDataSourceProvider);
      await ds.delete(id);
      return ds.list();
    });
  }
}
