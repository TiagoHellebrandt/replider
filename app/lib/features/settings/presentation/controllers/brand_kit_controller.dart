import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';

/// The company voice layer, and the seller's personal one.
///
/// Two models rather than one because they answer to different people: the
/// company layer decides what the BUSINESS may claim — an admin decision, and
/// in healthcare a regulatory one — while the overlay is how one person signs
/// their own messages.
class BrandKit {
  const BrandKit({
    this.companyName = '',
    this.about = '',
    this.toneOfVoice = '',
    this.signature = '',
    this.allowedClaims = const [],
    this.forbiddenClaims = const [],
  });

  final String companyName;
  final String about;
  final String toneOfVoice;
  final String signature;

  /// True things a salesperson may say. Listed rather than implied, so a new
  /// seller has a source instead of a guess.
  final List<String> allowedClaims;

  /// Things nobody may say, at any level. The AI is instructed against them
  /// AND the draft is checked deterministically, because an instruction is a
  /// request and this is a compliance rule.
  final List<String> forbiddenClaims;

  factory BrandKit.fromJson(Map<String, dynamic> json) => BrandKit(
        companyName: json['company_name'] as String? ?? '',
        about: json['about'] as String? ?? '',
        toneOfVoice: json['tone_of_voice'] as String? ?? '',
        signature: json['signature'] as String? ?? '',
        allowedClaims: _list(json['allowed_claims']),
        forbiddenClaims: _list(json['forbidden_claims']),
      );

  Map<String, dynamic> toJson() => <String, dynamic>{
        'company_name': companyName,
        'about': about,
        'tone_of_voice': toneOfVoice,
        'signature': signature,
        'allowed_claims': allowedClaims,
        'forbidden_claims': forbiddenClaims,
      };

  static List<String> _list(dynamic v) =>
      (v as List<dynamic>? ?? const <dynamic>[])
          .whereType<String>()
          .where((s) => s.trim().isNotEmpty)
          .toList();
}

class SellerOverlay {
  const SellerOverlay({this.toneNote = '', this.signature = ''});

  final String toneNote;
  final String signature;

  factory SellerOverlay.fromJson(Map<String, dynamic> json) => SellerOverlay(
        toneNote: json['tone_note'] as String? ?? '',
        signature: json['signature'] as String? ?? '',
      );
}

/// The account's brand kit. The server answers with an empty one rather than
/// a 404 when none exists, so this never has to special-case "not created
/// yet" — a form that branches on that gets the branch wrong.
final brandKitProvider = FutureProvider<BrandKit>((ref) async {
  final res =
      await ref.watch(dioProvider).get<Map<String, dynamic>>('/brand-kit');
  return BrandKit.fromJson(res.data ?? const <String, dynamic>{});
});

final sellerOverlayProvider = FutureProvider<SellerOverlay>((ref) async {
  final res = await ref
      .watch(dioProvider)
      .get<Map<String, dynamic>>('/me/seller-overlay');
  return SellerOverlay.fromJson(res.data ?? const <String, dynamic>{});
});

final brandActionsProvider = Provider<BrandActions>(
  (ref) => BrandActions(ref.watch(dioProvider)),
);

class BrandActions {
  const BrandActions(this._dio);

  final Dio _dio;

  Future<void> saveKit(BrandKit kit) async {
    await _dio.put<dynamic>('/brand-kit', data: kit.toJson());
  }

  /// The overlay is scoped to the caller by the token, never by the body —
  /// there is no user id to send, so this cannot rewrite a colleague's
  /// signature.
  Future<void> saveOverlay(SellerOverlay o) async {
    await _dio.put<dynamic>('/me/seller-overlay', data: <String, dynamic>{
      'tone_note': o.toneNote,
      'signature': o.signature,
    });
  }
}
