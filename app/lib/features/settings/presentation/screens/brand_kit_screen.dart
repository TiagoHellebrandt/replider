import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/components/settings_section_card.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/settings/presentation/controllers/brand_kit_controller.dart';
import 'package:app/features/sales/presentation/controllers/products_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Who the company is, and what it may never claim.
///
/// The forbidden list is the reason this screen exists. In healthcare a
/// sentence like "approved by the regulator" on a device that is not is a
/// regulatory problem, not a style preference — and the salesperson writing it
/// usually does not know that. Writing the rule down once, at company level,
/// is what stops every seller from having to.
///
/// The company NAME is deliberately not editable here: it lives on the
/// account settings screen, and two doors to the same fact is how the two
/// copies diverge. This screen carries everything else the kit holds.
class BrandKitScreen extends ConsumerWidget {
  const BrandKitScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final kit = ref.watch(brandKitProvider);
    final isAdmin =
        ref.watch(authControllerProvider).asData?.value?.isAdmin ?? false;

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        // A deep link outside the shell: without this the screen has neither a
        // back button nor a menu — you get in and only get out by editing the
        // URL.
        leading: IconButton(
          tooltip: t.common.back,
          icon: const Icon(Icons.arrow_back),
          onPressed: () =>
              context.canPop() ? context.pop() : context.go('/settings'),
        ),
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        title: Text(
          t.brand.title,
          style: TextStyle(
            fontSize: 18,
            fontWeight: FontWeight.w700,
            color: colors.textPrimary,
          ),
        ),
      ),
      body: kit.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => RetryableError(
          error: err,
          onRetry: () => ref.invalidate(brandKitProvider),
        ),
        data: (data) => _BrandForm(kit: data, canEdit: isAdmin),
      ),
    );
  }
}

class _BrandForm extends ConsumerStatefulWidget {
  const _BrandForm({required this.kit, required this.canEdit});

  final BrandKit kit;

  /// A salesperson READS this screen — they need to know the rules they are
  /// held to — but only an admin changes what the business may claim.
  final bool canEdit;

  @override
  ConsumerState<_BrandForm> createState() => _BrandFormState();
}

class _BrandFormState extends ConsumerState<_BrandForm> {
  late final _about = TextEditingController(text: widget.kit.about)
    ..addListener(_recompute);
  late final _tone = TextEditingController(text: widget.kit.toneOfVoice)
    ..addListener(_recompute);
  late final _signature = TextEditingController(text: widget.kit.signature)
    ..addListener(_recompute);
  late List<String> _allowed = [...widget.kit.allowedClaims];

  /// The account's "why ours" library. Its own endpoint (not part of the brand
  /// kit payload), loaded once and saved with the same button — two round
  /// trips are an implementation detail, and a second Save would be a second
  /// thing to forget.
  List<String>? _differentiators;
  List<String> _differentiatorsLoaded = const [];

  @override
  void initState() {
    super.initState();
    _loadDifferentiators();
  }

  /// Loaded once, from its own endpoint. A failure leaves the section
  /// disabled rather than showing an empty list somebody would then "fix" by
  /// saving over the library they cannot see.
  Future<void> _loadDifferentiators() async {
    try {
      final res =
          await ref.read(salesRemoteDataSourceProvider).getDifferentiators();
      final data = res.data;
      final items = data is Map ? data['items'] : null;
      final list = <String>[
        if (items is List)
          for (final it in items)
            if (it is Map && (it['label'] as String? ?? '').trim().isNotEmpty)
              (it['label'] as String).trim(),
      ];
      if (!mounted) return;
      setState(() {
        _differentiators = list;
        _differentiatorsLoaded = List<String>.from(list);
      });
    } catch (_) {
      // Left null: the section stays read-only instead of inviting an edit
      // that would overwrite what did not load.
    }
  }

  late List<String> _forbidden = [...widget.kit.forbiddenClaims];
  bool _saving = false;
  bool _dirty = false;

  @override
  void dispose() {
    _about.dispose();
    _tone.dispose();
    _signature.dispose();
    super.dispose();
  }

  void _recompute() {
    final k = widget.kit;
    final dirty = _about.text.trim() != k.about ||
        _tone.text.trim() != k.toneOfVoice ||
        _signature.text.trim() != k.signature ||
        !_sameList(_allowed, k.allowedClaims) ||
        !_sameList(_forbidden, k.forbiddenClaims) ||
        !_sameList(
            _differentiators ?? _differentiatorsLoaded, _differentiatorsLoaded);
    if (dirty != _dirty) setState(() => _dirty = dirty);
  }

