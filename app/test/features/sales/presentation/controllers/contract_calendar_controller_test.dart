import 'package:app/features/sales/presentation/controllers/contract_calendar_controller.dart';
import 'package:flutter_test/flutter_test.dart';

// The buying calendar is the only forward-looking read in the product. These
// pin what the screen may claim from the payload — and, just as important,
// what it must refuse to claim when a field never arrived.
void main() {
  group('ContractWindow', () {
    test('reads the clock, the holder and both doors', () {
      final w = ContractWindow.fromJson(<String, dynamic>{
        'pncp_id': 'c1',
        'url': 'https://pncp.gov.br/app/contratos/c1',
        'buyer_name': 'Hospital Santa Rosa',
        'supplier_name': 'Concorrente Ltda',
        'object': 'monitor multiparametrico',
        'value_cents': 42000000,
        'city': 'Cuiabá',
        'uf': 'MT',
        'days_left': 45,
        'price_registry': true,
        'rides_others': true,
      });

      expect(w.daysLeft, 45);
      expect(w.location, 'Cuiabá/MT');
      expect(w.priceRegistry, isTrue);
      expect(w.ridesOthers, isTrue);
      // 45 days is not this week's call; 60 is the line.
      expect(w.isNear, isTrue);
      expect(w.hasClosed, isFalse);
    });

    // A window that closed last week is not history: the buyer is very
    // probably mid-process right now, which is the most reachable moment there
    // is. It stays in the list, lower down.
    test('a closed window is negative, not hidden', () {
      final w = ContractWindow.fromJson(
          <String, dynamic>{'pncp_id': 'c2', 'days_left': -5});
      expect(w.hasClosed, isTrue);
      expect(w.isNear, isFalse);
    });

    test('a window with no published end date claims no clock', () {
      final w = ContractWindow.fromJson(<String, dynamic>{'pncp_id': 'c3'});
      expect(w.daysLeft, isNull);
      expect(w.isNear, isFalse);
      expect(w.hasClosed, isFalse);
    });
  });

  group('ContractCalendar', () {
    // Three very different facts render as zero rows. A screen that cannot
    // tell them apart reports a working feature as broken.
    test('an empty answer carries which kind of empty it is', () {
      final noTerritory = ContractCalendar.fromJson(<String, dynamic>{
        'entries': <dynamic>[],
        'states': <dynamic>[],
        'keywords': <dynamic>[],
      });
      expect(noTerritory.hasTerritory, isFalse);

      final noWords = ContractCalendar.fromJson(<String, dynamic>{
        'entries': <dynamic>[],
        'states': <dynamic>['MT'],
        'keywords': <dynamic>[],
      });
      expect(noWords.hasTerritory, isTrue);
      expect(noWords.hasKeywords, isFalse);

      final scanning = ContractCalendar.fromJson(<String, dynamic>{
        'entries': <dynamic>[],
        'states': <dynamic>['MT'],
        'keywords': <dynamic>['monitor'],
        'scanning': true,
      });
      expect(scanning.hasKeywords, isTrue);
      expect(scanning.scanning, isTrue);
    });

    test('an older server that sends nothing does not crash the screen', () {
      final c = ContractCalendar.fromJson(<String, dynamic>{});
      expect(c.entries, isEmpty);
      expect(c.windowDays, 180);
      expect(c.scanning, isFalse);
    });
  });
}
