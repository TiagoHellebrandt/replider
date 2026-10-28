import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// How the Settings sections are drawn.
//
// A seção "Conta" tinha sete retângulos soltos, um por opção, enquanto Canais,
// Team, Service and Compliance grouped everything into one card with dividers.
// Two designs on one screen, with no difference in meaning between them — a
// single screen looked like two, and the extra separation suggested a distinction
// that does not exist.
//
// A section with a single option uses a one-item card, and that is correct. What
// this test forbids is a section spreading its options across several cards.

void main() {
  final src = File(
    'lib/features/settings/presentation/screens/settings_screen.dart',
  ).readAsStringSync();
  final tileSrc = File(
    'lib/features/settings/presentation/widgets/settings_tile.dart',
  ).readAsStringSync();
  final lines = src.split('\n');

  test('nenhuma seção espalha suas opções por vários cartões', () {
    final offenders = <String>[];
    String? current;
    var groups = 0;

    void close() {
      if (current != null && groups > 1) {
        offenders.add('$current ($groups cartões)');
      }
    }

    for (var i = 0; i < lines.length; i++) {
      final line = lines[i];
      if (line.contains('SettingsSectionLabel(')) {
        close();
        current =
            RegExp(r't\.[\w.]+').firstMatch(line)?.group(0) ?? line.trim();
        groups = 0;
      } else if (line.contains('SettingsGroup(children:')) {
        // O cartão de "Sair" fica sozinho de propósito, depois da última
        // section and with no heading of its own: it is destructive, kept apart
        // from the rest. Without this it counts against whatever section
        // happened to be last.
        final ahead = lines.skip(i + 1).take(25).join('\n');
        if (!ahead.contains('logout_confirm_title')) groups++;
      }
    }
    close();

    expect(
      offenders,
      isEmpty,
      reason: 'Estas seções espalham opções por vários cartões, enquanto as '
          'vizinhas agrupam num só com Divider entre as linhas:\n'
          '  ${offenders.join('\n  ')}\n\n'
          'Misturar os dois desenhos faz a mesma tela parecer duas.',
    );
  });

  // Half the headings arrived upper-cased in the JSON and half did not, putting
  // two conventions on one screen — part of the "looks like two screens" that
  // motivated this file. Presentation decides the case, always; the text holds the
  // word as it is written.
  test("the headings' upper case belongs to the widget, not the text", () {
    expect(tileSrc, contains('title.toUpperCase()'),
        reason: 'without this the headings depend on how each string '
            'foi digitada');

    final offenders = <String>[];
    for (final locale in ['pt', 'en']) {
      final json =
          jsonDecode(File('lib/i18n/$locale.i18n.json').readAsStringSync())
              as Map<String, dynamic>;
      final settings = json['settings'] as Map<String, dynamic>;
      settings.forEach((key, value) {
        if (value is! String) return;
        if (!key.startsWith('section_')) return;
        if (value == value.toUpperCase() && value != value.toLowerCase()) {
          offenders.add('$locale.$key = "$value"');
        }
      });
    }
    offenders.sort();

    expect(offenders, isEmpty,
        reason: 'Estes rótulos já vêm gritando do arquivo de tradução:\n'
            '  ${offenders.join('\n  ')}');
  });

  // The specific version above only catches what somebody remembered to list.
  // This one sweeps the whole screen: "Email templates" arrived with no subtitle
  // among seven neighbours that had one, and nobody noticed until someone opened
  // the screen and saw the gap.
  test('no option is left without a subtitle', () {
    final offenders = <String>[];
    // Every ListTile keyed `settings_*_tile` is an option in the list.
    final tiles = RegExp(r"key: const Key\('(settings_\w+_tile)'\)")
        .allMatches(src)
        .toList();

    for (var i = 0; i < tiles.length; i++) {
      final start = tiles[i].start;
      // Up to the start of the next item, or the end of the file for the last.
      final end = i + 1 < tiles.length ? tiles[i + 1].start : src.length;
      final body = src.substring(start, end);
      if (!body.contains('subtitle:')) offenders.add(tiles[i].group(1)!);
    }
    offenders.sort();

    expect(
      offenders,
      isEmpty,
      reason:
          'These options do not say what they are, among neighbours that do '
          '— the empty space on the row reads as a loading defect:\n'
          '  ${offenders.join('\n  ')}',
    );
  });
}
