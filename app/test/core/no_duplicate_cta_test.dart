import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// Duas telas mostravam DOIS botões para a mesma ação ao mesmo tempo: o do
// estado vazio ("Criar primeiro produto") e o flutuante ("Novo produto"),
// apontando para a mesma rota, um logo acima do outro.
//
// O botão flutuante existe para quem já tem lista e precisa da ação sempre à
// mão. Com a lista vazia, o estado vazio já traz o botão — maior, explicado, no
// centro. Dois botões idênticos não dão escolha; dão dúvida sobre haver
// diferença entre eles.

void main() {
  test('o botão flutuante some quando a lista está vazia', () {
    final offenders = <String>[];

    for (final f in Directory('lib/features')
        .listSync(recursive: true)
        .whereType<File>()) {
      final p = f.path;
      if (!p.endsWith('.dart') || p.endsWith('.g.dart')) continue;
      if (!p.contains('/presentation/screens/')) continue;

      final src = f.readAsStringSync();
      if (!src.contains('floatingActionButton')) continue;
      // Só interessa quem também oferece um "criar o primeiro" no vazio.
      if (!RegExp(r'create_first|ctaLabel:').hasMatch(src)) continue;

      // Incondicional é quando o FAB vem colado no parâmetro. Qualquer
      // guarda (ternário, if, variável) quebra essa adjacência.
      final unconditional =
          RegExp(r'floatingActionButton:\s*FloatingActionButton').hasMatch(src);

      if (unconditional) offenders.add(p.split('/').last);
    }
    offenders.sort();

    expect(
      offenders,
      isEmpty,
      reason: 'Estas telas mostram o botão flutuante E o botão do estado '
          'vazio ao mesmo tempo, para a mesma ação:\n'
          '  ${offenders.join('\n  ')}\n\n'
          'Condicione o flutuante a haver itens na lista.',
    );
  });
}
