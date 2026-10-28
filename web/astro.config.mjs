// @ts-check
import { defineConfig } from "astro/config";

import tailwindcss from "@tailwindcss/vite";
import react from "@astrojs/react";
import mdx from "@astrojs/mdx";
import sitemap from "@astrojs/sitemap";
import serviceWorker from "./integrations/service-worker.mjs";

const SITE = "https://replider.com";

// https://astro.build/config
export default defineConfig({
  // US-138: `site` is required for @astrojs/sitemap to emit absolute URLs.
  site: SITE,

  vite: {
    plugins: [tailwindcss()],
  },

  // US-138: @astrojs/sitemap walks every statically-generated route at build
  // time (all pages + blog) and emits sitemap-index.xml + sitemap-0.xml. The
  // i18n block mirrors the site's en/pt routing so each URL gets hreflang
  // alternates inside the sitemap.
  integrations: [
    react(),
    // US-158: MDX powers the blog Content Collection (`src/content/blog/`).
    mdx(),
    // US-158: Blog posts are now real per-slug routes
    // (`[lang]/blog/[slug].astro`), so @astrojs/sitemap auto-discovers them at
    // build time — the old `customPages` shim from `blog-posts.ts` is gone.
    sitemap({
      i18n: {
        defaultLocale: "pt",
        locales: { pt: "pt" },
      },
    }),
    // US-167: Workbox service worker for repeat-visit performance. Runs after
    // the static build so its precache manifest covers every emitted asset.
    serviceWorker(),
  ],

  // US-133: Built-in image optimization (Astro 6 `astro:assets`, replaces the
  // deprecated @astrojs/image integration). Sharp transforms <Image>/<Picture>
  // sources to AVIF/WebP with responsive srcsets at build time.
  image: {
    service: { entrypoint: "astro/assets/services/sharp" },
    responsiveStyles: true,
  },

  i18n: {
    defaultLocale: "pt",
    locales: ["pt"],
    routing: {
      // A home vive em /pt/ desde sempre; manter o prefixo preserva toda
      // URL já indexada e simplifica os 301 do corte.
      prefixDefaultLocale: true,
    },
  },

  // Landing v2 (2026-08-20): o site de atendimento saiu do ar. Cada página
  // apagada responde 301 — páginas some, link não. /en/* inteiro cai em
  // /pt/* (pt-only); o que não existe mais em /pt cai na home.
  redirects: buildRedirects(),
});

function buildRedirects() {
  /** Páginas apagadas no corte (verticais, comparações, blog, etc.). */
  const removed = [
    "accounting-firms",
    "advogado",
    "auto-repair",
    "chiropractors",
    "clinica",
    "dental-clinics",
    "ecommerce",
    "gyms-fitness",
    "home-cleaning",
    "hvac",
    "law-firms",
    "med-spas",
    "medical-clinics",
    "plumbing",
    "real-estate",
    "restaurants",
    "roofing",
    "salons-barbershops",
    "veterinary",
    "roi",
    "templates",
    "industries",
    "blog",
    "compare",
  ];
  /** @type {Record<string, import("astro").RedirectConfig>} */
  const out = {};
  for (const slug of removed) {
    out[`/pt/${slug}`] = { status: 301, destination: "/pt/" };
    out[`/en/${slug}`] = { status: 301, destination: "/pt/" };
  }
  // "Features" virou a seção Como funciona da home.
  out["/pt/features"] = { status: 301, destination: "/pt/#como-funciona" };
  out["/en/features"] = { status: 301, destination: "/pt/#como-funciona" };
  // pt-only: o inglês aponta pro equivalente em português, página a
  // página (um destino dinâmico exigiria uma rota dinâmica que não existe).
  out["/en"] = { status: 301, destination: "/pt/" };
  for (const page of ["privacy", "terms", "dpa"]) {
    out[`/en/${page}`] = { status: 301, destination: `/pt/${page}` };
  }
  return out;
}
