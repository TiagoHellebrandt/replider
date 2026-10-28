import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';

/// One address this account must not write to.
///
/// The reason and the diagnostic are both shown, because the question a
/// person arrives with is "why can't I email them?" — and the answer is the
/// remote mail server's own words, not a policy of ours.
class EmailSuppression {
  const EmailSuppression({
    required this.address,
    required this.reason,
    this.diagnostic = '',
    this.createdAt,
  });

  final String address;

  /// `hard_bounce`, `complaint` or `manual`.
  final String reason;
  final String diagnostic;
  final DateTime? createdAt;

  /// A complaint is NOT a hard bounce: the address works, and the person
  /// told their provider we are abusive. Worth showing differently.
  bool get isComplaint => reason == 'complaint';

  factory EmailSuppression.fromJson(Map<String, dynamic> json) =>
      EmailSuppression(
        address: json['address'] as String? ?? '',
        reason: json['reason'] as String? ?? 'hard_bounce',
        diagnostic: json['diagnostic'] as String? ?? '',
        createdAt: DateTime.tryParse(json['created_at'] as String? ?? ''),
      );
}

final emailSuppressionsProvider =
    FutureProvider<List<EmailSuppression>>((ref) async {
  final res =
      await ref.watch(dioProvider).get<List<dynamic>>('/email-suppressions');
  return (res.data ?? <dynamic>[])
      .map((e) => EmailSuppression.fromJson(e as Map<String, dynamic>))
      .toList();
});

final emailSuppressionActionsProvider = Provider<EmailSuppressionActions>(
  (ref) => EmailSuppressionActions(ref.watch(dioProvider)),
);

class EmailSuppressionActions {
  const EmailSuppressionActions(this._dio);

  final Dio _dio;

  /// Puts an address back into circulation.
  ///
  /// Exists because a suppression can be wrong: an address is fixed, a
  /// mailbox is emptied, a person changes their mind. Nothing about a bounce
  /// should be permanent to the point of being unappealable.
  Future<void> release(String address) async {
    await _dio.delete<dynamic>('/email-suppressions/$address');
  }
}
