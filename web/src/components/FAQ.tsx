import React, { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { ChevronDown } from "lucide-react";

type FAQCategory = "billing" | "technical" | "sales";

interface FAQItem {
  question: string;
  answer: string;
  category?: FAQCategory;
}

const CATEGORY_ORDER: FAQCategory[] = ["billing", "technical", "sales"];

export const FAQ: React.FC<{ t: any }> = ({ t }) => {
  const allItems: FAQItem[] = t["faq.items"] || [];
  // The first tab that actually has questions. Hardcoding "billing" showed
  // an empty accordion whenever a dictionary shipped no billing items —
  // which is exactly what the sales-led FAQ does.
  const firstCategory =
    CATEGORY_ORDER.find((cat) =>
      allItems.some((item) => (item.category ?? "billing") === cat),
    ) ?? "billing";
  const [activeCategory, setActiveCategory] =
    useState<FAQCategory>(firstCategory);
  const [openIndex, setOpenIndex] = useState<number | null>(null);

  const tabLabels: Record<FAQCategory, string> = {
    billing: t["faq.tab_billing"] || "Billing",
    technical: t["faq.tab_technical"] || "Technical",
    sales: t["faq.tab_sales"] || "Sales",
  };

  // Only show tabs that actually have items.
  const categories = CATEGORY_ORDER.filter((cat) =>
    allItems.some((item) => item.category === cat),
  );

  const items = allItems.filter(
    (item) => (item.category ?? "billing") === activeCategory,
  );

  const selectCategory = (category: FAQCategory) => {
    setActiveCategory(category);
    setOpenIndex(null); // keep accordion closed on tab switch
  };

  const toggle = (index: number) => {
    setOpenIndex(openIndex === index ? null : index);
  };

  return (
    <section className="py-16 md:py-24 px-4 sm:px-6 bg-[#030303] relative overflow-hidden">
      <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] bg-indigo-600/5 blur-[150px] rounded-full pointer-events-none" />

      <div className="max-w-3xl mx-auto relative z-10">
        <div className="text-center mb-16">
          <motion.h2
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            className="text-3xl md:text-5xl font-black mb-4 md:mb-6 bg-gradient-to-b from-white to-white/60 bg-clip-text text-transparent"
          >
            {t["faq.title"]}
          </motion.h2>
          <motion.p
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ delay: 0.1 }}
            className="text-white/50 text-lg max-w-2xl mx-auto"
          >
            {t["faq.subtitle"]}
          </motion.p>
        </div>

        <div
          className="flex flex-wrap justify-center gap-2 mb-8"
          role="tablist"
          aria-label={t["faq.title"]}
        >
          {categories.map((category) => (
            <button
              key={category}
              role="tab"
              aria-selected={activeCategory === category}
              onClick={() => selectCategory(category)}
              className={`px-5 py-2 rounded-full text-sm font-semibold transition-all duration-200 cursor-pointer border ${
                activeCategory === category
                  ? "bg-blue-500/15 border-blue-500/40 text-white"
                  : "bg-white/[0.02] border-white/5 text-white/50 hover:text-white/80 hover:border-white/15"
              }`}
            >
              {tabLabels[category]}
            </button>
          ))}
        </div>

        <div className="space-y-3">
          {items.map((item, index) => (
            <motion.div
              key={index}
              initial={{ opacity: 0, y: 10 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ delay: index * 0.05 }}
              className={`rounded-2xl border transition-all duration-300 ${
                openIndex === index
                  ? "bg-white/[0.04] border-blue-500/30"
                  : "bg-white/[0.02] border-white/5 hover:border-white/15"
              }`}
            >
              <button
                onClick={() => toggle(index)}
                aria-expanded={openIndex === index}
                className="w-full flex items-center justify-between p-5 text-left cursor-pointer"
              >
                <span className="text-white font-semibold text-sm md:text-base pr-4">
                  {item.question}
                </span>
                <motion.div
                  animate={{ rotate: openIndex === index ? 180 : 0 }}
                  transition={{ duration: 0.2 }}
                  className="shrink-0"
                >
                  <ChevronDown
                    className="w-5 h-5 text-white/40"
                    aria-hidden="true"
                  />
                </motion.div>
              </button>

              <AnimatePresence>
                {openIndex === index && (
                  <motion.div
                    initial={{ height: 0, opacity: 0 }}
                    animate={{ height: "auto", opacity: 1 }}
                    exit={{ height: 0, opacity: 0 }}
                    transition={{ duration: 0.25, ease: "easeInOut" }}
                    className="overflow-hidden"
                  >
                    <div className="px-5 pb-5 text-white/60 text-sm leading-relaxed">
                      {item.answer}
                    </div>
                  </motion.div>
                )}
              </AnimatePresence>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
};
