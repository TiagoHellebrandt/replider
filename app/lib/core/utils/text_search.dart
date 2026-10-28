/// Text folding for search over Brazilian-Portuguese data.
///
/// Half of what a salesperson searches for was typed by somebody else: a
/// business name imported from Google Places, a city written by whoever
/// filled the form. Accents come and go between those sources, so a search
/// that compares them literally fails on the most ordinary word in the
/// dataset — "clínica" finding nothing while "Clinica Ficticia" sits on the
/// board.
///
/// Mirrors `domain.FoldAccents` on the API side; keep the two in step.
String foldForSearch(String input) {
  final out = StringBuffer();
  for (final rune in input.toLowerCase().runes) {
    out.write(_folded[String.fromCharCode(rune)] ?? String.fromCharCode(rune));
  }
  return out.toString();
}

const _folded = <String, String>{
  'á': 'a',
  'à': 'a',
  'â': 'a',
  'ã': 'a',
  'ä': 'a',
  'é': 'e',
  'è': 'e',
  'ê': 'e',
  'ë': 'e',
  'í': 'i',
  'ì': 'i',
  'î': 'i',
  'ï': 'i',
  'ó': 'o',
  'ò': 'o',
  'ô': 'o',
  'õ': 'o',
  'ö': 'o',
  'ú': 'u',
  'ù': 'u',
  'û': 'u',
  'ü': 'u',
  'ç': 'c',
  'ñ': 'n',
};
