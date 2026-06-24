CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_welfare_records_success_remarks
    ON welfare_records(remarks)
    WHERE status = 'success';
