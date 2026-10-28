package domain

// ContactSales is the canonical list of channels the dashboard surfaces
// when a customer hits a contact-required plan (Partner / Enterprise).
// Centralising these keeps the API + Flutter app + marketing site in
// sync — every surface that needs to render "Falar com especialista"
// pulls from here. Update this struct, redeploy, and the prompt
// copy/links are correct everywhere.
type ContactSalesChannels struct {
	Email        string `json:"email"`
	WhatsApp     string `json:"whatsapp"`      // human-readable, e.g. "+55 11 90000-0000"
	WhatsAppLink string `json:"whatsapp_link"` // wa.me deep-link for one-click chat
	WhatsAppText string `json:"whatsapp_text"` // suggested prefilled message body (URL-decoded)
	// BookingURL is the calendar page where a prospect books a call
	// (Google Calendar appointment schedule). Sales-led since 2026-08-19:
	// the product shows no price list; the conversation is the offer. Empty
	// when unconfigured, and the clients then fall back to WhatsApp/e-mail.
	BookingURL string `json:"booking_url,omitempty"`
}

// DefaultContactSales returns the production contact details. Hard-coded
// so a misconfigured env var can't accidentally hide the contact CTA on
// the customer-facing billing screen — these are public-facing channels
// staffed by the sales team.
func DefaultContactSales() ContactSalesChannels {
	return ContactSalesChannels{
		Email:        "support@replider.com",
		WhatsApp:     "+55 11 90000-0000",
		WhatsAppLink: "https://wa.me/5511900000000?text=" + waPrefill,
		WhatsAppText: "Olá! Quero conhecer o Replider para a minha empresa.",
	}
}

// waPrefill is the URL-encoded form of WhatsAppText for embedding in
// the wa.me link directly. Kept as a private constant so a typo in one
// place can't drift from the human-readable WhatsAppText above.
const waPrefill = "Ol%C3%A1!%20Quero%20conhecer%20o%20Replider%20para%20a%20minha%20empresa."
