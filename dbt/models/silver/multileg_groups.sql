{{ config(materialized='table') }}

-- Join legs that the exchange already keyed with order_id.
-- Wrapper legs (fiat deposit, auto-withdrawal) are tagged non_event;
-- the trade leg is the only accounting event. Go re-applies the same
-- rule so unit tests don't need DuckDB.

SELECT
  order_id,
  COUNT(*) AS n_legs,
  COUNTIF(record_type = 'trade') AS n_trades,
  COUNTIF(record_type = 'deposit') AS n_deposits,
  COUNTIF(record_type = 'withdrawal') AS n_withdrawals,
  CASE
    WHEN COUNTIF(record_type = 'trade') = 1
     AND COUNTIF(record_type = 'deposit') >= 1
     AND COUNTIF(record_type = 'withdrawal') >= 1
    THEN 'wire_buy_withdraw'
    ELSE 'unknown_group'
  END AS pattern
FROM {{ ref('cleaned_records') }}
WHERE order_id IS NOT NULL AND order_id != ''
GROUP BY order_id
