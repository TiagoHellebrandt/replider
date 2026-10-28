import 'dart:async';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:just_audio/just_audio.dart' as ja;
import 'package:path_provider/path_provider.dart';
import 'package:record/record.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/dashboard/presentation/widgets/media_send_helper.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/i18n/strings.g.dart';

/// Max recording length. Beyond this, WhatsApp's 16MB cap at our 64kbps
/// AAC starts to risk rejection and the user is almost certainly better
/// served by breaking the message in two anyway.
const Duration _maxDuration = Duration(minutes: 15);

/// Recorder lifecycle: idle→recording→paused↔recording→preview→sent|cancel.
enum _RecState { recording, paused, preview, sending }

class AudioRecorderBar extends ConsumerStatefulWidget {
  final String conversationId;
  final VoidCallback onClose;

  const AudioRecorderBar({
    super.key,
    required this.conversationId,
    required this.onClose,
  });

  @override
  ConsumerState<AudioRecorderBar> createState() => _AudioRecorderBarState();
}

class _AudioRecorderBarState extends ConsumerState<AudioRecorderBar> {
  final _recorder = AudioRecorder();
  final _player = ja.AudioPlayer();
  Timer? _ticker;
  StreamSubscription<Amplitude>? _ampSub;
  Duration _elapsed = Duration.zero;
  _RecState _state = _RecState.recording;
  String? _path; // native temp file; null on web (blob URL returned by stop())
  String? _blobUrl;
  Uint8List? _bytes;
  // Rolling amplitude history (normalized 0..1). Fixed-size so the
  // waveform has a consistent visual density regardless of duration.
  final List<double> _amps = List<double>.filled(32, 0);
  int _ampCursor = 0;

  @override
  void initState() {
    super.initState();
    _start();
  }

  @override
  void dispose() {
    // Cancel any in-flight recording/playback BEFORE disposing the
    // underlying plugin objects — disposing a live recorder leaks its
    // native resource on some platforms.
    _ticker?.cancel();
    _ampSub?.cancel();
    _player.dispose();
    _recorder.stop().catchError((_) => null).whenComplete(() {
      _recorder.dispose();
      _cleanupTempFile();
    });
    super.dispose();
  }

  Future<void> _cleanupTempFile() async {
    final p = _path;
    if (p == null) return;
    try {
      final f = File(p);
      if (await f.exists()) await f.delete();
    } catch (_) {}
  }

  Future<void> _start() async {
    try {
      // Don't pre-check via hasPermission(): on web it returns false
      // when the user hasn't decided yet AND does NOT trigger the
      // browser prompt. Calling start() is what invokes getUserMedia
      // and fires the permission dialog natively. We only interpret
      // hasPermission() = false as "denied" AFTER start() fails, to
      // distinguish a real denial from "never asked."
      // 80 kbps stays above the browser MediaRecorder floor (~75 kbps
      // for AAC in Chrome/Safari) — the old 64 kbps tripped a "clamping
      // calculated video bitrate" console warning and, on some builds,
      // aborted the recording before start() resolved.
      final config = const RecordConfig(
        encoder: AudioEncoder.aacLc,
        bitRate: 80000,
        sampleRate: 44100,
      );
      final path = kIsWeb ? '' : await _tempPath();
      await _recorder.start(config, path: path);
      if (!kIsWeb) _path = path;
      _startTicker();
      _startAmpStream();
    } catch (e) {
      // Permission denied vs other failures: check post-hoc so the
      // copy matches the actual cause.
      final granted = await _recorder.hasPermission().catchError((_) => false);
      if (!granted) {
        _showPermissionDialog();
        return;
      }
      _fail(t.audio_recorder.start_failed(error: parseApiError(e)));
    }
  }

  Future<String> _tempPath() async {
    final dir = await getTemporaryDirectory();
    final ts = DateTime.now().millisecondsSinceEpoch;
    return '${dir.path}/voice_$ts.m4a';
  }

