import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// A live button that swallows the click.
//
// The pattern showed up FIVE times in one sweep: "Approve and send" with an
// empty approach, "Save" a date with no date, "Record sale" with no amount,
// Signals' Save with no keyword, and the commission rule with no name. Always
// the same shape:
//
//   onPressed: _busy ? null : _save,             // aceso
//   ...
//   Future<void> _save() async {
//     final text = _campo.text.trim();
//     if (text.isEmpty) return;                  // and does nothing
//
// Whoever clicks gets no answer at all — or gets a round-trip to the server to
// hear what the screen already knew. Both read as broken, and in one of them the
// button was the highest-consequence one in the product (approving a send).
//
// The rule: if the handler gives up because a TEXT FIELD is empty, the button
// that calls it has to know that before the click.
//
// Deliberately limited to text fields: a handler that returns because the person
// cancelled a file picker is not a dead button — the click did exactly what it
// promised, and the person is the one who backed out.

void main() {
  test('no button stays live over a handler that gives up silently', () {
    final offenders = <String>[];

    for (final f in Directory('lib/features')
        .listSync(recursive: true)
        .whereType<File>()) {
      final p = f.path;
      if (!p.endsWith('.dart') || p.endsWith('.g.dart')) continue;
      if (!p.contains('/presentation/')) continue;

      final src = f.readAsStringSync();

      // 1. Handlers that give up because a text field is empty.
      final bailing = <String>{};
      // `Future<void> _x() async {` e `void _x() {` — o segundo escapava,
      // and that is how the company-identity "Add" slipped through.
      final handler = RegExp(
          r'(?:Future<[^>]*>|void)\s+(_\w+)\([^)]*\)\s*(?:async\s*)?\{([\s\S]*?)\n  \}');
      for (final m in handler.allMatches(src)) {
        final body = m.group(2)!;

        // Locals that came from a controller: `final text = _x.text.trim();`
        final fromField = <String>{};
        for (final a
            in RegExp(r'(?:final|var)\s+(\w+)\s*=\s*[^;]*\.text\b[^;]*;')
                .allMatches(body)) {
          fromField.add(a.group(1)!);
        }

        // Where the handler opens something. Giving up AFTER opening a dialog is
        // not a dead click: the click did what it promised, and whoever closed
        // without filling it in is the one who backed out — the dialog's own
        // button is the one that has to know, and that one is checked on its
        // own.
        final opensUI =
            RegExp(r'show(Dialog|ModalBottomSheet|AppDialog|ConfirmDialog)')
                    .firstMatch(body)
                    ?.start ??
                body.length;

        for (final b
            in RegExp(r'if\s*\(\s*([\w.()]+)\.isEmpty\s*\)\s*(\{\s*)?return;')
                .allMatches(body)) {
          if (b.start > opensUI) continue;
          final subject = b.group(1)!;
          final root = subject.split('.').first;
          if (subject.contains('.text') || fromField.contains(root)) {
            bailing.add(m.group(1)!);
          }
        }
      }
      if (bailing.isEmpty) continue;

      // 2. Buttons that call them without looking at the same emptiness. Any
      // mention of "empty" in the guard counts — `_busy || empty`,
      // `text.isNotEmpty`, or a getter whose name says it knows (`_canSave`).
      for (final name in bailing) {
        final call = RegExp('onPressed:([^,\n]*\n?[^,]*?)$name\\b');
        for (final m in call.allMatches(src)) {
          // `SnackBarAction.onPressed` is not nullable: there is no dead version
          // of that button. The notice disappears on its own in seconds and the
          // text it re-offers has just been handed back to the field, so the
          // empty case only happens if the person clears it first — and
          // resurrecting what they cleared would be worse than doing nothing.
          final before =
              src.substring(m.start - 300 < 0 ? 0 : m.start - 300, m.start);
          if (before.contains('SnackBarAction(')) continue;

          final guard = m.group(1)!.toLowerCase();
          // "empty" covers `isEmpty`/`isNotEmpty`/`empty`; a getter whose name
          // claims to know (`_canSave`, `_codeValid`) counts as the author
          // saying the button already looks at this.
          if (guard.contains('empty') ||
              guard.contains('valid') ||
              RegExp(r'_can\w+').hasMatch(guard)) {
            continue;
          }
          offenders.add('${p.split('/').last} → $name');
        }
      }
    }
    offenders.sort();

    expect(
      offenders,
      isEmpty,
      reason:
          'Estes botões aceitam o clique e o handler volta sem fazer nada:\n'
          '  ${offenders.join('\n  ')}\n\n'
          'Condicione o onPressed ao mesmo vazio que o handler checa, ou diga '
          'na tela por que o botão não serve agora.',
    );
  });
}
