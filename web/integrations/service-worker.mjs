// US-167: Astro integration that produces a Workbox service worker at build.
//
// Astro 6 has no first-party PWA integration (@vite-pwa/astro peer-deps only up
// to Astro 5), so this small integration owns the pipeline:
//   1. esbuild bundles `src/sw.js` (resolving the workbox-* runtime imports)
//      into a single browser-ready IIFE.
//   2. workbox-build `injectManifest` replaces `self.__WB_MANIFEST` in that
//      bundle with a precache manifest of every hashed asset in `dist/`.
//   3. The result is written to `dist/sw.js` (registered from Layout.astro).
//
// Running in `astro:build:done` guarantees `dist/` is fully populated, so the
// precache manifest covers all emitted CSS/JS/font/image files.
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import fs from 'node:fs/promises';
import { build as esbuild } from 'esbuild';
import { injectManifest } from 'workbox-build';

export default function serviceWorker() {
  return {
    name: 'replider-service-worker',
    hooks: {
      'astro:build:done': async ({ dir, logger }) => {
        const outDir = fileURLToPath(dir); // dist/
        const swSrc = path.resolve('src/sw.js');
        const swTmp = path.join(outDir, 'sw.src.js');
        const swDest = path.join(outDir, 'sw.js');

        // 1. Bundle the worker source so the workbox-* imports resolve.
        await esbuild({
          entryPoints: [swSrc],
          outfile: swTmp,
          bundle: true,
          format: 'iife',
          target: 'es2020',
          minify: true,
          platform: 'browser',
        });

        // 2. Inject the precache manifest of built assets.
        const { count, size, warnings } = await injectManifest({
          swSrc: swTmp,
          swDest,
          globDirectory: outDir,
          globPatterns: [
            '**/*.{js,css,svg,png,jpg,jpeg,webp,avif,gif,ico,woff,woff2,ttf}',
            'offline.html',
          ],
          // Never precache the worker itself, its temp bundle, or sourcemaps.
          globIgnores: ['sw.js', 'sw.src.js', '**/*.map'],
          maximumFileSizeToCacheInBytes: 5 * 1024 * 1024,
        });
        warnings.forEach((w) => logger.warn(w));

        // 3. Drop the intermediate bundle.
        await fs.rm(swTmp, { force: true });

        logger.info(
          `service worker: ${count} assets precached (${(size / 1024).toFixed(1)} KiB)`,
        );
      },
    },
  };
}
