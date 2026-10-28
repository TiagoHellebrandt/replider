import 'package:dio/dio.dart';

import 'file_download_stub.dart' if (dart.library.html) 'file_download_web.dart'
    as impl;

/// Downloading a generated file (§6.18, I3).
///
/// The export endpoint returns bytes, not JSON — an XLSX is a zip container, and
/// the CSV carries a UTF-8 BOM that must survive untouched or Excel renders the
/// accents as mojibake. So the response is read as raw bytes and written
/// verbatim.

/// downloadOptions makes Dio return the body as bytes instead of trying to
/// decode it as JSON.
Options downloadOptions() => Options(responseType: ResponseType.bytes);

/// filenameFrom reads the server's suggested name out of Content-Disposition.
///
/// The server names the file (entity plus date) because it is the one that knows
/// what was exported — a downloads folder with four files called "export.csv" is
/// a downloads folder nobody can use. Returns null when the header is missing or
/// malformed, and the caller falls back to its own name.
String? filenameFrom(Map<String, List<String>> headers) {
  final values =
      headers['content-disposition'] ?? headers['Content-Disposition'];
  if (values == null || values.isEmpty) return null;
  final match = RegExp(r'filename="?([^";]+)"?').firstMatch(values.first);
  final name = match?.group(1)?.trim();
  return (name == null || name.isEmpty) ? null : name;
}

/// saveDownloadedFile hands the bytes to the platform's download surface.
Future<bool> saveDownloadedFile(String filename, List<int> bytes) =>
    impl.saveBytes(filename, bytes);
