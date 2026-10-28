import 'package:json_annotation/json_annotation.dart';

part 'transparent_checkout_response.g.dart';

/// Discriminated response of `POST /payments/transparent-checkout`.
///
/// - `provider`: "abacatepay" (BRL) or "stripe" (non-BRL / AbacatePay off).
/// - `method`: "pix" or "card".
/// - `url`: present for every card / Stripe checkout (open externally).
/// - `pix`: present only for a BRL PIX charge (rendered inline).
@JsonSerializable(fieldRename: FieldRename.snake)
class TransparentCheckoutResponse {
  final String provider;
  final String method;
  final String? url;
  final PixChargeModel? pix;

  const TransparentCheckoutResponse({
    required this.provider,
    required this.method,
    this.url,
    this.pix,
  });

  bool get isPix => method == 'pix' && pix != null;
  bool get isStripe => provider == 'stripe';

  factory TransparentCheckoutResponse.fromJson(Map<String, dynamic> json) =>
      _$TransparentCheckoutResponseFromJson(json);
  Map<String, dynamic> toJson() => _$TransparentCheckoutResponseToJson(this);
}

/// A minted transparent PIX charge: copy-paste code + QR image + amount.
@JsonSerializable(fieldRename: FieldRename.snake)
class PixChargeModel {
  final String id;

  /// PIX copy-paste string (copia-e-cola).
  final String brCode;

  /// QR code PNG as a `data:image/png;base64,...` URI.
  final String brCodeBase64;

  /// Amount in cents (BRL).
  final int amount;

  /// AbacatePay charge status (PENDING|PAID|EXPIRED|...).
  final String status;

  final String? expiresAt;

  const PixChargeModel({
    required this.id,
    required this.brCode,
    required this.brCodeBase64,
    required this.amount,
    required this.status,
    this.expiresAt,
  });

  factory PixChargeModel.fromJson(Map<String, dynamic> json) =>
      _$PixChargeModelFromJson(json);
  Map<String, dynamic> toJson() => _$PixChargeModelToJson(this);
}

/// Response of `GET /payments/transparent-checkout/{id}/status`.
@JsonSerializable(fieldRename: FieldRename.snake)
class ChargeStatusResponse {
  final String status;
  final bool paid;

  const ChargeStatusResponse({required this.status, required this.paid});

  factory ChargeStatusResponse.fromJson(Map<String, dynamic> json) =>
      _$ChargeStatusResponseFromJson(json);
  Map<String, dynamic> toJson() => _$ChargeStatusResponseToJson(this);
}
