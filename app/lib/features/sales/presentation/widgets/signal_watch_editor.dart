import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/features/sales/presentation/controllers/signals_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// The signal-watch editor — the one piece of the old Sinais tab that
/// survived phase 3. The feed itself lives in the Radar now; this dialog is
/// how the licitação poller's keywords get changed.
/// A comma-separated list with nothing in it is not a list.
bool _radarHasKeyword(String raw) =>
    raw.split(',').any((w) => w.trim().isNotEmpty);

/// The 27 federative units, mirrored from `domain.NormalizeUF` on the server.
///
/// A closed set rather than a two-letter shape test: "XX" passes any shape
/// check and then matches no tender ever, which is a watch that looks saved
/// and is quietly empty forever.
const _ufs = <String>{
  'AC', 'AL', 'AM', 'AP', 'BA', 'CE', 'DF', 'ES', 'GO', //
  'MA', 'MG', 'MS', 'MT', 'PA', 'PB', 'PE', 'PI', 'PR', //
  'RJ', 'RN', 'RO', 'RR', 'RS', 'SC', 'SE', 'SP', 'TO', //
};

/// Parses "mt, sp" into `[MT, SP]`, dropping blanks, unknown codes and
/// duplicates while keeping the order typed.
List<String> parseWatchStates(String raw) {
  final out = <String>[];
  for (final part in raw.split(',')) {
    final uf = part.trim().toUpperCase();
    if (_ufs.contains(uf) && !out.contains(uf)) out.add(uf);
  }
  return out;
}

/// Asks which words to follow, and where, and saves the watch.
///
/// Comma-separated free text rather than a tag editor: the person types the
/// words they already use for what they sell, and a first setup that takes
/// one sentence is one people finish.
///
/// The state is asked for and required. A watch with no UF used to mean
/// "anywhere", and anywhere is the whole country — the poll's scope is the
/// union of every watch, so one unscoped row dragged every other account's
/// search across Brazil and earned the rate limiting that emptied this very
/// screen. The server refuses one now; asking here is how the person finds
/// out while they can still answer.
Future<void> editSignalWatch(
    BuildContext context, WidgetRef ref, SignalWatch? existing) async {
  final controller =
      TextEditingController(text: existing?.keywords.join(', ') ?? '');
  final statesController =
      TextEditingController(text: existing?.states.join(', ') ?? '');
  final messenger = ScaffoldMessenger.of(context);

  final confirmed = await showDialog<bool>(
    context: context,
    // StatefulBuilder so the save button can follow what is typed: it used
    // to accept the click over an empty field, close, and only then say
    // the field was empty — losing what the person was looking at to
    // deliver news it had before they clicked.
    builder: (dialogContext) => StatefulBuilder(
      builder: (dialogContext, setDialogState) => AlertDialog(
        title: Text(t.signals.watch_dialog_title),
        // Bounded width: an AlertDialog gives its content unbounded height,
        // and a bare Column with a multi-line field overflows into it.
        content: SizedBox(
          width: 420,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(t.signals.watch_dialog_body),
              const SizedBox(height: 12),
              TextField(
                key: const Key('signals_watch_input'),
                controller: controller,
                autofocus: true,
                maxLines: 2,
                onChanged: (_) => setDialogState(() {}),
                decoration: InputDecoration(hintText: t.signals.watch_hint),
              ),
              const SizedBox(height: 20),
              Text(t.signals.watch_states_body),
              const SizedBox(height: 12),
              TextField(
                key: const Key('signals_watch_states_input'),
                controller: statesController,
                onChanged: (_) => setDialogState(() {}),
                decoration: InputDecoration(
                  labelText: t.signals.watch_states_label,
                  hintText: t.signals.watch_states_hint,
                ),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: Text(t.common.cancel),
          ),
          FilledButton(
            key: const Key('signals_watch_save'),
            onPressed: _radarHasKeyword(controller.text) &&
                    parseWatchStates(statesController.text).isNotEmpty
                ? () => Navigator.of(dialogContext).pop(true)
                : null,
            child: Text(t.common.save),
          ),
        ],
      ),
    ),
  );
  if (confirmed != true) return;

  final keywords = controller.text
      .split(',')
      .map((w) => w.trim())
      .where((w) => w.isNotEmpty)
      .toList();
  final states = parseWatchStates(statesController.text);
  // Disposed after this frame: the dialog is still animating out and its
  // TextField rebuilds once more on the way, which asserts against a
  // controller already disposed.
  WidgetsBinding.instance.addPostFrameCallback((_) {
    controller.dispose();
    statesController.dispose();
  });
  if (keywords.isEmpty) {
    messenger
        .showSnackBar(SnackBar(content: Text(t.signals.watch_empty_error)));
    return;
  }
  if (states.isEmpty) {
    messenger
        .showSnackBar(SnackBar(content: Text(t.signals.watch_states_error)));
    return;
  }

  try {
    await ref
        .read(signalActionsProvider)
        .saveWatch(id: existing?.id, keywords: keywords, states: states);
    ref.invalidate(signalWatchesProvider);
    ref.invalidate(signalsProvider);
    messenger.showSnackBar(SnackBar(content: Text(t.signals.watch_saved)));
  } catch (e) {
    messenger.showSnackBar(SnackBar(
      key: const Key('signal_error'),
      content: Text(parseApiError(e)),
    ));
  }
}
