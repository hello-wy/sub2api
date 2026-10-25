package service

import "context"

// GetAccountPlatformCounts ignores list filters so shortcuts remain stable while switching providers.
func (s *adminServiceImpl) GetAccountPlatformCounts(ctx context.Context) ([]AccountPlatformCount, error) {
	return s.accountPlatformRepo.CountByPlatform(ctx)
}
