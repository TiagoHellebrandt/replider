import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:app/design_system/theme_extensions.dart';

/// Renders WhatsApp-flavored markup the way the customer's WhatsApp
/// app would show it:
///   *bold*    → bold
///   _italic_  → italic
///   ~strike~  → line-through
///   `code`    → monospace + subtle background
/// Plus auto-linkifies http(s):// URLs so the attendant can open them
/// without copy/paste.
///
/// Ranges are non-nesting (WhatsApp's own renderer has the same
/// limitation): the first matching pair wins, others fall through as
/// literal text. Keeps the implementation a single linear pass — fast
/// even for the chattiest threads.
class WhatsAppText extends StatelessWidget {
  final String text;
  final TextStyle baseStyle;

  /// Whether the reader can drag-select this text.
  ///
  /// True everywhere it reads as a document (the support thread, the template
  /// preview). False where an ANCESTOR already claims the long press: two
  /// long-press recognizers on the same text both enter the gesture arena, the
  /// deeper one tends to win, and the ancestor's menu can simply fail to open —
  /// while the selection that won leaves a Material selection handle painted
  /// over the text. Turning selection off there is what makes the ancestor's
  /// gesture reliable; it costs nothing when that ancestor offers copying.
  final bool selectable;

  const WhatsAppText(
    this.text, {
    super.key,
    required this.baseStyle,
    this.selectable = true,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final span =
        TextSpan(style: baseStyle, children: _parse(text, baseStyle, colors));
    // Text.rich, not an IgnorePointer around SelectableText: the parsed spans
    // carry tap recognizers on links, and IgnorePointer would kill those too.
    return selectable ? SelectableText.rich(span) : Text.rich(span);
  }

  /// Linear scan: at each char, check if any token starts here. If so,
  /// look ahead for its closing delimiter and emit one span; otherwise
  /// accumulate plain text. URL detection runs first because it has
  /// the longest match window and shouldn't be split by markup chars
  /// inside the URL itself.
  List<InlineSpan> _parse(
      String src, TextStyle base, AppColorsExtension colors) {
    final out = <InlineSpan>[];
    final buf = StringBuffer();

    void flushPlain() {
      if (buf.isNotEmpty) {
        out.add(TextSpan(text: buf.toString()));
        buf.clear();
      }
    }

    int i = 0;
    while (i < src.length) {
      // 1. URL — any whitespace-delimited http(s) token.
      final urlMatch = _urlAt(src, i);
      if (urlMatch != null) {
        flushPlain();
        final url = src.substring(i, i + urlMatch);
        out.add(TextSpan(
          text: url,
          style: base.copyWith(
            color: colors.bluePrimary,
            decoration: TextDecoration.underline,
          ),
          recognizer: TapGestureRecognizer()..onTap = () => _open(url),
        ));
        i += urlMatch;
        continue;
      }

      // 2. Markup pair: only when the delimiter is at a word boundary
      //    (start of string, after whitespace, or after punctuation).
      //    Prevents `a*b*c` (a literal asterisk-pair inside a token)
      //    from rendering as bold.
      final ch = src[i];
      const delims = {'*': FontWeight.bold, '_': null, '~': null, '`': null};
      if (delims.containsKey(ch) && _atBoundary(src, i)) {
        final close = _findClose(src, i + 1, ch);
        if (close > i + 1) {
          flushPlain();
          final inner = src.substring(i + 1, close);
          TextStyle style;
          switch (ch) {
            case '*':
              style = base.copyWith(fontWeight: FontWeight.bold);
            case '_':
              style = base.copyWith(fontStyle: FontStyle.italic);
            case '~':
              style = base.copyWith(decoration: TextDecoration.lineThrough);
            case '`':
              style = base.copyWith(
                fontFamily: 'monospace',
                backgroundColor: colors.glassMedium,
              );
            default:
              style = base;
          }
          out.add(TextSpan(text: inner, style: style));
          i = close + 1;
          continue;
        }
      }

      buf.write(ch);
      i++;
    }
    flushPlain();
    return out;
  }

  /// Returns the length of the URL starting at [from] or null if none.
  /// Anchored on `http://` / `https://` and ends at the first whitespace
  /// or end-of-string. Trims trailing punctuation that's almost
  /// always part of the surrounding sentence (`.,)]`).
  int? _urlAt(String src, int from) {
    if (!src.startsWith('http://', from) && !src.startsWith('https://', from)) {
      return null;
    }
    var end = from;
    while (end < src.length && !RegExp(r'\s').hasMatch(src[end])) {
      end++;
    }
    // Strip trailing common sentence punctuation.
    while (end > from + 8 && '.,);!?]>'.contains(src[end - 1])) {
      end--;
    }
    return end - from;
  }

  bool _atBoundary(String src, int i) {
    if (i == 0) return true;
    final prev = src[i - 1];
    return RegExp('[\\s\\(\\[\\{>"\'-]').hasMatch(prev);
  }

  /// Find a matching closing delimiter, requiring it to also be at a
  /// word boundary on the right (the next char is whitespace or
  /// punctuation, or it's the end of string). Returns -1 if not found.
  int _findClose(String src, int from, String delim) {
    for (var i = from; i < src.length; i++) {
      if (src[i] != delim) continue;
      // Skip empty pairs (`**`, `__`).
      if (i == from) return -1;
      final next = i + 1 >= src.length ? ' ' : src[i + 1];
      if (RegExp('[\\s\\.,!?\\)\\]\\}>:\'"-]').hasMatch(next)) {
        return i;
      }
    }
    return -1;
  }

  Future<void> _open(String url) async {
    final uri = Uri.tryParse(url);
    if (uri == null) return;
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    }
  }
}
