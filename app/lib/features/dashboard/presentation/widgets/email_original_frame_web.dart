// ignore_for_file: avoid_web_libraries_in_flutter, deprecated_member_use
import 'dart:html' as html;
import 'dart:ui_web' as ui_web;

import 'package:flutter/widgets.dart';

/// Web: the original email rendered INSIDE the panel.
///
/// The document itself is served by the API with a strict CSP (that policy is
/// the protection, and only a browser enforces it) — embedding it in a fully
/// sandboxed iframe keeps that enforcement and adds another wall: the empty
/// sandbox attribute gives the frame a unique origin, no scripts, no forms,
/// no navigation. A phishing email can render here and still touch nothing.
const bool inlineOriginalSupported = true;

final Set<String> _registered = {};

/// onLoad callbacks live OUTSIDE the registered factory closure: the factory
/// is registered once per URL and would otherwise capture the first caller's
/// callback forever — a re-opened reader would never learn its frame loaded.
final Map<String, void Function()?> _onLoad = {};

Widget buildEmailOriginalFrame(String url, {void Function()? onLoad}) {
  final viewType = 'email-original-${url.hashCode}';
  _onLoad[viewType] = onLoad;
  if (!_registered.contains(viewType)) {
    _registered.add(viewType);
    ui_web.platformViewRegistry.registerViewFactory(viewType, (int _) {
      final frame = html.IFrameElement()
        ..src = url
        ..style.border = 'none'
        ..style.width = '100%'
        ..style.height = '100%'
        ..style.backgroundColor = '#ffffff';
      frame.setAttribute('sandbox', '');
      frame.onLoad.listen((_) => _onLoad[viewType]?.call());
      return frame;
    });
  }
  return HtmlElementView(viewType: viewType);
}
