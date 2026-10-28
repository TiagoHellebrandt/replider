import React, { useMemo, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { X } from "lucide-react";
import { trackEvent } from "../lib/analytics";

const emailRegex = /^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/;

type Status = "idle" | "loading" | "success" | "error";

const API_BASE_URL =
  (import.meta as any).env?.PUBLIC_API_BASE_URL || "https://api.replider.com";

// Static, locale-agnostic option values (proper-noun channels + numeric
// ranges) — no translation needed, they read the same in every language.
const SIZE_OPTIONS = ["1-10", "11-50", "51-200", "201-1000", "1000+"];
const VOLUME_OPTIONS = ["< 1k", "1k - 10k", "10k - 50k", "50k - 200k", "200k+"];
const CHANNEL_OPTIONS = ["WhatsApp", "Telegram", "Instagram", "Other"];

export interface SalesModalProps {
  t: Record<string, string>;
  open: boolean;
  onClose: () => void;
}

export const SalesModal: React.FC<SalesModalProps> = ({ t, open, onClose }) => {
  const [company, setCompany] = useState("");
  const [email, setEmail] = useState("");
  const [size, setSize] = useState("");
  const [channels, setChannels] = useState<string[]>([]);
  const [volume, setVolume] = useState("");
  const [message, setMessage] = useState("");
  // Honeypot: hidden from real users; only bots fill it. The API silently
  // drops any submission with a non-empty "website" value.
  const [website, setWebsite] = useState("");
  const [status, setStatus] = useState<Status>("idle");
  const [touched, setTouched] = useState(false);

  const isValid = useMemo(
    () =>
      company.trim().length > 0 &&
      emailRegex.test(email.trim()) &&
      size.length > 0 &&
      channels.length > 0 &&
      volume.length > 0,
    [company, email, size, channels, volume],
  );

  const toggleChannel = (channel: string) => {
    setChannels((prev) =>
      prev.includes(channel)
        ? prev.filter((c) => c !== channel)
        : [...prev, channel],
    );
    if (status === "error") setStatus("idle");
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setTouched(true);
    if (!isValid) return;

    setStatus("loading");
    try {
      const res = await fetch(`${API_BASE_URL}/v1/sales/qualify`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          company: company.trim(),
          email: email.trim(),
          company_size: size,
          channels: channels.join(", "),
          monthly_volume: volume,
          message: message.trim(),
          website,
        }),
      });
      if (res.ok) {
        setStatus("success");
        trackEvent("Lead", { plan: "enterprise" });
      } else {
        setStatus("error");
      }
    } catch {
      setStatus("error");
    }
  };

  const inputClass =
    "w-full px-4 py-3 rounded-xl bg-white/5 border border-white/10 text-white text-sm placeholder-white/30 focus:outline-none focus:border-white/30 transition-colors";

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          className="fixed inset-0 z-[100] flex items-center justify-center p-4"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          role="dialog"
          aria-modal="true"
          aria-label={t["pricing.sales_form_title"]}
        >
          {/* Backdrop */}
          <div
            className="absolute inset-0 bg-black/70 backdrop-blur-sm"
            onClick={onClose}
          />

          <motion.div
            className="relative w-full max-w-lg max-h-[90vh] overflow-y-auto rounded-3xl border border-white/10 bg-[#0b0b12] p-6 sm:p-8 shadow-2xl"
            initial={{ opacity: 0, scale: 0.95, y: 20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: 20 }}
            transition={{ duration: 0.2 }}
          >
            <button
              type="button"
              onClick={onClose}
              aria-label={t["pricing.sales_form_close"]}
              className="absolute top-4 right-4 p-2 rounded-lg text-white/40 hover:text-white hover:bg-white/5 transition-colors"
            >
              <X className="w-5 h-5" />
            </button>

            {status === "success" ? (
              <div className="text-center py-10">
                <h3 className="text-2xl font-black text-white mb-3">
                  {t["pricing.sales_form_title"]}
                </h3>
                <p className="text-green-400 text-base">
                  {t["pricing.sales_form_success"]}
                </p>
                <button
                  type="button"
                  onClick={onClose}
                  className="mt-8 px-6 py-3 rounded-2xl bg-white text-black font-black text-sm hover:bg-white/90 transition-colors"
                >
                  {t["pricing.sales_form_close"]}
                </button>
              </div>
            ) : (
              <>
                <h3 className="text-2xl font-black text-white mb-1 pr-8">
                  {t["pricing.sales_form_title"]}
                </h3>
                <p className="text-white/50 text-sm mb-6">
                  {t["pricing.sales_form_subtitle"]}
                </p>

                <form onSubmit={handleSubmit} noValidate className="space-y-4">
                  {/* Honeypot */}
                  <input
                    type="text"
                    name="website"
                    tabIndex={-1}
                    autoComplete="off"
                    aria-hidden="true"
                    value={website}
                    onChange={(e) => setWebsite(e.target.value)}
                    className="absolute left-[-9999px] h-0 w-0 opacity-0"
                    style={{ position: "absolute", left: "-9999px" }}
                  />

                  <div>
                    <label
                      htmlFor="sales-company"
                      className="block text-white/70 text-xs font-semibold mb-1.5"
                    >
                      {t["pricing.sales_form_company"]} *
                    </label>
                    <input
                      id="sales-company"
                      type="text"
                      value={company}
                      onChange={(e) => setCompany(e.target.value)}
                      className={inputClass}
                      disabled={status === "loading"}
                    />
                  </div>

                  <div>
                    <label
                      htmlFor="sales-email"
                      className="block text-white/70 text-xs font-semibold mb-1.5"
                    >
                      {t["pricing.sales_form_email"]} *
                    </label>
                    <input
                      id="sales-email"
                      type="email"
                      inputMode="email"
                      autoComplete="email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      className={inputClass}
                      disabled={status === "loading"}
                    />
                  </div>

                  <div>
                    <label
                      htmlFor="sales-size"
                      className="block text-white/70 text-xs font-semibold mb-1.5"
                    >
                      {t["pricing.sales_form_size"]} *
                    </label>
                    <select
                      id="sales-size"
                      value={size}
                      onChange={(e) => setSize(e.target.value)}
                      className={`${inputClass} appearance-none`}
                      disabled={status === "loading"}
                    >
                      <option value="" disabled>
                        {t["pricing.sales_form_size_placeholder"]}
                      </option>
                      {SIZE_OPTIONS.map((opt) => (
                        <option key={opt} value={opt} className="bg-[#0b0b12]">
                          {opt}
                        </option>
                      ))}
                    </select>
                  </div>

                  <div>
                    <span className="block text-white/70 text-xs font-semibold mb-1.5">
                      {t["pricing.sales_form_channels"]} *
                    </span>
                    <div className="flex flex-wrap gap-2">
                      {CHANNEL_OPTIONS.map((channel) => {
                        const active = channels.includes(channel);
                        return (
                          <button
                            key={channel}
                            type="button"
                            onClick={() => toggleChannel(channel)}
                            aria-pressed={active}
                            disabled={status === "loading"}
                            className={`px-3.5 py-2 rounded-xl text-xs font-semibold border transition-colors ${
                              active
                                ? "bg-blue-600 border-blue-500 text-white"
                                : "bg-white/5 border-white/10 text-white/60 hover:border-white/30"
                            }`}
                          >
                            {channel}
                          </button>
                        );
                      })}
                    </div>
                  </div>

                  <div>
                    <label
                      htmlFor="sales-volume"
                      className="block text-white/70 text-xs font-semibold mb-1.5"
                    >
                      {t["pricing.sales_form_volume"]} *
                    </label>
                    <select
                      id="sales-volume"
                      value={volume}
                      onChange={(e) => setVolume(e.target.value)}
                      className={`${inputClass} appearance-none`}
                      disabled={status === "loading"}
                    >
                      <option value="" disabled>
                        {t["pricing.sales_form_volume_placeholder"]}
                      </option>
                      {VOLUME_OPTIONS.map((opt) => (
                        <option key={opt} value={opt} className="bg-[#0b0b12]">
                          {opt}
                        </option>
                      ))}
                    </select>
                  </div>

                  <div>
                    <label
                      htmlFor="sales-message"
                      className="block text-white/70 text-xs font-semibold mb-1.5"
                    >
                      {t["pricing.sales_form_message"]}
                    </label>
                    <textarea
                      id="sales-message"
                      value={message}
                      onChange={(e) => setMessage(e.target.value)}
                      rows={3}
                      className={`${inputClass} resize-none`}
                      disabled={status === "loading"}
                    />
                  </div>

                  {touched && !isValid && (
                    <p role="alert" className="text-red-400 text-xs">
                      {t["pricing.sales_form_required"]}
                    </p>
                  )}
                  {status === "error" && (
                    <p role="alert" className="text-red-400 text-xs">
                      {t["pricing.sales_form_error"]}
                    </p>
                  )}

                  <button
                    type="submit"
                    disabled={status === "loading"}
                    className="w-full py-3.5 rounded-2xl bg-white text-black font-black text-sm hover:bg-white/90 disabled:bg-white/40 disabled:cursor-not-allowed transition-colors"
                  >
                    {status === "loading"
                      ? t["pricing.sales_form_submitting"]
                      : t["pricing.sales_form_submit"]}
                  </button>
                </form>
              </>
            )}
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
};
