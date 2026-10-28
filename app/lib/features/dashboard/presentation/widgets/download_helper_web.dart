// ignore_for_file: avoid_web_libraries_in_flutter, deprecated_member_use
import 'dart:html' as html;

import 'package:dio/dio.dart';

/// Web implementation: fetches the signed URL as bytes, wraps them in
/// a Blob, and synthesizes an <a download="..."> click. The
/// cross-origin anchor download attribute is ignored by browsers when
/// the href points at a third-party host (GCS), so going through a
/// same-origin blob: URL is the only way to force a download prompt
/// instead of an in-tab preview.
Future<bool> downloadFile(String url, String filename) async {
  try {
    final resp = await Dio().get<List<int>>(
      url,
      options: Options(responseType: ResponseType.bytes),
    );
    final bytes = resp.data;
    if (bytes == null || bytes.isEmpty) return false;

    final blob = html.Blob([bytes]);
    final blobUrl = html.Url.createObjectUrlFromBlob(blob);
    final anchor = html.AnchorElement(href: blobUrl)
      ..download = filename.isNotEmpty ? filename : 'imagem'
      ..style.display = 'none';
    html.document.body?.append(anchor);
    anchor.click();
    anchor.remove();
    html.Url.revokeObjectUrl(blobUrl);
    return true;
  } catch (_) {
    return false;
  }
}
