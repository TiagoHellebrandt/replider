import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import "package:app/i18n/strings.g.dart";
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/lgpd/data/datasources/lgpd_remote_datasource.dart';
import 'package:app/features/lgpd/domain/models/audit_log_model.dart';

/// Datasource provider — thin wiring around the Retrofit client. Not a
/// @riverpod annotation because the riverpod_generator is currently
/// cascade-failing on several pre-existing controllers; writing the
/// providers by hand isolates this feature from that breakage.
final lgpdRemoteDataSourceProvider = Provider<LgpdRemoteDataSource>((ref) {
  final dio = ref.watch(dioProvider);
  return LgpdRemoteDataSource(dio);
});

/// ExportResult is what [LgpdController.exportContact] returns. The UI
/// uses it to decide whether to show a "download JSON" button and to
/// render the preview counts.
class ExportResult {
  final Map<String, dynamic> bundle;
  final int conversations;
  final int messages;
  final int campaigns;

  const ExportResult({
    required this.bundle,
    required this.conversations,
    required this.messages,
    required this.campaigns,
  });
}

/// LGPD admin actions. Exposed as a plain Notifier because there's no
/// persistent state — each call is one-shot and returns its own result.
class LgpdController extends Notifier<void> {
  @override
  void build() {}

  /// Fetches all data tied to [externalId]. Returns the bundle + count
  /// breakdown; throws a pt-BR [String] on failure so the caller can show
  /// it directly via a snackbar.
  Future<ExportResult> exportContact(String externalId) async {
    final ds = ref.read(lgpdRemoteDataSourceProvider);
    try {
      final res =
          await ds.exportContact(<String, dynamic>{'external_id': externalId});
      final data = res.data;
      if (data is! Map<String, dynamic>) {
        throw 'Resposta inesperada do servidor';
      }
      final convs = (data['conversations'] as List?) ?? const [];
      final msgs = (data['messages'] as List?) ?? const [];
      final camps = (data['campaign_participations'] as List?) ?? const [];
      return ExportResult(
        bundle: data,
        conversations: convs.length,
        messages: msgs.length,
        campaigns: camps.length,
      );
    } on DioException catch (e) {
      throw _errorOf(e);
    }
  }

  /// Anonymizes the contact and all related data. Returns null on success,
  /// or a pt-BR error string on failure.
  Future<String?> anonymizeContact(String externalId) async {
    final ds = ref.read(lgpdRemoteDataSourceProvider);
    try {
      await ds.anonymizeContact(<String, dynamic>{'external_id': externalId});
      return null;
    } on DioException catch (e) {
      return _errorOf(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }

  /// GET /audit/export.csv with the given filters. Returns the raw CSV
  /// text on success. The screen pipes the text into the clipboard so
  /// the user can paste into a .csv — web-first pragmatic export that
  /// avoids pulling in an html/file-save dep.
  Future<({String? csv, String? error})> exportAuditCSV(
      AuditLogFilter f) async {
    final dio = ref.read(dioProvider);
    try {
      final res = await dio.get<String>(
        '/audit/export.csv',
        queryParameters: {
          if (f.action != null) 'action': f.action,
          if (f.entityType != null) 'entity_type': f.entityType,
          if (f.actorId != null) 'actor_id': f.actorId,
          if (f.from != null) 'from': f.from!.toUtc().toIso8601String(),
          if (f.to != null) 'to': f.to!.toUtc().toIso8601String(),
        },
        options: Options(responseType: ResponseType.plain),
      );
      return (csv: res.data ?? '', error: null);
    } on DioException catch (e) {
      return (csv: null, error: _errorOf(e));
    } catch (e) {
      return (csv: null, error: t.errors.UNKNOWN);
    }
  }

  /// POST /admin/smtp/test — sends a canary email to the caller's own
  /// address so they can confirm SMTP is wired end-to-end. Returns null on
  /// success; an error string when the server can't send.
  Future<String?> sendSMTPTestEmail() async {
    final dio = ref.read(dioProvider);
    try {
      await dio.post<dynamic>('/admin/smtp/test');
      return null;
    } on DioException catch (e) {
      return _errorOf(e);
    } catch (e) {
      return t.errors.UNKNOWN;
    }
  }
}

final lgpdControllerProvider =
    NotifierProvider<LgpdController, void>(LgpdController.new);

/// Audit log listing — a paginated AsyncNotifier. Exposes `refresh`,
/// `loadMore`, and `applyFilters` so the screen can surface a filter bar
/// without owning state. Filters are kept here because they must persist
/// across pagination — loadMore needs to forward them too.
class AuditLogList extends AsyncNotifier<AuditLogPage> {
  int _offset = 0;
  static const int _limit = 50;
  AuditLogFilter _filter = const AuditLogFilter();

  AuditLogFilter get filter => _filter;

  @override
  Future<AuditLogPage> build() async {
    final ds = ref.read(lgpdRemoteDataSourceProvider);
    final res = await ds.listAudit(
      _limit,
      _offset,
      action: _filter.action,
      entityType: _filter.entityType,
      actorId: _filter.actorId,
      from: _filter.from?.toUtc().toIso8601String(),
      to: _filter.to?.toUtc().toIso8601String(),
    );
    final data = res.data as Map<String, dynamic>? ?? const {};
    return AuditLogPage.fromJson(data);
  }

  Future<void> refresh() async {
    _offset = 0;
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() => build());
  }

  Future<void> applyFilters(AuditLogFilter f) async {
    _filter = f;
    _offset = 0;
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() => build());
  }

  Future<void> loadMore() async {
    final current = state.asData?.value;
    if (current == null) return;
    if (current.entries.length >= current.total) return;
    _offset = current.entries.length;
    final ds = ref.read(lgpdRemoteDataSourceProvider);
    final res = await ds.listAudit(
      _limit,
      _offset,
      action: _filter.action,
      entityType: _filter.entityType,
      actorId: _filter.actorId,
      from: _filter.from?.toUtc().toIso8601String(),
      to: _filter.to?.toUtc().toIso8601String(),
    );
    final next = AuditLogPage.fromJson(res.data as Map<String, dynamic>);
    state = AsyncValue.data(AuditLogPage(
      total: next.total,
      entries: [...current.entries, ...next.entries],
    ));
  }
}

/// Immutable filter set passed to [AuditLogList.applyFilters]. All fields
/// optional — nil means "don't filter on this dimension".
class AuditLogFilter {
  final String? action;
  final String? entityType;
  final String? actorId;
  final DateTime? from;
  final DateTime? to;

  const AuditLogFilter({
    this.action,
    this.entityType,
    this.actorId,
    this.from,
    this.to,
  });

  bool get isEmpty =>
      action == null &&
      entityType == null &&
      actorId == null &&
      from == null &&
      to == null;

  AuditLogFilter copyWith({
    Object? action = _sentinel,
    Object? entityType = _sentinel,
    Object? actorId = _sentinel,
    Object? from = _sentinel,
    Object? to = _sentinel,
  }) =>
      AuditLogFilter(
        action: identical(action, _sentinel) ? this.action : action as String?,
        entityType: identical(entityType, _sentinel)
            ? this.entityType
            : entityType as String?,
        actorId:
            identical(actorId, _sentinel) ? this.actorId : actorId as String?,
        from: identical(from, _sentinel) ? this.from : from as DateTime?,
        to: identical(to, _sentinel) ? this.to : to as DateTime?,
      );

  static const _sentinel = Object();
}

final auditLogListProvider =
    AsyncNotifierProvider<AuditLogList, AuditLogPage>(AuditLogList.new);

String _errorOf(DioException e) => parseApiError(e);
