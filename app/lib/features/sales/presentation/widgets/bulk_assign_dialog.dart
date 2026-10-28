import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/primary_pill_button.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/board_controller.dart';
import 'package:app/features/settings/presentation/controllers/team_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// How many leads one redistribution may touch at a time.
///
/// Mirrors the server's ceiling. Duplicated on purpose: the client has to know
/// BEFORE sending, so the dialog can say how many will actually go — an
/// "assign 64" that comes back an error is worse than "assign 200 of these
/// 300".
const bulkAssignMax = 200;

/// Hands over every ownerless lead at once.
///
/// Sixty-four unowned leads, one at a time, is work nobody does — and a field
/// nobody fills in becomes a field nobody trusts.
///
/// Takes the ids the screen already counted rather than a filter: the number
/// on the button is the number that changes. A filter evaluated server-side
/// would also catch whatever entered the funnel between the count and the
/// click.
Future<({int assigned, int skipped})?> showBulkAssignDialog(
  BuildContext context, {
  required List<String> leadIds,
  required bool filtered,
}) {
  return showDialog<({int assigned, int skipped})>(
    context: context,
    builder: (_) => _BulkAssignDialog(leadIds: leadIds, filtered: filtered),
  );
}

class _BulkAssignDialog extends ConsumerStatefulWidget {
  const _BulkAssignDialog({required this.leadIds, required this.filtered});

  final List<String> leadIds;

  /// True when the search or the filter are on — the count is then not "the
  /// whole funnel", and the dialog has to say so.
  final bool filtered;

  @override
  ConsumerState<_BulkAssignDialog> createState() => _BulkAssignDialogState();
}

class _BulkAssignDialogState extends ConsumerState<_BulkAssignDialog> {
  String? _ownerId;
  bool _saving = false;
  String? _error;

  List<String> get _targets => widget.leadIds.take(bulkAssignMax).toList();

  Future<void> _submit() async {
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      final res = await ref.read(boardServiceProvider).bulkAssign(
            leadIds: _targets,
            ownerUserId: _ownerId,
          );
      if (!mounted) return;
      Navigator.of(context).pop(res);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _saving = false;
        _error = parseApiError(e);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // Deactivated members excluded: assigning work to someone who left is
    // the same as losing it.
    final team = (ref.watch(teamMembersProvider).asData?.value ?? const [])
        .where((m) => m.isActive)
        .toList();

    return AlertDialog(
      backgroundColor: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      title: Text(
        t.board.bulk_assign_title,
        style:
            TextStyle(color: colors.textPrimary, fontWeight: FontWeight.w700),
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.board.bulk_assign_body(count: widget.leadIds.length.toString()),
            key: const Key('bulk_assign_count'),
            style: TextStyle(color: colors.textSecondary, fontSize: 13),
          ),
          if (widget.filtered) ...[
            const SizedBox(height: 6),
            // The count came from what is on screen. Calling that "the whole
            // funnel" with a search active would promise what will not
            // happen.
            Text(t.board.bulk_assign_filtered,
                key: const Key('bulk_assign_filtered'),
                style: TextStyle(color: colors.textMuted, fontSize: 11)),
          ],
          if (widget.leadIds.length > bulkAssignMax) ...[
            const SizedBox(height: 6),
            Text(t.board.bulk_assign_cap,
                key: const Key('bulk_assign_cap'),
                style: TextStyle(color: colors.orangeAlert, fontSize: 11)),
          ],
          const SizedBox(height: 14),
          DropdownButtonFormField<String>(
            key: const Key('bulk_assign_owner'),
            initialValue: _ownerId,
            isExpanded: true,
            hint: Text(t.board.owner_none,
                style: TextStyle(color: colors.textMuted, fontSize: 14)),
            dropdownColor: colors.backgroundSecondary,
            style: TextStyle(color: colors.textPrimary, fontSize: 14),
            decoration: InputDecoration(
              filled: true,
              fillColor: colors.backgroundTertiary,
              isDense: true,
            ),
            items: [
              for (final member in team)
                DropdownMenuItem<String>(
                  value: member.id,
                  child: Text(
                      member.name.trim().isEmpty ? member.email : member.name),
                ),
            ],
            onChanged: _saving ? null : (v) => setState(() => _ownerId = v),
          ),
          if (_error != null) ...[
            const SizedBox(height: 12),
            Text(_error!,
                key: const Key('bulk_assign_error'),
                style: TextStyle(color: colors.redAlert, fontSize: 12)),
          ],
        ],
      ),
      actions: [
        TextButton(
          onPressed: _saving ? null : () => Navigator.of(context).pop(),
          child: Text(t.common.cancel,
              style: TextStyle(color: colors.textSecondary)),
        ),
        PrimaryPillButton(
          key: const Key('bulk_assign_submit'),
          // With nobody picked there is nothing to do: this dialog exists to
          // GIVE an owner. Taking it away is one lead at a time, in the panel,
          // where you can see whom you are taking it from.
          onPressed: _saving || _ownerId == null ? null : _submit,
          text: t.board.bulk_assign_confirm(count: _targets.length.toString()),
          isLoading: _saving,
        ),
      ],
    );
  }
}
