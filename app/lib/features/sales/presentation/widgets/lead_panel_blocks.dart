import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:app/core/auth/account_role.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/sales/presentation/controllers/lead_detail_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Where the message goes, and what to do with it.
///
/// The panel used to show a draft and a timeline and never the number, so
/// deciding whether to phone someone meant starting a send just to read the
/// confirmation dialog. In a product for salespeople who live on the phone,
/// that was the wrong thing to hide.
class LeadContactBlock extends ConsumerWidget {
  const LeadContactBlock({super.key, required this.lead});

  final LeadDetail lead;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(t.lead_detail.contact,
            style: typography.body
                .copyWith(color: colors.textSecondary, fontSize: 12)),
        const SizedBox(height: 6),
        if (lead.recipientAddress.isNotEmpty)
          SelectableText(
            lead.recipientAddress,
            key: const Key('lead_detail_address'),
            style: typography.body.copyWith(color: Colors.white, fontSize: 14),
          ),
        if (lead.city != null)
          Text(lead.city!,
              key: const Key('lead_detail_city'),
              style: typography.body
                  .copyWith(color: colors.textMuted, fontSize: 12)),
        const SizedBox(height: 8),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            if (lead.canCall)
              OutlinedButton.icon(
                key: const Key('lead_detail_call'),
                onPressed: () => _launch('tel:${lead.recipientAddress}'),
                icon: const Icon(Icons.call, size: 16),
                label: Text(t.lead_detail.call),
              ),
            if (lead.website != null)
              OutlinedButton.icon(
                key: const Key('lead_detail_website'),
                onPressed: () => _launch(_withScheme(lead.website!)),
                icon: const Icon(Icons.language, size: 16),
                label: Text(t.lead_detail.website),
              ),
            if (lead.conversationId != null &&
                // The inbox is attendant-gated and a vendedor deliberately
                // does not satisfy attendant — for them this button led to a
                // screen their shell does not show and whose API answers
                // 403. Their read of the exchange is the timeline block.
                AccountRole.from(
                        ref.watch(authControllerProvider).asData?.value?.role)
                    .satisfies(AccountRole.attendant))
              FilledButton.icon(
                key: const Key('lead_detail_open_conversation'),
                onPressed: () {
                  // Same handoff the rest of the app uses: select, then
                  // navigate. Inventing a second channel for it would give
                  // the inbox two sources of truth about what is open.
                  ref
                      .read(selectedConversationProvider.notifier)
                      .select(lead.conversationId!);
                  Navigator.of(context).maybePop();
                  context.go('/home');
                },
                icon: const Icon(Icons.forum_outlined, size: 16),
                label: Text(t.lead_detail.open_conversation),
              )
            else if (lead.conversationId == null)
              Text(t.lead_detail.no_conversation,
                  key: const Key('lead_detail_no_conversation'),
                  style: typography.body
                      .copyWith(color: colors.textMuted, fontSize: 12)),
          ],
        ),
      ],
    );
  }
}

/// A site captured from a directory often arrives without a scheme, and
/// launching "empresa.com.br" opens nothing.
String _withScheme(String url) => url.startsWith('http') ? url : 'https://$url';

Future<void> _launch(String url) async {
  final uri = Uri.tryParse(url);
  if (uri != null) {
    await launchUrl(uri, mode: LaunchMode.externalApplication);
  }
}

/// What is known about the company — or, when nothing is, what to do about it.
///
/// The old panel dropped this whole section for an unenriched lead: no
/// heading, no explanation, no way to fill it, even though the endpoint that
/// fills it already existed. A salesperson opening a prospected lead saw only
/// a draft, which is why the screen read as "it only talks about outreach".
class LeadProfileBlock extends StatelessWidget {
  const LeadProfileBlock({
    super.key,
    required this.lead,
    required this.onEnrich,
    required this.enriching,
    this.showHeading = true,
  });

  final LeadDetail lead;
  final VoidCallback onEnrich;
  final bool enriching;

