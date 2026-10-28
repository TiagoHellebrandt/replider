package domain

import (
	"context"
	"time"
)

// CampaignStatus is the lifecycle of a broadcast campaign.
type CampaignStatus string

const (
	CampaignDraft     CampaignStatus = "draft"
	CampaignScheduled CampaignStatus = "scheduled"
	CampaignSending   CampaignStatus = "sending"
	CampaignDone      CampaignStatus = "done"
	CampaignFailed    CampaignStatus = "failed"
	CampaignCanceled  CampaignStatus = "canceled"
)

// RecipientStatus is the lifecycle of an individual recipient delivery.
// Mirrors the WhatsApp Cloud API status semantics.
type RecipientStatus string

const (
	RecipientPending   RecipientStatus = "pending"
	RecipientSent      RecipientStatus = "sent"
	RecipientDelivered RecipientStatus = "delivered"
	RecipientRead      RecipientStatus = "read"
	RecipientFailed    RecipientStatus = "failed"
)

// Campaign is a broadcast send via a channel + template.
type Campaign struct {
	ID              string
	AccountID       string
	ChannelID       string
	Name            string
	TemplateName    string
	TemplateLang    string
	TemplateVars    []string // positional values for {{1}}..{{N}}
	Status          CampaignStatus
	ScheduledAt     *time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
	TotalRecipients int
	SentCount       int
	DeliveredCount  int
	ReadCount       int
	FailedCount     int
	CreatedBy       *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// CampaignRecipient is the per-contact record inside a campaign.
type CampaignRecipient struct {
	ID            string
	CampaignID    string
	Recipient     string   // phone / chat_id
	Vars          []string // overrides on top of campaign.TemplateVars
	Status        RecipientStatus
	ProviderMsgID *string // wamid / equivalent
	ErrorMessage  *string
	SentAt        *time.Time
	DeliveredAt   *time.Time
	ReadAt        *time.Time
	FailedAt      *time.Time
	CreatedAt     time.Time
}

// CampaignRepository persists campaigns and their recipients.
type CampaignRepository interface {
	Create(ctx context.Context, c *Campaign) error
	GetByID(ctx context.Context, id, accountID string) (*Campaign, error)
	ListByAccount(ctx context.Context, accountID string, limit, offset int32) ([]*Campaign, error)
	UpdateStatus(ctx context.Context, id string, status CampaignStatus) error
	IncrementCounter(ctx context.Context, id string, status RecipientStatus) error
	Delete(ctx context.Context, id, accountID string) error
	ListScheduledDue(ctx context.Context, limit int32) ([]*Campaign, error)

	CreateRecipient(ctx context.Context, r *CampaignRecipient) error
	ListRecipients(ctx context.Context, campaignID string, limit, offset int32) ([]*CampaignRecipient, error)
	ListPendingRecipients(ctx context.Context, campaignID string, limit int32) ([]*CampaignRecipient, error)
	MarkRecipientSent(ctx context.Context, recipientID, providerMsgID string) error
	MarkRecipientFailed(ctx context.Context, recipientID, errorMessage string) error
	UpdateRecipientStatusByProviderID(ctx context.Context, providerMsgID string, status RecipientStatus) error
	GetCampaignByProviderMsgID(ctx context.Context, providerMsgID string) (*Campaign, error)
}
