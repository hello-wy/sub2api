-- Bind a balance-funded subscription purchase to the caller's idempotency key.
-- payment_trade_no is otherwise empty for internal balance payments.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_payment_orders_balance_subscription_idempotency
    ON payment_orders (user_id, payment_trade_no)
    WHERE payment_type = 'balance'
      AND order_type = 'subscription'
      AND payment_trade_no <> '';
