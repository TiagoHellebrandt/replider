import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// An error branch that renders nothing tells the person the feature is absent.
//
// On the settings screen the whole "Meu Perfil" row vanished when the read
// failed — somebody looking for it would conclude the option does not exist. On
// the WhatsApp diagnostics card the usage strip vanished too, making "não
// consegui ler o consumo" identical to "plano sem limite", which is the other
// reason that strip is hidden. And on the channels list the webhook-health
// badge vanished, making "está saudável" identical to "não consegui verificar"
// on the one indicator that warns a number is about to stop receiving.
//
// Disappearing is defensible in a decoration. It is not defensible where the
// blank space is indistinguishable from a real, different answer.

/// Places where an error may render nothing, each with why that is honest.
const _allowedSilent = <String, String>{
  'register_screen.dart':
      'badge opcional de cupom — ausência não se confunde com nada',
  'contact_detail_sheet.dart':
      'bloco acessório da ficha; o painel já mostra o próprio erro',
};

void main() {
  test('an error branch does not render nothing where blank means something',
      () {
    final offenders = <String>[];

    for (final f
        in Directory('lib').listSync(recursive: true).whereType<File>()) {
      final p = f.path;
      if (!p.endsWith('.dart') || p.endsWith('.g.dart')) continue;

      final name = p.split('/').last;
      if (_allowedSilent.containsKey(name)) continue;

      final src = f.readAsStringSync();
      // `error: (_, __) => const SizedBox.shrink()` and its one-underscore
      // sibling: the shape that swallows a failure whole.
      final silent = RegExp(
        r'error:\s*\(_,\s*_+\)\s*=>\s*const\s+SizedBox\.shrink\(\)',
      );
      if (silent.hasMatch(src)) offenders.add(name);
    }
    offenders.sort();

    expect(
      offenders,
      isEmpty,
      reason: 'Estes trocam a falha por espaço em branco, onde o branco se '
          'confunde com uma resposta diferente e verdadeira:\n'
          '  ${offenders.join('\n  ')}\n\n'
          'Mostre o que falhou (nem que seja uma linha com toque para tentar '
          'de novo), ou registre em _allowedSilent explicando por que sumir '
          'não engana ninguém ali.',
    );
  });

  test('an exemption disappears with its file', () {
    final existing = Directory('lib')
        .listSync(recursive: true)
        .whereType<File>()
        .map((f) => f.path.split('/').last)
        .toSet();

    final stale =
        _allowedSilent.keys.where((n) => !existing.contains(n)).toList();

    expect(stale, isEmpty,
        reason: 'Arquivos dispensados que não existem mais: $stale');
  });
}
