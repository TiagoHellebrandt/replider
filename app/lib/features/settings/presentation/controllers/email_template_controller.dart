import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';

/// One reusable message.
///
/// The derived facts — variables, word count, too-long — come from the SERVER
/// rather than being recomputed here. The renderer that substitutes variables
/// lives there too, and two implementations of "what is a variable" would
/// eventually disagree about a real message.
class EmailTemplate {
  const EmailTemplate({
    required this.id,
    required this.name,
    this.subject = '',
    this.body = '',
    this.version = 1,
    this.variables = const [],
    this.wordCount = 0,
    this.tooLong = false,
    this.stats = const [],
  });

  final String id;
  final String name;
  final String subject;
  final String body;

  /// Bumped when the TEXT changes, so the numbers below always describe the
  /// words that earned them.
  final int version;

  final List<String> variables;
  final int wordCount;

  /// A warning, never a limit: the person writing knows their market and the
  /// hundred-word line is a rule of thumb.
  final bool tooLong;

  final List<TemplateStats> stats;

  factory EmailTemplate.fromJson(Map<String, dynamic> json) => EmailTemplate(
        id: json['id'] as String? ?? '',
        name: json['name'] as String? ?? '',
        subject: json['subject'] as String? ?? '',
        body: json['body'] as String? ?? '',
        version: (json['version'] as num?)?.toInt() ?? 1,
        variables: (json['variables'] as List<dynamic>? ?? const <dynamic>[])
            .whereType<String>()
            .toList(),
        wordCount: (json['word_count'] as num?)?.toInt() ?? 0,
        tooLong: json['too_long'] as bool? ?? false,
        stats: (json['stats'] as List<dynamic>? ?? const <dynamic>[])
            .whereType<Map<String, dynamic>>()
            .map(TemplateStats.fromJson)
            .toList(),
      );
}

class TemplateStats {
  const TemplateStats({
    required this.version,
    required this.sent,
    required this.opened,
    required this.clicked,
    required this.replied,
    required this.bounced,
  });

  final int version;
  final int sent;

  /// Weak by nature: Apple Mail Privacy Protection pre-fetches images, which
  /// inflates this by roughly half. Shown with that caveat attached, never
  /// on its own.
  final int opened;

  final int clicked;
  final int replied;
  final int bounced;

  /// Null when nothing was sent. Zero replies out of zero sends is not a
  /// zero rate, it is unknown — and rendering 0% would condemn a template
  /// nobody has used yet.
  double? get replyRate => sent == 0 ? null : replied / sent;

  double? get bounceRate => sent == 0 ? null : bounced / sent;

  /// Mailbox providers throttle a sender well before ten percent, and by then
  /// the damage takes weeks to undo. Needs a real sample: one bounce out of
  /// two sends is 50% and means nothing.
  bool get needsListWarning {
    if (sent < 20) return false;
    final r = bounceRate;
    return r != null && r >= 0.05;
  }

  factory TemplateStats.fromJson(Map<String, dynamic> json) => TemplateStats(
        version: (json['version'] as num?)?.toInt() ?? 1,
        sent: (json['sent'] as num?)?.toInt() ?? 0,
        opened: (json['opened'] as num?)?.toInt() ?? 0,
        clicked: (json['clicked'] as num?)?.toInt() ?? 0,
        replied: (json['replied'] as num?)?.toInt() ?? 0,
        bounced: (json['bounced'] as num?)?.toInt() ?? 0,
      );
}

final emailTemplatesProvider = FutureProvider<List<EmailTemplate>>((ref) async {
  final res =
      await ref.watch(dioProvider).get<List<dynamic>>('/email-templates');
  return (res.data ?? const <dynamic>[])
      .whereType<Map<String, dynamic>>()
      .map(EmailTemplate.fromJson)
      .toList();
});

/// One template with its per-version numbers. Separate from the list because
/// the list does not carry stats — fetching them for every row would be a
/// query per template to render a screen most people scroll past.
final emailTemplateProvider =
    FutureProvider.autoDispose.family<EmailTemplate, String>((ref, id) async {
  final res = await ref
      .watch(dioProvider)
      .get<Map<String, dynamic>>('/email-templates/$id');
  return EmailTemplate.fromJson(res.data ?? const <String, dynamic>{});
});

final emailTemplateActionsProvider = Provider<EmailTemplateActions>(
  (ref) => EmailTemplateActions(ref.watch(dioProvider)),
);

class EmailTemplateActions {
  const EmailTemplateActions(this._dio);

  final Dio _dio;

  /// Pass an empty id to create.
  Future<EmailTemplate> save({
    String id = '',
    required String name,
    required String subject,
    required String body,
  }) async {
    final res = await _dio.put<Map<String, dynamic>>(
      '/email-templates/${id.isEmpty ? 'new' : id}',
      data: <String, dynamic>{'name': name, 'subject': subject, 'body': body},
    );
    return EmailTemplate.fromJson(res.data ?? const <String, dynamic>{});
  }
}
