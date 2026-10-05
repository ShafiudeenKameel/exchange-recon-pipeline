// Package pipeline is the Silver orchestrator: multi-leg tagging, fee-split,
// then internal-transfer matching. Order matters:
//
  // 1. Multi-leg groups (shared order_id) — linking is deterministic.
  // 2. Fee-split on remaining trades.
  // 3. Internal-transfer matching on remaining in/out — probabilistic.
package pipeline

import (
	"sort"

	"github.com/shopspring/decimal"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/classify"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/feesplit"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/match"
)

func Process(recs []classify.RawRecord) []classify.EconomicEvent {
	consumed := map[string]bool{}
	var events []classify.EconomicEvent

	events = append(events, expandMultileg(recs, consumed)...)

	var rest []classify.RawRecord
	for _, r := range recs {
		if !consumed[r.RecordID] {
			rest = append(rest, r)
		}
	}

	xfer := match.ResolveInternalTransfers(rest)
	xferUsed := map[string]bool{}
	for _, e := range xfer {
		events = append(events, e)
		for _, id := range e.SourceIDs {
			xferUsed[id] = true
			consumed[id] = true
		}
	}

	var leftover []classify.RawRecord
	for _, r := range recs {
		if !consumed[r.RecordID] {
			leftover = append(leftover, r)
		}
	}

	for _, r := range leftover {
		switch r.Type {
		case classify.TypeTrade:
			events = append(events, feesplit.Expand(r)...)
		case classify.TypeDeposit:
			if isFiat(r.Asset) {
				continue // fiat in isolation is not a crypto accounting event
			}
			events = append(events, classify.EconomicEvent{
				EventID:        r.RecordID + ":acquisition",
				SourceIDs:      []string{r.RecordID},
				Kind:           classify.KindAcquisition,
				Asset:          r.Asset,
				Quantity:       r.Amount,
				CostBasisUSD:   r.Amount.Mul(r.AssetFVPerUnit),
				Classification: "deposit",
				Confidence:     decimal.RequireFromString("1"),
			})
		case classify.TypeWithdrawal:
			if isFiat(r.Asset) {
				continue
			}
			events = append(events, classify.EconomicEvent{
				EventID:        r.RecordID + ":disposal",
				SourceIDs:      []string{r.RecordID},
				Kind:           classify.KindDisposal,
				Asset:          r.Asset,
				Quantity:       r.Amount,
				ProceedsUSD:    r.Amount.Mul(r.AssetFVPerUnit),
				Classification: "withdrawal",
				Confidence:     decimal.RequireFromString("1"),
			})
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].EventID < events[j].EventID })
	return events
}

func expandMultileg(recs []classify.RawRecord, consumed map[string]bool) []classify.EconomicEvent {
	groups := map[string][]classify.RawRecord{}
	for _, r := range recs {
		if r.OrderID == "" {
			continue
		}
		groups[r.OrderID] = append(groups[r.OrderID], r)
	}
	var events []classify.EconomicEvent
	for oid, g := range groups {
		if len(g) < 2 {
			continue
		}
		var trade *classify.RawRecord
		nDep, nWd, nTrade := 0, 0, 0
		for i := range g {
			switch g[i].Type {
			case classify.TypeDeposit:
				nDep++
			case classify.TypeWithdrawal:
				nWd++
			case classify.TypeTrade:
				nTrade++
				trade = &g[i]
			}
		}
		// Classic "wire in → buy crypto → auto-withdraw" chain.
		if nTrade == 1 && nDep >= 1 && nWd >= 1 && trade != nil {
			for _, r := range g {
				consumed[r.RecordID] = true
				if r.RecordID == trade.RecordID {
					continue
				}
				events = append(events, classify.EconomicEvent{
					EventID:        "multileg-skip-" + r.RecordID,
					SourceIDs:      []string{r.RecordID, oid},
					Kind:           classify.KindNonEvent,
					Asset:          r.Asset,
					Quantity:       r.Amount,
					Classification: "multileg_wrapper",
					Confidence:     decimal.RequireFromString("1"),
				})
			}
			events = append(events, feesplit.Expand(*trade)...)
		}
	}
	return events
}

func isFiat(a string) bool {
	return a == "USD" || a == "EUR" || a == "GBP"
}
