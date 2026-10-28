import 'package:flutter_test/flutter_test.dart';
import 'package:app/core/utils/currency_format.dart';

void main() {
  group('formatCurrencyCents', () {
    test('BRL uses the R\$ symbol and a decimal comma', () {
      expect(formatCurrencyCents(19900, 'brl'), 'R\$ 199,00');
      expect(formatCurrencyCents(4990, 'BRL'), 'R\$ 49,90');
      expect(formatCurrencyCents(70, 'brl'), 'R\$ 0,70');
    });

    test('USD uses the \$ symbol and a decimal point', () {
      expect(formatCurrencyCents(3900, 'usd'), '\$39.00');
      expect(formatCurrencyCents(19900, 'USD'), '\$199.00');
      expect(formatCurrencyCents(70, 'usd'), '\$0.70');
    });

    test('thousands are grouped, because a payslip is read by a person', () {
      expect(formatCurrencyCents(423000, 'brl'), 'R\$ 4.230,00');
      expect(formatCurrencyCents(100000000, 'brl'), 'R\$ 1.000.000,00');
      expect(formatCurrencyCents(423000, 'usd'), '\$4,230.00');
    });

    test('a negative amount signs before the symbol, not after it', () {
      // A deduction, not a currency called "minus reais".
      expect(formatCurrencyCents(-20000, 'brl'), '-R\$ 200,00');
      expect(formatCurrencyCents(-20050, 'brl'), '-R\$ 200,50');
      expect(formatCurrencyCents(-3900, 'usd'), '-\$39.00');
    });

    test('zero and sub-unit cents pad to two decimals', () {
      expect(formatCurrencyCents(0, 'brl'), 'R\$ 0,00');
      expect(formatCurrencyCents(5, 'usd'), '\$0.05');
    });
  });
}
