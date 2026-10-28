import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/products_controller.dart';
import 'package:app/features/sales/domain/models/product_model.dart';
import 'package:app/i18n/strings.g.dart';

/// What the manufacturer's brochure said, on the product's own screen.
///
/// The import was already storing brand, model, code and the source page in
/// the product's attributes — and the edit screen showed none of it. Somebody
/// who had just imported ninety products opened one, saw a name, a short
/// description and "R$ 0,00", and concluded the reader had thrown the
/// brochure away. Most of it was there; nothing rendered it.
///
/// Read-only on purpose. This block is what the FILE said; editing it here
/// would quietly turn a quotable source into someone's memory of it. The
/// fields a person legitimately corrects (name, description, price, cycle)
/// stay in the form above.
class ProductBrochureSheet extends ConsumerStatefulWidget {
  const ProductBrochureSheet({super.key, required this.product});

  final ProductModel product;

  @override
  ConsumerState<ProductBrochureSheet> createState() =>
      _ProductBrochureSheetState();
}

class _ProductBrochureSheetState extends ConsumerState<ProductBrochureSheet> {
  bool _opening = false;

  /// null while loading, empty when the page had no usable artwork. The three
  /// states are distinct on screen: a spinner, an honest "nothing here", or
  /// the pictures. Collapsing loading into empty makes the gallery look broken
  /// for the second it takes to sign the URLs.
  List<_BrochureImage>? _images;
  bool _imagesFailed = false;

  @override
  void initState() {
    super.initState();
    if ((widget.product.sourceId ?? '').isNotEmpty) {
      _loadImages();
    }
  }

  Future<void> _loadImages() async {
    try {
      final res = await ref
          .read(salesRemoteDataSourceProvider)
          .getProductImages(widget.product.id);
      final data = res.data;
      final list = data is List ? data : const <dynamic>[];
      if (!mounted) return;
      setState(() {
        _images = [
          for (final item in list)
            if (item is Map<String, dynamic>) _BrochureImage.fromJson(item),
        ];
      });
    } catch (_) {
      // Pictures are a convenience on top of the catalogue. A failure hides
      // the strip rather than shouting over a screen the person opened to
      // read a spec sheet.
      if (mounted) setState(() => _imagesFailed = true);
    }
  }

