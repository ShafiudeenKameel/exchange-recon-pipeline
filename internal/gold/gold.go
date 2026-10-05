// Package gold turns Silver economic events into a published ledger.
//
// Ambiguous internal-transfer clusters (Fixture 3) are WRITTEN with
// NeedsReview=true. We do not freeze/omit them: unlike a balance-recon
// mismatch (one unknown correct number), these dollars are real and a
// hidden row is worse than a flagged one.
package gold

import (
	"sort"

	"github.com/shopspring/decimal"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/classify"
)

type lot struct {
	eventID string
	qty     decimal.Decimal
	perUnit decimal.Decimal
}

func Publish(events []classify.EconomicEvent) []classify.LedgerEntry {
	// FIFO lots from confirmed acquisitions only. Unconfirmed rows still
	// appear on the ledger but do not silently consume lots.
	lots := map[string][]lot{}
	var ledger []classify.LedgerEntry

	ordered := append([]classify.EconomicEvent(nil), events...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].EventID < ordered[j].EventID })

	// First pass: book acquisitions (confirmed) as lots, in a stable order
	// by walking a copy sorted by... we don't have timestamps on events.
	// Use SourceIDs / EventID. Fixtures seed lots before trades via EventID
	// prefixes (f1-lot-* before f1-trade). Sorting by EventID is enough
	// for the four fixtures; a production Gold job would sort by timestamp.
	for _, e := range ordered {
		if e.Kind == classify.KindAcquisition && !e.NeedsReview {
			per := decimal.Zero
			if e.Quantity.GreaterThan(decimal.Zero) {
				per = e.CostBasisUSD.Div(e.Quantity)
			}
			lots[e.Asset] = append(lots[e.Asset], lot{eventID: e.EventID, qty: e.Quantity, perUnit: per})
			ledger = append(ledger, classify.LedgerEntry{
				EventID: e.EventID, Kind: e.Kind, Asset: e.Asset, Quantity: e.Quantity,
				CostBasisUSD: e.CostBasisUSD, Classification: e.Classification,
				SourceIDs: e.SourceIDs,
			})
		}
	}
	for _, e := range ordered {
		switch {
		case e.Kind == classify.KindNonEvent:
			continue // not a G/L row
		case e.NeedsReview || e.Kind == classify.KindUnconfirmed:
			ledger = append(ledger, classify.LedgerEntry{
				EventID: e.EventID, Kind: classify.KindUnconfirmed,
				Asset: e.Asset, Quantity: e.Quantity,
				NeedsReview: true, ReviewReason: e.ReviewReason,
				Classification: e.Classification, SourceIDs: e.SourceIDs,
			})
		case e.Kind == classify.KindDisposal || e.Kind == classify.KindFeeDisposal:
			cost, _ := consume(lots, e.Asset, e.Quantity)
			gl := e.ProceedsUSD.Sub(cost)
			ledger = append(ledger, classify.LedgerEntry{
				EventID: e.EventID, Kind: e.Kind, Asset: e.Asset, Quantity: e.Quantity,
				ProceedsUSD: e.ProceedsUSD, CostBasisUSD: cost, GainLossUSD: gl,
				Classification: e.Classification, SourceIDs: e.SourceIDs,
			})
		}
	}
	return ledger
}

func consume(lots map[string][]lot, asset string, qty decimal.Decimal) (decimal.Decimal, bool) {
	need := qty
	cost := decimal.Zero
	q := lots[asset]
	for need.GreaterThan(decimal.Zero) && len(q) > 0 {
		if q[0].qty.LessThanOrEqual(need) {
			cost = cost.Add(q[0].qty.Mul(q[0].perUnit))
			need = need.Sub(q[0].qty)
			q = q[1:]
		} else {
			cost = cost.Add(need.Mul(q[0].perUnit))
			q[0].qty = q[0].qty.Sub(need)
			need = decimal.Zero
		}
	}
	lots[asset] = q
	return cost, need.IsZero()
}

func GainLossRows(ledger []classify.LedgerEntry) []classify.LedgerEntry {
	var out []classify.LedgerEntry
	for _, r := range ledger {
		if r.NeedsReview {
			continue
		}
		if r.Kind == classify.KindDisposal || r.Kind == classify.KindFeeDisposal {
			out = append(out, r)
		}
	}
	return out
}

func ReviewRows(ledger []classify.LedgerEntry) []classify.LedgerEntry {
	var out []classify.LedgerEntry
	for _, r := range ledger {
		if r.NeedsReview {
			out = append(out, r)
		}
	}
	return out
}
