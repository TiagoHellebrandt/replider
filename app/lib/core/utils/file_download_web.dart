// ignore_for_file: avoid_web_libraries_in_flutter, deprecated_member_use
import 'dart:html' as html;
import 'dart:typed_data';

/// Web: wraps the bytes in a same-origin blob URL and synthesizes a download
/// click. Same approach as the owner panel's CSV saver, generalised to bytes so
/// an XLSX (which is a zip, not text) survives the round trip.
Future<bool> saveBytes(String filename, List<int> bytes) async {
  try {
    final blob = html.Blob([Uint8List.fromList(bytes)]);
    final url = html.Url.createObjectUrlFromBlob(blob);
    final anchor = html.AnchorElement(href: url)
      ..download = filename
      ..style.display = 'none';
    html.document.body?.append(anchor);
    anchor.click();
    anchor.remove();
    html.Url.revokeObjectUrl(url);
    return true;
  } catch (_) {
    return false;
  }
}
