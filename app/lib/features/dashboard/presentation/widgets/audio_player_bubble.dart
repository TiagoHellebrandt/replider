import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:just_audio/just_audio.dart' as ja;

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

/// Inline audio player for `audio` and `voice` bubbles. Lazy-loads the
/// source on first play so the chat list doesn't fire N HTTP requests
/// up front for every audio message on screen. The "Transcrever" button
/// posts to the backend, which pipes the media through Whisper/Gemini
/// and returns text displayed inline so the attendant can skim without
/// listening.
class AudioPlayerBubble extends ConsumerStatefulWidget {
  final String messageId;
  final String? url;
  final AppColorsExtension colors;
  final bool isVoice;

  /// Transcript already known for this message (inbound audio is
  /// transcribed at webhook time). When present the bubble renders it
  /// straight away and hides the "Transcrever" button — there is
  /// nothing left to request.
  final String? initialTranscript;

  const AudioPlayerBubble({
    super.key,
    required this.messageId,
    required this.url,
    required this.colors,
    this.isVoice = false,
    this.initialTranscript,
  });

  @override
  ConsumerState<AudioPlayerBubble> createState() => _AudioPlayerBubbleState();
}

class _AudioPlayerBubbleState extends ConsumerState<AudioPlayerBubble> {
  final _player = ja.AudioPlayer();
  StreamSubscription<Duration?>? _durationSub;
  bool _loaded = false;
  bool _loading = false;
  String? _loadError;
  String? _transcript;
  bool _transcribing = false;
  String? _transcribeError;

  @override
  void initState() {
    super.initState();
    // A transcript known up front (inbound audio is transcribed at
    // webhook time) is shown immediately; the "Transcrever" button is
    // then hidden since there is nothing left to fetch.
    final pre = widget.initialTranscript?.trim();
    if (pre != null && pre.isNotEmpty) {
      _transcript = pre;
    }
    // Preload on mount so the duration + progress bar render correctly
    // before the user clicks play. Browsers lazy-fetch the audio once
    // we call setUrl, so the cost is metadata only — cheap enough for
    // chat lists where users expect to see "00:12" not "00:00".
    WidgetsBinding.instance.addPostFrameCallback((_) => _ensureLoaded());
    // Duration isn't present synchronously — it arrives once the
    // browser parses the audio metadata. Subscribing forces a rebuild
    // so the total time flips from "00:00" to the real value.
    _durationSub = _player.durationStream.listen((_) {
      if (mounted) setState(() {});
    });
  }

  @override
  void dispose() {
    _durationSub?.cancel();
    _player.dispose();
    super.dispose();
  }

  Future<void> _ensureLoaded() async {
    if (_loaded || _loading) return;
    final url = widget.url;
    if (url == null || url.isEmpty) {
      setState(() => _loadError = t.audio_player.load_failed);
      return;
    }
    setState(() {
      _loading = true;
      _loadError = null;
    });
    try {
      await _player.setUrl(url);
      if (!mounted) return;
      setState(() {
        _loaded = true;
        _loading = false;
      });
    } catch (_) {
      // Surface the failure instead of swallowing it — a silent dead
      // play button gives the attendant nothing to act on.
      if (!mounted) return;
      setState(() {
        _loading = false;
        _loadError = t.audio_player.load_failed;
      });
    }
  }

  Future<void> _togglePlay() async {
    if (!_loaded) await _ensureLoaded();
    if (!_loaded) return;
    if (_player.playing) {
      await _player.pause();
    } else {
      if (_player.processingState == ja.ProcessingState.completed) {
        await _player.seek(Duration.zero);
      }
      await _player.play();
    }
  }

  Future<void> _transcribe() async {
    if (_transcribing) return;
    setState(() {
      _transcribing = true;
      _transcribeError = null;
    });
    try {
      final dio = ref.read(dioProvider);
      final res =
          await dio.post<dynamic>('/messages/${widget.messageId}/transcribe');
      final data = res.data as Map<String, dynamic>;
      if (!mounted) return;
      setState(() {
        _transcript = data['transcript'] as String? ?? '';
        _transcribing = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _transcribeError = t.audio_player.transcribe_failed;
        _transcribing = false;
      });
    }
  }

