package fixtures

import (
	"github.com/shopspring/decimal"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/classify"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

const Owner = "user-1"
const T0 int64 = 1_700_000_000 // arbitrary epoch

// Fixture1FeeInThirdAsset:
// Sell 2 ETH for 6,000 USDC, fee 0.0005 BTC (unrelated to either trade leg).
// Prior lots exist so Gold can compute two independent G/L numbers.
func Fixture1FeeInThirdAsset() []classify.RawRecord {
	return []classify.RawRecord{
		{
			RecordID: "f1-lot-eth", Exchange: "coinbase_prime",
			AccountID: "trading", AccountLabel: "Trading", OwnerID: Owner,
			Type: classify.TypeDeposit, Asset: "ETH", Amount: d("2"),
			AssetFVPerUnit: d("2000"), Timestamp: T0 - 86400,
		},
		{
			RecordID: "f1-lot-btc", Exchange: "coinbase_prime",
			AccountID: "trading", AccountLabel: "Trading", OwnerID: Owner,
			Type: classify.TypeDeposit, Asset: "BTC", Amount: d("1"),
			AssetFVPerUnit: d("40000"), Timestamp: T0 - 86400,
		},
		{
			RecordID: "f1-trade", Exchange: "coinbase_prime",
			AccountID: "trading", AccountLabel: "Trading", OwnerID: Owner,
			Type: classify.TypeTrade, Side: "sell",
			Asset: "ETH", Amount: d("2"),
			CounterAsset: "USDC", CounterAmount: d("6000"),
			FeeAsset: "BTC", FeeAmount: d("0.0005"),
			FeeFVPerUnit: d("40000"), AssetFVPerUnit: d("3000"),
			Timestamp: T0,
		},
	}
}

// Fixture2CleanInternalTransfer:
// 5 ETH Trading → Cold Storage, T and T+45s, same owner. No other 5 ETH flow.
func Fixture2CleanInternalTransfer() []classify.RawRecord {
	return []classify.RawRecord{
		{
			RecordID: "f2-wd", Exchange: "kraken",
			AccountID: "trading", AccountLabel: "Trading", OwnerID: Owner,
			Type: classify.TypeWithdrawal, Asset: "ETH", Amount: d("5"),
			Timestamp: T0,
		},
		{
			RecordID: "f2-dep", Exchange: "kraken",
			AccountID: "cold", AccountLabel: "Cold Storage", OwnerID: Owner,
			Type: classify.TypeDeposit, Asset: "ETH", Amount: d("5"),
			Timestamp: T0 + 45,
		},
	}
}

// Fixture3FalseMatchTrap:
// Same internal transfer as Fixture 2, PLUS a real external buy of 5 ETH
// into Trading at T+10s. Naive "same amount, same asset, close time" matching
// could pair the withdrawal with the purchase.
func Fixture3FalseMatchTrap() []classify.RawRecord {
	recs := Fixture2CleanInternalTransfer()
	recs[0].RecordID = "f3-wd"
	recs[1].RecordID = "f3-dep"
	buy := classify.RawRecord{
		RecordID: "f3-ext-buy", Exchange: "kraken",
		AccountID: "trading", AccountLabel: "Trading", OwnerID: Owner,
		Type: classify.TypeTrade, Side: "buy",
		Asset: "ETH", Amount: d("5"),
		CounterAsset: "USD", CounterAmount: d("15000"),
		Counterparty: "unrelated-otc-desk",
		AssetFVPerUnit: d("3000"), CounterFVPerUnit: d("1"),
		Timestamp: T0 + 10,
	}
	return append(recs, buy)
}

// Fixture4MultiLegOrder:
// order_id ORD-88291: fiat deposit $10,000 → trade USD→3 ETH → auto-withdraw
// 3 ETH to a connected wallet. Only the trade is an accounting event.
func Fixture4MultiLegOrder() []classify.RawRecord {
	const oid = "ORD-88291"
	return []classify.RawRecord{
		{
			RecordID: "f4-fiat-in", Exchange: "coinbase_prime",
			AccountID: "trading", AccountLabel: "Trading", OwnerID: Owner,
			Type: classify.TypeDeposit, Asset: "USD", Amount: d("10000"),
			OrderID: oid, Timestamp: T0,
		},
		{
			RecordID: "f4-trade", Exchange: "coinbase_prime",
			AccountID: "trading", AccountLabel: "Trading", OwnerID: Owner,
			Type: classify.TypeTrade, Side: "buy",
			Asset: "ETH", Amount: d("3"),
			CounterAsset: "USD", CounterAmount: d("10000"),
			OrderID: oid, Timestamp: T0 + 5,
			AssetFVPerUnit: d("3333.333333333"),
		},
		{
			RecordID: "f4-auto-wd", Exchange: "coinbase_prime",
			AccountID: "trading", AccountLabel: "Trading", OwnerID: Owner,
			Type: classify.TypeWithdrawal, Asset: "ETH", Amount: d("3"),
			OrderID: oid, Counterparty: "connected-wallet",
			Timestamp: T0 + 12,
		},
	}
}
