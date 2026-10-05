// Package classify is the Silver-layer event model shared by fee-split,
// internal-transfer matching, and Gold. Monetary fields are decimal.Decimal
// only — never float64.
package classify

import "github.com/shopspring/decimal"

// HighConfidenceThreshold is the auto-match floor for internal transfers.
//
// WHY 0.90, not 0.5 or 0.99:
// A real internal transfer is exact amount + exact asset + same owner +
// sub-two-minute timing. That cluster scores well above 0.90. Lowering the
// bar would auto-resolve the Fixture 3 collision (two plausible 5 ETH ins).
// Raising it to 0.99 would reject legitimate T+45s transfers with no collision.
//
// INVARIANT stronger than the number: if more than one plausible candidate
// exists, we NEVER auto-match regardless of score. Ambiguity is a human
// problem, not a threshold-tuning problem.
const HighConfidenceThreshold = 0.90

// MatchWindowSec is how close two legs may be to even be considered.
// Exchange internal books are usually seconds, not hours.
const MatchWindowSec int64 = 120

type RecordType string

const (
	TypeTrade      RecordType = "trade"
	TypeWithdrawal RecordType = "withdrawal"
	TypeDeposit    RecordType = "deposit"
	TypeFee        RecordType = "fee"
)

type EventKind string

const (
	KindAcquisition   EventKind = "acquisition"
	KindDisposal      EventKind = "disposal"
	KindFeeDisposal   EventKind = "fee_disposal"
	KindNonEvent      EventKind = "non_event"      // matched internal transfer / multi-leg wrapper
	KindUnconfirmed   EventKind = "unconfirmed"    // published, needs human review
)

// RawRecord is one exchange-shaped row AFTER Bronze landing / dbt cleanup.
// Amounts are already decimal strings parsed by the caller.
type RawRecord struct {
	RecordID        string
	Exchange        string
	AccountID       string
	AccountLabel    string
	OwnerID         string
	Type            RecordType
	Asset           string
	Amount          decimal.Decimal
	Side            string // buy | sell (trades only)
	CounterAsset    string
	CounterAmount   decimal.Decimal
	FeeAsset        string
	FeeAmount       decimal.Decimal
	FeeFVPerUnit    decimal.Decimal // USD fair value of one fee-asset unit at Timestamp
	AssetFVPerUnit  decimal.Decimal
	CounterFVPerUnit decimal.Decimal
	OrderID         string
	Counterparty    string // empty for book-internal movements
	Timestamp       int64  // unix seconds
}

// EconomicEvent is the Silver output grain: one accounting-relevant (or
// explicitly non-event / unconfirmed) fact. Gold reads this list.
type EconomicEvent struct {
	EventID        string
	SourceIDs      []string
	Kind           EventKind
	Asset          string
	Quantity       decimal.Decimal
	ProceedsUSD    decimal.Decimal // disposals: what we received (or FV of fee asset)
	CostBasisUSD   decimal.Decimal // acquisitions: what we paid; disposals filled later in Gold
	Classification string
	Confidence     decimal.Decimal
	NeedsReview    bool
	ReviewReason   string
}

// LedgerEntry is Gold: a published row. Ambiguous matches are still here,
// with NeedsReview=true — we do NOT hide them (unlike Project 1 freeze).
type LedgerEntry struct {
	EventID        string
	Kind           EventKind
	Asset          string
	Quantity       decimal.Decimal
	ProceedsUSD    decimal.Decimal
	CostBasisUSD   decimal.Decimal
	GainLossUSD    decimal.Decimal
	NeedsReview    bool
	ReviewReason   string
	Classification string
	SourceIDs      []string
}
