# Exchange Data Reconciliation Pipeline

Portfolio project aimed at a job-description line that is easy to wave at and hard to implement:

> Build and maintain ingestion pipelines for exchange/custodian data (Coinbase Prime, Kraken, etc.), handling **internal transfers**, **fees in a different asset than the trade**, and **multi-leg transactions** correctly.

There is **no live exchange account**. Four mock fixtures are engineered so each hard case is a deterministic unit test.

## The three problems

### 1. Fee in a third asset

A trade can sell 2 ETH for 6,000 USDC and charge **0.0005 BTC** as the fee. BTC is on neither side of the book.

Paying that fee is a **disposal of BTC**, the same way paying gas is a disposal of ETH. Mixing it into the ETH cost basis or the USDC proceeds would hide a third asset inside someone else's G/L.

**Fixture 1** asserts Gold contains **two** gain/loss rows: ETH disposal and BTC fee disposal.

### 2. Internal transfers (no shared id)

5 ETH leaves the Trading sub-account and 45 seconds later 5 ETH arrives in Cold Storage. The exchange emits a withdrawal and a deposit **with no join key**.

That is a **relocation**, not a sale. Matched correctly, it must produce **zero** gain/loss.

Matching is probabilistic (amount + asset + owner + close time). It lives in **Go**, not dbt, because it is a candidate-comparison problem with vetoes, not a set join.

**Fixture 2** is the clean unique pair → `internal_transfer` non-event.

### 3. Multi-leg orders (shared `order_id`)

Buying ETH with a bank wire often lands as three rows that **do** share `order_id`:

1. fiat deposit $10,000  
2. trade USD → 3 ETH  
3. auto-withdraw 3 ETH to a connected wallet  

Linking is easy (that is why this part **is** a dbt `GROUP BY order_id`). The trap is treating legs 1 and 3 as crypto income/disposal. Only the **trade** is an accounting event: 3 ETH acquired, cost basis $10,000.

**Fixture 4** asserts exactly that.

## The adversarial case (Fixture 3)

Same internal transfer as Fixture 2, **plus** a real OTC buy of 5 ETH into Trading at T+10s.

A naive matcher sees two inbound 5 ETH legs near one outbound 5 ETH and may pair the withdrawal with the **purchase**. That:

- destroys the purchase's cost basis, and  
- leaves the cold-storage deposit looking like income.

**Rule:** if more than one plausible candidate exists, **never auto-match**. Publish every involved row on Gold with `needs_review=true` and a reason. The dollars are real; hiding the cluster is worse than a visible flag.

## Publish-with-flag vs freeze (Project 1)

| Situation | What we know | What we do |
|-----------|--------------|------------|
| Project 1 recon fail | There is **one** correct on-chain balance; we don't have it | **Freeze** Gold — don't publish an invented number |
| Fixture 3 collision | **Several legitimate interpretations** need a human | **Publish with `needs_review`** — don't hide material flow |

Implemented on purpose as the opposite of a freeze.

## Architecture

```
Bronze   mock Kraken / Coinbase Prime JSON, untouched
           data/bronze/*.json  (cmd/bronze)

Silver   dbt (DuckDB): dedup, type-as-string, GROUP BY order_id
         Go: fee-split + internal-transfer confidence scoring

Gold     Go ledger: confirmed G/L + flagged unconfirmed rows
```

Idempotent: fixtures are static; `record_id` is the grain; re-running tests/CLIs rewrites the same events.

All money is `shopspring/decimal` / STRING. No floats.

## Confidence scoring (Go)

A pair is even **eligible** only if: same owner, same asset, exact amount, |Δt| ≤ 120s.

Score (unique pairs only):

- 0.55 base (already exact on amount/asset/owner)
- +0.30 × (1 − Δt/120)  closer in time is better
- +0.10 if the account id changes (the usual relocation)
- −0.40 if either leg names an external counterparty

**Auto-match iff** exactly one eligible inbound for that outbound **and** either:

- neither leg names an external counterparty (unique same-owner relocation), or  
- score ≥ **0.90**

**If candidate_count > 1: stop.** Scores are irrelevant. Flag the whole cluster.

## Tests

```bash
make test
```

Four tests, one per fixture, all required to pass.

```bash
make bronze   # write data/bronze/*.json
make gold     # print Gold JSON for each fixture
```

Optional dbt (needs `dbt-duckdb`):

```bash
cd dbt && dbt seed --profiles-dir . && dbt run --profiles-dir .
```

## Layout

```
internal/fixtures   the four cases, verbatim
internal/feesplit   third-asset fee → extra disposal
internal/match      internal-transfer engine
internal/pipeline   orchestrates multileg → match → fee-split
internal/gold       ledger + FIFO lots for G/L
dbt/                set-based cleanup + order_id groups
```

## Not tax advice

Mock data, mock lots, interview artifact.
