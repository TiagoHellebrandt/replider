import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// Uma tela que procura um item por id numa lista tem TRÊS estados, não dois:
// carregando, encontrado e ausente. Quem esquece o terceiro trata "a lista
// chegou e o item não está nela" como se ainda estivesse esperando.
//
// Foi o que aconteceu em produção:
//
//   /sales/funnels/id-invalido    → girava para sempre, sem mensagem
//   /sales/products/id-invalido   → abria "Editar produto" com o formulário
//                                   em branco e o botão Salvar ativo
//
// O segundo é o pior: a tela afirma estar editando algo que não existe.

void main() {
  test('quem procura item por id trata o caso de não achar', () {
    final offenders = <String>[];

    for (final f in Directory('lib/features')
        .listSync(recursive: true)
        .whereType<File>()) {
      final p = f.path;
      if (!p.endsWith('.dart') || p.endsWith('.g.dart')) continue;
      if (!p.contains('/presentation/screens/')) continue;

      final src = f.readAsStringSync();

      // Busca de um item por id dentro de uma lista carregada.
      final looksUpById =
          RegExp(r'\.where\(\s*\(?\w+\s+\w+\)?\s*=>\s*\w+\.id\s*==')
              .hasMatch(src);
      if (!looksUpById) continue;

      // Duas formas honestas de tratar a ausência: uma tela de "não
      // encontrado", ou um tratamento inline no próprio ponto de uso
      // (`x.isEmpty ? … : …`), que serve para lookups auxiliares — o nome de
      // um estágio, por exemplo, onde faltar não invalida a tela.
      final hasNotFoundScreen = src.contains('not_found');
      final handlesInline = RegExp(r'\w+\.isEmpty\s*\?').hasMatch(src);

      if (!hasNotFoundScreen && !handlesInline) {
        offenders.add(p.split('/').last);
      }
    }
    offenders.sort();

    expect(
      offenders,
      isEmpty,
      reason: 'Estas telas procuram um item por id e não dizem nada quando a '
          'lista chega sem ele — a pessoa fica olhando um spinner eterno ou '
          'um formulário em branco que promete editar o que não existe:\n'
          '  ${offenders.join('\n  ')}',
    );
  });
}
