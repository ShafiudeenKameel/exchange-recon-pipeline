{{ config(materialized='table') }}

-- Set-based cleanup: unique raw rows, typed decimals as STRING (never FLOAT).
-- Multi-leg linking is a GROUP on order_id — that IS relational. The
-- probabilistic internal-transfer matcher is NOT; it lives in Go.

SELECT
  record_id,
  exchange,
  account_id,
  account_label,
  owner_id,
  record_type,
  asset,
  amount,
  side,
  counter_asset,
  counter_amount,
  fee_asset,
  fee_amount,
  order_id,
  counterparty,
  ts_unix
FROM (
  SELECT
    *,
    ROW_NUMBER() OVER (PARTITION BY record_id ORDER BY ts_unix) AS rn
  FROM {{ ref('raw_exchange_records') }}
)
WHERE rn = 1
