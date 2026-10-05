// Package match detects internal transfers with a confidence score.
//
// WHY GO, NOT dbt/SQL:
// Pairing N withdrawals against M deposits is a matching problem with
// scenario-specific vetoes (same owner required, competing candidates
// veto auto-match, external counterparty penalizes). That is awkward as a
// self-join-plus-window and natural as a small imperative loop — and it
// is unit-testable with the four fixtures without a warehouse.
//
// Publish-with-flag vs Project 1 freeze:
// Project 1 recon failure = one correct on-chain number we don't have →
// withhold Gold. Here Fixture 3 = several legitimate interpretations
// (internal transfer + coincidental buy, vs two unrelated flows). Hiding
// the rows would hide five ETH of real money. We PUBLISH every involved
// record with NeedsReview=true so a human is invited to look.
package match

import (
	"fmt"
	"math"
	"sort"

	"github.com/shopspring/decimal"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/classify"
)

type scoredPair struct {
	out, in classify.RawRecord
	score   float64
}

// ResolveInternalTransfers annotates records that look like same-owner
// relocations. It never emits G/L; callers keep NonEvent / Unconfirmed.
func ResolveInternalTransfers(recs []classify.RawRecord) []classify.EconomicEvent {
	outs := outbound(recs)
	ins := inbound(recs)

	usedOut := map[string]bool{}
	usedIn := map[string]bool{}
	var events []classify.EconomicEvent

	type bucket struct {
		out classify.RawRecord
		ins []classify.RawRecord
	}
	var ambiguous []bucket
	var unique []scoredPair

	for _, o := range outs {
		var cands []classify.RawRecord
		for _, i := range ins {
			if !plausible(o, i) {
				continue
			}
			cands = append(cands, i)
		}
		if len(cands) == 0 {
			continue
		}
		if len(cands) > 1 {
			ambiguous = append(ambiguous, bucket{out: o, ins: cands})
			continue
		}
		only := cands[0]
		s := score(o, only)
		// Unique eligible pair with no named external counterparty is
		// the Fixture 2 happy path — uniqueness is the confidence.
		// Named counterparties still need to clear the numeric bar.
		external := o.Counterparty != "" || only.Counterparty != ""
		if !external || s >= classify.HighConfidenceThreshold {
			unique = append(unique, scoredPair{out: o, in: only, score: s})
		} else {
			ambiguous = append(ambiguous, bucket{out: o, ins: cands})
		}
	}

	// Unique high-confidence pairs → non-events (pure relocation).
	for _, p := range unique {
		if usedOut[p.out.RecordID] || usedIn[p.in.RecordID] {
			continue
		}
		usedOut[p.out.RecordID] = true
		usedIn[p.in.RecordID] = true
		events = append(events, classify.EconomicEvent{
			EventID:        "xfer-" + p.out.RecordID + "-" + p.in.RecordID,
			SourceIDs:      []string{p.out.RecordID, p.in.RecordID},
			Kind:           classify.KindNonEvent,
			Asset:          p.out.Asset,
			Quantity:       p.out.Amount,
			Classification: "internal_transfer",
			Confidence:     decimal.NewFromFloat(p.score),
			NeedsReview:    false,
		})
	}

	// Ambiguous: publish ALL involved legs with a visible review flag.
	flagged := map[string]classify.RawRecord{}
	for _, b := range ambiguous {
		flagged[b.out.RecordID] = b.out
		for _, i := range b.ins {
			flagged[i.RecordID] = i
		}
	}
	ids := make([]string, 0, len(flagged))
	for id := range flagged {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r := flagged[id]
		events = append(events, unconfirmed(r, "multiple plausible internal-transfer candidates; not auto-matched"))
	}
	return events
}

func unconfirmed(r classify.RawRecord, reason string) classify.EconomicEvent {
	kind := classify.KindUnconfirmed
	asset, qty := r.Asset, r.Amount
	class := string(r.Type)
	return classify.EconomicEvent{
		EventID:        "review-" + r.RecordID,
		SourceIDs:      []string{r.RecordID},
		Kind:           kind,
		Asset:          asset,
		Quantity:       qty,
		Classification: class,
		Confidence:     decimal.Zero,
		NeedsReview:    true,
		ReviewReason:   reason,
	}
}

func outbound(recs []classify.RawRecord) []classify.RawRecord {
	var out []classify.RawRecord
	for _, r := range recs {
		switch r.Type {
		case classify.TypeWithdrawal:
			out = append(out, r)
		case classify.TypeTrade:
			if r.Side == "sell" {
				out = append(out, r)
			}
		}
	}
	return out
}

func inbound(recs []classify.RawRecord) []classify.RawRecord {
	var out []classify.RawRecord
	for _, r := range recs {
		switch r.Type {
		case classify.TypeDeposit:
			out = append(out, r)
		case classify.TypeTrade:
			if r.Side == "buy" {
				out = append(out, r)
			}
		}
	}
	return out
}

func plausible(out, in classify.RawRecord) bool {
	if out.OwnerID == "" || out.OwnerID != in.OwnerID {
		return false
	}
	if out.Asset != in.Asset {
		return false
	}
	if !out.Amount.Equal(in.Amount) {
		return false
	}
	if out.RecordID == in.RecordID {
		return false
	}
	dt := out.Timestamp - in.Timestamp
	if dt < 0 {
		dt = -dt
	}
	return dt <= classify.MatchWindowSec
}

func score(out, in classify.RawRecord) float64 {
	// Exact amount/asset/owner already required by plausible().
	s := 0.55
	dt := math.Abs(float64(out.Timestamp - in.Timestamp))
	s += 0.30 * (1.0 - dt/float64(classify.MatchWindowSec))
	if out.AccountID != in.AccountID {
		s += 0.10 // relocation across sub-accounts is the happy path
	}
	if in.Counterparty != "" || out.Counterparty != "" {
		s -= 0.40 // named external counterparty: probably not internal
	}
	if s < 0 {
		return 0
	}
	if s > 1 {
		return 1
	}
	return s
}

func DebugPair(out, in classify.RawRecord) string {
	return fmt.Sprintf("%s->%s score=%.3f", out.RecordID, in.RecordID, score(out, in))
}
