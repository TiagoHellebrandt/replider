import React, { useEffect, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { MessageCircle, X } from "lucide-react";
import { whatsappLink } from "../data/contact";

interface Props {
  /** Visible label shown in the expanded tooltip card. */
  label: string;
  /** Hint shown under the label inside the tooltip. */
  subtitle: string;
  /** "Iniciar conversa" / "Start chat" button copy. */
  cta: string;
  /** URL-encoded prefilled message body for wa.me. */
  prefill: string;
}

/**
 * Floating WhatsApp support button. Sits in the bottom-right corner of
 * every page rendered through Layout.astro. Tap-once expands a small
 * card with "start chat" CTA so we don't lose the user to wa.me on a
 * stray click. The WABA business number comes from the shared
 * `whatsappLink` helper (src/data/contact.ts), kept in sync with the
 * same support line surfaced via domain.DefaultContactSales on the API.
 *
 * Persisted dismiss state in sessionStorage — once the user closes the
 * card they don't see it expand again on every page. The pulsing icon
 * stays visible so they can re-open if they change their minds.
 */
export const WhatsAppFab: React.FC<Props> = ({
  label,
  subtitle,
  cta,
  prefill,
}) => {
  const [open, setOpen] = useState(false);
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined") return;
    setDismissed(sessionStorage.getItem("wa-fab-dismissed") === "1");
  }, []);

  const dismiss = () => {
    setOpen(false);
    setDismissed(true);
    if (typeof window !== "undefined") {
      sessionStorage.setItem("wa-fab-dismissed", "1");
    }
  };

  const href = whatsappLink(prefill);

  return (
    <div className="fixed bottom-5 right-5 z-50 flex flex-col items-end gap-3">
      <AnimatePresence>
        {open && !dismissed && (
          <motion.div
            initial={{ opacity: 0, y: 10, scale: 0.95 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 10, scale: 0.95 }}
            transition={{ duration: 0.18 }}
            className="bg-[#0d1418] text-white rounded-2xl shadow-2xl border border-white/10 w-[280px] p-4"
          >
            <div className="flex items-start justify-between gap-3 mb-3">
              <div>
                <div className="font-bold text-sm">{label}</div>
                <div className="text-white/60 text-xs mt-1 leading-relaxed">
                  {subtitle}
                </div>
              </div>
              <button
                onClick={dismiss}
                className="text-white/40 hover:text-white/80 transition-colors"
                aria-label="Close"
              >
                <X className="w-4 h-4" aria-hidden="true" />
              </button>
            </div>
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              className="block w-full bg-[#25D366] hover:bg-[#1eba57] text-white font-bold text-sm text-center py-2.5 rounded-xl transition-colors no-underline"
            >
              {cta}
            </a>
          </motion.div>
        )}
      </AnimatePresence>

      <button
        onClick={() => setOpen((v) => !v)}
        aria-label={label}
        className="relative w-14 h-14 rounded-full bg-[#25D366] hover:bg-[#1eba57] shadow-[0_8px_24px_rgba(37,211,102,0.4)] flex items-center justify-center transition-all hover:scale-105"
      >
        <span className="absolute inset-0 rounded-full bg-[#25D366] opacity-50 animate-ping" />
        <MessageCircle
          className="w-7 h-7 text-white relative z-10"
          aria-hidden="true"
        />
      </button>
    </div>
  );
};
