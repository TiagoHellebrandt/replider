package whatsapp

// WebhookPayload represents the root structure of a WhatsApp Cloud API webhook request
type WebhookPayload struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

// Entry contains an array of changes related to a specific WhatsApp Business Account
type Entry struct {
	ID      string   `json:"id"`
	Time    int64    `json:"time"` // unix-seconds when the event occurred — used as a replay-attack guard
	Changes []Change `json:"changes"`
}

// Change represents the specific notification/event from WhatsApp
type Change struct {
	Value Value  `json:"value"`
	Field string `json:"field"`
}

// Value contains the details of the message, statuses, and contacts
type Value struct {
	MessagingProduct string    `json:"messaging_product"`
	Metadata         Metadata  `json:"metadata"`
	Contacts         []Contact `json:"contacts"`
	Messages         []Message `json:"messages"`
	Statuses         []Status  `json:"statuses"`
}

// Metadata includes the phone number details of the business receiving the message
type Metadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

// Contact contains user profile info
type Contact struct {
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
	WaID string `json:"wa_id"`
}

// Message represents the actual user message (text, image, audio, etc.)
type Message struct {
	From        string       `json:"from"`
	ID          string       `json:"id"`
	Timestamp   string       `json:"timestamp"`
	Type        string       `json:"type"`
	Text        *Text        `json:"text,omitempty"`
	Image       *MediaItem   `json:"image,omitempty"`
	Audio       *MediaItem   `json:"audio,omitempty"`
	Voice       *MediaItem   `json:"voice,omitempty"`
	Video       *MediaItem   `json:"video,omitempty"`
	Document    *MediaItem   `json:"document,omitempty"`
	Sticker     *MediaItem   `json:"sticker,omitempty"`
	Location    *Location    `json:"location,omitempty"`
	Interactive *Interactive `json:"interactive,omitempty"`
	Button      *Button      `json:"button,omitempty"` // template quick-reply button tap
	// Reaction is the emoji a customer reacted with on a previous
	// outbound message. We surface it in the conversation as "Reagiu
	// com 👍" so attendants see context without needing to scroll.
	Reaction *Reaction `json:"reaction,omitempty"`
	// Contacts is a vCard payload — customer shared one or more
	// contacts. Could be useful (lead handoff) or noise (spam vcards).
	// We render the names + phones inline.
	//
	// The tag is `contacts`, which is what Meta sends. It read
	// `contacts_payload` until 2026-08-17 and therefore never decoded: every
	// shared contact card became a message with type `text` and an EMPTY body,
	// which renders as a small blank bubble — indistinguishable from nothing
	// having happened. There is no collision to disambiguate here; the
	// `contacts` on ChangeValue (the sender's profile) is a different struct.
	Contacts []ContactCard `json:"contacts,omitempty"`
	// Order is the customer placing an order through a Catalog
	// integration. We log the line items for the agent to pick up.
	Order *Order `json:"order,omitempty"`
	// System is a service event from WhatsApp itself ("user changed
	// number from X to Y", etc.) — must be tracked so we can update
	// contact records, but never replied to.
	System *SystemEvent `json:"system,omitempty"`
	// Errors carries inbound parsing failures Meta surfaces (banned
	// number, unsupported media). Worth persisting for ops.
	Errors []MessageError `json:"errors,omitempty"`
	// Referral is attached to the FIRST inbound message when the
	// customer arrived via a Click-to-WhatsApp ad or a deep link with
	// referral parameters. Carries source attribution that marketing
	// teams need (CTWA CLID, source URL, headline) and a sample of
	// the ad media. Dropping this silently — which the parser used to
	// do — costs revenue attribution for every paid-traffic customer.
	Referral *Referral `json:"referral,omitempty"`
	// Context is set when the customer's message is a reply to a
	// previous bot/agent message. Carrying the wamid lets the agent
	// know which message is being answered without inferring from
	// timestamps.
	Context *MessageContext `json:"context,omitempty"`
}

