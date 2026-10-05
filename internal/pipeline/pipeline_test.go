package pipeline_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/classify"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/fixtures"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/gold"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/pipeline"
)

func TestFixture1FeeInThirdAssetProducesTwoGainLossRecords(t *testing.T) {
	events := pipeline.Process(fixtures.Fixture1FeeInThirdAsset())
	ledger := gold.Publish(events)
	gl := gold.GainLossRows(ledger)
	if len(gl) != 2 {
		t.Fatalf("want 2 gain/loss records (ETH trade + BTC fee), got %d: %+v", len(gl), gl)
	}
	byAsset := map[string]classify.LedgerEntry{}
	for _, r := range gl {
		byAsset[r.Asset] = r
	}
	eth, ok := byAsset["ETH"]
	if !ok || eth.Kind != classify.KindDisposal {
		t.Fatalf("missing ETH disposal: %+v", gl)
	}
	btc, ok := byAsset["BTC"]
	if !ok || btc.Kind != classify.KindFeeDisposal {
		t.Fatalf("missing BTC fee disposal: %+v", gl)
	}
	if !btc.Quantity.Equal(decimal.RequireFromString("0.0005")) {
		t.Fatalf("BTC fee qty: %s", btc.Quantity)
	}
	// ETH: lots at $2000, sell 2 for $6000 → cost 4000, gain 2000
	if !eth.GainLossUSD.Equal(decimal.RequireFromString("2000")) {
		t.Fatalf("ETH gain want 2000 got %s (cost=%s proceeds=%s)", eth.GainLossUSD, eth.CostBasisUSD, eth.ProceedsUSD)
	}
	// BTC fee: 0.0005 * 40000 FV = 20 proceeds, basis 20 → 0 G/L
	if !btc.ProceedsUSD.Equal(decimal.RequireFromString("20")) {
		t.Fatalf("BTC fee proceeds want 20 got %s", btc.ProceedsUSD)
	}
}

func TestFixture2CleanInternalTransferProducesZeroGainLoss(t *testing.T) {
	events := pipeline.Process(fixtures.Fixture2CleanInternalTransfer())
	ledger := gold.Publish(events)
	gl := gold.GainLossRows(ledger)
	if len(gl) != 0 {
		t.Fatalf("internal transfer must not create G/L, got %+v", gl)
	}
	for _, e := range events {
		if e.Kind != classify.KindNonEvent {
			t.Fatalf("want only non-events, got %+v", e)
		}
		if e.Classification != "internal_transfer" {
			t.Fatalf("want internal_transfer, got %s", e.Classification)
		}
		if e.NeedsReview {
			t.Fatalf("unique high-confidence pair must not need review")
		}
	}
}

func TestFixture3FalseMatchTrapIsPublishedNeedsReview(t *testing.T) {
	events := pipeline.Process(fixtures.Fixture3FalseMatchTrap())
	ledger := gold.Publish(events)
	if len(gold.GainLossRows(ledger)) != 0 {
		t.Fatalf("must not silently auto-classify G/L when candidates collide: %+v", gold.GainLossRows(ledger))
	}
	review := gold.ReviewRows(ledger)
	if len(review) < 3 {
		t.Fatalf("want all 3 colliding legs published for review, got %d: %+v", len(review), review)
	}
	ids := map[string]bool{}
	for _, r := range review {
		if !r.NeedsReview {
			t.Fatalf("review row missing flag: %+v", r)
		}
		if r.ReviewReason == "" {
			t.Fatalf("review row missing reason: %+v", r)
		}
		for _, id := range r.SourceIDs {
			ids[id] = true
		}
	}
	for _, need := range []string{"f3-wd", "f3-dep", "f3-ext-buy"} {
		if !ids[need] {
			t.Fatalf("missing %s on the review ledger (hiding it is the failure mode we are preventing)", need)
		}
	}
}

func TestFixture4MultiLegOnlyTradeIsAccountingEvent(t *testing.T) {
	events := pipeline.Process(fixtures.Fixture4MultiLegOrder())
	ledger := gold.Publish(events)
	var acq []classify.LedgerEntry
	for _, r := range ledger {
		if r.Kind == classify.KindAcquisition {
			acq = append(acq, r)
		}
		if r.Kind == classify.KindDisposal || r.Kind == classify.KindFeeDisposal {
			t.Fatalf("wrapper legs must not dispose/acquire extra: %+v", r)
		}
	}
	if len(acq) != 1 {
		t.Fatalf("want exactly 1 acquisition (the trade), got %d: %+v", len(acq), acq)
	}
	if acq[0].Asset != "ETH" || !acq[0].Quantity.Equal(decimal.RequireFromString("3")) {
		t.Fatalf("want 3 ETH, got %s %s", acq[0].Quantity, acq[0].Asset)
	}
	if !acq[0].CostBasisUSD.Equal(decimal.RequireFromString("10000")) {
		t.Fatalf("cost basis want 10000 got %s", acq[0].CostBasisUSD)
	}
	gl := gold.GainLossRows(ledger)
	if len(gl) != 0 {
		t.Fatalf("buy has no realized G/L yet, got %+v", gl)
	}
}
