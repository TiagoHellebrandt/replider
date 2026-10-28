import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/sales/domain/models/discovered_contact.dart';

/// "Traz os clientes que você já tem", read from the customer's Sent folder.
///
/// Every call here is part of a REVIEW. The scan produces candidates, the list
/// shows them, and only an explicit selection creates anything — so there is no
/// method that imports everything, on purpose.

final discoveredContactsProvider =
    FutureProvider.autoDispose<DiscoveredContactsPage>((ref) async {
  final dio = ref.watch(dioProvider);
  final res = await dio.get<Map<String, dynamic>>(
    '/discovered-contacts',
    queryParameters: <String, dynamic>{'limit': 200},
  );
  return DiscoveredContactsPage.fromJson(res.data ?? const <String, dynamic>{});
});

class DiscoveredContactActions {
  const DiscoveredContactActions(this._ref);

  final Ref _ref;

  /// Reads the Sent folder and records what it finds.
  ///
  /// Slow by nature — it walks a mailbox — so the screen shows progress and the
  /// result reports what was read, not just that something happened.
  Future<SentScanResult> scan({required String channelId}) async {
    final dio = _ref.read(dioProvider);
    final res = await dio.post<Map<String, dynamic>>(
      '/channels/$channelId/discover-contacts',
    );
    return SentScanResult.fromJson(res.data ?? const <String, dynamic>{});
  }

  /// Creates a contact + lead for each confirmed address.
  Future<ImportContactsResult> import({
    required List<String> addresses,
    required String funnelId,
  }) async {
    final dio = _ref.read(dioProvider);
    final res = await dio.post<Map<String, dynamic>>(
      '/discovered-contacts/import',
      data: <String, dynamic>{'addresses': addresses, 'funnel_id': funnelId},
    );
    return ImportContactsResult.fromJson(res.data ?? const <String, dynamic>{});
  }

  /// Records "not a customer", so a rescan stops offering them.
  Future<void> dismiss({required List<String> addresses}) async {
    final dio = _ref.read(dioProvider);
    await dio.post<dynamic>(
      '/discovered-contacts/dismiss',
      data: <String, dynamic>{'addresses': addresses},
    );
  }
}

final discoveredContactActionsProvider = Provider<DiscoveredContactActions>(
  DiscoveredContactActions.new,
);
