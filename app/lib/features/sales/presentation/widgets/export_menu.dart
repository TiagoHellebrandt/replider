import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/core/utils/file_download.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

/// The export button that sits in every list's header (§6.18, I3).
///
/// One widget for every screen, taking the filters the screen is currently
/// showing. The promise is "what you are looking at, in a file" — so the caller
/// passes its live filter state rather than the widget guessing, and a screen
/// that forgets to is a screen whose export is visibly wrong rather than
/// silently so.
///
/// Every download is recorded server-side (who, what, which filters). The menu
/// says so, because somebody taking the customer list should know it is not
/// anonymous.
class ExportMenu extends ConsumerStatefulWidget {
  const ExportMenu({
    super.key,
    required this.entity,
    this.filters = const {},
  });

  /// leads | contacts | sales | conversations | signals.
  final String entity;

  /// The screen's ACTIVE filters. Empty values are dropped before the request,
  /// so a cleared filter is not sent as "".
  final Map<String, String> filters;

  @override
  ConsumerState<ExportMenu> createState() => _ExportMenuState();
}

class _ExportMenuState extends ConsumerState<ExportMenu> {
  bool _busy = false;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return PopupMenuButton<String>(
      key: const Key('export_menu'),
      enabled: !_busy,
      tooltip: t.export.tooltip,
      icon: _busy
          ? SizedBox(
              width: 16,
              height: 16,
              child: CircularProgressIndicator(
                  strokeWidth: 2, color: colors.textSecondary),
            )
          : Icon(Icons.download_outlined,
              size: 20, color: colors.textSecondary),
      onSelected: _export,
      itemBuilder: (context) => [
        PopupMenuItem<String>(
          value: 'xlsx',
          child: Text(t.export.xlsx),
        ),
        PopupMenuItem<String>(
          value: 'csv',
          child: Text(t.export.csv),
        ),
        PopupMenuItem<String>(
          enabled: false,
          height: 32,
          child: Text(
            // Said in the menu rather than after the fact: somebody taking the
            // customer list should know the download is not anonymous BEFORE
            // they take it.
            t.export.audited,
            style: TextStyle(color: colors.textSecondary, fontSize: 10),
          ),
        ),
      ],
    );
  }

  Future<void> _export(String format) async {
    setState(() => _busy = true);
    try {
      final query = <String, dynamic>{'format': format};
      widget.filters.forEach((key, value) {
        if (value.trim().isNotEmpty) query[key] = value;
      });

      final response = await ref.read(dioProvider).get<List<int>>(
            '/export/${widget.entity}',
            queryParameters: query,
            options: downloadOptions(),
          );

      final filename =
          filenameFrom(response.headers.map) ?? '${widget.entity}.$format';
      await saveDownloadedFile(filename, response.data ?? const []);

      if (mounted) {
        final truncated =
            response.headers.value('x-export-truncated') == 'true';
        final rows = response.headers.value('x-export-rows') ?? '?';
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(truncated
              // Somebody who asked for "all leads" and got exactly the ceiling
              // has to know the file is not all of them, or they will
              // reconcile it against a dashboard and blame the dashboard.
              ? t.export.truncated(rows: rows)
              : t.export.done(rows: rows)),
        ));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(parseApiError(e))));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }
}
