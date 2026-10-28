import { describe, it, expect } from "vitest";
import { ui, defaultLang } from "../../src/i18n/ui";

// pt-only desde a landing v2: o teste de paridade en/pt virou sanidade do
// único dicionário que restou.

describe("i18n translations", () => {
  const ptKeys = Object.keys(ui.pt) as (keyof typeof ui.pt)[];

  it('has "pt" as the default language', () => {
    expect(defaultLang).toBe("pt");
  });

  it("no Portuguese value is an empty string", () => {
    const empty = ptKeys.filter((key) => {
      const val = ui.pt[key];
      return typeof val === "string" && val.trim() === "";
    });
    expect(empty).toEqual([]);
  });

  it("the FAQ items are structured question/answer pairs", () => {
    const items = ui.pt["faq.items"];
    expect(Array.isArray(items)).toBe(true);
    for (const item of items) {
      expect(item.question.trim().length).toBeGreaterThan(0);
      expect(item.answer.trim().length).toBeGreaterThan(0);
    }
  });
});
