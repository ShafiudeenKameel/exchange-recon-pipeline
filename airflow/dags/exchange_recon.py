"""
exchange_recon_pipeline

Bronze mock JSON is already on disk (cmd/bronze). This DAG:
  1) dbt seed + run (dedup, order_id grouping)
  2) Go gold CLI over each fixture (matching + fee-split + ledger)

No live exchange APIs. Deterministic fixtures only.
"""

from datetime import datetime, timedelta
import os

from airflow import DAG
from airflow.operators.bash import BashOperator

ROOT = os.environ.get("EXCHANGE_RECON_ROOT", "/opt/airflow/project")

with DAG(
    dag_id="exchange_recon_pipeline",
    start_date=datetime(2024, 1, 1),
    schedule=None,
    catchup=False,
    default_args={"retries": 1, "retry_delay": timedelta(minutes=2)},
    tags=["reconciliation", "exchange"],
) as dag:
    bronze = BashOperator(
        task_id="land_bronze_fixtures",
        bash_command=f"cd {ROOT} && ./bin/bronze",
    )
    silver_dbt = BashOperator(
        task_id="dbt_cleanup_and_multileg",
        bash_command=f"cd {ROOT}/dbt && dbt seed --profiles-dir . && dbt run --profiles-dir .",
    )
    gold = BashOperator(
        task_id="classify_and_publish_gold",
        bash_command=f"cd {ROOT} && ./bin/gold",
    )
    bronze >> silver_dbt >> gold
