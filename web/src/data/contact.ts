// Business contact details — single source of truth (US-146).
//
// The WhatsApp support/sales number used by every "Talk to us" CTA on the
// marketing site (the floating FAB and the "Falar com especialista" / sales
// CTAs in Pricing). It MUST be a WhatsApp Business Account (WABA) number, not
// a personal line — a personal number risks being flagged/banned by Meta and
// gives no business-grade reliability.
//
// Overridable per environment via PUBLIC_WHATSAPP_NUMBER (digits only, full
// international format without "+", e.g. 5511900000000) so staging/prod can
// point at the provisioned WABA number without a code change. The committed
// fallback is the business support line.

const env: Record<string, string | undefined> =
  (typeof import.meta !== "undefined" && (import.meta as any).env) || {};

// WABA business support number (digits only, country code first, no "+").
export const WHATSAPP_NUMBER: string =
  env.PUBLIC_WHATSAPP_NUMBER || "5511900000000";

// Build a wa.me deep link with an optional URL-encoded prefilled message.
export const whatsappLink = (encodedText?: string): string =>
  `https://wa.me/${WHATSAPP_NUMBER}${encodedText ? `?text=${encodedText}` : ""}`;

// The calendar page a prospect books a call on (Google Calendar appointment
// schedule). Sales-led since 2026-08-19: the site shows no price list; the
// conversation is the offer. Public URL, set per environment via
// PUBLIC_BOOKING_URL. Empty = the "book a call" CTA falls back to WhatsApp.
export const BOOKING_URL: string = env.PUBLIC_BOOKING_URL || "";
