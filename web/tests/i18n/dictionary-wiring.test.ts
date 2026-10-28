import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// The React islands don't receive the `t` FUNCTION — each one gets a plain
// object assembled by hand in `[lang]/index.astro`, key by key. Nothing checks
// that the spellings on the two sides agree, and a mismatch fails SILENTLY:
// the lookup returns undefined and the string simply isn't rendered.
//
// That is how the pricing section shipped to production with no <h2> at all
// (`title:` in the dictionary vs `t["pricing.title"]` in the component) and how
// the free-tier card fell back to its hardcoded English strings on the
// Portuguese page — both live on replider.com/pt until this was found by
// reading the DOM.
//
// This walks every dictionary/component pair and fails on any key a component
// reads that its dictionary does not provide.

const ROOT = join(__dirname, "..", "..");
const ASTRO = readFileSync(
  join(ROOT, "src", "pages", "[lang]", "index.astro"),
  "utf-8",
);

/** Dictionary name in index.astro → the component it is passed to. */
const PAIRS: Array<[string, string]> = [
  // Pricing.tsx / FeatureComparison.tsx left the landing on 2026-08-19
  // (sales-led: no price list); BookCall.tsx took their place and carries
  // the sales form, which reads the `pricing.sales_form_*` keys.
  ["bookT", "BookCall.tsx"],
  ["bookT", "SalesModal.tsx"],
  ["faqT", "FAQ.tsx"],
];

/** Extracts a `const <name> = { … }` literal by brace matching. */
function dictionaryBody(name: string): string {
  const start = ASTRO.search(new RegExp(`^const ${name} = \\{$`, "m"));
  if (start < 0) throw new Error(`dictionary ${name} not found in index.astro`);
  let i = ASTRO.indexOf("{", start) + 1;
  let depth = 1;
  while (depth > 0 && i < ASTRO.length) {
    if (ASTRO[i] === "{") depth++;
    else if (ASTRO[i] === "}") depth--;
    i++;
  }
  return ASTRO.slice(start, i);
}

/** Keys the dictionary actually provides, quoted or bare. */
function providedKeys(body: string): Set<string> {
  const keys = new Set<string>();
  for (const m of body.matchAll(/"([^"]+)":/g)) keys.add(m[1]);
  for (const m of body.matchAll(/^\s*([A-Za-z_][A-Za-z0-9_]*):/gm)) {
    keys.add(m[1]);
  }
  return keys;
}

/** Keys the component reads as t["…"]. */
function consumedKeys(file: string): Set<string> {
  const src = readFileSync(join(ROOT, "src", "components", file), "utf-8");
  const keys = new Set<string>();
  for (const m of src.matchAll(/t\["([^"]+)"\]/g)) keys.add(m[1]);
  return keys;
}

describe("island translation dictionaries", () => {
  for (const [dict, component] of PAIRS) {
    it(`${dict} provides every key ${component} reads`, () => {
      const provided = providedKeys(dictionaryBody(dict));
      const missing = [...consumedKeys(component)]
        .filter((k) => !provided.has(k))
        .sort();

      expect(
        missing,
        `${component} reads keys that ${dict} never puts in the object, so ` +
          `they render as nothing (or fall back to hardcoded English):\n  ` +
          `${missing.join("\n  ")}`,
      ).toEqual([]);
    });
  }

  it("the pt dictionary has no empty values", () => {
    // pt-only desde a landing v2 — o que sobrou de paridade é sanidade:
    // nenhuma chave vazia (uma chave vazia renderiza como nada, em silêncio).
    const ui = readFileSync(join(ROOT, "src", "i18n", "ui.ts"), "utf-8");
    const empties = [...ui.matchAll(/"([^"]+)":\s*""/g)].map((m) => m[1]);
    expect(empties).toEqual([]);
  });
});
