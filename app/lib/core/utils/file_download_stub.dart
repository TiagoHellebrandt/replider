/// Non-web fallback: there is no download surface, so the caller is told the
/// save did not happen rather than being shown a success it cannot verify.
Future<bool> saveBytes(String filename, List<int> bytes) async => false;
