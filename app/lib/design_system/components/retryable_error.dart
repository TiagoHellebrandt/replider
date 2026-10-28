import 'package:flutter/material.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

/// Inline error block with a retry action. Replaces the dozen slightly
/// different "Erro ao carregar X" copies scattered across list screens,
/// and — more importantly — gives the user a way OUT. A lot of our
/// screens used to show the error and nothing else, leaving the user
/// stuck on a static message.
///
/// Usage:
///
///   asyncValue.when(
///     data: (list) => ...,
///     loading: () => const Center(child: CircularProgressIndicator()),
///     error: (err, _) => RetryableError(
///       error: err,
///       onRetry: () => ref.invalidate(myProvider),
///     ),
///   );
class RetryableError extends StatelessWidget {
  final Object? error;
  final VoidCallback onRetry;

  /// Optional copy shown above the reason. Defaults to a generic "we
  /// couldn't load this" — screens usually don't need to override.
  final String? title;

  const RetryableError({
    super.key,
    required this.error,
    required this.onRetry,
    this.title,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, color: colors.redAlert, size: 48),
            const SizedBox(height: 12),
            Text(
              title ?? 'Não foi possível carregar',
              style: TextStyle(
                  color: colors.textPrimary,
                  fontSize: 16,
                  fontWeight: FontWeight.w600),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 6),
            Text(
              parseApiError(error),
              style: TextStyle(color: colors.textMuted, fontSize: 12),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            TextButton.icon(
              onPressed: onRetry,
              icon: const Icon(Icons.refresh, size: 16),
              label: Text(t.auth.retry),
            ),
          ],
        ),
      ),
    );
  }
}
