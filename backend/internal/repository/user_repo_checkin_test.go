//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryHasUserQQMatchesAttributeName(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newUserRepositoryWithSQL(nil, db)
	mock.ExpectQuery(`(?s)SELECT EXISTS\(.*LOWER\(d\.key\) LIKE '%qq%'.*LOWER\(d\.name\) LIKE '%qq%'.*NULLIF\(BTRIM\(v\.value\), ''\) IS NOT NULL`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	bound, err := repo.HasUserQQ(ctx, 42)

	require.NoError(t, err)
	require.True(t, bound)
	require.NoError(t, mock.ExpectationsWereMet())
}
