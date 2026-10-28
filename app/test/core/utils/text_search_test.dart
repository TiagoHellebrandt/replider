import 'package:app/core/utils/text_search.dart';
import 'package:flutter_test/flutter_test.dart';

// Found on the live board: typing "clínica" returned nothing while a card
// named "Clinica Ficticia" sat in plain sight. Half the text in this product
// was typed by somebody else — a business name imported from Google Places, a
// customer's own WhatsApp profile name — so the accent is a coin flip and a
// literal comparison loses it.

void main() {
  test('folds the accents Portuguese actually uses', () {
    expect(foldForSearch('Clínica'), 'clinica');
    expect(foldForSearch('São João'), 'sao joao');
    expect(foldForSearch('Açúcar'), 'acucar');
    expect(foldForSearch('AVÓ'), 'avo');
    expect(foldForSearch('Münchén'), 'munchen');
  });

  test('leaves everything else alone', () {
    expect(foldForSearch('+55 11 90000-0000'), '+55 11 90000-0000');
    expect(foldForSearch(''), '');
    expect(foldForSearch('Ortopedia 24h'), 'ortopedia 24h');
  });

  test('either spelling finds the other', () {
    const written = 'Clinica Ficticia';
    expect(foldForSearch(written).contains(foldForSearch('clínica')), isTrue);
    expect(foldForSearch('Clínica Fictícia').contains(foldForSearch('clinica')),
        isTrue);
  });
}
