package service

import (
	"context"
	"errors"
)

type AccountImportTransactionAdmin interface {
	WithAccountImportTransaction(context.Context, func(context.Context, AdminService) error) error
}

type AccountImportTransactionRepository interface {
	WithAccountImportTransaction(context.Context, func(context.Context, AccountRepository, ProxyRepository) error) error
}

type accountImportCommitKey struct{}
type accountImportCommitHooks struct {
	hooks []func()
	admin *adminServiceImpl
}

func AccountImportSideEffectAdmin(ctx context.Context, fallback AdminService) AdminService {
	if pending, _ := ctx.Value(accountImportCommitKey{}).(*accountImportCommitHooks); pending != nil {
		return pending.admin
	}
	return fallback
}

// RunAccountImportSideEffect defers external work until all imported records
// commit. A rejected import never refreshes tokens or starts upstream probes.
func RunAccountImportSideEffect(ctx context.Context, operation func()) {
	if pending, _ := ctx.Value(accountImportCommitKey{}).(*accountImportCommitHooks); pending != nil {
		pending.hooks = append(pending.hooks, operation)
		return
	}
	operation()
}

func (s *adminServiceImpl) WithAccountImportTransaction(ctx context.Context, operation func(context.Context, AdminService) error) error {
	repo, ok := s.accountRepo.(AccountImportTransactionRepository)
	if !ok {
		return errors.New("atomic account import is unavailable")
	}
	pending := &accountImportCommitHooks{admin: s}
	ctx = context.WithValue(ctx, accountImportCommitKey{}, pending)
	err := repo.WithAccountImportTransaction(ctx, func(ctx context.Context, accounts AccountRepository, proxies ProxyRepository) error {
		inner := *s
		inner.accountRepo = accounts
		inner.proxyRepo = proxies
		return operation(ctx, &inner)
	})
	if err == nil {
		for _, operation := range pending.hooks {
			operation()
		}
	}
	return err
}
