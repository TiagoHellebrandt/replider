import 'package:json_annotation/json_annotation.dart';

import 'package:app/core/utils/currency_format.dart';

part 'addon_pack_model.g.dart';

@JsonSerializable(fieldRename: FieldRename.snake)

/// A one-time credit pack that adds AI tokens to the account.
/// Purchased via Stripe one-time checkout. See `GET /v1/payments/addon-packs`.
class AddonPackModel {
  final String id;
  final String name;
  final int tokens;
  final int price; // cents, in [currency]
  final String currency; // 'brl' | 'usd'

  const AddonPackModel({
    required this.id,
    required this.name,
    required this.tokens,
    required this.price,
    this.currency = 'brl',
  });

  factory AddonPackModel.fromJson(Map<String, dynamic> json) =>
      _$AddonPackModelFromJson(json);
  Map<String, dynamic> toJson() => _$AddonPackModelToJson(this);

  /// Formats the price in the pack's own currency — the API serves the
  /// BRL or USD ladder based on the account currency, so USD packs render
  /// as `$9` and BRL packs as `R$ 47`.
  String get formattedPrice => formatCurrencyCents(price, currency);
  String get formattedTokens => '${(tokens / 1000).toStringAsFixed(0)}K';
}
