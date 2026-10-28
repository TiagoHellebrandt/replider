import 'package:flutter/material.dart';

import 'package:app/design_system/components/cached_network_image.dart';

/// Avatar circle with a deterministic gradient derived from the
/// contact's identifier (name or phone). Same input always produces
/// the same colors so the customer's avatar stays stable across
/// renders, sessions, and devices — without that, an angry customer
/// would visually "change" between conversations.
///
/// Falls back to two-letter initials (first + last name) for empty
/// avatar URLs. Image errors silently switch to initials too so a
/// 404 from Meta CDN doesn't paint a broken-image icon.
class GradientAvatar extends StatelessWidget {
  final String seed;
  final String name;
  final String? imageUrl;
  final double radius;
  final TextStyle? textStyle;

  const GradientAvatar({
    super.key,
    required this.seed,
    required this.name,
    this.imageUrl,
    this.radius = 22,
    this.textStyle,
  });

  @override
  Widget build(BuildContext context) {
    final hasImage = imageUrl != null && imageUrl!.isNotEmpty;
    final size = radius * 2;
    final initials = _initials(name);
    final gradient = _gradientFor(seed.isNotEmpty ? seed : name);

    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        gradient: hasImage ? null : gradient,
      ),
      clipBehavior: Clip.antiAlias,
      child: hasImage
          ? AppNetworkImage(
              url: imageUrl,
              width: size,
              height: size,
              renderWidth: size,
              fit: BoxFit.cover,
              // Shown on both an empty URL and a CDN load error so a 404
              // from Meta paints the gradient initials, not a broken icon.
              fallback: _initialsLayer(initials, gradient),
            )
          : _initialsLayer(initials, gradient),
    );
  }

  Widget _initialsLayer(String initials, LinearGradient gradient) {
    return Container(
      decoration: BoxDecoration(gradient: gradient),
      alignment: Alignment.center,
      child: Text(
        initials,
        style: textStyle ??
            TextStyle(
              color: Colors.white,
              fontSize: radius * 0.85,
              fontWeight: FontWeight.w700,
              letterSpacing: 0.5,
            ),
      ),
    );
  }

  static String _initials(String name) {
    final cleaned = name.trim();
    if (cleaned.isEmpty) return '?';
    final parts =
        cleaned.split(RegExp(r'\s+')).where((p) => p.isNotEmpty).toList();
    if (parts.length == 1) return parts.first[0].toUpperCase();
    return (parts.first[0] + parts.last[0]).toUpperCase();
  }

  // FNV-1a 32-bit hash → palette index. Same string always picks the
  // same gradient pair regardless of platform — Dart's String.hashCode
  // is intentionally non-stable across releases, so we hash explicitly.
  static LinearGradient _gradientFor(String input) {
    var hash = 0x811c9dc5;
    for (final c in input.codeUnits) {
      hash ^= c;
      hash = (hash * 0x01000193) & 0xFFFFFFFF;
    }
    final pair = _palette[hash % _palette.length];
    return LinearGradient(
      begin: Alignment.topLeft,
      end: Alignment.bottomRight,
      colors: pair,
    );
  }
}

// Curated pairs that hold up on dark backgrounds — second color is
// always darker / more saturated so the white initials stay legible.
const List<List<Color>> _palette = [
  [Color(0xFF8B5CF6), Color(0xFF6366F1)], // violet → indigo
  [Color(0xFFEC4899), Color(0xFFBE185D)], // pink → rose
  [Color(0xFF14B8A6), Color(0xFF0F766E)], // teal
  [Color(0xFFF59E0B), Color(0xFFB45309)], // amber
  [Color(0xFF3B82F6), Color(0xFF1D4ED8)], // blue
  [Color(0xFF10B981), Color(0xFF047857)], // emerald
  [Color(0xFFEF4444), Color(0xFFB91C1C)], // red
  [Color(0xFF8B5CF6), Color(0xFFA855F7)], // violet → fuchsia
  [Color(0xFF06B6D4), Color(0xFF0E7490)], // cyan
  [Color(0xFFEAB308), Color(0xFFA16207)], // yellow → ochre
  [Color(0xFF22C55E), Color(0xFF15803D)], // green
  [Color(0xFFA855F7), Color(0xFF7E22CE)], // purple
];