  static bool _sameList(List<String> a, List<String> b) {
    if (a.length != b.length) return false;
    for (var i = 0; i < a.length; i++) {
      if (a[i] != b[i]) return false;
    }
    return true;
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return SettingsPageBody(
      children: [
        SettingsSectionCard(
          title: t.brand.identity_section,
          icon: Icons.business,
          subtitle: t.brand.subtitle,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              _field(_about, t.brand.about, t.brand.about_hint, colors,
                  lines: 3),
              const SizedBox(height: 12),
              _field(_tone, t.brand.tone, t.brand.tone_hint, colors, lines: 2),
              const SizedBox(height: 12),
              _field(_signature, t.brand.signature, null, colors, lines: 2),
            ],
          ),
        ),
        const SizedBox(height: 24),
        SettingsSectionCard(
          title: t.brand.allowed,
          icon: Icons.check_circle_outline,
          subtitle: t.brand.allowed_hint,
          child: _ClaimList(
            key: const Key('brand_allowed'),
            items: _allowed,
            enabled: widget.canEdit,
            onChanged: (v) {
              setState(() => _allowed = v);
              _recompute();
            },
          ),
        ),
        const SizedBox(height: 24),
        // Why yours rather than the one they already own — the sentence the
        // draft had no source for, which is what made every proposal generic.
        // Below the allowed claims because it is a subset of them in spirit:
        // things the company stands behind.
        SettingsSectionCard(
          title: t.brand.differentiators,
          icon: Icons.workspace_premium_outlined,
          subtitle: t.brand.differentiators_hint,
          child: _ClaimList(
            key: const Key('brand_differentiators'),
            items: _differentiators ?? const [],
            enabled: widget.canEdit && _differentiators != null,
            onChanged: (v) {
              setState(() => _differentiators = v);
              _recompute();
            },
          ),
        ),
        const SizedBox(height: 24),
        // Rendered in the alert colour, and last among the lists, because it
        // is the one list with a consequence outside the conversation.
        SettingsSectionCard(
          title: t.brand.forbidden,
          icon: Icons.block,
          accent: colors.redAlert,
          subtitle: t.brand.forbidden_hint,
          child: _ClaimList(
            key: const Key('brand_forbidden'),
            items: _forbidden,
            enabled: widget.canEdit,
            accent: colors.redAlert,
            onChanged: (v) {
              setState(() => _forbidden = v);
              _recompute();
            },
          ),
        ),
        if (widget.canEdit) ...[
          const SizedBox(height: 16),
          Align(
            alignment: Alignment.centerRight,
            child: ElevatedButton(
              key: const Key('brand_save'),
              onPressed: (!_dirty || _saving) ? null : _save,
              style: ElevatedButton.styleFrom(
                backgroundColor: colors.bluePrimary,
                foregroundColor: Colors.white,
                padding:
                    const EdgeInsets.symmetric(horizontal: 24, vertical: 12),
              ),
              child: _saving
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(
                          strokeWidth: 2, color: Colors.white),
                    )
                  : Text(t.common.save),
            ),
          ),
        ],
        const SizedBox(height: 32),
        const _SellerOverlaySection(),
      ],
    );
  }

  Widget _field(
    TextEditingController c,
    String label,
    String? hint,
    AppColorsExtension colors, {
    int lines = 1,
  }) =>
      TextField(
        controller: c,
        enabled: widget.canEdit,
        maxLines: lines,
        style: TextStyle(color: colors.textPrimary),
        decoration: InputDecoration(
          labelText: label,
          hintText: hint,
          hintStyle: TextStyle(color: colors.textDisabled, fontSize: 12),
        ),
      );

  Future<void> _save() async {
    setState(() => _saving = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref.read(brandActionsProvider).saveKit(BrandKit(
            // Not edited here — the account screen owns the name; the kit
            // keeps carrying whatever it already had.
            companyName: widget.kit.companyName,
            about: _about.text.trim(),
            toneOfVoice: _tone.text.trim(),
            signature: _signature.text.trim(),
            allowedClaims: _allowed,
            forbiddenClaims: _forbidden,
          ));
      // The differentiators live behind their own endpoint. Saved with the
      // same button because two round trips are an implementation detail, and
      // a second Save button is a second thing to forget.
      final diffs = _differentiators;
      if (diffs != null && !_sameList(diffs, _differentiatorsLoaded)) {
        await ref.read(salesRemoteDataSourceProvider).saveDifferentiators({
          'items': [
            for (var i = 0; i < diffs.length; i++)
              {'label': diffs[i], 'position': i},
          ],
        });
        _differentiatorsLoaded = List<String>.from(diffs);
      }
      ref.invalidate(brandKitProvider);
      messenger.showSnackBar(SnackBar(content: Text(t.brand.saved)));
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: const Key('brand_save_error'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }
}

/// A list of short phrases, added one at a time.
///
/// A free-text box would be quicker to build and wrong: these are matched
/// literally against every draft, so "cure, approved by the regulator" typed as
/// one line would become a single rule that matches neither. The surrounding
/// [SettingsSectionCard] carries the title and hint.
class _ClaimList extends StatefulWidget {
  const _ClaimList({
    super.key,
    required this.items,
    required this.enabled,
    required this.onChanged,
    this.accent,
  });

  final List<String> items;
  final bool enabled;
  final Color? accent;
  final ValueChanged<List<String>> onChanged;

  @override
  State<_ClaimList> createState() => _ClaimListState();
}

class _ClaimListState extends State<_ClaimList> {
  final _input = TextEditingController();

  @override
  void dispose() {
    _input.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final accent = widget.accent ?? colors.textSecondary;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            for (final item in widget.items)
              Chip(
                label: Text(item),
                backgroundColor: accent.withValues(alpha: 0.12),
                side: BorderSide(color: accent.withValues(alpha: 0.4)),
                onDeleted: widget.enabled ? () => _remove(item) : null,
              ),
          ],
        ),
        if (widget.enabled) ...[
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _input,
                  style: TextStyle(color: colors.textPrimary),
                  // The button beside it follows what is typed.
                  onChanged: (_) => setState(() {}),
                  onSubmitted: (_) => _add(),
                  decoration: const InputDecoration(
                    isDense: true,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              // `_add` recusa vazio e repetido, e recusava calado:
              // clicking did nothing and said nothing about why.
              OutlinedButton(
                onPressed: _canAdd ? _add : null,
                child: Text(t.brand.add),
              ),
            ],
          ),
        ],
      ],
    );
  }

  /// What `_add` accepts: something written, and not already in the list.
  bool get _canAdd {
    final v = _input.text.trim();
    return v.isNotEmpty && !widget.items.contains(v);
  }

  void _add() {
    final v = _input.text.trim();
    // A blank rule would match every text ever written; a duplicate would
    // report the same violation twice.
    if (v.isEmpty || widget.items.contains(v)) return;
    widget.onChanged([...widget.items, v]);
    _input.clear();
  }

  void _remove(String item) =>
      widget.onChanged(widget.items.where((i) => i != item).toList());
}