// Referral mirrors the Click-to-WhatsApp / deep-link attribution
// payload Meta attaches to the first inbound. CTWA_CLID is the click
// identifier marketing teams use to tie the conversation back to a
// specific ad creative + budget pool.
type Referral struct {
	SourceURL    string `json:"source_url,omitempty"`
	SourceType   string `json:"source_type,omitempty"` // "ad" / "post"
	SourceID     string `json:"source_id,omitempty"`
	Headline     string `json:"headline,omitempty"`
	Body         string `json:"body,omitempty"`
	MediaType    string `json:"media_type,omitempty"`
	ImageURL     string `json:"image_url,omitempty"`
	VideoURL     string `json:"video_url,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	CTWACLID     string `json:"ctwa_clid,omitempty"`
}

// MessageContext is Meta's `context` envelope for replies — when the
// customer taps "Reply" on a previous bot/agent message, this carries
// the original wamid so the agent can stitch the reply back to the
// triggering message even when several bubbles arrived in between.
type MessageContext struct {
	From            string `json:"from,omitempty"`
	ID              string `json:"id,omitempty"`
	ReferredProduct *struct {
		CatalogID         string `json:"catalog_id"`
		ProductRetailerID string `json:"product_retailer_id"`
	} `json:"referred_product,omitempty"`
}

// Reaction carries the emoji a customer reacted with on a previous
// outbound message. message_id correlates back to our wamid.
type Reaction struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
}

// ContactCard mirrors the subset of vCard fields the Cloud API parses
// out of a customer-shared contact. Names + phones are enough for the
// agent to act on; everything else stays in the metadata blob.
type ContactCard struct {
	Name struct {
		FormattedName string `json:"formatted_name"`
		FirstName     string `json:"first_name,omitempty"`
		LastName      string `json:"last_name,omitempty"`
	} `json:"name"`
	Phones []struct {
		Phone string `json:"phone"`
		Type  string `json:"type,omitempty"`
		WaID  string `json:"wa_id,omitempty"`
	} `json:"phones"`
	// VCard is the full card, base64 in every payload we have seen. It carries
	// fields the parsed `name`/`phones` above drop — notably
	// X-WA-BIZ-DESCRIPTION, which is how a WhatsApp Business account describes
	// itself and is often the only thing that says WHAT the shared number is.
	//
	// Deliberately stripped before the metadata blob is written (see the
	// `contacts` case in the adapter): a vCard may embed a PHOTO in base64 and
	// would then be tens of KB of duplication on every message row. What we
	// want out of it is extracted into BusinessDescription instead.
	VCard string `json:"vcard,omitempty"`
	// BusinessDescription and BusinessName are read OUT of VCard by the parser;
	// WhatsApp never sends these keys. They exist so the useful half of the
	// card survives once the raw card is dropped.
	//
	// BusinessName is the name the business declares for itself, which is a
	// DIFFERENT fact from Name above: that one comes from the card's FN, i.e.
	// whatever the person sharing had saved in their own phonebook. Both are
	// kept because either can be the one an attendant recognises.
	BusinessDescription string `json:"business_description,omitempty"`
	BusinessName        string `json:"business_name,omitempty"`
}

// Order is the structured payload customers send when checking out a
// Catalog. We summarize line items in the conversation transcript so
// agents can confirm without leaving Replider.
type Order struct {
	CatalogID    string `json:"catalog_id"`
	Text         string `json:"text,omitempty"`
	ProductItems []struct {
		ProductRetailerID string  `json:"product_retailer_id"`
		Quantity          int     `json:"quantity"`
		ItemPrice         float64 `json:"item_price"`
		Currency          string  `json:"currency"`
	} `json:"product_items"`
}

// SystemEvent covers messages WhatsApp itself injects into the thread
// — typically `customer_changed_number` so we can update the contact
// record. We log it but never run the AI on it.
type SystemEvent struct {
	Type     string `json:"type"`
	Body     string `json:"body,omitempty"`
	NewWaID  string `json:"new_wa_id,omitempty"`
	Identity string `json:"identity,omitempty"`
	Customer string `json:"customer,omitempty"`
}

// MessageError mirrors Meta's per-message error envelope for inbound
// failures (e.g. unsupported media). Surfaced for ops visibility only.
type MessageError struct {
	Code    int    `json:"code"`
	Title   string `json:"title"`
	Message string `json:"message,omitempty"`
}

// Interactive represents an interactive response: button_reply,
// list_reply, or nfm_reply (Flows native form submission). nfm_reply
// carries the JSON the user submitted — we store it verbatim in the
// message metadata so the agent can act on the structured fields
// later (booking forms, lead capture, etc.).
type Interactive struct {
	Type        string       `json:"type"`
	ButtonReply *ButtonReply `json:"button_reply,omitempty"`
	ListReply   *ListReply   `json:"list_reply,omitempty"`
	NFMReply    *NFMReply    `json:"nfm_reply,omitempty"`
}

// NFMReply carries the JSON-encoded form data a Flow submission emits.
// Body is a human-readable summary; ResponseJSON is the full payload.
type NFMReply struct {
	ResponseJSON string `json:"response_json"`
	Body         string `json:"body,omitempty"`
	Name         string `json:"name,omitempty"`
}

// ButtonReply is the user's response to a reply button
type ButtonReply struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ListReply is the user's response to a list selection
type ListReply struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// Button represents a template quick-reply button tap
type Button struct {
	Text    string `json:"text"`
	Payload string `json:"payload"`
}

// Text contains the content for type = "text"
type Text struct {
	Body string `json:"body"`
}

// MediaItem contains the data for media messages (image, audio, video, document, sticker, voice)
type MediaItem struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type,omitempty"`
	Sha256   string `json:"sha256,omitempty"`
	Caption  string `json:"caption,omitempty"`  // for image and video
	FileName string `json:"filename,omitempty"` // for document
}

