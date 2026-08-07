ALTER TABLE batch_image_jobs
    ADD COLUMN IF NOT EXISTS balance_package_id BIGINT,
    ADD COLUMN IF NOT EXISTS balance_package_hold_usd DECIMAL(20,10) NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS batch_image_jobs_balance_package_id_idx
    ON batch_image_jobs(balance_package_id)
    WHERE balance_package_id IS NOT NULL;
