package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountPlatformCounts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rows    *sqlmock.Rows
		failure error
		want    []service.AccountPlatformCount
	}{
		{name: "counts every non-deleted provider independent of status and pagination", rows: sqlmock.NewRows([]string{"platform", "count"}).AddRow("deepseek", 2).AddRow("openai", 45), want: []service.AccountPlatformCount{{Platform: "deepseek", Count: 2}, {Platform: "openai", Count: 45}}},
		{name: "empty inventory returns an array", rows: sqlmock.NewRows([]string{"platform", "count"}), want: []service.AccountPlatformCount{}},
		{name: "query failure propagates", failure: errors.New("database unavailable")},
		{name: "partial results are not accepted", rows: sqlmock.NewRows([]string{"platform", "count"}).AddRow("openai", 45).RowError(0, errors.New("read failure"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			query := mock.ExpectQuery(`SELECT platform, COUNT\(\*\) FROM accounts WHERE deleted_at IS NULL GROUP BY platform ORDER BY platform`)
			if tc.failure != nil {
				query.WillReturnError(tc.failure)
			} else {
				query.WillReturnRows(tc.rows)
			}
			repo := newAccountRepositoryWithSQL(nil, db, nil)
			counts, err := repo.CountByPlatform(context.Background())
			if tc.want != nil {
				require.NoError(t, err)
				require.Equal(t, tc.want, counts)
			} else {
				require.Error(t, err)
				require.Nil(t, counts)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