// Location contains geographic coordinates for location messages
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

// Status represents message delivery/read receipts plus the
// conversation/pricing envelope Meta attaches to the FIRST status
// update of a billable conversation.
type Status struct {
	ID        string `json:"id"`
	Status    string `json:"status"` // "sent", "delivered", "read", "failed"
	Timestamp string `json:"timestamp"`
	Recipient string `json:"recipient_id"`
	// Conversation only appears on the first status of a 24h window.
	// It tells us which billable conversation the outbound counted as
	// (marketing/utility/authentication/service/referral_conversion).
	Conversation *StatusConversation `json:"conversation,omitempty"`
	// Pricing mirrors the conversation but with billable + model so
	// we can route metered usage to Stripe accurately.
	Pricing *StatusPricing `json:"pricing,omitempty"`
	// Errors carries Meta's failure envelope on `status="failed"`.
	Errors []MessageError `json:"errors,omitempty"`
}

type StatusConversation struct {
	ID                  string `json:"id"`
	ExpirationTimestamp string `json:"expiration_timestamp,omitempty"`
	Origin              struct {
		Type string `json:"type"`
	} `json:"origin"`
}

type StatusPricing struct {
	Billable     bool   `json:"billable"`
	PricingModel string `json:"pricing_model,omitempty"`
	Category     string `json:"category,omitempty"`
}

// ParsedStatus holds a normalized delivery status update.
type ParsedStatus struct {
	PhoneNumberID string
	MessageID     string
	Status        string // "sent", "delivered", "read", "failed"
	Recipient     string
	Timestamp     string
	// ConversationOrigin / PricingCategory drive billing accuracy and
	// the dashboard's 24h-window indicator. Empty when Meta didn't
	// attach an envelope (most non-first status updates).
	ConversationID      string
	ConversationOrigin  string
	ConversationExpires string
	PricingCategory     string
	PricingBillable     bool
	// FailedReason is the human-readable Meta error on status="failed".
	// Surfaced to the agent so they know why the send didn't reach.
	FailedReason string
}