  String _fmt(Duration d) {
    String two(int n) => n.toString().padLeft(2, '0');
    return '${two(d.inMinutes)}:${two(d.inSeconds % 60)}';
  }

  @override
  Widget build(BuildContext context) {
    final colors = widget.colors;
    return ConstrainedBox(
      constraints: const BoxConstraints(maxWidth: 320),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              StreamBuilder<ja.PlayerState>(
                stream: _player.playerStateStream,
                builder: (_, snap) {
                  final playing = snap.data?.playing ?? false;
                  final icon = _loading
                      ? Icons.hourglass_top
                      : playing
                          ? Icons.pause_circle
                          : Icons.play_circle;
                  return IconButton(
                    padding: EdgeInsets.zero,
                    constraints:
                        const BoxConstraints(minWidth: 36, minHeight: 36),
                    icon: Icon(icon, color: colors.bluePrimary, size: 32),
                    onPressed: _loading ? null : _togglePlay,
                  );
                },
              ),
              const SizedBox(width: 8),
              Expanded(
                child: StreamBuilder<Duration>(
                  stream: _player.positionStream,
                  builder: (_, snap) {
                    final pos = snap.data ?? Duration.zero;
                    final total = _player.duration ?? Duration.zero;
                    final ratio = total.inMilliseconds == 0
                        ? 0.0
                        : pos.inMilliseconds / total.inMilliseconds;
                    return Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        LinearProgressIndicator(
                          value: ratio.clamp(0.0, 1.0),
                          minHeight: 3,
                          color: colors.bluePrimary,
                          backgroundColor: colors.borderLight,
                        ),
                        const SizedBox(height: 4),
                        Row(
                          children: [
                            Icon(
                              widget.isVoice
                                  ? Icons.mic_rounded
                                  : Icons.audiotrack_rounded,
                              size: 12,
                              color: colors.textMuted,
                            ),
                            const SizedBox(width: 4),
                            Text(
                              total == Duration.zero
                                  ? _fmt(pos)
                                  : '${_fmt(pos)} / ${_fmt(total)}',
                              style: TextStyle(
                                fontSize: 11,
                                color: colors.textMuted,
                                fontFeatures: const [
                                  FontFeature.tabularFigures(),
                                ],
                              ),
                            ),
                          ],
                        ),
                      ],
                    );
                  },
                ),
              ),
              // Hidden once a transcript exists — already transcribed
              // messages render the text below, nothing left to request.
              if (_transcript == null)
                IconButton(
                  padding: EdgeInsets.zero,
                  constraints:
                      const BoxConstraints(minWidth: 28, minHeight: 28),
                  icon: Icon(Icons.translate,
                      color: colors.textSecondary, size: 18),
                  tooltip: t.audio_player.transcribe_tooltip,
                  onPressed: _transcribing ? null : _transcribe,
                ),
            ],
          ),
          if (_transcribing)
            Padding(
              padding: const EdgeInsets.only(top: 6),
              child: Row(
                children: [
                  SizedBox(
                    width: 12,
                    height: 12,
                    child: CircularProgressIndicator(
                      strokeWidth: 1.5,
                      color: colors.textMuted,
                    ),
                  ),
                  const SizedBox(width: 6),
                  Text(
                    t.audio_player.transcribing,
                    style: TextStyle(
                      fontSize: 11,
                      color: colors.textMuted,
                      fontStyle: FontStyle.italic,
                    ),
                  ),
                ],
              ),
            ),
          if (_transcript != null && _transcript!.isNotEmpty)
            Container(
              margin: const EdgeInsets.only(top: 8),
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: colors.glassLight,
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: colors.borderLight),
              ),
              child: Text(
                _transcript!,
                style: TextStyle(color: colors.textPrimary, fontSize: 13),
              ),
            ),
          if (_loadError != null)
            Padding(
              padding: const EdgeInsets.only(top: 6),
              child: Text(
                _loadError!,
                style: TextStyle(color: Colors.red.shade300, fontSize: 11),
              ),
            ),
          if (_transcribeError != null)
            Padding(
              padding: const EdgeInsets.only(top: 6),
              child: Text(
                _transcribeError!,
                style: TextStyle(color: Colors.red.shade300, fontSize: 11),
              ),
            ),
        ],
      ),
    );
  }
}