  /// False when the block lives inside a section that already names itself —
  /// two "Profile" headings six pixels apart are noise, not hierarchy.
  final bool showHeading;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (showHeading) ...[
          Text(t.lead_detail.profile,
              style: typography.body
                  .copyWith(color: colors.textSecondary, fontSize: 12)),
          const SizedBox(height: 6),
        ],
        // The facts ALWAYS, when there are any — they answer "what do we know
        // about this company?", which is the question a lead is opened with.
        // They used to be the `if` branch of an `else if` over the research
        // state, so saying "we searched and found nothing" cost hiding the name
        // and address already known.
        for (final f in lead.facts)
          Padding(
            padding: const EdgeInsets.only(bottom: 4),
            child: RichText(
              text: TextSpan(children: [
                TextSpan(
                  text: '${f.label}: ',
                  style: typography.body
                      .copyWith(color: colors.textMuted, fontSize: 13),
                ),
                TextSpan(
                  text: f.value,
                  style: typography.body
                      .copyWith(color: Colors.white, fontSize: 13),
                ),
              ]),
            ),
          ),
        // The decision-maker has a profile, and nobody saw it.
        //
        // Enrichment writes `linkedin_url`/`instagram_url` and no funnel screen
        // read them — only the old lead list did. The search found the decider's
        // LinkedIn and the panel made it disappear.
        if (lead.linkedinUrl != null || lead.instagramUrl != null) ...[
          const SizedBox(height: 6),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              if (lead.linkedinUrl != null)
                OutlinedButton.icon(
                  key: const Key('lead_detail_linkedin'),
                  onPressed: () => _launch(_withScheme(lead.linkedinUrl!)),
                  icon: const Icon(Icons.person_search_outlined, size: 16),
                  label: const Text('LinkedIn'),
                ),
              if (lead.instagramUrl != null)
                OutlinedButton.icon(
                  key: const Key('lead_detail_instagram'),
                  onPressed: () => _launch(_withScheme(lead.instagramUrl!)),
                  icon: const Icon(Icons.camera_alt_outlined, size: 16),
                  label: const Text('Instagram'),
                ),
            ],
          ),
        ],
        // The research STATE, as its own line rather than an alternative to
        // the facts. "Nobody has looked yet", "I am looking", "I looked and
        // found nothing" and "I could not look, and here is why" are four
        // different things — and the fourth disappears along with the third if
        // they depend on there being facts on screen.
        if (lead.facts.isNotEmpty) const SizedBox(height: 10),
        if (lead.isResearching)
          Text(t.lead_detail.profile_researching,
              key: const Key('lead_detail_profile_researching'),
              style:
                  typography.body.copyWith(color: Colors.white, fontSize: 13))
        else if (!lead.researchFoundSomething) ...[
          Text(
              lead.researchBlocked
                  ? t.lead_detail.profile_blocked
                  : lead.researchedNothing
                      ? t.lead_detail.profile_found_nothing
                      : t.lead_detail.profile_empty,
              key: Key(lead.researchBlocked
                  ? 'lead_detail_profile_blocked'
                  : lead.researchedNothing
                      ? 'lead_detail_profile_found_nothing'
                      : 'lead_detail_profile_empty'),
              style:
                  typography.body.copyWith(color: Colors.white, fontSize: 13)),
          const SizedBox(height: 4),
          // Says what the research BUYS, not what it does. "Finds the decision
          // maker, the trade and the address so the approach can cite something
          // real" is a reason to click; "enrich lead" is jargon.
          Text(
              lead.researchBlocked
                  ? t.lead_detail.profile_blocked_hint
                  : lead.researchedNothing
                      ? t.lead_detail.profile_found_nothing_hint
                      : t.lead_detail.profile_empty_hint,
              style: typography.body.copyWith(
                  color: colors.textSecondary, fontSize: 12, height: 1.4)),
        ],
        // Where the AI got this from.
        //
        // The facts appeared with no provenance: a partner's name read off a
        // LinkedIn page and one the model inferred looked exactly alike.
        // Whoever is about to cite it in an approach needs to know which of the
        // two they are about to assert to a stranger.
        //
        // The server already sent the URLs in `enrichment.sources` and no
        // screen read them.
        if (lead.sources.isNotEmpty) ...[
          const SizedBox(height: 10),
          Text(t.board.sources,
              key: const Key('lead_detail_sources'),
              style: typography.body
                  .copyWith(color: colors.textSecondary, fontSize: 12)),
          const SizedBox(height: 2),
          Text(t.board.sources_hint,
              style: typography.body
                  .copyWith(color: colors.textMuted, fontSize: 11)),
          const SizedBox(height: 4),
          for (final url in lead.sources)
            Padding(
              padding: const EdgeInsets.only(bottom: 2),
              child: Text(
                _sourceHost(url),
                style: typography.body.copyWith(
                    color: colors.textSecondary,
                    fontSize: 12,
                    // Selectable: the domain answers "can I trust this?", and
                    // the full URL answers "where exactly?" for whoever wants
                    // to check.
                    height: 1.3),
              ),
            ),
        ],
        const SizedBox(height: 8),
        OutlinedButton.icon(
          key: const Key('lead_detail_enrich'),
          onPressed: enriching || lead.isResearching ? null : onEnrich,
          icon: const Icon(Icons.travel_explore, size: 16),
          // "Pesquisar esta empresa" after a search already ran offers work
          // the product has done; "Pesquisar de novo" says what the click
          // actually costs.
          label: Text(enriching || lead.isResearching
              ? t.lead_detail.enriching
              : lead.researchedAt != null
                  ? t.lead_detail.enrich_again
                  : t.lead_detail.enrich),
        ),
      ],
    );
  }
}

