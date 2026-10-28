import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/onboarding/presentation/controllers/onboarding_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// First onboarding step — the company's identity.
///
/// Asks for the business name, which is used both as the account's
/// display name AND as the identity the preview chat uses ("You are the
/// virtual assistant for {company}"), plus the two brand-kit fields the
/// outbound composer reads: what the business does and how it talks.
///
/// Those two live here rather than only in Settings because of what the
/// customer does first: they open a lead, click "Gerar abordagem", and
/// judge the product by that draft. With an empty brand kit the composer
/// has the company's NAME and nothing else — the message it writes is
/// correct and generic, which is the worst possible first impression for
/// a tool sold on writing good ones. One sentence, asked once, changes
/// every draft after it.
///
/// For
/// social-login users (Google/Meta) this is critical: without it the
/// account is named after the person and the preview bot would
/// introduce itself as "Tiago's assistant" instead of "Clínica
/// Sorriso's assistant". For email signup we still ask to confirm —
/// a 3-second step that doubles as a clean aha-moment lead-in.
class StepCompanyName extends ConsumerStatefulWidget {
  final TextEditingController companyNameController;
  final TextEditingController aboutController;

  const StepCompanyName({
    super.key,
    required this.companyNameController,
    required this.aboutController,
  });

  @override
  ConsumerState<StepCompanyName> createState() => _StepCompanyNameState();
}

class _StepCompanyNameState extends ConsumerState<StepCompanyName> {
  /// Owned here, not by the screen: the cities are a step-1 concern only,
  /// and the screen already threads two controllers through for the fields
  /// that outlive the step (name → account, about → brand kit).
  late final TextEditingController _citiesController = TextEditingController(
    text: ref.read(onboardingControllerProvider).territoryCities,
  );

  @override
  void dispose() {
    _citiesController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    final selectedTone =
        ref.watch(onboardingControllerProvider.select((s) => s.toneOfVoice));
    final lines =
        ref.watch(onboardingControllerProvider.select((s) => s.businessLines));
    final uf =
        ref.watch(onboardingControllerProvider.select((s) => s.territoryUF));
    // A resume hydrates the cities after this widget was built; the field
    // must show them without the person retyping.
    ref.listen(onboardingControllerProvider.select((s) => s.territoryCities),
        (_, next) {
      if (_citiesController.text.isEmpty && next.isNotEmpty) {
        _citiesController.text = next;
      }
    });

    final screenHeight = MediaQuery.of(context).size.height;
    final compact = screenHeight < 700;

    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 520),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              SizedBox(height: compact ? 12 : 40),
              Center(
                child: Container(
                  width: compact ? 48 : 72,
                  height: compact ? 48 : 72,
                  decoration: BoxDecoration(
                    color: colors.bluePrimary.withValues(alpha: 0.12),
                    shape: BoxShape.circle,
                  ),
                  child: Icon(Icons.business,
                      size: compact ? 24 : 36, color: colors.bluePrimary),
                ),
              ),
              SizedBox(height: compact ? 10 : 20),
              Text(
                t.onboarding.step_company_title,
                style: typography.display.copyWith(
                  fontSize: compact ? 20 : 24,
                  color: Colors.white,
                  fontWeight: FontWeight.w700,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 8),
              Text(
                t.onboarding.step_company_subtitle,
                style: typography.body.copyWith(
                  color: colors.textSecondary,
                  fontSize: 14,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 4),
              // Which fields gate the Next button. All four are required and
              // NONE used to say so — the button just sat grey with no hint
              // of what was missing.
              Text(
                t.onboarding.company_required_hint,
                key: const Key('onboarding_required_hint'),
                style: typography.body
                    .copyWith(color: colors.textMuted, fontSize: 12),
                textAlign: TextAlign.center,
              ),
              SizedBox(height: compact ? 20 : 32),
              TextField(
                controller: widget.companyNameController,
                autofocus: true,
                textCapitalization: TextCapitalization.words,
                style: TextStyle(color: colors.textPrimary, fontSize: 16),
                onChanged: (v) => ref
                    .read(onboardingControllerProvider.notifier)
                    .updateField('accountName', v),
                decoration: InputDecoration(
                  labelText: '${t.onboarding.company_name_hint} *',
                  labelStyle: TextStyle(color: colors.textMuted),
                  hintText: t.onboarding.step_company_placeholder,
                  hintStyle:
                      TextStyle(color: colors.textDisabled, fontSize: 13),
                  prefixIcon:
                      Icon(Icons.storefront_outlined, color: colors.textMuted),
                ),
              ),
              const SizedBox(height: 16),
              TextField(
                key: const Key('onboarding_about_field'),
                controller: widget.aboutController,
                minLines: 2,
                maxLines: 3,
                style: TextStyle(color: colors.textPrimary, fontSize: 15),
                onChanged: (v) => ref
                    .read(onboardingControllerProvider.notifier)
                    .updateField('about', v),
                decoration: InputDecoration(
                  labelText: '${t.onboarding.company_about_label} *',
                  labelStyle: TextStyle(color: colors.textMuted),
                  hintText: t.onboarding.company_about_placeholder,
                  hintStyle:
                      TextStyle(color: colors.textDisabled, fontSize: 13),
                  helperText: t.onboarding.company_about_help,
                  helperStyle: TextStyle(color: colors.textMuted, fontSize: 12),
                ),
              ),
              SizedBox(height: compact ? 16 : 24),

              // WHAT SHE DOES. Three ways a company earns from equipment,
              // and a distributor is often all three. Chips, not a
              // dropdown: it is a set, and the three labels ARE the
              // explanation.
              Text(
                '${t.onboarding.company_lines_label} *',
                style: typography.body
                    .copyWith(color: colors.textSecondary, fontSize: 13),
              ),
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  for (final line in _businessLines)
                    FilterChip(
                      key: Key('onboarding_line_${line.key}'),
                      label: Text(line.label),
                      selected: lines.contains(line.key),
                      onSelected: (on) {
                        final next = Set<String>.of(lines);
                        if (on) {
                          next.add(line.key);
                        } else {
                          next.remove(line.key);
                        }
                        ref
                            .read(onboardingControllerProvider.notifier)
                            .updateField('businessLines', next);
                      },
                    ),
                ],
              ),
              SizedBox(height: compact ? 16 : 24),

