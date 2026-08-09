-- Welfare records are a reward ledger: non-winning draws are not distributions.
DELETE FROM welfare_records
WHERE source_type = 'lottery_draw' AND reward_type = 'none';

-- Backfill a readable prize name and convert subscription plan prices from
-- CNY to platform quota at the lottery valuation of 1 CNY = 10 USD.
UPDATE welfare_records AS records
SET amount = CASE
        WHEN draws.prize_type = 'subscription' THEN COALESCE(plans.price * 10, records.amount)
        ELSE draws.reward_amount
    END,
    remarks = '抽奖奖励 #' || draws.id::text || ' · ' || draws.prize_label,
    reward_type = draws.prize_type
FROM lottery_draws AS draws
LEFT JOIN redeem_codes AS redeem
    ON redeem.code = draws.reward_ref
   AND redeem.owner_user_id = draws.user_id
LEFT JOIN LATERAL (
    SELECT price
    FROM subscription_plans
    WHERE group_id = redeem.group_id AND for_sale = true
    ORDER BY sort_order, id
    LIMIT 1
) AS plans ON true
WHERE records.source_type = 'lottery_draw'
  AND records.source_id = draws.id::text
  AND draws.prize_type <> 'none';
