ALTER TABLE lottery_ticket_ledger
    ADD COLUMN IF NOT EXISTS adjustment_reason TEXT;

ALTER TABLE lottery_ticket_ledger
    ADD CONSTRAINT chk_lottery_ticket_ledger_adjustment_reason_length
    CHECK (adjustment_reason IS NULL OR char_length(adjustment_reason) <= 500);
