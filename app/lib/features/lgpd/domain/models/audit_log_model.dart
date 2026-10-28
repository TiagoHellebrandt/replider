import 'package:app/i18n/strings.g.dart';

/// Read-only view of one entry in the audit_logs table. Payload is kept as
/// a raw Map because the fields vary per action (e.g. lgpd.export carries
/// counts; lgpd.anonymize carries the new_external_id).
class AuditLogEntry {
  final String id;
  final String accountId;
  final String? actorId;
  final String action;
  final String entityType;
  final String entityId;
  final Map<String, dynamic> payload;
  final DateTime createdAt;

  const AuditLogEntry({
    required this.id,
    required this.accountId,
    required this.action,
    required this.entityType,
    required this.entityId,
    required this.payload,
    required this.createdAt,
    this.actorId,
  });

  factory AuditLogEntry.fromJson(Map<String, dynamic> json) {
    DateTime parsed;
    try {
      parsed = DateTime.parse(json['created_at'] as String? ?? '');
    } catch (_) {
      parsed = DateTime.now();
    }
    final rawPayload = json['payload'];
    Map<String, dynamic> payload = const {};
    if (rawPayload is Map<String, dynamic>) {
      payload = rawPayload;
    } else if (rawPayload is List<dynamic>) {
      // Server emits raw bytes for jsonb — an empty payload can round-trip
      // as base64 on some clients. Gracefully degrade to empty.
      payload = const {};
    }
    return AuditLogEntry(
      id: json['id'] as String? ?? '',
      accountId: json['account_id'] as String? ?? '',
      actorId: json['actor_id'] as String?,
      action: json['action'] as String? ?? '',
      entityType: json['entity_type'] as String? ?? '',
      entityId: json['entity_id'] as String? ?? '',
      payload: payload,
      createdAt: parsed,
    );
  }

  /// Human-readable label for the common action codes.
  String get actionLabel {
    switch (action) {
      case 'lgpd.export':
        return t.lgpd.audit_action_export;
      case 'lgpd.anonymize':
        return t.lgpd.audit_action_anonymize;
      default:
        return action;
    }
  }
}

class AuditLogPage {
  final int total;
  final List<AuditLogEntry> entries;

  const AuditLogPage({required this.total, required this.entries});

  factory AuditLogPage.fromJson(Map<String, dynamic> json) {
    final raw = json['entries'] as List<dynamic>? ?? const [];
    return AuditLogPage(
      total: (json['total'] as num?)?.toInt() ?? 0,
      entries: raw
          .whereType<Map<String, dynamic>>()
          .map(AuditLogEntry.fromJson)
          .toList(),
    );
  }
}
