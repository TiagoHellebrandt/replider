// US-158: Blog CMS via Astro Content Collections (Content Layer API).
//
// Replaces the hand-maintained `src/data/blog-posts.ts` array. Posts are MDX
// files under `src/content/blog/{en,pt}/<slug>.mdx`; the glob loader discovers
// them at build time and the schema validates frontmatter. The per-slug route
// (`src/pages/[lang]/blog/[slug].astro`) and the listing (`blog.astro`) both
// read this collection, so adding a post = dropping in one MDX file.
import { defineCollection, z } from "astro:content";
import { glob } from "astro/loaders";

const blog = defineCollection({
  loader: glob({ pattern: "**/*.{md,mdx}", base: "./src/content/blog" }),
  schema: z.object({
    title: z.string(),
    excerpt: z.string(),
    date: z.coerce.date(),
    tag: z.string(),
    lang: z.enum(["en", "pt"]),
    slug: z.string(),
  }),
});

export const collections = { blog };
