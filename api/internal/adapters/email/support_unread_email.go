package email

import "context"

// SendSupportUnreadEmail is the canal-interno US-004 fallback: the thread
// owner hasn't read a platform-side support reply within the configured
// window, so they get ONE email pointing back at the widget. Fired by
// services.SupportFallbackWorker through its narrow
// SupportFallbackEmailSender port; locale resolves from the customer
// account's billing currency (domain.EmailLocaleForCurrency).
func (a *SMTPAdapter) SendSupportUnreadEmail(to, userName, ctaURL, locale string) error {
	loc := resolveLocale(locale)
	d := a.brand(loc)
	d.BrandColor = "#3B82F6" // blue accent — informational, not alarming

	d.Subject = tr(loc, "support_unread_subject")
	d.HeaderTitle = tr(loc, "support_unread_title")
	d.Greeting = greet(loc, userName)
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` + escapeHTML(tr(loc, "support_unread_intro")) + `</p>`)
	d.CTALabel = tr(loc, "support_unread_cta")
	d.CTAURL = ctaURL
	d.FootnoteHTML = asHTML(escapeHTML(tr(loc, "support_unread_footnote")))
	d.PlainText = tr(loc, "support_unread_intro") + "\n\n" + ctaURL +
		"\n\n" + tr(loc, "support_unread_footnote") + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}
