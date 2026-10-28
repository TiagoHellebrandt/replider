import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// Nada é ilimitado, e a tela do cliente não pode dizer que é.
//
// A tela de Plano e Uso anunciava "Armazenamento ilimitado", "Tokens de IA
// ilimitados", "Assentos ilimitados" — e nada disso tinha sido prometido por
// ninguém. Saía de um detalhe interno: um plano sob contrato com a coluna em
// branco resolvia para um sentinela de "sem teto", e a tela traduzia sentinela
// como promessa.
//
// Duas coisas erradas de uma vez. A conta pode encher um disco que não é
// infinito — e aí a conversa é sobre uma promessa escrita na tela. E o cliente
// foi informado de algo que não é verdade, que é o problema mesmo quando dá
// tudo certo.
//
// O servidor agora responde com um número real até no caso não negociado
// (domain.PlatformCeiling*). Este teste guarda o outro lado: que a palavra não
// volte pela porta da tradução.

void main() {
  test('nenhuma frase de cobrança promete algo ilimitado', () {
    final offenders = <String>[];

    for (final locale in ['pt', 'en']) {
      final json =
          jsonDecode(File('lib/i18n/$locale.i18n.json').readAsStringSync())
              as Map<String, dynamic>;
      final billing = json['billing'] as Map<String, dynamic>;

      billing.forEach((key, value) {
        if (value is! String) return;
        final v = value.toLowerCase();
        if (v.contains('ilimitad') || v.contains('unlimited')) {
          offenders.add('$locale.billing.$key = "$value"');
        }
      });
    }
    offenders.sort();

    expect(
      offenders,
      isEmpty,
      reason: 'Estas frases prometem teto infinito na tela de cobrança:\n'
          '  ${offenders.join('\n  ')}\n\n'
          'Disco acaba e token custa. Se o número não pode ser afirmado, a '
          'linha não aparece — mas nunca se promete o que não se pode '
          'entregar.',
    );
  });

  test('a tela de cobrança não tem mais caminho de "ilimitado"', () {
    for (final path in [
      'lib/features/billing/presentation/screens/billing_screen.dart',
      'lib/features/billing/presentation/widgets/upgrade_plan_modal.dart',
    ]) {
      final src = File(path).readAsStringSync();
      expect(src.contains('_unlimited'), isFalse,
          reason: '$path voltou a chamar uma frase de ilimitado');
      expect(src.contains('isPracticallyUnlimited'), isFalse,
          reason: '$path voltou a tratar 999 como "ilimitado" — 999 canais '
              'é 999 canais, e o número diz mais que a palavra');
    }
  });
}
