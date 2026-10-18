package service

import (
	"context"
	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// withRefundWalletTransaction keeps refund balance mutations atomic.
func (s *PaymentService) withRefundWalletTransaction(ctx context.Context, apply func(context.Context) error) error {
	if s.entClient == nil || dbent.TxFromContext(ctx) != nil {
		return apply(ctx)
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txctx := dbent.NewTxContext(ctx, tx)
	if err = apply(txctx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PaymentService) deductRefundBalance(ctx context.Context, p *RefundPlan) (float64, error) {
	var deducted float64
	err := s.withRefundWalletTransaction(ctx, func(txctx context.Context) error {
		var err error
		deducted, err = s.deductAvailableBalance(txctx, p.Order.UserID, p.BalanceToDeduct)
		return err
	})
	return deducted, err
}
