package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// CountByPlatform reads only the provider and count; credentials and account details never leave the database.
func (r *accountRepository) CountByPlatform(ctx context.Context) ([]service.AccountPlatformCount, error) {
	rows, err := r.sql.QueryContext(ctx, `SELECT platform, COUNT(*) FROM accounts WHERE deleted_at IS NULL GROUP BY platform ORDER BY platform`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make([]service.AccountPlatformCount, 0)
	for rows.Next() {
		var count service.AccountPlatformCount
		if err := rows.Scan(&count.Platform, &count.Count); err != nil {
			return nil, err
		}
		counts = append(counts, count)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}
