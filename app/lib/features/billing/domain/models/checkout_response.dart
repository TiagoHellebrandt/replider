import 'package:json_annotation/json_annotation.dart';

part 'checkout_response.g.dart';

@JsonSerializable(fieldRename: FieldRename.snake)
class CheckoutResponse {
  final String url;

  const CheckoutResponse({required this.url});

  factory CheckoutResponse.fromJson(Map<String, dynamic> json) =>
      _$CheckoutResponseFromJson(json);
  Map<String, dynamic> toJson() => _$CheckoutResponseToJson(this);
}
