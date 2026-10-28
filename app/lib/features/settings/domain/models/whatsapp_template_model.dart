import 'package:app/i18n/strings.g.dart';

enum WhatsAppTemplateStatus {
  approved,
  pending,
  rejected,
  paused,
  disabled,
  unknown;

  static WhatsAppTemplateStatus fromMeta(String? raw) {
    switch ((raw ?? '').toUpperCase()) {
      case 'APPROVED':
        return WhatsAppTemplateStatus.approved;
      case 'PENDING':
      case 'IN_APPEAL':
      case 'PENDING_DELETION':
        return WhatsAppTemplateStatus.pending;
      case 'REJECTED':
        return WhatsAppTemplateStatus.rejected;
      case 'PAUSED':
        return WhatsAppTemplateStatus.paused;
      case 'DISABLED':
        return WhatsAppTemplateStatus.disabled;
      default:
        return WhatsAppTemplateStatus.unknown;
    }
  }

  String get label {
    switch (this) {
      case WhatsAppTemplateStatus.approved:
        return t.whatsapp.template_status_approved;
      case WhatsAppTemplateStatus.pending:
        return t.whatsapp.template_status_pending;
      case WhatsAppTemplateStatus.rejected:
        return t.whatsapp.template_status_rejected;
      case WhatsAppTemplateStatus.paused:
        return t.whatsapp.template_status_paused;
      case WhatsAppTemplateStatus.disabled:
        return t.whatsapp.template_status_disabled;
      case WhatsAppTemplateStatus.unknown:
        return '—';
    }
  }
}

enum WhatsAppTemplateCategory {
  marketing,
  utility,
  authentication;

  static WhatsAppTemplateCategory? fromMeta(String? raw) {
    switch ((raw ?? '').toUpperCase()) {
      case 'MARKETING':
        return WhatsAppTemplateCategory.marketing;
      case 'UTILITY':
        return WhatsAppTemplateCategory.utility;
      case 'AUTHENTICATION':
        return WhatsAppTemplateCategory.authentication;
      default:
        return null;
    }
  }

  String get label {
    switch (this) {
      case WhatsAppTemplateCategory.marketing:
        return t.settings.wa_category_marketing;
      case WhatsAppTemplateCategory.utility:
        return t.settings.wa_category_utility;
      case WhatsAppTemplateCategory.authentication:
        return t.whatsapp.category_authentication;
    }
  }

  String get metaValue {
    switch (this) {
      case WhatsAppTemplateCategory.marketing:
        return 'MARKETING';
      case WhatsAppTemplateCategory.utility:
        return 'UTILITY';
      case WhatsAppTemplateCategory.authentication:
        return 'AUTHENTICATION';
    }
  }
}

class WhatsAppTemplateModel {
  final String name;
  final WhatsAppTemplateStatus status;
  final String language;
  final WhatsAppTemplateCategory? category;
  final String? rawCategory;
  final List<Map<String, dynamic>> components;

  /// Human-readable reason Meta rejected this template. Populated only
  /// when `status == REJECTED`. Surfaced under the status badge so the
  /// user can fix the template instead of guessing what went wrong.
  final String? rejectedReason;

  const WhatsAppTemplateModel({
    required this.name,
    required this.status,
    required this.language,
    required this.category,
    required this.rawCategory,
    required this.components,
    this.rejectedReason,
  });

  factory WhatsAppTemplateModel.fromJson(Map<String, dynamic> json) {
    final rawComps = (json['components'] as List?) ?? const [];
    final reason = (json['rejected_reason'] as String?)?.trim();
    return WhatsAppTemplateModel(
      name: json['name'] as String? ?? '',
      status: WhatsAppTemplateStatus.fromMeta(json['status'] as String?),
      language: json['language'] as String? ?? '',
      category: WhatsAppTemplateCategory.fromMeta(json['category'] as String?),
      rawCategory: json['category'] as String?,
      components: rawComps
          .whereType<Map>()
          .map((e) => e.map((k, v) => MapEntry(k.toString(), v)))
          .toList(),
      rejectedReason: (reason == null || reason.isEmpty || reason == 'NONE')
          ? null
          : reason,
    );
  }

  /// Extract the BODY text for preview (first 2 lines).
  String get bodyPreview {
    for (final c in components) {
      if ((c['type'] as String?)?.toUpperCase() == 'BODY') {
        return (c['text'] as String?) ?? '';
      }
    }
    return '';
  }

  /// Full BODY text (no truncation), used by the preview bubble.
  String get bodyText {
    for (final c in components) {
      if ((c['type'] as String?)?.toUpperCase() == 'BODY') {
        return (c['text'] as String?) ?? '';
      }
    }
    return '';
  }

  /// Example values Meta stored alongside the BODY. Returned as an ordered
  /// list matching {{1}}..{{n}} placeholder positions. Empty when the
  /// template was approved without examples (older approvals).
  List<String> get bodyExamples {
    for (final c in components) {
      if ((c['type'] as String?)?.toUpperCase() != 'BODY') continue;
      final ex = c['example'];
      if (ex is Map && ex['body_text'] is List) {
        final outer = ex['body_text'] as List;
        if (outer.isNotEmpty && outer.first is List) {
          return (outer.first as List).map((e) => e?.toString() ?? '').toList();
        }
      }
    }
    return const [];
  }

  /// Optional footer text (small muted line at the bottom of the bubble).
  String? get footerText {
    for (final c in components) {
      if ((c['type'] as String?)?.toUpperCase() == 'FOOTER') {
        final t = c['text'] as String?;
        if (t != null && t.isNotEmpty) return t;
      }
    }
    return null;
  }

  /// Interactive buttons defined on the template. Each entry has at least
  /// a `type` (QUICK_REPLY / URL / PHONE_NUMBER) and a `text` label.
  List<Map<String, dynamic>> get buttons {
    for (final c in components) {
      if ((c['type'] as String?)?.toUpperCase() == 'BUTTONS') {
        final raw = c['buttons'];
        if (raw is List) {
          return raw
              .whereType<Map>()
              .map((m) => Map<String, dynamic>.from(m))
              .toList();
        }
      }
    }
    return const [];
  }

  /// Body text with {{1}}, {{2}}, ... substituted by the stored examples
  /// (falling back to the placeholder itself if no example is available).
  /// Used as the "as-seen-by-customer" preview.
  String get renderedBody {
    final text = bodyText;
    final ex = bodyExamples;
    if (text.isEmpty || ex.isEmpty) return text;
    return text.replaceAllMapped(RegExp(r'\{\{(\d+)\}\}'), (m) {
      final idx = int.tryParse(m.group(1) ?? '');
      if (idx != null && idx >= 1 && idx <= ex.length) return ex[idx - 1];
      return m.group(0)!;
    });
  }

  String get languageLabel {
    switch (language) {
      case 'pt_BR':
        return t.whatsapp.language_pt_br;
      case 'en_US':
        return t.whatsapp.language_en_us;
      case 'es':
      case 'es_ES':
      case 'es_MX':
        return t.whatsapp.language_es;
      default:
        return language;
    }
  }
}
