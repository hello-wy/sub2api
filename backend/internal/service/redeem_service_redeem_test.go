package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type redeemRejectRepo struct {
	code      RedeemCode
	useCalled bool
}

func (r *redeemRejectRepo) Create(ctx context.Context, code *RedeemCode) error {
	panic("unexpected Create call")
}

func (r *redeemRejectRepo) CreateBatch(ctx context.Context, codes []RedeemCode) error {
	panic("unexpected CreateBatch call")
}

func (r *redeemRejectRepo) GetByID(ctx context.Context, id int64) (*RedeemCode, error) {
	if r.code.ID != id {
		return nil, ErrRedeemCodeNotFound
	}
	clone := r.code
	return &clone, nil
}

func (r *redeemRejectRepo) GetByCode(ctx context.Context, code string) (*RedeemCode, error) {
	if r.code.Code != code {
		return nil, ErrRedeemCodeNotFound
	}
	clone := r.code
	return &clone, nil
}

func (r *redeemRejectRepo) Update(ctx context.Context, code *RedeemCode) error {
	panic("unexpected Update call")
}

func (r *redeemRejectRepo) BatchUpdate(ctx context.Context, ids []int64, fields RedeemCodeBatchUpdateFields) (int64, error) {
	panic("unexpected BatchUpdate call")
}

func (r *redeemRejectRepo) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (r *redeemRejectRepo) Use(ctx context.Context, id, userID int64) error {
	r.useCalled = true
	r.code.Status = StatusUsed
	r.code.UsedBy = &userID
	return nil
}

func (r *redeemRejectRepo) List(ctx context.Context, params pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (r *redeemRejectRepo) ListWithFilters(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (r *redeemRejectRepo) ListByUser(ctx context.Context, userID int64, limit int) ([]RedeemCode, error) {
	panic("unexpected ListByUser call")
}

func (r *redeemRejectRepo) ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserPaginated call")
}

func (r *redeemRejectRepo) SumPositiveBalanceByUser(ctx context.Context, userID int64) (float64, error) {
	panic("unexpected SumPositiveBalanceByUser call")
}

func TestRedeemRejectsInvitationCodeBeforeTransaction(t *testing.T) {
	ctx := context.Background()
	redeemRepo := &redeemRejectRepo{
		code: RedeemCode{
			ID:     1,
			Code:   "INVITE-001",
			Type:   RedeemTypeInvitation,
			Status: StatusUnused,
		},
	}
	redeemService := NewRedeemService(redeemRepo, nil, nil, nil, nil, nil, nil, nil)

	got, err := redeemService.Redeem(ctx, 2, redeemRepo.code.Code)

	require.Nil(t, got)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.Equal(t, "REDEEM_CODE_UNSUPPORTED_TYPE", infraerrors.Reason(err))
	require.Equal(t, "invitation codes can only be used during registration", infraerrors.Message(err))
	require.False(t, redeemRepo.useCalled)
	require.Equal(t, StatusUnused, redeemRepo.code.Status)
	require.Nil(t, redeemRepo.code.UsedBy)
}

func TestRedeemRequiresConfirmationBeforeOverwritingActiveSubscription(t *testing.T) {
	expiresAt := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	query := regexp.QuoteMeta(`
SELECT id, term_version, expires_at
FROM user_subscriptions
WHERE user_id = $1
  AND group_id = $2
  AND deleted_at IS NULL
  AND status = 'active'
  AND expires_at > NOW()
FOR UPDATE`)
	expectActive := func(termVersion int64) {
		mock.ExpectQuery(query).
			WithArgs(int64(2), int64(3)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "term_version", "expires_at"}).AddRow(int64(11), termVersion, expiresAt))
	}

	redeemService := &RedeemService{}
	expectActive(7)
	err = redeemService.ensureSubscriptionOverwriteConfirmationLocked(context.Background(), db, 2, 3, 30, RedeemOptions{})

	require.Error(t, err)
	require.True(t, infraerrors.IsConflict(err))
	require.Equal(t, "SUBSCRIPTION_OVERWRITE_CONFIRMATION_REQUIRED", infraerrors.Reason(err))
	require.Equal(t, "11", infraerrors.FromError(err).Metadata["subscription_id"])
	require.Equal(t, "7", infraerrors.FromError(err).Metadata["term_version"])
	require.Equal(t, expiresAt.Format(time.RFC3339Nano), infraerrors.FromError(err).Metadata["expires_at"])

	expectActive(7)
	err = redeemService.ensureSubscriptionOverwriteConfirmationLocked(context.Background(), db, 2, 3, 30, RedeemOptions{
		ConfirmSubscriptionOverwrite:    true,
		ExpectedSubscriptionID:          11,
		ExpectedSubscriptionTermVersion: 7,
		ExpectedSubscriptionExpiresAt:   &expiresAt,
	})
	require.NoError(t, err)

	// A renewal increments term_version while keeping the row ID. The old user
	// confirmation must not replace that newer term.
	expectActive(8)
	err = redeemService.ensureSubscriptionOverwriteConfirmationLocked(context.Background(), db, 2, 3, 30, RedeemOptions{
		ConfirmSubscriptionOverwrite:    true,
		ExpectedSubscriptionID:          11,
		ExpectedSubscriptionTermVersion: 7,
		ExpectedSubscriptionExpiresAt:   &expiresAt,
	})
	require.Error(t, err)
	require.Equal(t, "8", infraerrors.FromError(err).Metadata["term_version"])
	require.NoError(t, mock.ExpectationsWereMet())
}
