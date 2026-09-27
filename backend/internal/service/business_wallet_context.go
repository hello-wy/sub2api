package service

import (
	"context"
	"encoding/json"
	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// withBusinessWalletContext attaches an explicit refund identity to the wallet
// trigger in the SAME transaction. Provider calls remain outside this transaction.
func (s *PaymentService) withBusinessWalletContext(ctx context.Context, orderID int64, operation string, apply func(context.Context) error) error {
	if s.entClient == nil {
		return apply(ctx)
	}
	payload, _ := json.Marshal(map[string]any{"order_id": orderID, "operation": operation})
	if tx := dbent.TxFromContext(ctx); tx != nil {
		if _, err := tx.Client().ExecContext(ctx, `SELECT set_config('sub2api.business_wallet',$1,true)`, string(payload)); err != nil {
			return err
		}
		return apply(ctx)
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txctx := dbent.NewTxContext(ctx, tx)
	if _, err = tx.Client().ExecContext(txctx, `SELECT set_config('sub2api.business_wallet',$1,true)`, string(payload)); err != nil {
		return err
	}
	if err = apply(txctx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PaymentService) deductRefundBalance(ctx context.Context, p *RefundPlan) (float64, error) {
	var deducted float64
	err := s.withBusinessWalletContext(ctx, p.OrderID, "refund_reserve", func(txctx context.Context) error {
		var err error
		deducted, err = s.deductAvailableBalance(txctx, p.Order.UserID, p.BalanceToDeduct)
		return err
	})
	return deducted, err
}
