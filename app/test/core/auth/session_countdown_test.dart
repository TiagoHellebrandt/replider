import 'dart:convert';

import 'package:app/core/auth/session_countdown.dart';
import 'package:flutter_test/flutter_test.dart';

// A sessão dura 24 horas e não se renova, então uma vez por dia a pessoa é
// posta para fora no meio de alguma coisa. O prazo já viaja dentro do próprio
// token; ler o `exp` é o que permite avisar antes, em vez de descobrir junto
// com o 401.

String _token(Map<String, dynamic> payload) {
  String seg(Map<String, dynamic> m) =>
      base64Url.encode(utf8.encode(jsonEncode(m))).replaceAll('=', '');
  return '${seg({'alg': 'HS256'})}.${seg(payload)}.assinatura-nao-e-lida-aqui';
}

void main() {
  test('lê o prazo do token', () {
    final exp = DateTime.utc(2026, 7, 31, 23, 0);
    final at = jwtExpiry(_token({'exp': exp.millisecondsSinceEpoch ~/ 1000}));

    expect(at, exp);
  });

  test('base64url sem padding é o normal num JWT', () {
    // Um payload cujo comprimento força padding — se a conta estiver errada,
    // o decode explode e o app deixa de avisar sem que ninguém perceba.
    for (final extra in ['a', 'ab', 'abc', 'abcd']) {
      final t = _token({
        'exp': 1800000000,
        'pad': extra,
      });
      expect(jwtExpiry(t), isNotNull, reason: 'falhou com "$extra"');
    }
  });

  test('qualquer coisa ilegível não vira aviso nenhum', () {
    for (final junk in [
      '',
      'nao-e-jwt',
      'a.b',
      'a.b.c',
      '${base64Url.encode(utf8.encode('{}'))}.x.y',
    ]) {
      expect(jwtExpiry(junk), isNull, reason: junk);
    }
    // Token bem formado, mas sem `exp`: também não há o que dizer.
    expect(jwtExpiry(_token({'sub': 'u1'})), isNull);
  });

  test('avisa só dentro da janela', () {
    expect(shouldWarnAboutSession(null), isFalse,
        reason: 'sem prazo conhecido, nada a avisar');
    expect(shouldWarnAboutSession(const Duration(hours: 3)), isFalse);
    expect(shouldWarnAboutSession(kSessionWarningWindow), isTrue);
    expect(shouldWarnAboutSession(const Duration(minutes: 1)), isTrue);
    expect(shouldWarnAboutSession(Duration.zero), isTrue);
  });
}
