import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/analytics/analytics_client.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/core/auth/account_role.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/onboarding/domain/onboarding_variant.dart';
import 'package:app/features/settings/domain/models/channel_model.dart'
    show isSensitiveConfigKey;
import 'package:app/i18n/strings.g.dart';

part 'onboarding_controller.g.dart';

@immutable
class OnboardingState {
  final int currentStep;

  /// Business name displayed as the "company identity" in the preview
  /// chat system prompt and stored as accounts.name. For social-login
  /// users this starts as their personal name (the only thing Google/
  /// Meta handed us) and is overwritten during step 0.
  final String accountName;

  /// One line about what the business does, stored on brand_kits.about.
  /// This is what stops the first AI-written approach from reading like
  /// a template: with it empty the composer has the company's NAME and
  /// nothing else, so it writes around the gap. Collected here rather
  /// than left to Settings because the draft a customer judges the
  /// product by is the first one, and by then onboarding is over.
  final String about;

  /// How the business talks, stored on brand_kits.tone_of_voice. Picked
  /// from presets — prose, because it goes verbatim into the prompt.
  final String toneOfVoice;

  /// How the company earns — any of `sell`, `rent`, `service`. Stored on
  /// accounts.business_lines. A hospital-equipment distributor is often all
  /// three, and each line reads a different signal downstream, which is why
  /// this is a set and why it is asked before anything else.
  final Set<String> businessLines;

  /// Where she sells: a state code and, optionally, the cities inside it as
  /// the person typed them ("Cuiabá, Várzea Grande"). Split on save.
  final String territoryUF;
  final String territoryCities;

  final String? channelType;
  final String channelName;
  final Map<String, String> channelConfig;
  final bool channelConnected;

  final bool isLoading;
  final String? error;

  const OnboardingState({
    this.currentStep = 0,
    this.accountName = '',
    this.about = '',
    this.toneOfVoice = '',
    this.businessLines = const {},
    this.territoryUF = '',
    this.territoryCities = '',
    this.channelType,
    this.channelName = '',
    this.channelConfig = const {},
    this.channelConnected = false,
    this.isLoading = false,
    this.error,
  });

  OnboardingState copyWith({
    int? currentStep,
    String? accountName,
    String? about,
    String? toneOfVoice,
    Set<String>? businessLines,
    String? territoryUF,
    String? territoryCities,
    String? Function()? channelType,
    String? channelName,
    Map<String, String>? channelConfig,
    bool? channelConnected,
    bool? isLoading,
    String? Function()? error,
  }) {
    return OnboardingState(
      currentStep: currentStep ?? this.currentStep,
      accountName: accountName ?? this.accountName,
      about: about ?? this.about,
      toneOfVoice: toneOfVoice ?? this.toneOfVoice,
      businessLines: businessLines ?? this.businessLines,
      territoryUF: territoryUF ?? this.territoryUF,
      territoryCities: territoryCities ?? this.territoryCities,
      channelType: channelType != null ? channelType() : this.channelType,
      channelName: channelName ?? this.channelName,
      channelConfig: channelConfig ?? this.channelConfig,
      channelConnected: channelConnected ?? this.channelConnected,
      isLoading: isLoading ?? this.isLoading,
      error: error != null ? error() : this.error,
    );
  }
}

@riverpod
class OnboardingController extends _$OnboardingController {
  Timer? _saveDebounce;

  /// Set once the initial GET /me/onboarding-progress completes so
  /// later saves don't race against hydration and clobber persisted
  /// state with an empty draft.
  bool _hydrated = false;

  @override
  OnboardingState build() {
    ref.onDispose(() {
      _saveDebounce?.cancel();
    });
    _hydrate();
    return const OnboardingState();
  }

