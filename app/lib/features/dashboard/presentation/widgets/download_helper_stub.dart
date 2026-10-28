import 'package:url_launcher/url_launcher.dart';

/// Fallback implementation used on platforms that lack the browser
/// download APIs (mobile). Hands off to the system handler so the user
/// gets the image in the browser/gallery; there's no filesystem prompt
/// like on desktop, but that matches the expected mobile UX.
Future<bool> downloadFile(String url, String filename) async {
  final uri = Uri.tryParse(url);
  if (uri == null) return false;
  return launchUrl(uri, mode: LaunchMode.externalApplication);
}
