import 'package:flutter/widgets.dart';

/// Non-web: no iframe to embed. The caller falls back to the external
/// "ver original" button, which is the browser-based path.
const bool inlineOriginalSupported = false;

Widget buildEmailOriginalFrame(String url, {void Function()? onLoad}) =>
    const SizedBox.shrink();
