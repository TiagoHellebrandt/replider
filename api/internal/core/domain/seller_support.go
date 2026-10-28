package domain

import "time"

// Seller support view (canal-interno US-007): the seller panel groups
// internal CS threads by carteira CUSTOMER account, so context never
// fragments across the account's users. One flat row per thread comes
// out of the repository; the use case folds rows into account groups.

// SellerSupportThreadRow is one internal thread as read from the DB —
// flat, carrying its account grouping keys plus the account's shared
// note (repeated per row; the use case dedupes while grouping).
type SellerSupportThreadRow struct {
	AccountID   string
	AccountName string

	ConversationID string
	UserID         string
	UserName       string
	UserRole       string
	LastMessage    string
	LastMessageAt  *time.Time
	// UnreadCount = customer (inbound) messages newer than the last
	// platform (outbound) reply — "awaiting the seller", computable
	// without a platform-side read cursor.
	UnreadCount int

	Note          string
	NoteUpdatedBy string
	NoteUpdatedAt *time.Time
}

// SellerSupportThread is one user's internal thread inside an account
// group, as served to the seller panel.
type SellerSupportThread struct {
	ConversationID string     `json:"conversation_id"`
	UserID         string     `json:"user_id"`
	UserName       string     `json:"user_name"`
	UserRole       string     `json:"user_role"`
	LastMessage    string     `json:"last_message"`
	LastMessageAt  *time.Time `json:"last_message_at,omitempty"`
	UnreadCount    int        `json:"unread_count"`
}

// SellerSupportAccountGroup is one carteira account with all its users'
// internal threads and the account-level shared note.
type SellerSupportAccountGroup struct {
	AccountID   string                `json:"account_id"`
	AccountName string                `json:"account_name"`
	Note        *AccountInternalNote  `json:"note,omitempty"`
	UnreadCount int                   `json:"unread_count"`
	Threads     []SellerSupportThread `json:"threads"`
}

// AccountInternalNote is the shared account-level note (US-007) with its
// visible audit metadata (who edited, when).
type AccountInternalNote struct {
	AccountID     string     `json:"account_id"`
	Note          string     `json:"note"`
	UpdatedByName string     `json:"updated_by_name,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}