/// The seller's own layer: adds to the company's, never replaces it.
class _SellerOverlaySection extends ConsumerStatefulWidget {
  const _SellerOverlaySection();

  @override
  ConsumerState<_SellerOverlaySection> createState() =>
      _SellerOverlaySectionState();
}

class _SellerOverlaySectionState extends ConsumerState<_SellerOverlaySection> {
  final _tone = TextEditingController();
  final _signature = TextEditingController();
  bool _hydrated = false;
  bool _saving = false;
  bool _dirty = false;
  SellerOverlay? _base;

  @override
  void initState() {
    super.initState();
    _tone.addListener(_recompute);
    _signature.addListener(_recompute);
  }

  @override
  void dispose() {
    _tone.dispose();
    _signature.dispose();
    super.dispose();
  }

  void _recompute() {
    final b = _base;
    if (b == null) return;
    final dirty = _tone.text.trim() != b.toneNote ||
        _signature.text.trim() != b.signature;
    if (dirty != _dirty) setState(() => _dirty = dirty);
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final overlay = ref.watch(sellerOverlayProvider);

    return overlay.maybeWhen(
      orElse: () => const SizedBox.shrink(),
      data: (o) {
        if (!_hydrated) {
          _tone.text = o.toneNote;
          _signature.text = o.signature;
          _base = o;
          _hydrated = true;
        }
        return SettingsSectionCard(
          title: t.brand.personal,
          icon: Icons.person_outline,
          // Says the limit out loud, so nobody discovers it by having a
          // draft rewritten under them.
          subtitle: t.brand.personal_subtitle,
          subtitleKey: const Key('brand_personal_limit'),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextField(
                key: const Key('brand_tone_note'),
                controller: _tone,
                maxLines: 2,
                style: TextStyle(color: colors.textPrimary),
                decoration: InputDecoration(
                  labelText: t.brand.tone_note,
                  hintText: t.brand.tone_note_hint,
                  hintStyle:
                      TextStyle(color: colors.textDisabled, fontSize: 12),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                key: const Key('brand_personal_signature'),
                controller: _signature,
                maxLines: 2,
                style: TextStyle(color: colors.textPrimary),
                decoration: InputDecoration(
                  labelText: t.brand.personal_signature,
                  hintText: t.brand.personal_signature_hint,
                  hintStyle:
                      TextStyle(color: colors.textDisabled, fontSize: 12),
                ),
              ),
              const SizedBox(height: 16),
              Align(
                alignment: Alignment.centerRight,
                child: ElevatedButton(
                  key: const Key('brand_personal_save'),
                  onPressed: (!_dirty || _saving) ? null : _save,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: colors.bluePrimary,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(
                        horizontal: 24, vertical: 12),
                  ),
                  child: _saving
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(
                              strokeWidth: 2, color: Colors.white),
                        )
                      : Text(t.common.save),
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  Future<void> _save() async {
    setState(() => _saving = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final saved = SellerOverlay(
        toneNote: _tone.text.trim(),
        signature: _signature.text.trim(),
      );
      await ref.read(brandActionsProvider).saveOverlay(saved);
      _base = saved;
      _recompute();
      ref.invalidate(sellerOverlayProvider);
      messenger.showSnackBar(SnackBar(content: Text(t.brand.personal_saved)));
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: const Key('brand_personal_error'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }
}