  Future<void> _openBrochure() async {
    setState(() => _opening = true);
    try {
      final res = await ref
          .read(salesRemoteDataSourceProvider)
          .getProductSource(widget.product.id);
      final data = res.data;
      final url = data is Map ? data['url'] as String? : null;
      if (url == null || url.isEmpty) {
        throw StateError('no url');
      }
      // A top-level navigation, not an <img>: the signed URL is fetched by the
      // browser itself, so no CORS and no proxy needed.
      await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        key: const Key('product_brochure_error'),
        content: Text(parseApiError(e)),
        behavior: SnackBarBehavior.floating,
      ));
    } finally {
      if (mounted) setState(() => _opening = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final p = widget.product;
    if (!p.hasSheet && (p.sourceId ?? '').isEmpty) {
      return const SizedBox.shrink();
    }
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    final pairs = <(String, String)>[
      if (p.brand.isNotEmpty) (t.sales.products.sheet_brand, p.brand),
      if (p.model.isNotEmpty) (t.sales.products.sheet_model, p.model),
      if (p.sku.isNotEmpty) (t.sales.products.sheet_sku, p.sku),
      if (p.unit.isNotEmpty) (t.sales.products.sheet_unit, p.unit),
    ];

    return Container(
      key: const Key('product_brochure_sheet'),
      margin: const EdgeInsets.only(top: 24),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: colors.glassLight,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: colors.borderLight),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.description_outlined,
                  size: 16, color: colors.textMuted),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  t.sales.products.sheet_title,
                  style: TextStyle(
                    color: colors.textSecondary,
                    fontSize: 12,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 0.6,
                  ),
                ),
              ),
              if ((p.sourceId ?? '').isNotEmpty)
                TextButton.icon(
                  key: const Key('product_open_brochure'),
                  onPressed: _opening ? null : _openBrochure,
                  icon: _opening
                      ? const SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(strokeWidth: 2))
                      : const Icon(Icons.open_in_new, size: 16),
                  label: Text(p.sourcePage > 0
                      ? t.sales.products.open_brochure(page: '${p.sourcePage}')
                      : t.sales.products.open_brochure_no_page),
                ),
            ],
          ),
          if (pairs.isNotEmpty) ...[
            const SizedBox(height: 12),
            Wrap(
              spacing: 24,
              runSpacing: 10,
              children: [
                for (final (label, value) in pairs)
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(label,
                          style:
                              TextStyle(color: colors.textMuted, fontSize: 11)),
                      Text(value,
                          style: TextStyle(
                              color: colors.textPrimary,
                              fontSize: 13,
                              fontWeight: FontWeight.w600)),
                    ],
                  ),
              ],
            ),
          ],
          if (p.specs.isNotEmpty) ...[
            const SizedBox(height: 16),
            Text(t.sales.products.sheet_specs,
                style: TextStyle(color: colors.textMuted, fontSize: 11)),
            const SizedBox(height: 4),
            // SelectableText: half the reason to keep the sheet is copying a
            // line of it into a bid.
            SelectableText(
              p.specs,
              key: const Key('product_specs'),
              style: TextStyle(
                  color: colors.textSecondary, fontSize: 13, height: 1.45),
            ),
          ],
          if ((p.sourceId ?? '').isNotEmpty && !_imagesFailed) _gallery(colors),
          if (p.accessories.isNotEmpty)
            _bullets(colors, t.sales.products.sheet_accessories, p.accessories,
                const Key('product_accessories')),
          if (p.standards.isNotEmpty)
            _bullets(colors, t.sales.products.sheet_standards, p.standards,
                const Key('product_standards')),
        ],
      ),
    );
  }

  /// The pictures printed on this product's page.
  ///
  /// Three distinct states, on purpose: a spinner while the URLs are being
  /// signed, an honest "nothing usable here", or the strip. Collapsing the
  /// first into the second makes a working gallery look broken for the second
  /// it takes to load.
  Widget _gallery(AppColorsExtension colors) {
    final images = _images;
    return Padding(
      padding: const EdgeInsets.only(top: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(t.sales.products.sheet_images,
              style: TextStyle(color: colors.textMuted, fontSize: 11)),
          const SizedBox(height: 8),
          if (images == null)
            const SizedBox(
              key: Key('product_images_loading'),
              height: 96,
              child: Center(
                child: SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ),
            )
          else if (images.isEmpty)
            Text(
              t.sales.products.sheet_images_empty,
              key: const Key('product_images_empty'),
              style: TextStyle(color: colors.textMuted, fontSize: 12),
            )
          else
            SizedBox(
              key: const Key('product_images_strip'),
              height: 120,
              child: ListView.separated(
                scrollDirection: Axis.horizontal,
                itemCount: images.length,
                separatorBuilder: (_, __) => const SizedBox(width: 10),
                itemBuilder: (context, i) {
                  final img = images[i];
                  return Tooltip(
                    message: t.sales.products.image_page(page: '${img.page}'),
                    child: InkWell(
                      onTap: () => launchUrl(Uri.parse(img.url),
                          mode: LaunchMode.externalApplication),
                      borderRadius: BorderRadius.circular(10),
                      child: ClipRRect(
                        borderRadius: BorderRadius.circular(10),
                        child: AppNetworkImage(
                          url: img.url,
                          width: 140,
                          height: 120,
                          fit: BoxFit.cover,
                        ),
                      ),
                    ),
                  );
                },
              ),
            ),
        ],
      ),
    );
  }

  Widget _bullets(
      AppColorsExtension colors, String label, List<String> items, Key key) {
    return Padding(
      key: key,
      padding: const EdgeInsets.only(top: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: TextStyle(color: colors.textMuted, fontSize: 11)),
          const SizedBox(height: 4),
          for (final item in items)
            Padding(
              padding: const EdgeInsets.only(top: 2),
              child: Text('• $item',
                  style: TextStyle(color: colors.textSecondary, fontSize: 13)),
            ),
        ],
      ),
    );
  }
}

/// One picture out of the brochure, as the API hands it over.
class _BrochureImage {
  const _BrochureImage({required this.url, required this.page});

  final String url;
  final int page;

  factory _BrochureImage.fromJson(Map<String, dynamic> json) => _BrochureImage(
        url: (json['url'] as String?) ?? '',
        page: (json['page'] as num?)?.toInt() ?? 0,
      );
}
