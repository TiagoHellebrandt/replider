import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';

// A forma de um campo de texto.
//
// Reportado olhando a tela: "tem textfields que estao quadrados, outros
// redondos, nao tem um padrao?". Não tinha. Cada tela escrevia a própria
// borda — 143 delas em ~50 arquivos, com raio 4 (o padrão do Flutter, que sai
// quadrado), 8, 10, 12, 14 e 100. Um campo quadrado ao lado de um arredondado,
// sem nenhuma diferença de significado entre os dois.
//
// Agora existe uma única `inputDecorationTheme`, e estes testes existem para
// que a próxima tela não volte a decidir sozinha.

void main() {
  test('o tema define a forma de todo campo', () {
    final deco = AppTheme.darkTheme.inputDecorationTheme;

    for (final border in [
      deco.border,
      deco.enabledBorder,
      deco.focusedBorder,
      deco.errorBorder,
    ]) {
      expect(border, isA<OutlineInputBorder>(),
          reason: 'sem isto o campo volta ao retângulo padrão do Flutter');
      final radius = (border! as OutlineInputBorder).borderRadius;
      expect(radius, BorderRadius.circular(12),
          reason: '12 é o raio do resto do sistema');
    }
  });

  test('nenhuma tela desenha a própria borda de campo', () {
    final offenders = <String>[];

    for (final f in [
      ...Directory('lib/features').listSync(recursive: true),
      ...Directory('lib/design_system').listSync(recursive: true),
    ].whereType<File>()) {
      final p = f.path;
      if (!p.endsWith('.dart') || p.endsWith('.g.dart')) continue;
      // O tema É o lugar onde a borda pode ser escrita.
      if (p.endsWith('app_theme.dart')) continue;

      final src = f.readAsStringSync();
      if (src.contains('OutlineInputBorder') ||
          src.contains('UnderlineInputBorder')) {
        offenders.add(p.replaceFirst('lib/', ''));
      }
    }
    offenders.sort();

    expect(
      offenders,
      isEmpty,
      reason: 'Estes arquivos desenham a própria borda de campo, em vez de '
          'herdar a do tema:\n'
          '  ${offenders.join('\n  ')}\n\n'
          'Se uma tela precisa mesmo de outra forma, mude o tema — ou '
          'escreva aqui por que aquela tela é diferente.',
    );
  });
}
