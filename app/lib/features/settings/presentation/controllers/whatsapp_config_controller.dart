import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/error/api_error_parser.dart';
import "package:app/i18n/strings.g.dart";
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/settings/data/datasources/whatsapp_config_remote_datasource.dart';
import 'package:app/features/settings/domain/models/whatsapp_template_model.dart';
import 'package:app/features/settings/domain/models/whatsapp_error.dart';

part 'whatsapp_config_controller.g.dart';

@riverpod
WhatsAppConfigRemoteDataSource whatsAppConfigRemoteDataSource(Ref ref) {
  final dio = ref.watch(dioProvider);
  return WhatsAppConfigRemoteDataSource(dio);
}

@riverpod
class WhatsAppTemplates extends _$WhatsAppTemplates {
  @override
  Future<List<WhatsAppTemplateModel>> build(String channelId) async {
    final ds = ref.read(whatsAppConfigRemoteDataSourceProvider);
    final res = await ds.listTemplates(channelId);
    final raw = res.data as List<dynamic>? ?? [];
    return raw
        .map((e) => WhatsAppTemplateModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<String?> create({
    required String name,
    required WhatsAppTemplateCategory category,
    required String language,
    required List<Map<String, dynamic>> components,
  }) async {
    final ds = ref.read(whatsAppConfigRemoteDataSourceProvider);
    try {
      await ds.createTemplate(channelId, {
        'name': name,
        'category': category.metaValue,
        'language': language,
        'components': components,
      });
      ref.invalidateSelf();
      // Fire-and-forget polling: Meta often approves templates within
      // a minute. Refresh at 10s/30s/60s/120s so the status flips from
      // "Em análise" to "Aprovado" without the user manually pulling
      // to refresh. Guards against disposal by checking `ref.mounted`.
      _scheduleApprovalPolling(name);
      return null;
    } on DioException catch (e) {
      return _extractError(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }

  void _scheduleApprovalPolling(String name) {
    const delays = [
      Duration(seconds: 10),
      Duration(seconds: 30),
      Duration(seconds: 60),
      Duration(seconds: 120),
    ];
    for (final d in delays) {
      Future<void>.delayed(d, () {
        if (!ref.mounted) return;
        final asyncState = state;
        if (!asyncState.hasValue) return;
        final current = asyncState.value;
        if (current == null || current.isEmpty) return;
        final tracked = current.firstWhere(
          (t) => t.name == name,
          orElse: () => current.first,
        );
        // Stop polling as soon as Meta made a decision.
        if (tracked.status == WhatsAppTemplateStatus.approved ||
            tracked.status == WhatsAppTemplateStatus.rejected) {
          return;
        }
        ref.invalidateSelf();
      });
    }
  }

  Future<String?> delete(String name) async {
    final ds = ref.read(whatsAppConfigRemoteDataSourceProvider);
    try {
      await ds.deleteTemplate(channelId, name);
      ref.invalidateSelf();
      return null;
    } on DioException catch (e) {
      return _extractError(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }
}

String _extractError(DioException e) {
  // WhatsApp Meta error codes get richer humanized copy first; the
  // generic parseApiError handles localized backend codes and Dio-type-
  // aware network messages otherwise.
  final parsed = WhatsAppError.fromDio(e);
  if (parsed != null) return parsed.humanize();
  return parseApiError(e);
}
