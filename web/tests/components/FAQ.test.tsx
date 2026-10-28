import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { FAQ } from "../../src/components/FAQ";
import { ui } from "../../src/i18n/ui";

// pt-only desde a landing v2: o FAQ é vendas-led (licitação, aprovação
// humana, preço na conversa) e vem em abas por categoria.

function buildFAQT() {
  const src = ui.pt;
  return {
    "faq.title": src["faq.title"],
    "faq.subtitle": src["faq.subtitle"],
    "faq.items": src["faq.items"],
    "faq.tab_billing": src["faq.tab_billing"],
    "faq.tab_technical": src["faq.tab_technical"],
    "faq.tab_sales": src["faq.tab_sales"],
  };
}

describe("FAQ component", () => {
  const t = buildFAQT();
  const items = ui.pt["faq.items"];

  it("renders title and subtitle", () => {
    render(<FAQ t={t} />);
    expect(screen.getByText(ui.pt["faq.title"])).toBeInTheDocument();
    expect(screen.getByText(ui.pt["faq.subtitle"])).toBeInTheDocument();
  });

  it("has the sales-led questions the vertical actually asks", () => {
    const questions = items.map((i) => i.question);
    expect(questions).toContain("Funciona com licitação?");
    expect(questions).toContain("Vocês mandam mensagem sem eu ver?");
    expect(questions).toContain("Quanto custa?");
    // 6–8 per the landing plan: enough to answer, short of a manual.
    expect(items.length).toBeGreaterThanOrEqual(6);
    expect(items.length).toBeLessThanOrEqual(8);
  });

  it("opens on the first tab that has items (no billing questions exist)", () => {
    render(<FAQ t={t} />);
    // With no billing items the old hardcoded default rendered an EMPTY
    // accordion; the first populated category must win.
    const technical = items.filter((i) => i.category === "technical");
    expect(technical.length).toBeGreaterThan(0);
    expect(screen.getByText(technical[0].question)).toBeInTheDocument();
  });

  it("clicking a question reveals its answer", () => {
    render(<FAQ t={t} />);
    const technical = items.filter((i) => i.category === "technical");
    const q = technical[0];

    expect(screen.queryByText(q.answer)).not.toBeInTheDocument();
    fireEvent.click(screen.getByText(q.question));
    expect(screen.getByText(q.answer)).toBeInTheDocument();
  });

  it("switching tabs swaps the question set", () => {
    render(<FAQ t={t} />);
    const sales = items.filter((i) => i.category === "sales");

    fireEvent.click(screen.getByRole("tab", { name: ui.pt["faq.tab_sales"] }));
    expect(screen.getByText(sales[0].question)).toBeInTheDocument();
  });
});