  /// Pulls any previously saved step index + draft fields from the API
  /// so a user who closed the tab mid-flow resumes where they left off.
  /// Silently no-ops on error — the default empty state is safe.
  Future<void> _hydrate() async {
    try {
      final dio = ref.read(dioProvider);
      final res = await dio.get<dynamic>('/me/onboarding-progress');
      final data = res.data as Map<String, dynamic>;
      final draft =
          (data['draft'] as Map?)?.cast<String, dynamic>() ?? const {};
      final step = (data['current_step'] as int?) ?? 0;
      // Snap a persisted step into the active flow. A draft saved under
      // the deleted full flow (e.g. on the knowledge step, index 2) must
      // not strand the person on a page the wizard no longer renders —
      // advance them to the next visible step instead.
      final active = ref.read(onboardingStepIndicesProvider);
      var resolvedStep = step.clamp(0, OnboardingVariantSteps.lastStepIndex);
      if (!active.contains(resolvedStep)) {
        resolvedStep = active.firstWhere(
          (i) => i >= resolvedStep,
          orElse: () => active.last,
        );
      }
      state = state.copyWith(
        currentStep: resolvedStep,
        accountName: draft['account_name'] as String? ??
            (data['account_name'] as String?) ??
            state.accountName,
        businessLines: (draft['business_lines'] as List?)
                ?.map((e) => e.toString())
                .toSet() ??
            state.businessLines,
        territoryUF: draft['territory_uf'] as String? ?? state.territoryUF,
        territoryCities:
            draft['territory_cities'] as String? ?? state.territoryCities,
        channelConnected:
            draft['channel_connected'] as bool? ?? state.channelConnected,
        about: draft['about'] as String? ?? state.about,
        toneOfVoice: draft['tone_of_voice'] as String? ?? state.toneOfVoice,
        channelType: () => draft['channel_type'] as String?,
        channelName: draft['channel_name'] as String? ?? state.channelName,
        channelConfig: (draft['channel_config'] as Map?)
                ?.map((k, v) => MapEntry(k.toString(), v.toString())) ??
            state.channelConfig,
      );
    } catch (_) {
      // Best-effort: first-time users and offline users land on the
      // empty default state, which is correct.
    } finally {
      _hydrated = true;
    }
    await _hydrateBusinessProfile();
  }

  /// The business profile lives on the account, not in the draft — it is
  /// real data from the moment it is saved — so a resume reads it back
  /// from /me. Separate try: a failure here must not undo the draft above.
  Future<void> _hydrateBusinessProfile() async {
    try {
      final dio = ref.read(dioProvider);
      final res = await dio.get<dynamic>('/me');
      final account =
          ((res.data as Map?)?['account'] as Map?)?.cast<String, dynamic>();
      if (account == null) return;
      final lines = (account['business_lines'] as List?)
              ?.map((e) => e.toString())
              .toSet() ??
          const <String>{};
      final territory = (account['territory'] as Map?)?.cast<String, dynamic>();
      state = state.copyWith(
        businessLines: lines.isEmpty ? state.businessLines : lines,
        territoryUF: (territory?['uf'] as String?) ?? state.territoryUF,
        territoryCities: ((territory?['cities'] as List?)
                ?.map((e) => e.toString())
                .join(', ')) ??
            state.territoryCities,
      );
    } catch (_) {
      // Same rule as the draft: the empty state is a correct starting point.
    }
  }

  /// Writes the current step + draft blob to the API. Called on step
  /// transitions (flushes immediately) and debounced on form edits.
  /// Non-fatal on failure — we surface the user-facing error only for
  /// actions the user explicitly initiated (submitStep*), not draft
  /// auto-save.
  Future<void> _saveDraft() async {
    if (!_hydrated) return;
    try {
      final dio = ref.read(dioProvider);
      await dio.put<dynamic>('/me/onboarding-draft', data: {
        'current_step': state.currentStep,
        'draft': {
          // The whole of step 0 rides the draft. It used to carry only
          // about/tone: fill the name, the chips and the UF, close the tab
          // before Próximo — all gone on resume.
          'account_name': state.accountName,
          'business_lines': state.businessLines.toList(),
          'territory_uf': state.territoryUF,
          'territory_cities': state.territoryCities,
          'channel_connected': state.channelConnected,
          'about': state.about,
          'tone_of_voice': state.toneOfVoice,
          'channel_type': state.channelType,
          'channel_name': state.channelName,
          // Credentials (auth_token, access_token, bot_token) never ride
          // the draft — it would persist them in plaintext server-side.
          // The user re-types the secret if they resume; the backend
          // strips them too (US-012), this just avoids sending them.
          'channel_config': Map<String, String>.fromEntries(
            state.channelConfig.entries
                .where((e) => !isSensitiveConfigKey(e.key)),
          ),
        },
      });
    } catch (_) {
      // Swallow: the next save will retry, and a lost draft is strictly
      // better than surfacing a snackbar every 2s on flaky networks.
    }
  }

  void _scheduleDraftSave() {
    _saveDebounce?.cancel();
    _saveDebounce = Timer(const Duration(milliseconds: 800), _saveDraft);
  }

  void goToStep(int step) {
    state = state.copyWith(currentStep: step, error: () => null);
    // Step transitions are intentional — flush immediately so a crash
    // right after clicking Next doesn't lose the advance.
    _saveDebounce?.cancel();
    _saveDraft();
  }

  /// Called by the WhatsApp embedded signup button after Meta's flow
  /// completes and our backend has already created the channel. Marks
  /// the step as done so [_submitChannel] skips the redundant POST /channels.
  void markChannelConnected() {
    state = state.copyWith(channelConnected: true);
    _scheduleDraftSave();
  }

