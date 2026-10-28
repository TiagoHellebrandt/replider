import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';

/// Relationship routines (§6.5, F4).
///
/// The cheapest revenue in B2B is the customer you already have, and the
/// cheapest way to lose them is silence. This is the small, unglamorous half
/// of that: remembering a birthday, and calling before the equipment breaks
/// rather than after.
class RelationshipDate {
  const RelationshipDate({
    required this.id,
    required this.kind,
    this.contactId = '',
    this.month = 0,
    this.day = 0,
    this.dueAt,
    this.label = '',
    this.notes = '',
  });

  final String id;

  /// birthday | founded | sector | maintenance | custom.
  final String kind;

  /// Empty for an account-wide date. Those show on this screen but never
  /// become a message on their own — firing one would message the whole
  /// portfolio, which is a campaign, not a routine.
  final String contactId;

  /// A RECURRING date: no year, because what matters about a birthday is
  /// that it comes round.
  final int month;
  final int day;

  /// A ONE-OFF date (maintenance). Null on a recurring one.
  final DateTime? dueAt;

  final String label;
  final String notes;

  bool get accountWide => contactId.isEmpty;
  bool get recurring => dueAt == null;

  factory RelationshipDate.fromJson(Map<String, dynamic> json) =>
      RelationshipDate(
        id: json['id'] as String? ?? '',
        kind: json['kind'] as String? ?? 'custom',
        contactId: json['contact_id'] as String? ?? '',
        month: (json['month'] as num?)?.toInt() ?? 0,
        day: (json['day'] as num?)?.toInt() ?? 0,
        dueAt: DateTime.tryParse(json['due_at'] as String? ?? ''),
        label: json['label'] as String? ?? '',
        notes: json['notes'] as String? ?? '',
      );
}

/// One professional day from the shipped catalogue.
///
/// Served rather than hardcoded in the client: nobody remembers that Dia do
/// Fonoaudiólogo is 9 December, and two copies of the list would eventually
/// disagree about a date somebody greets a customer on.
class SectorDate {
  const SectorDate({
    required this.key,
    required this.label,
    required this.month,
    required this.day,
  });

  final String key;
  final String label;
  final int month;
  final int day;

  factory SectorDate.fromJson(Map<String, dynamic> json) => SectorDate(
        key: json['key'] as String? ?? '',
        label: json['label'] as String? ?? '',
        month: (json['month'] as num?)?.toInt() ?? 0,
        day: (json['day'] as num?)?.toInt() ?? 0,
      );
}

final relationshipDatesProvider =
    FutureProvider<List<RelationshipDate>>((ref) async {
  final res =
      await ref.watch(dioProvider).get<List<dynamic>>('/relationship-dates');
  return (res.data ?? <dynamic>[])
      .map((e) => RelationshipDate.fromJson(e as Map<String, dynamic>))
      .toList();
});

final sectorDatesProvider = FutureProvider<List<SectorDate>>((ref) async {
  final res = await ref
      .watch(dioProvider)
      .get<List<dynamic>>('/relationship-dates/sectors');
  return (res.data ?? <dynamic>[])
      .map((e) => SectorDate.fromJson(e as Map<String, dynamic>))
      .toList();
});

final relationshipActionsProvider = Provider<RelationshipActions>(
  (ref) => RelationshipActions(ref.watch(dioProvider)),
);

class RelationshipActions {
  const RelationshipActions(this._dio);

  final Dio _dio;

  Future<void> save({
    required String kind,
    String contactId = '',
    int month = 0,
    int day = 0,
    DateTime? dueAt,
    String sectorKey = '',
    String label = '',
    String notes = '',
    String id = '',
  }) async {
    await _dio.post<Map<String, dynamic>>(
      '/relationship-dates',
      data: <String, dynamic>{
        if (id.isNotEmpty) 'id': id,
        'kind': kind,
        if (contactId.isNotEmpty) 'contact_id': contactId,
        if (month > 0) 'month': month,
        if (day > 0) 'day': day,
        if (dueAt != null) 'due_at': dueAt.toUtc().toIso8601String(),
        if (sectorKey.isNotEmpty) 'sector_key': sectorKey,
        if (label.isNotEmpty) 'label': label,
        if (notes.isNotEmpty) 'notes': notes,
      },
    );
  }

  Future<void> delete(String id) async {
    await _dio.delete<dynamic>('/relationship-dates/$id');
  }
}
