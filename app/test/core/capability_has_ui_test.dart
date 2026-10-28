import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// A method on a controller that no screen ever calls is a capability with no
// door: the server has the endpoint, the client has the code, and nobody using
// the product can reach it.
//
// The signals screen shipped that way — it told people "diga quais palavras
// acompanhar" while `saveWatch()` sat in the controller with no caller but a
// test double. Seven more are listed below, each with a live endpoint behind
// it.
//
// This test does not demand they all be built. It demands the list not grow
// silently: a NEW unreachable method fails, and every accepted one is written
// down with what the customer cannot do because of it.

/// Controller methods with no UI, accepted as known debt on 31/07/2026.
///
/// Removing an entry means the screen was built. Adding one is a decision to
/// ship a capability nobody can reach — write what it costs the customer.
const _knownDebt = <String, String>{
  'deleteWatch':
      'DELETE /signal-watches/:id existe — não há como parar de monitorar uma '
          'palavra',
  // Internos: chamados por outros controllers ou pela camada de rede, não por
  // tela. Não são capacidades de cliente.
  'updateUserInState': 'interno — sincroniza estado após login/refresh',
  'setTyping': 'interno — indicador de digitação via WebSocket',
  'setAlert': 'interno — alimentado por evento de WebSocket',
  'setStatus': 'interno — painel owner, acionado por outro controller',
};

final _ignoredNames = {
  'build',
  'toString',
  'copyWith',
  'fromJson',
  'toJson',
  'dispose',
  'noSuchMethod',
  'hashCode',
};

/// Every method declared on a controller: método -> arquivo.
Map<String, String> _declaredControllerMethods() {
  final declared = <String, String>{};
  final controllers = Directory('lib/features')
      .listSync(recursive: true)
      .whereType<File>()
      .where((f) =>
          f.path.contains('/presentation/controllers/') &&
          f.path.endsWith('.dart') &&
          !f.path.endsWith('.g.dart'));

  for (final file in controllers) {
    final src = file.readAsStringSync();
    final re = RegExp(
        r'^\s{2}(?:Future<[^>]*>|void|[A-Z]\w*)\s+([a-z]\w+)\s*\(',
        multiLine: true);
    for (final m in re.allMatches(src)) {
      final name = m.group(1)!;
      if (_ignoredNames.contains(name)) continue;
      declared[name] = file.path.split('/').last;
    }
  }
  return declared;
}

/// Everything that is not a controller: telas, widgets, rotas, serviços.
String _callerSources() {
  final buffer = StringBuffer();
  for (final f
      in Directory('lib').listSync(recursive: true).whereType<File>()) {
    final p = f.path;
    if (!p.endsWith('.dart') || p.endsWith('.g.dart')) continue;
    if (p.contains('/presentation/controllers/')) continue;
    buffer.write(f.readAsStringSync());
  }
  return buffer.toString();
}

void main() {
  test('a controller method that no screen calls is declared, not silent', () {
    final declared = _declaredControllerMethods();
    final callers = _callerSources();

    final unreachable = <String>[];
    declared.forEach((name, file) {
      // Conta chamada normal e tear-off (`onChanged: c.setX`).
      if (!RegExp(r'\.' + RegExp.escape(name) + r'\b').hasMatch(callers)) {
        if (!_knownDebt.containsKey(name)) unreachable.add('$name ($file)');
      }
    });
    unreachable.sort();

    expect(
      unreachable,
      isEmpty,
      reason: 'Estes métodos de controller não são chamados por nenhuma tela '
          'ou widget — o servidor pode até ter o endpoint, mas ninguém usando '
          'o produto alcança:\n  ${unreachable.join('\n  ')}\n\n'
          'Construa a interface, apague o método, ou — se for mesmo interno — '
          'registre em _knownDebt dizendo o que custa ao cliente.',
    );
  });

  test('a debt entry disappears when the screen gets built — or the method',
      () {
    // Sem isto a lista vira papel velho: uma entrada continuaria "justificando"
    // um método que já ganhou tela, escondendo o próximo. E uma entrada cujo
    // método já foi apagado (quatro ficaram órfãs depois do corte de
    // 2026-08-18) é dívida inventada — some junto com o método.
    final declared = _declaredControllerMethods();
    final callers = _callerSources();

    final resolved = _knownDebt.keys
        .where(
            (n) => RegExp(r'\.' + RegExp.escape(n) + r'\b').hasMatch(callers))
        .toList();
    final orphans =
        _knownDebt.keys.where((n) => !declared.containsKey(n)).toList();

    expect(resolved, isEmpty,
        reason: 'Estes já têm interface e devem sair de _knownDebt: $resolved');
    expect(orphans, isEmpty,
        reason: 'Estes métodos não existem em controller nenhum e devem sair '
            'de _knownDebt: $orphans');
  });
}