  void updateField(String field, dynamic value) {
    switch (field) {
      case 'accountName':
        state = state.copyWith(accountName: value as String);
      case 'about':
        state = state.copyWith(about: value as String);
      case 'toneOfVoice':
        state = state.copyWith(toneOfVoice: value as String);
      case 'businessLines':
        state = state.copyWith(businessLines: value as Set<String>);
      case 'territoryUF':
        state = state.copyWith(territoryUF: value as String);
      case 'territoryCities':
        state = state.copyWith(territoryCities: value as String);
      case 'channelType':
        state = state.copyWith(
          channelType: () => value as String?,
          // Changing the channel tile invalidates any in-progress connection.
          channelConnected: false,
        );
      case 'channelName':
        state = state.copyWith(channelName: value as String);
      case 'channelConfig':
        state = state.copyWith(channelConfig: value as Map<String, String>);
    }
    _scheduleDraftSave();
  }

  /// Whether step one has what it needs: a name, one line about the
  /// business, at least one way it earns, and the state it sells in. The
  /// Next button reads this; the submit re-checks it.
  bool get stepCompanyComplete =>
      state.accountName.trim().isNotEmpty &&
      state.about.trim().isNotEmpty &&
      state.businessLines.isNotEmpty &&
      state.territoryUF.trim().isNotEmpty;

  /// The cities as typed, split and cleaned. Exposed for the request body
  /// and for tests; the server dedups again.
  List<String> get territoryCityList => state.territoryCities
      .split(RegExp(r'[,;\n]'))
      .map((c) => c.trim())
      .where((c) => c.isNotEmpty)
      .toList();

  /// First step: persists the business name on accounts.name, what the
  /// company does and where on the account's business profile, and the
  /// identity fields on the brand kit. If a required field was left empty
  /// this silently no-ops so the disabled-Next handling in the UI remains
  /// the only gating logic.
  Future<void> submitStepAccountName() async {
    if (!stepCompanyComplete) return;
    state = state.copyWith(isLoading: true, error: () => null);
    try {
      final dio = ref.read(dioProvider);
      await dio.patch<dynamic>('/accounts', data: {
        'name': state.accountName.trim(),
        'business_lines': state.businessLines.toList(),
        'territory': {
          'uf': state.territoryUF.trim().toUpperCase(),
          'cities': territoryCityList,
        },
      });
      await _saveBrandKit(dio);
      // Next step from the flow, never a hardcoded index: a `1` (the old
      // agent step) once landed every account on a page outside the
      // stepIndices, which hid the entire bottom nav — no Next, no Back,
      // no Skip — until a page reload rescued it.
      // The ACCOUNT's flow, not the variant's raw list: a self-serve account
      // never sees the BYOK step, and sending it to an index the screen does
      // not render is exactly the bug this comment already warns about.
      final nextStep = ref.read(onboardingStepIndicesProvider)[1];
      state = state.copyWith(isLoading: false, currentStep: nextStep);
      _saveDebounce?.cancel();
      _saveDraft();
    } catch (e) {
      state = state.copyWith(isLoading: false, error: () => parseApiError(e));
    }
  }

  /// Persists the identity fields on brand_kits.
  ///
  /// PUT /brand-kit is a full replace, so this reads the current kit
  /// first and writes it back with only the three onboarding fields
  /// changed — an account redoing onboarding must not lose the claims
  /// it configured in Settings.
  ///
  /// A failed read is the one case that skips the write entirely:
  /// writing a partial kit over an unknown one would silently drop
  /// exactly those claims. The error propagates, so the user stays on
  /// the step and Next retries — both calls are idempotent.
  Future<void> _saveBrandKit(Dio dio) async {
    final res = await dio.get<dynamic>('/brand-kit');
    final current = (res.data as Map?)?.cast<String, dynamic>() ?? const {};
    await dio.put<dynamic>('/brand-kit', data: {
      ...current,
      'company_name': state.accountName.trim(),
      'about': state.about.trim(),
      'tone_of_voice': state.toneOfVoice.trim(),
    });
  }

  /// Twilio Account SID: "AC" + 32 hex chars. Mirrors the server-side
  /// rule (domain.IsTwilioAccountSID) so the user gets a specific
  /// message instead of a generic 400 after the roundtrip.
  static final _twilioSidRe = RegExp(r'^AC[0-9a-fA-F]{32}$');

  /// E.164: +, country code, up to 15 digits (domain.IsE164 mirror).
  static final _e164Re = RegExp(r'^\+[1-9]\d{1,14}$');

