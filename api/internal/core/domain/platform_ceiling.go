package domain

// Platform ceilings: what answers when NOTHING was negotiated.
//
// A contract plan with a blank column used to resolve to UnlimitedLimit, and the
// screen translated that as "Unlimited storage". Nobody promised any such thing —
// it was the value the system returns when the column is empty, turning into a
// promise in front of the customer. Two things wrong at once: the account can
// fill a disk that is not unlimited, and the customer was told something untrue.
//
// These numbers are NOT the commercial offer: the offer lives in the plan's
// column or the account's override, and still wins. This is what holds while
// nobody has written the number — a real ceiling, generous enough not to get in
// the way of whoever is being served, and finite enough to be true.
//
// Referência: o maior plano listado (Escala) vende 100 GB, 15 agentes, 10 mil
// leads/month. The ceilings sit above that, so a contract with no number written
// never gets LESS than the largest off-the-shelf plan.
const (
	CeilingAgents                = 50
	CeilingChannels              = 999
	CeilingSeats                 = 50
	CeilingConversationsPerMonth = 50000
	CeilingAITokensPerMonth      = 1_000_000_000
	CeilingStorageMB             = 512000 // 500 GB
	CeilingBroadcastPerMonth     = 50000
	CeilingRecipientsPerCampaign = 20000
	CeilingLeadsPerMonth         = 10000
	CeilingEnrichmentsPerMonth   = 10000
	CeilingAIDraftsPerMonth      = 20000
	CeilingDocumentExtractions   = 5000
	CeilingEmailMailboxes        = 50
	CeilingFunnels               = 999
)
