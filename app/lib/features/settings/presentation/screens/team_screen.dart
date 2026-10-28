import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/core/auth/account_role.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/error/plan_limit_handler.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/settings/domain/models/team_member_model.dart';
import 'package:app/features/settings/presentation/controllers/team_controller.dart';
import 'package:app/i18n/strings.g.dart';

class TeamScreen extends ConsumerStatefulWidget {
  const TeamScreen({super.key});

  @override
  ConsumerState<TeamScreen> createState() => _TeamScreenState();
}

class _TeamScreenState extends ConsumerState<TeamScreen> {
  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final membersAsync = ref.watch(teamMembersProvider);

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        leading: IconButton(
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () => context.go('/settings'),
        ),
        title: Text(
          t.team.title,
          style: TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w700,
            color: colors.textPrimary,
          ),
        ),
        actions: [
          Padding(
            padding: const EdgeInsets.only(right: 8),
            child: TextButton.icon(
              onPressed: () => _showInviteDialog(context, ref, colors),
              icon: Icon(Icons.person_add_outlined,
                  size: 18, color: colors.bluePrimary),
              label: Text(
                t.team.invite_short,
                style: TextStyle(color: colors.bluePrimary, fontSize: 13),
              ),
            ),
          ),
        ],
      ),
      // Wider than the 640 the form screens use: the member cards carry
      // e-mail + role + menu, but edge-to-edge on a monitor reads as a
      // banner. (The live-supervision panel, the round-robin section and
      // the attendant metrics that used to sit here were the v1 inbox's
      // supervisor tooling — cut 2026-09-01.)
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 840),
          child: Column(
            children: [
              Expanded(
                child: membersAsync.when(
                  data: (members) {
                    if (members.isEmpty) {
                      return Center(
                        child: Column(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            Icon(Icons.group_outlined,
                                size: 64, color: colors.textDisabled),
                            const SizedBox(height: 16),
                            Text(
                              t.team.empty_subtitle,
                              style: TextStyle(
                                  color: colors.textMuted, fontSize: 16),
                            ),
                            const SizedBox(height: 16),
                            ElevatedButton.icon(
                              onPressed: () =>
                                  _showInviteDialog(context, ref, colors),
                              icon: const Icon(Icons.person_add, size: 18),
                              label: Text(t.team.invite_member),
                              style: ElevatedButton.styleFrom(
                                backgroundColor: colors.bluePrimary,
                                foregroundColor: Colors.white,
                              ),
                            ),
                          ],
                        ),
                      );
                    }
                    return ListView.builder(
                      padding: const EdgeInsets.all(16),
                      itemCount: members.length,
                      itemBuilder: (context, index) {
                        final member = members[index];
                        return Padding(
                          padding: const EdgeInsets.only(bottom: 8),
                          child: _buildMemberCard(
                            context,
                            ref,
                            colors,
                            member,
                          ),
                        );
                      },
                    );
                  },
                  loading: () =>
                      const Center(child: CircularProgressIndicator()),
                  error: (err, _) => Center(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(Icons.error_outline,
                            color: colors.redAlert, size: 48),
                        const SizedBox(height: 12),
                        Text(
                          t.settings.error_loading_team,
                          style: TextStyle(
                              color: colors.textSecondary, fontSize: 16),
                        ),
                        const SizedBox(height: 8),
                        Text(
                          parseApiError(err),
                          style:
                              TextStyle(color: colors.textMuted, fontSize: 12),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                        const SizedBox(height: 16),
                        TextButton.icon(
                          onPressed: () =>
                              ref.read(teamMembersProvider.notifier).refresh(),
                          icon: const Icon(Icons.refresh, size: 16),
                          label: Text(t.common.retry),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildMemberCard(
    BuildContext context,
    WidgetRef ref,
    AppColorsExtension colors,
    TeamMemberModel member,
  ) {
    final role = AccountRole.from(member.role);
    final isAdmin = role.satisfies(AccountRole.admin);
    // The owner is not a role the two-way toggle can express: flipping it
    // would silently demote the account's owner to admin.
    final roleIsFixed = role == AccountRole.owner;
    final initial = member.name.isNotEmpty ? member.name[0].toUpperCase() : '?';

    return Container(
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      child: ListTile(
        contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
        leading: CircleAvatar(
          backgroundColor: member.isActive
              ? colors.bluePrimary.withValues(alpha: 0.15)
              : colors.glassMedium,
          child: Text(
            initial,
            style: TextStyle(
              color: member.isActive ? colors.bluePrimary : colors.textMuted,
              fontWeight: FontWeight.w700,
            ),
          ),
        ),
        title: Row(
          children: [
            Flexible(
              child: Text(
                member.name,
                style: TextStyle(
                  color:
                      member.isActive ? colors.textPrimary : colors.textMuted,
                  fontWeight: FontWeight.w600,
                  fontSize: 15,
                ),
                overflow: TextOverflow.ellipsis,
              ),
            ),
            const SizedBox(width: 8),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
              decoration: BoxDecoration(
                color: isAdmin
                    ? colors.bluePrimary.withValues(alpha: 0.15)
                    : colors.glassMedium,
                borderRadius: BorderRadius.circular(6),
              ),
              child: Text(
                teamRoleLabel(role),
                style: TextStyle(
                  color: isAdmin ? colors.bluePrimary : colors.textMuted,
                  fontSize: 11,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
            if (!member.isActive) ...[
              const SizedBox(width: 6),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: colors.redAlert.withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(6),
                ),
                child: Text(
                  t.team.inactive,
                  style: TextStyle(
                    color: colors.redAlert,
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
            if (member.isPendingInvite) ...[
              const SizedBox(width: 6),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: colors.orangeAlert.withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(6),
                ),
                child: Text(
                  t.team.pending_badge,
                  style: TextStyle(
                    color: colors.orangeAlert,
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
          ],
        ),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              member.email,
              style: TextStyle(color: colors.textMuted, fontSize: 13),
            ),
            if (member.lastSeenAt != null)
              Text(
                _formatLastSeen(member.lastSeenAt!),
                style: TextStyle(color: colors.textDisabled, fontSize: 11),
              ),
          ],
        ),
        trailing: member.isActive &&
                member.id != ref.read(authControllerProvider).asData?.value?.id
            ? PopupMenuButton<String>(
                icon: Icon(Icons.more_vert, color: colors.textMuted, size: 20),
                color: colors.backgroundSecondary,
                onSelected: (value) async {
                  switch (value) {
                    case 'role':
                      _showChangeRoleDialog(context, ref, colors, member);
                    case 'remove':
                      _showRemoveConfirmation(context, ref, colors, member);
                    case 'resend':
                      final err = await ref
                          .read(teamMembersProvider.notifier)
                          .resendInvite(member.id);
                      if (context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                          content: Text(err ?? t.team.invite_resent),
                          backgroundColor: err == null ? null : colors.redAlert,
                        ));
                      }
                  }
                },
                itemBuilder: (ctx) => [
                  if (member.isPendingInvite)
                    PopupMenuItem(
                      value: 'resend',
                      child: Row(
                        children: [
                          Icon(Icons.mail_outline,
                              size: 18, color: colors.bluePrimary),
                          const SizedBox(width: 8),
                          Text(t.team.resend_invite,
                              style: TextStyle(color: colors.textPrimary)),
                        ],
                      ),
                    ),
                  if (!roleIsFixed)
                    PopupMenuItem(
                      value: 'role',
                      child: Row(
                        children: [
                          Icon(Icons.swap_horiz,
                              size: 18, color: colors.textSecondary),
                          const SizedBox(width: 8),
                          Text(t.team.change_role,
                              style: TextStyle(color: colors.textPrimary)),
                        ],
                      ),
                    ),
                  PopupMenuItem(
                    value: 'remove',
                    child: Row(
                      children: [
                        Icon(Icons.person_remove,
                            size: 18, color: colors.redAlert),
                        const SizedBox(width: 8),
                        Text(t.common.remove,
                            style: TextStyle(color: colors.redAlert)),
                      ],
                    ),
                  ),
                ],
              )
            : null,
      ),
    );
  }

  /// Renders "Ativo agora" / "Ativo 3 dias atrás" given an ISO8601
  /// timestamp. Falls back to an empty string on bad input so the row
  /// just doesn't show the subtitle line — never a hard error to the
  /// user.
  String _formatLastSeen(String iso) {
    final parsed = DateTime.tryParse(iso);
    if (parsed == null) return '';
    final now = DateTime.now().toUtc();
    final diff = now.difference(parsed.toUtc());
    if (diff.inMinutes < 5) return t.team.active_now;
    String when;
    if (diff.inHours < 1) {
      when = t.team.ago_minutes(count: diff.inMinutes.toString());
    } else if (diff.inDays < 1) {
      when = t.team.ago_hours(count: diff.inHours.toString());
    } else if (diff.inDays < 30) {
      when = t.team.ago_days(count: diff.inDays.toString());
    } else {
      when = t.team.ago_months(count: (diff.inDays / 30).floor().toString());
    }
    return t.team.last_seen_ago(when: when);
  }

  void _showInviteDialog(
    BuildContext context,
    WidgetRef ref,
    AppColorsExtension colors,
  ) {
    final nameController = TextEditingController();
    final emailController = TextEditingController();
    // The product sells to SALES teams and the default hire is a
    // vendedora — yet this dropdown offered only admin/attendant, the v1
    // call-center pair, while the role system knew five. Each option says
    // what the person will actually see.
    var selectedRole = 'vendedor';
    final roles = [
      (
        value: 'vendedor',
        label: t.team.role_vendedor,
        hint: t.team.role_vendedor_hint
      ),
      (
        value: 'gestor',
        label: t.team.role_gestor,
        hint: t.team.role_gestor_hint
      ),
      (
        value: 'attendant',
        label: t.team.role_attendant,
        hint: t.team.role_attendant_hint
      ),
      (value: 'admin', label: t.team.role_admin, hint: t.team.role_admin_hint),
    ];

    showDialog<void>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialogState) {
          return AlertDialog(
            backgroundColor: colors.backgroundSecondary,
            shape:
                RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
            title: Text(
              t.team.invite_member,
              style: TextStyle(
                  color: colors.textPrimary, fontWeight: FontWeight.w700),
            ),
            content: SizedBox(
              width: 340,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  TextField(
                    controller: nameController,
                    autofocus: true,
                    style: TextStyle(color: colors.textPrimary),
                    decoration: InputDecoration(
                      labelText: t.profile.name,
                      labelStyle: TextStyle(color: colors.textMuted),
                    ),
                  ),
                  const SizedBox(height: 16),
                  TextField(
                    controller: emailController,
                    keyboardType: TextInputType.emailAddress,
                    style: TextStyle(color: colors.textPrimary),
                    decoration: InputDecoration(
                      labelText: t.profile.email,
                      labelStyle: TextStyle(color: colors.textMuted),
                    ),
                  ),
                  const SizedBox(height: 16),
                  Text(
                    t.team.role_label,
                    style: TextStyle(
                      color: colors.textSecondary,
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: 6),
                  Container(
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(10),
                      border: Border.all(color: colors.borderMedium),
                    ),
                    padding: const EdgeInsets.symmetric(horizontal: 12),
                    child: DropdownButtonHideUnderline(
                      child: DropdownButton<String>(
                        value: selectedRole,
                        isExpanded: true,
                        dropdownColor: colors.backgroundSecondary,
                        style:
                            TextStyle(color: colors.textPrimary, fontSize: 14),
                        isDense: false,
                        itemHeight: null,
                        // The closed button shows just the label; the
                        // two-line explainer lives in the open list.
                        selectedItemBuilder: (context) => [
                          for (final r in roles)
                            Align(
                              alignment: Alignment.centerLeft,
                              child: Text(r.label),
                            ),
                        ],
                        items: roles
                            .map((r) => DropdownMenuItem(
                                  value: r.value,
                                  child: Padding(
                                    padding:
                                        const EdgeInsets.symmetric(vertical: 6),
                                    child: Column(
                                      mainAxisSize: MainAxisSize.min,
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: [
                                        Text(r.label),
                                        Text(
                                          r.hint,
                                          style: TextStyle(
                                              color: colors.textMuted,
                                              fontSize: 11),
                                        ),
                                      ],
                                    ),
                                  ),
                                ))
                            .toList(),
                        onChanged: (val) {
                          if (val == null) return;
                          setDialogState(() => selectedRole = val);
                        },
                      ),
                    ),
                  ),
                ],
              ),
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(ctx),
                child: Text(t.common.cancel,
                    style: TextStyle(color: colors.textMuted)),
              ),
              ElevatedButton(
                onPressed: () async {
                  final name = nameController.text.trim();
                  final email = emailController.text.trim();
                  // Never a silent return, and never a POST with an address
                  // the mail server will bounce.
                  String? problem;
                  if (name.isEmpty || email.isEmpty) {
                    problem = t.team.invite_fields_required;
                  } else if (!RegExp(r'^[^@\s]+@[^@\s]+\.[^@\s]+$')
                      .hasMatch(email)) {
                    problem = t.team.invite_email_invalid;
                  }
                  if (problem != null) {
                    ScaffoldMessenger.of(ctx).showSnackBar(SnackBar(
                      content: Text(problem),
                      behavior: SnackBarBehavior.floating,
                    ));
                    return;
                  }
                  Navigator.pop(ctx);
                  final r = await ref
                      .read(teamMembersProvider.notifier)
                      .inviteMember(name, email, selectedRole);
                  if (!context.mounted) return;
                  if (r.error != null) {
                    // Plan-limit path (seats): handler pops upgrade
                    // modal + returns the localized "$current/$limit"
                    // copy. Fallback to whatever generic error we got.
                    final planMsg = handlePlanLimitError(context, r.raw);
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(
                        content: Text(planMsg ?? r.error!),
                        backgroundColor: Colors.red.shade700,
                        behavior: SnackBarBehavior.floating,
                      ),
                    );
                  } else {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(
                        content: Text(t.team.member_invited),
                        behavior: SnackBarBehavior.floating,
                      ),
                    );
                  }
                },
                style: ElevatedButton.styleFrom(
                  backgroundColor: colors.bluePrimary,
                  foregroundColor: Colors.white,
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(10)),
                ),
                child: Text(t.team.invite_member),
              ),
            ],
          );
        },
      ),
    ).whenComplete(() {
      // Dialog dismissed — children unmounted, safe to dispose the
      // controllers held in the enclosing closure. Without this they
      // leak on every invite-dialog open/close cycle.
      nameController.dispose();
      emailController.dispose();
    });
  }

  void _showChangeRoleDialog(
    BuildContext context,
    WidgetRef ref,
    AppColorsExtension colors,
    TeamMemberModel member,
  ) {
    // A PICKER, not a blind toggle. The old version computed
    // `admin ? attendant : admin`, so changing a Gestor's or a Vendedor's
    // role silently promoted them to Admin — the widest role in the
    // account, granted by a dialog that never showed a choice.
    var selectedRole = member.role;
    final roles = [
      (value: 'vendedor', label: t.team.role_vendedor),
      (value: 'gestor', label: t.team.role_gestor),
      (value: 'attendant', label: t.team.role_attendant),
      (value: 'admin', label: t.team.role_admin),
    ];

    showDialog<void>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialogState) => AlertDialog(
          backgroundColor: colors.backgroundSecondary,
          shape:
              RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
          title: Text(
            t.team.change_role_title,
            style: TextStyle(
                color: colors.textPrimary, fontWeight: FontWeight.w700),
          ),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                member.name,
                style: TextStyle(color: colors.textSecondary, fontSize: 14),
              ),
              const SizedBox(height: 12),
              for (final r in roles)
                RadioListTile<String>(
                  key: Key('change_role_${r.value}'),
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  title: Text(r.label,
                      style: TextStyle(color: colors.textPrimary)),
                  value: r.value,
                  groupValue: selectedRole,
                  onChanged: (v) =>
                      setDialogState(() => selectedRole = v ?? selectedRole),
                ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: Text(t.common.cancel,
                  style: TextStyle(color: colors.textMuted)),
            ),
            ElevatedButton(
              onPressed: selectedRole == member.role
                  ? null
                  : () async {
                      Navigator.pop(ctx);
                      final error = await ref
                          .read(teamMembersProvider.notifier)
                          .updateRole(member.id, selectedRole);
                      if (error != null && context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(
                            content: Text(error),
                            backgroundColor: Colors.red.shade700,
                            behavior: SnackBarBehavior.floating,
                          ),
                        );
                      } else if (context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(
                            content: Text(t.team.role_updated),
                            behavior: SnackBarBehavior.floating,
                          ),
                        );
                      }
                    },
              style: ElevatedButton.styleFrom(
                backgroundColor: colors.bluePrimary,
                foregroundColor: Colors.white,
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(10)),
              ),
              child: Text(t.common.confirm),
            ),
          ],
        ),
      ),
    );
  }

  void _showRemoveConfirmation(
    BuildContext context,
    WidgetRef ref,
    AppColorsExtension colors,
    TeamMemberModel member,
  ) {
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: colors.backgroundSecondary,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: Text(
          t.team.remove_title,
          style:
              TextStyle(color: colors.textPrimary, fontWeight: FontWeight.w700),
        ),
        content: Text(
          t.team.remove_body(name: member.name),
          style: TextStyle(color: colors.textSecondary, fontSize: 14),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text(t.common.cancel,
                style: TextStyle(color: colors.textMuted)),
          ),
          ElevatedButton(
            onPressed: () async {
              Navigator.pop(ctx);
              final error = await ref
                  .read(teamMembersProvider.notifier)
                  .removeMember(member.id);
              if (error != null && context.mounted) {
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(
                    content: Text(error),
                    backgroundColor: Colors.red.shade700,
                    behavior: SnackBarBehavior.floating,
                  ),
                );
              } else if (context.mounted) {
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(
                    content: Text(t.team.member_removed),
                    behavior: SnackBarBehavior.floating,
                  ),
                );
              }
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: colors.redAlert,
              foregroundColor: Colors.white,
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10)),
            ),
            child: Text(t.common.remove),
          ),
        ],
      ),
    );
  }
}

/// Label for a per-account role. The badge used to read every non-admin as
/// "Atendente", which mislabels a gestor and a vendedor as inbox staff.
String teamRoleLabel(AccountRole role) => switch (role) {
      AccountRole.owner => t.team.role_owner,
      AccountRole.admin => t.team.role_admin,
      AccountRole.gestor => t.team.role_gestor,
      AccountRole.vendedor => t.team.role_vendedor,
      AccountRole.attendant => t.team.role_attendant,
    };