              // WHERE SHE SELLS. The state is required — it is what the
              // search of hospitals keys on — the cities narrow it and are
              // optional. Typed, not picked: a list of every municipality
              // in a state is a scroll nobody finishes.
              Text(
                '${t.onboarding.company_territory_label} *',
                style: typography.body
                    .copyWith(color: colors.textSecondary, fontSize: 13),
              ),
              const SizedBox(height: 8),
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                    width: 110,
                    // A plain DropdownButton, not the FormField: the value
                    // is controlled by the state (a resume hydrates it after
                    // first build), and the FormField only honours its
                    // initial value.
                    child: InputDecorator(
                      decoration: InputDecoration(
                        labelText: t.onboarding.company_territory_uf,
                        labelStyle: TextStyle(color: colors.textMuted),
                      ),
                      child: DropdownButtonHideUnderline(
                        child: DropdownButton<String>(
                          key: const Key('onboarding_territory_uf'),
                          value: uf.isEmpty ? null : uf,
                          isExpanded: true,
                          isDense: true,
                          dropdownColor: colors.backgroundSecondary,
                          style: TextStyle(
                              color: colors.textPrimary, fontSize: 15),
                          items: [
                            for (final code in kBrazilianStates)
                              DropdownMenuItem(value: code, child: Text(code)),
                          ],
                          onChanged: (v) => ref
                              .read(onboardingControllerProvider.notifier)
                              .updateField('territoryUF', v ?? ''),
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: TextField(
                      key: const Key('onboarding_territory_cities'),
                      controller: _citiesController,
                      textCapitalization: TextCapitalization.words,
                      style: TextStyle(color: colors.textPrimary, fontSize: 15),
                      onChanged: (v) => ref
                          .read(onboardingControllerProvider.notifier)
                          .updateField('territoryCities', v),
                      decoration: InputDecoration(
                        labelText: t.onboarding.company_territory_cities,
                        labelStyle: TextStyle(color: colors.textMuted),
                        hintText: t.onboarding.company_territory_cities_hint,
                        hintStyle:
                            TextStyle(color: colors.textDisabled, fontSize: 13),
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 20),
              Text(
                t.onboarding.company_tone_label,
                style: typography.body
                    .copyWith(color: colors.textSecondary, fontSize: 13),
              ),
              const SizedBox(height: 8),
              // Presets, not a text box: the value goes verbatim into the
              // prompt, so it has to be prose — and asking someone to
              // write prose about their own tone of voice is how a field
              // gets left blank.
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  for (final tone in _tones)
                    ChoiceChip(
                      key: Key('onboarding_tone_${tone.key}'),
                      label: Text(tone.label),
                      selected: selectedTone == tone.prose,
                      onSelected: (on) => ref
                          .read(onboardingControllerProvider.notifier)
                          .updateField('toneOfVoice', on ? tone.prose : ''),
                    ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// A tone preset: the chip's label and the sentence it stores.
class _Tone {
  const _Tone(this.key, this.label, this.prose);
  final String key;
  final String label;
  final String prose;
}

List<_Tone> get _tones => [
      _Tone('direct', t.onboarding.company_tone_direct,
          t.onboarding.company_tone_direct_prose),
      _Tone('consultative', t.onboarding.company_tone_consultative,
          t.onboarding.company_tone_consultative_prose),
      _Tone('warm', t.onboarding.company_tone_warm,
          t.onboarding.company_tone_warm_prose),
      _Tone('formal', t.onboarding.company_tone_formal,
          t.onboarding.company_tone_formal_prose),
    ];

/// One way a company earns from the equipment it handles: the chip's label
/// and the value the API stores (`accounts.business_lines`).
class _BusinessLine {
  const _BusinessLine(this.key, this.label);
  final String key;
  final String label;
}

List<_BusinessLine> get _businessLines => [
      _BusinessLine('sell', t.onboarding.company_line_sell),
      _BusinessLine('rent', t.onboarding.company_line_rent),
      _BusinessLine('service', t.onboarding.company_line_service),
    ];

/// The 26 states plus the Distrito Federal, in the order people expect.
const kBrazilianStates = [
  'AC', 'AL', 'AP', 'AM', 'BA', 'CE', 'DF', 'ES', 'GO', 'MA', 'MT', 'MS',
  'MG', //
  'PA', 'PB', 'PR', 'PE', 'PI', 'RJ', 'RN', 'RS', 'RO', 'RR', 'SC', 'SP',
  'SE', //
  'TO',
];
