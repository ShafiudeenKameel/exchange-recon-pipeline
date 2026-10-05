// Package feesplit turns one exchange trade row into the economic legs
// the ledger actually needs.
//
// WHY a separate disposal for a third-asset fee:
// Paying 0.0005 BTC to execute an ETH/USDC trade is a disposal of BTC, the
// same way paying gas is a disposal of ETH. It is NOT a cost adjustment of
// the ETH lot and NOT a reduction of USDC proceeds — those would silently
// mix three assets into one G/L number and make FIFO un-auditable.
package feesplit

import (
	"fmt"

	"github.com/shopspring/decimal"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/classify"
)

func Expand(rec classify.RawRecord) []classify.EconomicEvent {
	if rec.Type != classify.TypeTrade {
		return nil
	}

	var out []classify.EconomicEvent
	if rec.Side == "sell" {
		out = append(out, classify.EconomicEvent{
			EventID:        rec.RecordID + ":disposal",
			SourceIDs:      []string{rec.RecordID},
			Kind:           classify.KindDisposal,
			Asset:          rec.Asset,
			Quantity:       rec.Amount,
			ProceedsUSD:    rec.CounterAmount.Mul(fairOrOne(rec.CounterAsset, rec.CounterFVPerUnit)),
			Classification: "trade_sell",
			Confidence:     decimal.RequireFromString("1"),
		})
	} else { // buy
		out = append(out, classify.EconomicEvent{
			EventID:        rec.RecordID + ":acquisition",
			SourceIDs:      []string{rec.RecordID},
			Kind:           classify.KindAcquisition,
			Asset:          rec.Asset,
			Quantity:       rec.Amount,
			CostBasisUSD:   rec.CounterAmount.Mul(fairOrOne(rec.CounterAsset, rec.CounterFVPerUnit)),
			Classification: "trade_buy",
			Confidence:     decimal.RequireFromString("1"),
		})
	}

	if rec.FeeAmount.GreaterThan(decimal.Zero) && rec.FeeAsset != "" && isThirdAssetFee(rec) {
		out = append(out, classify.EconomicEvent{
			EventID:        rec.RecordID + ":fee",
			SourceIDs:      []string{rec.RecordID},
			Kind:           classify.KindFeeDisposal,
			Asset:          rec.FeeAsset,
			Quantity:       rec.FeeAmount,
			ProceedsUSD:    rec.FeeAmount.Mul(rec.FeeFVPerUnit),
			Classification: "fee_in_third_asset",
			Confidence:     decimal.RequireFromString("1"),
		})
	}
	return out
}

func isThirdAssetFee(rec classify.RawRecord) bool {
	fee := rec.FeeAsset
	return fee != rec.Asset && fee != rec.CounterAsset
}

func fairOrOne(asset string, fv decimal.Decimal) decimal.Decimal {
	if fv.IsZero() && (asset == "USD" || asset == "USDC" || asset == "USDT") {
		return decimal.NewFromInt(1)
	}
	if fv.IsZero() {
		return decimal.Zero
	}
	return fv
}

func EventID(recID, suffix string) string { return fmt.Sprintf("%s:%s", recID, suffix) }