  /// Client-side shape check of the channel config before POST
  /// /channels (US-012). Returns a localized message or null when ok.
  String? _validateChannelConfig() {
    if (state.channelType != 'sms') return null;
    final cfg = state.channelConfig;
    if (!_twilioSidRe.hasMatch(cfg['account_sid']?.trim() ?? '')) {
      return t.onboarding.invalid_sms_account_sid;
    }
    if ((cfg['auth_token']?.trim() ?? '').isEmpty) {
      return t.onboarding.invalid_sms_auth_token;
    }
    if (!_e164Re.hasMatch(cfg['from_number']?.trim() ?? '')) {
      return t.onboarding.invalid_sms_from_number;
    }
    return null;
  }

  /// Whether the email tile has SMTP details typed. Without them the email
  /// tile means "connect Gmail" (a card that creates the channel through the
  /// OAuth callback) or "later" — either way there is nothing to POST, and
  /// posting an empty config would be refused by the server for a reason
  /// the person cannot act on.
  bool get _emailNeedsPost =>
      state.channelType == 'email' &&
      (state.channelConfig['smtp_host']?.trim().isNotEmpty ?? false);

  /// Creates the selected channel when there is something to create.
  /// Returns false when validation or the request failed (the error is on
  /// the state); true when the step may advance — including when nothing
  /// needed posting.
  Future<bool> _submitChannel() async {
    final nothingToPost = state.channelType == null ||
        state.channelConnected ||
        (state.channelType == 'email' && !_emailNeedsPost);
    if (!nothingToPost) {
      final validationError = _validateChannelConfig();
      if (validationError != null) {
        state = state.copyWith(error: () => validationError);
        return false;
      }
      state = state.copyWith(isLoading: true, error: () => null);
      try {
        final dio = ref.read(dioProvider);
        final name = state.channelName.isNotEmpty
            ? state.channelName
            : switch (state.channelType) {
                'email' => 'E-mail',
                'telegram' => 'Telegram',
                'instagram' => 'Instagram',
                'sms' => 'SMS',
                _ => 'WhatsApp',
              };
        await dio.post<dynamic>('/channels', data: {
          'type': state.channelType,
          'name': name,
          'config': state.channelConfig,
        });
        ref.read(analyticsClientProvider).track(
          OnboardingEvents.channelConnected,
          {'type': state.channelType ?? 'unknown'},
        );
        state = state.copyWith(isLoading: false);
      } catch (e) {
        state = state.copyWith(isLoading: false, error: () => parseApiError(e));
        return false;
      }
    }
    return true;
  }

  /// Creates the channel (if any) and moves to [next] — the channel step is
  /// followed by the first offer, so it is never terminal.
  Future<void> submitChannelAndAdvance(int next) async {
    if (await _submitChannel()) goToStep(next);
  }

  /// [destination] is where the person lands: the queue when she just read
  /// her first offer (it is one tap from approving it), the landing screen
  /// otherwise. NEVER default to /home — that is the v1 inbox, which a
  /// vendedor's shell does not even show and whose API answers 403 for
  /// that role; skipping the wizard used to land exactly there.
  Future<void> completeOnboarding(BuildContext context,
      {String? destination}) async {
    // Landing BY ROLE: /start (and /prospect) 403 for an attendant, and an
    // invited attendant on a not-yet-onboarded account is routed through
    // this wizard — finishing it must not park them on an error screen.
    final role =
        AccountRole.from(ref.read(authControllerProvider).asData?.value?.role);
    final dest = role.satisfies(AccountRole.vendedor)
        ? (destination ?? '/start')
        : '/home';
    // 1. Persist on the backend so reloads / re-logins don't return to onboarding
    try {
      final dio = ref.read(dioProvider);
      await dio.patch<dynamic>('/accounts', data: {
        'onboarding_completed': true,
      });
    } catch (_) {
      // Non-fatal: worst case the user sees onboarding again on next login
    }

    // 2. Update local state BEFORE navigating. The router redirect at
    // app_router.dart bounces /home back to /onboarding while
    // user.isNew is true, so we must await the Riverpod state mutation
    // (and the saveUser write) before calling context.go('/home') —
    // otherwise the GoRouter redirect runs against the stale value and
    // the user gets stuck on this screen until a manual reload.
    final authNotifier = ref.read(authControllerProvider.notifier);
    final currentUser = ref.read(authControllerProvider).asData?.value;
    if (currentUser != null) {
      await authNotifier.updateUserInState(
        currentUser.name,
        currentUser.email,
        isNew: false,
      );
    }
    if (context.mounted) context.go(dest);
  }
}
