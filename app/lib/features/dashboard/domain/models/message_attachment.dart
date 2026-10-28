/// A file that arrived with a message.
///
/// The scan verdict travels with it because the difference between "checked
/// and clean", "nobody checked" and "infected" is what the reader has to see
/// BEFORE clicking — discovering it by clicking is how a product ends up
/// implying a safety it never verified.
library;

/// What is known about a stored file. Mirrors the server's five states.
enum AttachmentScanStatus {
  /// Just arrived; no scanner has seen it. The DEFAULT, so a crash between
  /// storing and scanning leaves a file unknown rather than safe.
  pending,

  /// A scanner looked and found nothing. The only downloadable state.
  clean,

  /// A scanner found something. The bytes are kept for evidence and are
  /// never handed to a browser.
  infected,

  /// No scanner is configured, or the file is past the size one accepts.
  /// Deliberately NOT the same as clean: "we did not look" and "we looked
  /// and it was fine" are different sentences.
  skipped,

  /// The scanner was reachable and errored. Also not clean.
  failed;

  static AttachmentScanStatus fromWire(String? v) => switch (v) {
        'clean' => AttachmentScanStatus.clean,
        'infected' => AttachmentScanStatus.infected,
        'skipped' => AttachmentScanStatus.skipped,
        'failed' => AttachmentScanStatus.failed,
        // Anything unrecognised — including a future status this build does
        // not know — reads as pending. Guessing "clean" for an unknown value
        // is the one mistake that cannot be walked back.
        _ => AttachmentScanStatus.pending,
      };

  /// Only a real clean verdict may be opened normally.
  bool get isDownloadable => this == AttachmentScanStatus.clean;

  /// Whether a scan could still change this verdict. Drives whether the UI
  /// offers to wait: an infected file has nothing to wait for.
  bool get mayStillSettle =>
      this == AttachmentScanStatus.pending ||
      this == AttachmentScanStatus.failed;
}

class MessageAttachment {
  const MessageAttachment({
    required this.id,
    required this.filename,
    required this.contentType,
    required this.sizeBytes,
    required this.isInline,
    required this.scanStatus,
    this.scanSignature,
  });

  final String id;
  final String filename;
  final String contentType;
  final int sizeBytes;

  /// Message furniture — a signature logo, a tracking pixel. Stored (the
  /// original view needs it) but never counted as "an attachment": counting
  /// it teaches people to ignore the indicator, and that is exactly when the
  /// real contract slips past.
  final bool isInline;

  final AttachmentScanStatus scanStatus;

  /// What the scanner found, by name. Shown because "infectado" with no name
  /// is impossible to argue with when it is a false positive.
  final String? scanSignature;

  /// An inline image big enough to be CONTENT — a pasted photo — rather
  /// than message furniture (signature logos run tens of KB, tracking
  /// pixels bytes). The text reader mode lists these as openable
  /// attachments, since unlike the formatted view it cannot render them
  /// in place.
  bool get isLargeInlineImage =>
      isInline &&
      contentType.toLowerCase().startsWith('image/') &&
      sizeBytes > 100 * 1024;

  /// Human-readable size. Integers below 1MB — nobody needs "0,98 MB".
  String get readableSize {
    if (sizeBytes < 1024) return '$sizeBytes B';
    if (sizeBytes < 1024 * 1024) return '${(sizeBytes / 1024).round()} KB';
    return '${(sizeBytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }

  factory MessageAttachment.fromJson(Map<String, dynamic> json) =>
      MessageAttachment(
        id: json['id'] as String? ?? '',
        filename: (json['filename'] as String? ?? '').trim(),
        contentType: json['content_type'] as String? ?? '',
        sizeBytes: (json['size_bytes'] as num?)?.toInt() ?? 0,
        isInline: json['is_inline'] as bool? ?? false,
        scanStatus:
            AttachmentScanStatus.fromWire(json['scan_status'] as String?),
        scanSignature:
            (json['scan_signature'] as String?)?.trim().isEmpty ?? true
                ? null
                : (json['scan_signature'] as String).trim(),
      );
}
