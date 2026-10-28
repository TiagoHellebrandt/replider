import 'package:flutter_test/flutter_test.dart';

import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';

// The brief arrives structured (agreed / pending / next step); rows written
// before the structured prompt carry only flowing text and must render
// through the fallback path.
void main() {
  test('a structured payload parses its three fields', () {
    final b = ConversationBriefData.fromJson(const {
      'text': 'Acordado: foto enviada\nPendente: orçamento',
      'agreed': 'foto enviada',
      'pending': 'orçamento',
      'next_step': 'seu: responder o e-mail',
      'generated_at': '2026-08-28T18:00:00Z',
    });

    expect(b.isStructured, isTrue);
    expect(b.agreed, 'foto enviada');
    expect(b.nextStep, 'seu: responder o e-mail');
    expect(b.generatedAt, isNotNull);
  });

  test('a pre-migration row is not structured and keeps its text', () {
    final b = ConversationBriefData.fromJson(const {
      'text': 'negociando desconto, aguarda diretor',
    });

    expect(b.isStructured, isFalse);
    expect(b.text, 'negociando desconto, aguarda diretor');
    expect(b.generatedAt, isNull);
  });
}
