import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/sales/domain/models/lead_model.dart';

part 'first_offer_controller.g.dart';

/// What POST /me/onboarding/first-offer answered (mirrors the Go
/// `usecases.FirstOfferResult`).
class FirstOffer {
  const FirstOffer({
    required this.status,
    this.reason = '',
    this.source = '',
    this.lead,
    this.draft = '',
  });

  /// `ready` or `nothing_to_offer`.
  final String status;

  /// Set when not ready: `no_customers_no_territory`, `no_business_found`.
  final String reason;

  /// `customer` (one she imported) or `prospect` (found in her territory).
  final String source;
  final LeadModel? lead;
  final String draft;

  bool get isReady => status == 'ready';

  factory FirstOffer.fromJson(Map<String, dynamic> json) => FirstOffer(
        status: json['status'] as String? ?? '',
        reason: json['reason'] as String? ?? '',
        source: json['source'] as String? ?? '',
        lead: json['lead'] is Map<String, dynamic>
            ? LeadModel.fromJson(json['lead'] as Map<String, dynamic>)
            : null,
        draft: json['draft'] as String? ?? '',
      );
}

/// Asks the server for the first offer once per wizard visit.
///
/// A POST, not a GET, because the server may create a lead and spend a
/// draft to answer — but the server is idempotent (a reload gets the same
/// offer), so retrying after a failure is safe and the step's Retry button
/// simply invalidates this.
@riverpod
class FirstOfferController extends _$FirstOfferController {
  @override
  Future<FirstOffer> build() async {
    final dio = ref.read(dioProvider);
    final res = await dio.post<dynamic>('/me/onboarding/first-offer');
    return FirstOffer.fromJson((res.data as Map).cast<String, dynamic>());
  }
}
