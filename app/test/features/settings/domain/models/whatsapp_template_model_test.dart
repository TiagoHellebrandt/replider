import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/settings/domain/models/whatsapp_template_model.dart';
import 'package:app/i18n/strings.g.dart';

void main() {
  setUp(() {
    LocaleSettings.setLocale(AppLocale.pt);
  });

  group('WhatsAppTemplateStatus', () {
    test('fromMeta maps known statuses', () {
      expect(WhatsAppTemplateStatus.fromMeta('APPROVED'),
          WhatsAppTemplateStatus.approved);
      expect(WhatsAppTemplateStatus.fromMeta('PENDING'),
          WhatsAppTemplateStatus.pending);
      expect(WhatsAppTemplateStatus.fromMeta('IN_APPEAL'),
          WhatsAppTemplateStatus.pending);
      expect(WhatsAppTemplateStatus.fromMeta('REJECTED'),
          WhatsAppTemplateStatus.rejected);
      expect(WhatsAppTemplateStatus.fromMeta('PAUSED'),
          WhatsAppTemplateStatus.paused);
      expect(WhatsAppTemplateStatus.fromMeta('DISABLED'),
          WhatsAppTemplateStatus.disabled);
    });

    test('fromMeta falls back to unknown on invalid input', () {
      expect(WhatsAppTemplateStatus.fromMeta(null),
          WhatsAppTemplateStatus.unknown);
      expect(
          WhatsAppTemplateStatus.fromMeta(''), WhatsAppTemplateStatus.unknown);
      expect(WhatsAppTemplateStatus.fromMeta('GARBAGE'),
          WhatsAppTemplateStatus.unknown);
    });

    test('labels are localized', () {
      expect(WhatsAppTemplateStatus.approved.label,
          t.whatsapp.template_status_approved);
      expect(WhatsAppTemplateStatus.pending.label,
          t.whatsapp.template_status_pending);
      expect(WhatsAppTemplateStatus.rejected.label,
          t.whatsapp.template_status_rejected);
    });
  });

  group('WhatsAppTemplateCategory', () {
    test('fromMeta + metaValue roundtrip', () {
      for (final v in ['MARKETING', 'UTILITY', 'AUTHENTICATION']) {
        final cat = WhatsAppTemplateCategory.fromMeta(v);
        expect(cat, isNotNull);
        expect(cat!.metaValue, v);
      }
    });

    test('fromMeta returns null for unknown', () {
      expect(WhatsAppTemplateCategory.fromMeta('FOO'), isNull);
      expect(WhatsAppTemplateCategory.fromMeta(null), isNull);
    });

    test('labels', () {
      expect(WhatsAppTemplateCategory.marketing.label,
          t.settings.wa_category_marketing);
      expect(WhatsAppTemplateCategory.utility.label,
          t.settings.wa_category_utility);
      expect(WhatsAppTemplateCategory.authentication.label,
          t.whatsapp.category_authentication);
    });

    test('metaValue covers all three', () {
      expect(WhatsAppTemplateCategory.marketing.metaValue, 'MARKETING');
      expect(WhatsAppTemplateCategory.utility.metaValue, 'UTILITY');
      expect(
          WhatsAppTemplateCategory.authentication.metaValue, 'AUTHENTICATION');
    });
  });

  group('WhatsAppTemplateStatus all labels', () {
    test('every enum value has a localized label', () {
      for (final s in WhatsAppTemplateStatus.values) {
        expect(s.label, isNotEmpty);
      }
    });

    test('IN_APPEAL and PENDING_DELETION also map to pending', () {
      expect(WhatsAppTemplateStatus.fromMeta('IN_APPEAL'),
          WhatsAppTemplateStatus.pending);
      expect(WhatsAppTemplateStatus.fromMeta('PENDING_DELETION'),
          WhatsAppTemplateStatus.pending);
    });

    test('paused and disabled and unknown labels', () {
      expect(WhatsAppTemplateStatus.paused.label,
          t.whatsapp.template_status_paused);
      expect(WhatsAppTemplateStatus.disabled.label,
          t.whatsapp.template_status_disabled);
      expect(WhatsAppTemplateStatus.unknown.label, '—');
    });
  });

  group('WhatsAppTemplateModel language labels', () {
    test('es/es_ES/es_MX all label as Spanish', () {
      for (final lang in ['es', 'es_ES', 'es_MX']) {
        final tpl = WhatsAppTemplateModel.fromJson(
            {'name': 'x', 'status': 'APPROVED', 'language': lang});
        expect(tpl.languageLabel, t.whatsapp.language_es);
      }
    });

    test('unknown language falls back to raw code', () {
      final tpl = WhatsAppTemplateModel.fromJson(
          {'name': 'x', 'status': 'APPROVED', 'language': 'fr_FR'});
      expect(tpl.languageLabel, 'fr_FR');
    });
  });

  group('WhatsAppTemplateModel.fromJson', () {
    test('parses typical APPROVED template', () {
      final json = {
        'name': 'boas_vindas',
        'status': 'APPROVED',
        'language': 'pt_BR',
        'category': 'UTILITY',
        'components': [
          {'type': 'BODY', 'text': 'Olá {{1}}, bem-vindo!'},
          {'type': 'FOOTER', 'text': 'Responda PARAR para sair'},
        ],
      };

      final tpl = WhatsAppTemplateModel.fromJson(json);
      expect(tpl.name, 'boas_vindas');
      expect(tpl.status, WhatsAppTemplateStatus.approved);
      expect(tpl.language, 'pt_BR');
      expect(tpl.category, WhatsAppTemplateCategory.utility);
      expect(tpl.components, hasLength(2));
      expect(tpl.bodyPreview, 'Olá {{1}}, bem-vindo!');
      expect(tpl.languageLabel, t.whatsapp.language_pt_br);
    });

    test('empty components yields empty body preview', () {
      final tpl = WhatsAppTemplateModel.fromJson(
          {'name': 'x', 'status': 'PENDING', 'language': 'en_US'});
      expect(tpl.bodyPreview, '');
      expect(tpl.languageLabel, t.whatsapp.language_en_us);
      expect(tpl.status, WhatsAppTemplateStatus.pending);
      expect(tpl.category, isNull);
      expect(tpl.rawCategory, isNull);
    });

    test('preserves rawCategory for unknown categories', () {
      final tpl = WhatsAppTemplateModel.fromJson({
        'name': 'x',
        'status': 'APPROVED',
        'language': 'pt_BR',
        'category': 'OTT_AUTHENTICATION',
      });
      expect(tpl.category, isNull);
      expect(tpl.rawCategory, 'OTT_AUTHENTICATION');
    });
  });
}
