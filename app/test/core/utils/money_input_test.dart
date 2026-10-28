import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/utils/money_input.dart';

// Five screens converted typed money with
//   raw.replaceAll('.', '').replaceAll(',', '.')
// which destroys the US notation: 1,234.56 lands as 1.23456, so a sale of
// $1,234.56 was stored as $1.23. A thousandfold error, silent, on the figure
// commissions are calculated from.
void main() {
  group('the same amount, either notation', () {
    test('Brazilian and US notation agree', () {
      expect(parseMoneyToCents('1.234,56'), 123456);
      expect(parseMoneyToCents('1,234.56'), 123456);
    });

    test('millions, either notation', () {
      expect(parseMoneyToCents('1.234.567,89'), 123456789);
      expect(parseMoneyToCents('1,234,567.89'), 123456789);
    });

    // The regression itself: this is the input that used to become 123.
    test('a US amount is not divided by a thousand', () {
      expect(parseMoneyToCents('1,234.56'), isNot(123));
      expect(parseMoneyToCents('1,234.56'), 123456);
    });
  });

  group('one separator, no other to disambiguate', () {
    // 1.234 and 1,234 are one thousand two hundred and thirty-four in BOTH
    // notations. Reading either as a decimal would turn 1,234 into 1.23 in the
    // one case where the two locales already agree.
    test('three trailing digits group thousands', () {
      expect(parseMoneyToCents('1.234'), 123400);
      expect(parseMoneyToCents('1,234'), 123400);
    });

    test('one or two trailing digits are a fraction', () {
      expect(parseMoneyToCents('1,5'), 150);
      expect(parseMoneyToCents('1.5'), 150);
      expect(parseMoneyToCents('1234,56'), 123456);
      expect(parseMoneyToCents('1234.56'), 123456);
    });

    test('a repeated separator always groups', () {
      expect(parseMoneyToCents('1.234.567'), 123456700);
      expect(parseMoneyToCents('1,234,567'), 123456700);
    });

    // A separator with nothing before it can only be a decimal point: ",56" is
    // how people type half a real when the field is already understood.
    test('a leading separator is decimal', () {
      expect(parseMoneyToCents(',56'), 56);
      expect(parseMoneyToCents('.56'), 56);
    });
  });

  group('what people actually paste', () {
    test('currency symbols and codes are noise', () {
      expect(parseMoneyToCents(r'R$ 1.234,56'), 123456);
      expect(parseMoneyToCents(r'$1,234.56'), 123456);
      expect(parseMoneyToCents('1234.56 USD'), 123456);
    });

    test('grouping spaces are grouping', () {
      expect(parseMoneyToCents('1 234,56'), 123456);
      expect(parseMoneyToCents('1 234,56'), 123456);
    });

    test('a plain integer is whole units', () {
      expect(parseMoneyToCents('1234'), 123400);
      expect(parseMoneyToCents('0'), 0);
    });
  });

  group('edges', () {
    test('nothing readable is zero, never a throw', () {
      for (final input in ['', '   ', 'abc', r'R$', '-', ',', '.']) {
        expect(parseMoneyToCents(input), 0, reason: 'input: "$input"');
      }
    });

    // Commission adjustments are legitimately negative — a clawback is a real
    // row in the ledger.
    test('a minus sign survives', () {
      expect(parseMoneyToCents('-1.234,56'), -123456);
      expect(parseMoneyToCents('-1,234.56'), -123456);
      expect(parseMoneyToCents(r'-R$ 200,00'), -20000);
    });

    // More precision than cents rounds rather than truncates: 1234.995 is
    // 1,235.00 to everyone reading it.
    test('more than two decimals round', () {
      expect(parseMoneyToCents('1234.995'), 123500);
      expect(parseMoneyToCents('1234,994'), 123499);
      expect(parseMoneyToCents('0.005'), 1);
    });
  });

  group('round trip through the field', () {
    test('BRL writes a decimal comma, others a point', () {
      expect(formatCentsForInput(123456, 'brl'), '1234,56');
      expect(formatCentsForInput(123456, 'usd'), '1234.56');
      expect(formatCentsForInput(-20000, 'brl'), '-200,00');
      expect(formatCentsForInput(5, 'usd'), '0.05');
    });

    // What a form does when it loads a stored amount and saves it untouched.
    test('formatting then parsing returns the same cents', () {
      for (final cents in [0, 5, 99, 100, 123456, 999999999, -4200]) {
        for (final currency in ['brl', 'usd']) {
          expect(
            parseMoneyToCents(formatCentsForInput(cents, currency)),
            cents,
            reason: '$cents in $currency',
          );
        }
      }
    });
  });
}