  void _startTicker() {
    _ticker?.cancel();
    _ticker = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      if (_state != _RecState.recording) return;
      setState(() => _elapsed += const Duration(seconds: 1));
      if (_elapsed >= _maxDuration) {
        // Auto-stop at the cap and drop into preview so the user can
        // still decide to send or redo. Clearer than silently cutting
        // and sending a truncated file.
        _stopToPreview();
      }
    });
  }

  void _startAmpStream() {
    _ampSub?.cancel();
    _ampSub = _recorder
        .onAmplitudeChanged(const Duration(milliseconds: 150))
        .listen((a) {
      if (!mounted) return;
      // Lingua-free normalization: record returns dBFS in [-160, 0].
      // Map -45..0 dB → 0..1 because anything below -45 is background
      // room noise and shouldn't move the needle.
      final db = a.current.isFinite ? a.current : -160.0;
      final clamped = db.clamp(-45.0, 0.0);
      final norm = (clamped + 45.0) / 45.0;
      setState(() {
        _amps[_ampCursor] = norm;
        _ampCursor = (_ampCursor + 1) % _amps.length;
      });
    });
  }

  Future<void> _togglePause() async {
    try {
      if (_state == _RecState.paused) {
        await _recorder.resume();
        setState(() => _state = _RecState.recording);
      } else if (_state == _RecState.recording) {
        await _recorder.pause();
        setState(() => _state = _RecState.paused);
      }
    } catch (e) {
      _fail(t.audio_recorder.pause_failed(error: parseApiError(e)));
    }
  }

  Future<void> _cancel() async {
    _ticker?.cancel();
    await _ampSub?.cancel();
    try {
      await _player.stop();
      final recording = await _recorder.isRecording();
      final paused = await _recorder.isPaused();
      if (recording || paused) {
        await _recorder.stop();
      }
    } catch (_) {}
    await _cleanupTempFile();
    widget.onClose();
  }

  /// Stops recording and transitions to preview (playable + re-recordable)
  /// instead of sending directly. Triggered by the stop button OR by
  /// hitting the duration cap.
  Future<void> _stopToPreview() async {
    if (_state == _RecState.preview || _state == _RecState.sending) return;
    _ticker?.cancel();
    await _ampSub?.cancel();
    try {
      final result = await _recorder.stop();
      if (result == null) {
        _fail(t.audio_recorder.empty_recording);
        return;
      }
      Uint8List? bytes;
      if (kIsWeb) {
        _blobUrl = result;
        bytes = await _fetchBlobWeb(result);
      } else {
        final file = File(result);
        if (await file.exists()) bytes = await file.readAsBytes();
      }
      if (bytes == null || bytes.isEmpty) {
        _fail(t.audio_recorder.empty_audio);
        return;
      }
      _bytes = bytes;
      // Load into just_audio so the user can preview. On web we play
      // the blob URL directly; on native we feed the file path.
      if (kIsWeb && _blobUrl != null) {
        await _player.setUrl(_blobUrl!);
      } else if (_path != null) {
        await _player.setFilePath(_path!);
      }
      if (!mounted) return;
      setState(() => _state = _RecState.preview);
    } catch (e) {
      _fail(t.audio_recorder.stop_failed(error: parseApiError(e)));
    }
  }

  /// Records again, discarding the current preview. Resets the buffer
  /// and re-enters the recording state.
  Future<void> _redo() async {
    await _player.stop();
    // Drop any stale recorder state before starting a new session —
    // `start()` will otherwise silently no-op on web when the previous
    // MediaRecorder is still holding the mic from the preview phase.
    try {
      final recording = await _recorder.isRecording();
      final paused = await _recorder.isPaused();
      if (recording || paused) {
        await _recorder.stop();
      }
    } catch (_) {}
    await _cleanupTempFile();
    _path = null;
    if (!mounted) return;
    setState(() {
      _state = _RecState.recording;
      _elapsed = Duration.zero;
      _bytes = null;
      _blobUrl = null;
      for (int i = 0; i < _amps.length; i++) {
        _amps[i] = 0;
      }
    });
    await _start();
  }

  Future<void> _send() async {
    if (_state == _RecState.sending) return;
    // If still recording (user skipped preview by long-pressing send
    // or similar), stop first so we have bytes.
    if (_state != _RecState.preview) {
      await _stopToPreview();
      if (_state != _RecState.preview) return; // failure already reported
    }
    final bytes = _bytes;
    if (bytes == null) return;
    setState(() => _state = _RecState.sending);
    try {
      if (!mounted) return;
      final filename = 'voice_${DateTime.now().millisecondsSinceEpoch}.m4a';
      final ok = await sendMediaToConversation(
        ref: ref,
        context: context,
        conversationId: widget.conversationId,
        bytes: bytes,
        filename: filename,
        // Tag it as a voice note, not a regular audio attachment, so
        // the bubble renders with the mic icon and downstream (Meta)
        // can treat it as `voice`.
        forceMessageType: 'voice',
        skipPreview: true,
      );
      if (ok && mounted) {
        widget.onClose();
      } else if (mounted) {
        setState(() => _state = _RecState.preview);
      }
    } catch (e) {
      _fail(t.audio_recorder.send_failed(error: parseApiError(e)));
    }
  }

  Future<Uint8List?> _fetchBlobWeb(String url) async {
    final dio = Dio();
    final res = await dio.get<List<int>>(
      url,
      options: Options(responseType: ResponseType.bytes),
    );
    final data = res.data;
    return data == null ? null : Uint8List.fromList(data);
  }

  void _fail(String msg) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(msg), backgroundColor: Colors.red.shade700),
    );
    widget.onClose();
  }

  void _showPermissionDialog() {
    if (!mounted) return;
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(t.audio_recorder.permission_blocked_title),
        content: Text(
          t.audio_recorder.permission_blocked_body,
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text(t.audio_recorder.ok),
          ),
        ],
      ),
    ).then((_) {
      if (mounted) widget.onClose();
    });
  }

  String _fmt(Duration d) {
    String two(int n) => n.toString().padLeft(2, '0');
    return '${two(d.inMinutes)}:${two(d.inSeconds % 60)}';
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        border: Border(top: BorderSide(color: colors.borderLight, width: 0.5)),
      ),
      child: _state == _RecState.preview || _state == _RecState.sending
          ? _buildPreview(colors)
          : _buildRecording(colors),
    );
  }

  Widget _buildRecording(AppColorsExtension colors) {
    final paused = _state == _RecState.paused;
    final pulseColor = paused ? colors.bluePrimary : Colors.red;
    return Row(
      children: [
        SizedBox(
          width: 36,
          height: 36,
          child: IconButton(
            icon: Icon(Icons.close, color: colors.textSecondary, size: 20),
            tooltip: t.audio_recorder.cancel_tooltip,
            onPressed: _cancel,
            padding: EdgeInsets.zero,
            splashRadius: 18,
          ),
        ),
        const SizedBox(width: 4),
        _Dot(color: pulseColor),
        const SizedBox(width: 8),
        Text(
          _fmt(_elapsed),
          style: TextStyle(
            color: colors.textPrimary,
            fontSize: 14,
            fontFeatures: const [FontFeature.tabularFigures()],
            fontWeight: FontWeight.w600,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: CustomPaint(
            size: const Size(double.infinity, 28),
            painter: _WaveformPainter(
              amps: _amps,
              cursor: _ampCursor,
              color: pulseColor,
            ),
          ),
        ),
        const SizedBox(width: 8),
        _CircleButton(
          icon: paused ? Icons.play_arrow : Icons.pause,
          bg: colors.glassMedium,
          fg: colors.textPrimary,
          tooltip: paused
              ? t.audio_recorder.resume_tooltip
              : t.audio_recorder.pause_tooltip,
          onPressed: _togglePause,
        ),
        const SizedBox(width: 6),
        _CircleButton(
          icon: Icons.check,
          bg: colors.bluePrimary,
          fg: Colors.white,
          tooltip: t.audio_recorder.stop_tooltip,
          onPressed: _stopToPreview,
        ),
      ],
    );
  }

  Widget _buildPreview(AppColorsExtension colors) {
    final sending = _state == _RecState.sending;
    return Row(
      children: [
        IconButton(
          icon: Icon(Icons.close, color: colors.textSecondary),
          tooltip: t.audio_recorder.discard_tooltip,
          onPressed: sending ? null : _cancel,
        ),
        IconButton(
          icon: Icon(Icons.refresh, color: colors.textSecondary),
          tooltip: t.audio_recorder.redo_tooltip,
          onPressed: sending ? null : _redo,
        ),
        const SizedBox(width: 4),
        StreamBuilder<ja.PlayerState>(
          stream: _player.playerStateStream,
          builder: (_, snap) {
            final playing = snap.data?.playing ?? false;
            return IconButton(
              icon: Icon(
                playing ? Icons.pause_circle : Icons.play_circle,
                color: colors.bluePrimary,
                size: 28,
              ),
              tooltip: playing
                  ? t.audio_recorder.pause_play_tooltip
                  : t.audio_recorder.play_tooltip,
              onPressed: sending
                  ? null
                  : () {
                      if (playing) {
                        _player.pause();
                      } else {
                        _player.seek(Duration.zero);
                        _player.play();
                      }
                    },
            );
          },
        ),
        Expanded(
          child: StreamBuilder<Duration>(
            stream: _player.positionStream,
            builder: (_, snap) {
              final pos = snap.data ?? Duration.zero;
              final total = _elapsed.inMilliseconds == 0
                  ? const Duration(milliseconds: 1)
                  : _elapsed;
              final progress =
                  (pos.inMilliseconds / total.inMilliseconds).clamp(0.0, 1.0);
              return Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  LinearProgressIndicator(
                    value: progress,
                    minHeight: 3,
                    color: colors.bluePrimary,
                    backgroundColor: colors.borderLight,
                  ),
                  const SizedBox(height: 4),
                  Text(
                    '${_fmt(pos)} / ${_fmt(_elapsed)}',
                    style: TextStyle(
                      color: colors.textMuted,
                      fontSize: 11,
                      fontFeatures: const [FontFeature.tabularFigures()],
                    ),
                  ),
                ],
              );
            },
          ),
        ),
        const SizedBox(width: 8),
        Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(
            gradient:
                LinearGradient(colors: [colors.blueDark, colors.indigoDark]),
            borderRadius: BorderRadius.circular(20),
          ),
          child: sending
              ? const Padding(
                  padding: EdgeInsets.all(10),
                  child: CircularProgressIndicator(
                      strokeWidth: 2, color: Colors.white),
                )
              : IconButton(
                  icon: const Icon(Icons.send, color: Colors.white, size: 18),
                  onPressed: _send,
                  padding: EdgeInsets.zero,
                  tooltip: t.audio_recorder.send_tooltip,
                ),
        ),
      ],
    );
  }
}

