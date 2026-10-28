/// Models for the Claude (MCP) integration settings screen.
///
/// `PatModel.token` is the one-time plaintext credential — present ONLY
/// in the create response; list calls return metadata + prefix only, so
/// the UI must surface the token before navigating away.
class PatModel {
  final String id;
  final String name;
  final String tokenPrefix;
  final DateTime? createdAt;
  final DateTime? lastUsedAt;
  final DateTime? expiresAt;
  final String? token;

  const PatModel({
    required this.id,
    required this.name,
    required this.tokenPrefix,
    this.createdAt,
    this.lastUsedAt,
    this.expiresAt,
    this.token,
  });

  factory PatModel.fromJson(Map<String, dynamic> json) {
    return PatModel(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      tokenPrefix: json['token_prefix'] as String? ?? '',
      createdAt: _parseDate(json['created_at']),
      lastUsedAt: _parseDate(json['last_used_at']),
      expiresAt: _parseDate(json['expires_at']),
      token: json['token'] as String?,
    );
  }
}

/// An active OAuth grant (e.g. the claude.ai connector) the user can
/// revoke from the dashboard.
class ClaudeConnectionModel {
  final String clientId;
  final String clientName;
  final DateTime? firstGrantedAt;
  final DateTime? lastUsedAt;

  const ClaudeConnectionModel({
    required this.clientId,
    required this.clientName,
    this.firstGrantedAt,
    this.lastUsedAt,
  });

  factory ClaudeConnectionModel.fromJson(Map<String, dynamic> json) {
    return ClaudeConnectionModel(
      clientId: json['client_id'] as String? ?? '',
      clientName: json['client_name'] as String? ?? '',
      firstGrantedAt: _parseDate(json['first_granted_at']),
      lastUsedAt: _parseDate(json['last_used_at']),
    );
  }
}

DateTime? _parseDate(dynamic v) {
  if (v == null) return null;
  if (v is String) return DateTime.tryParse(v);
  return null;
}