/// The thread in a few sentences: what was agreed, what is pending, whose
/// move.
///
/// For whoever opens a forty-message thread after a week away, or inherits a
/// lead from a colleague. Generated on read, so a conversation nobody revisits
/// never costs a model call.
class LeadConversationSummary extends ConsumerWidget {
  const LeadConversationSummary({super.key, required this.conversationId});

  final String conversationId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final state = ref.watch(conversationBriefProvider(conversationId));

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(t.lead_detail.summary,
            style: typography.body
                .copyWith(color: colors.textSecondary, fontSize: 12)),
        const SizedBox(height: 6),
        state.when(
          loading: () => Text(t.lead_detail.summary_generating,
              key: const Key('lead_detail_summary_loading'),
              style: typography.body
                  .copyWith(color: colors.textMuted, fontSize: 13)),
          // A summary that failed says so quietly: the whole thread is one tap
          // away, and a red block here would suggest the LEAD is broken.
          error: (_, __) => Text(t.lead_detail.summary_failed,
              key: const Key('lead_detail_summary_error'),
              style: typography.body
                  .copyWith(color: colors.textMuted, fontSize: 13)),
          data: (text) => Text(
            text ?? t.lead_detail.summary_short,
            key: const Key('lead_detail_summary'),
            style: typography.body.copyWith(
                color: text == null ? colors.textMuted : Colors.white,
                fontSize: 13,
                height: 1.4),
          ),
        ),
      ],
    );
  }
}

/// The summary of one conversation, or null when the thread is too short to
/// be worth one — which the server answers with 204 rather than an empty
/// string, because those mean different things.
final conversationBriefProvider = FutureProvider.autoDispose
    .family<String?, String>((ref, conversationId) async {
  final res = await ref
      .watch(dioProvider)
      .get<Map<String, dynamic>>('/conversations/$conversationId/summary');
  if (res.statusCode == 204 || res.data == null) return null;
  final text = (res.data!['text'] as String? ?? '').trim();
  return text.isEmpty ? null : text;
});

/// The source's domain, which is what answers "can I trust this?".
///
/// A raw search URL takes three lines and says nothing more: what changes the
/// decision to cite a fact is whether it came from the company's own site or
/// from some aggregator.
String _sourceHost(String url) {
  final host = Uri.tryParse(url)?.host ?? '';
  if (host.isEmpty) return url;
  return host.startsWith('www.') ? host.substring(4) : host;
}
