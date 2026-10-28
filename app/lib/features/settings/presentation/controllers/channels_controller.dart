import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/error/api_error_parser.dart';
import "package:app/i18n/strings.g.dart";
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/settings/data/datasources/channels_remote_datasource.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';
import 'package:app/features/settings/domain/models/whatsapp_error.dart';

part 'channels_controller.g.dart';

@riverpod
ChannelsRemoteDataSource channelsRemoteDataSource(Ref ref) {
  final dio = ref.watch(dioProvider);
  return ChannelsRemoteDataSource(dio);
}

@Riverpod(keepAlive: true)
class ChannelsList extends _$ChannelsList {
  @override
  Future<List<ChannelModel>> build() async => [];

  /// Loads all channels for the current account (derived from JWT on the API side).
  Future<void> load() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(channelsRemoteDataSourceProvider);
      final response = await datasource.getChannels();
      final rawList = response.data as List<dynamic>? ?? [];
      return rawList
          .map((item) => ChannelModel.fromJson(item as Map<String, dynamic>))
          .toList();
    });
  }

  Future<void> refresh() async {
    await load();
  }

  /// Creates a channel. Returns (error, raw) — raw is the DioException
  /// so the UI can route plan-limit responses into the upgrade modal
  /// via handlePlanLimitError() without re-parsing the envelope.
  Future<({String? error, Object? raw})> addChannel({
    required String name,
    required ChannelType type,
    required Map<String, dynamic> config,
  }) async {
    final datasource = ref.read(channelsRemoteDataSourceProvider);
    try {
      final body = <String, dynamic>{
        'name': name,
        'type': type.name,
        'config': config,
      };
      await datasource.createChannel(body);
      await refresh();
      return (error: null, raw: null);
    } on DioException catch (e) {
      return (error: _extractError(e), raw: e);
    } catch (e) {
      return (error: t.errors.UNKNOWN, raw: null);
    }
  }

  Future<String?> updateChannel({
    required String id,
    required String name,
    required Map<String, dynamic> config,
  }) async {
    final datasource = ref.read(channelsRemoteDataSourceProvider);
    try {
      // No agent_id: channels are human/SDR-only since the agent screens
      // were cut, and the picker left the dialog on 2026-09-01. Omitting
      // the key leaves a legacy assignment untouched server-side.
      final body = <String, dynamic>{'name': name};
      // Omit config entirely when caller passes an empty map so the
      // server preserves existing credentials. The WhatsApp edit form
      // sends `{}` when the user only changed the name (access_token
      // is masked and can't be re-hydrated safely — see channels_screen).
      if (config.isNotEmpty) {
        body['config'] = config;
      }
      await datasource.updateChannel(id, body);
      await refresh();
      return null;
    } on DioException catch (e) {
      return _extractError(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }

  Future<String?> toggleChannel(String id) async {
    final datasource = ref.read(channelsRemoteDataSourceProvider);
    try {
      await datasource.toggleChannel(id);
      await refresh();
      return null;
    } on DioException catch (e) {
      return _extractError(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }

  Future<String?> deleteChannel(String id) async {
    final datasource = ref.read(channelsRemoteDataSourceProvider);
    try {
      await datasource.deleteChannel(id);
      await refresh();
      return null;
    } on DioException catch (e) {
      return _extractError(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }

  Future<Map<String, dynamic>?> testChannel(String id) async {
    final datasource = ref.read(channelsRemoteDataSourceProvider);
    try {
      final response = await datasource.testChannel(id);
      return Map<String, dynamic>.from(response.data as Map);
    } on DioException catch (e) {
      return {'ok': false, 'error': _extractError(e)};
    } catch (e) {
      return {"ok": false, "error": t.errors.UNKNOWN};
    }
  }

  String _extractError(DioException e) {
    // WhatsApp-specific Meta error codes get richer humanized copy
    // (e.g. TOKEN_EXPIRED → "Reconect your WABA"), so we keep that
    // branch ahead of the generic parser. Everything else delegates to
    // parseApiError which handles localized backend codes + Dio-type-
    // aware network messages.
    final parsed = WhatsAppError.fromDio(e);
    if (parsed != null) return parsed.humanize();
    return parseApiError(e);
  }
}

/// Thin wrapper around the existing getWebhookStatus call that exposes
/// just the is_healthy flag — enough to render a green/red badge on
/// the channel list without baking the full response shape into the
/// UI. AutoDispose so leaving the screen stops polling; the card
/// itself re-subscribes when re-rendered.
@riverpod
Future<bool?> channelWebhookHealth(Ref ref, String channelId) async {
  final datasource = ref.watch(channelsRemoteDataSourceProvider);
  try {
    final res = await datasource.getWebhookStatus(channelId);
    final data = res.data;
    if (data is Map<String, dynamic>) {
      return data['is_healthy'] as bool?;
    }
    return null;
  } on DioException {
    // Unsupported channel types (webchat, instagram "em breve") return
    // NOT_SUPPORTED — surface as "unknown" (null) so the badge just
    // omits itself rather than turning red.
    return null;
  }
}