class _CircleButton extends StatelessWidget {
  final IconData icon;
  final Color bg;
  final Color fg;
  final String tooltip;
  final VoidCallback? onPressed;

  const _CircleButton({
    required this.icon,
    required this.bg,
    required this.fg,
    required this.tooltip,
    required this.onPressed,
  });

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: Material(
        color: bg,
        shape: const CircleBorder(),
        child: InkWell(
          customBorder: const CircleBorder(),
          onTap: onPressed,
          child: SizedBox(
            width: 36,
            height: 36,
            child: Center(child: Icon(icon, color: fg, size: 20)),
          ),
        ),
      ),
    );
  }
}

class _Dot extends StatefulWidget {
  final Color color;
  const _Dot({required this.color});
  @override
  State<_Dot> createState() => _DotState();
}

class _DotState extends State<_Dot> with SingleTickerProviderStateMixin {
  late final AnimationController _c = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 900),
  )..repeat(reverse: true);

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return FadeTransition(
      opacity: Tween(begin: 0.35, end: 1.0).animate(_c),
      child: Container(
        width: 10,
        height: 10,
        decoration: BoxDecoration(color: widget.color, shape: BoxShape.circle),
      ),
    );
  }
}

class _WaveformPainter extends CustomPainter {
  final List<double> amps;
  final int cursor;
  final Color color;

  _WaveformPainter(
      {required this.amps, required this.cursor, required this.color});

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = color
      ..strokeWidth = 2.5
      ..strokeCap = StrokeCap.round;
    final bars = amps.length;
    final slot = size.width / bars;
    final mid = size.height / 2;
    for (int i = 0; i < bars; i++) {
      // Rotate so the newest sample sits on the right edge, mimicking
      // the scrolling waveform users expect from voice apps.
      final idx = (cursor + i) % bars;
      final h = (amps[idx].clamp(0.05, 1.0)) * size.height;
      final x = slot * i + slot / 2;
      canvas.drawLine(Offset(x, mid - h / 2), Offset(x, mid + h / 2), paint);
    }
  }

  @override
  bool shouldRepaint(covariant _WaveformPainter old) =>
      old.cursor != cursor || !_listEq(old.amps, amps);

  static bool _listEq(List<double> a, List<double> b) {
    if (a.length != b.length) return false;
    for (int i = 0; i < a.length; i++) {
      if (a[i] != b[i]) return false;
    }
    return true;
  }
}
