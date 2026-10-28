import { useState } from "react";
import { SalesModal } from "./SalesModal";
import { BOOKING_URL, whatsappLink } from "../data/contact";

// "Let's talk about your operation" — the section that replaced the price
// list (sales-led, 2026-08-19).
//
// A hospital-equipment distributor does not read three tiers and pick one;
// she books a call and hears a number that fits her operation. So the site
// shows no plans. This section offers the conversation: the booking page
// when one is configured (PUBLIC_BOOKING_URL), WhatsApp, and the sales form
// for whoever would rather write.

interface BookCallProps {
  t: Record<string, string>;
}

const WA_PREFILL =
  "Ol%C3%A1!%20Quero%20conhecer%20o%20Replider%20para%20a%20minha%20empresa.";

export const BookCall: React.FC<BookCallProps> = ({ t }) => {
  const [salesOpen, setSalesOpen] = useState(false);
  const hasBooking = BOOKING_URL.length > 0;
  const wa = whatsappLink(WA_PREFILL);

  const primary =
    "inline-flex items-center justify-center gap-2 rounded-full bg-blue-600 px-7 py-3.5 text-sm font-bold text-white shadow-[0_0_30px_rgba(37,99,235,0.35)] transition hover:bg-blue-500 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-400";
  const secondary =
    "inline-flex items-center justify-center gap-2 rounded-full border border-white/15 px-7 py-3.5 text-sm font-semibold text-white/80 transition hover:border-white/40 hover:text-white focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-400";

  return (
    <section
      id="agendar"
      className="relative px-6 py-24 md:py-32"
      aria-labelledby="book-call-title"
    >
      {/* Old deep links (#pricing) still land here. */}
      <span id="pricing" className="absolute -top-24" aria-hidden="true" />
      <div className="mx-auto max-w-3xl text-center">
        <p className="mb-4 text-xs font-bold uppercase tracking-[0.3em] text-blue-400">
          {t["book.eyebrow"]}
        </p>
        <h2
          id="book-call-title"
          className="text-3xl font-bold tracking-tight text-white md:text-5xl"
        >
          {t["book.title"]}
        </h2>
        <p className="mx-auto mt-5 max-w-2xl text-base text-white/60 md:text-lg">
          {t["book.subtitle"]}
        </p>

        <div className="mt-10 flex flex-col items-center justify-center gap-3 sm:flex-row">
          {hasBooking ? (
            <a
              href={BOOKING_URL}
              target="_blank"
              rel="noopener noreferrer"
              className={primary}
              data-testid="book-call-primary"
            >
              {t["book.cta"]}
            </a>
          ) : (
            <a
              href={wa}
              target="_blank"
              rel="noopener noreferrer"
              className={primary}
              data-testid="book-call-primary"
            >
              {t["book.whatsapp"]}
            </a>
          )}
          {hasBooking && (
            <a
              href={wa}
              target="_blank"
              rel="noopener noreferrer"
              className={secondary}
            >
              {t["book.whatsapp"]}
            </a>
          )}
          <button
            type="button"
            onClick={() => setSalesOpen(true)}
            className={secondary}
          >
            {t["book.write"]}
          </button>
        </div>

        <p className="mt-8 text-sm text-white/40">{t["book.note"]}</p>
      </div>

      <SalesModal t={t} open={salesOpen} onClose={() => setSalesOpen(false)} />
    </section>
  );
};
