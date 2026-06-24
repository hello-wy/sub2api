package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/welfarerecord"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type welfareRepository struct {
	client *dbent.Client
}

func NewWelfareRepository(client *dbent.Client) service.WelfareRepository {
	return &welfareRepository{client: client}
}

func (r *welfareRepository) Create(ctx context.Context, userID int64, email string, amount float64, remarks string) (*service.WelfareRecord, error) {
	record, err := clientFromContext(ctx, r.client).WelfareRecord.Create().
		SetUserID(userID).
		SetUserEmail(email).
		SetAmount(amount).
		SetRemarks(remarks).
		SetStatus("success").
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.mapEntWelfareRecord(record), nil
}

func (r *welfareRepository) GetByID(ctx context.Context, id int64) (*service.WelfareRecord, error) {
	record, err := clientFromContext(ctx, r.client).WelfareRecord.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.mapEntWelfareRecord(record), nil
}

func (r *welfareRepository) MarkRevoked(ctx context.Context, id int64) (bool, error) {
	n, err := clientFromContext(ctx, r.client).WelfareRecord.Update().
		Where(welfarerecord.IDEQ(id), welfarerecord.StatusEQ("success")).
		SetStatus("revoked").
		Save(ctx)
	return n > 0, err
}

func (r *welfareRepository) ExistsSuccessByRemarks(ctx context.Context, remarks string) (bool, error) {
	return clientFromContext(ctx, r.client).WelfareRecord.Query().
		Where(welfarerecord.RemarksEQ(remarks), welfarerecord.StatusEQ("success")).
		Exist(ctx)
}

func (r *welfareRepository) List(ctx context.Context, params pagination.PaginationParams, searchEmail string) ([]service.WelfareRecord, *pagination.PaginationResult, error) {
	query := clientFromContext(ctx, r.client).WelfareRecord.Query()
	if searchEmail != "" {
		query = query.Where(welfarerecord.UserEmailContains(searchEmail))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	records, err := query.
		Order(dbent.Desc(welfarerecord.FieldCreatedAt)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	results := make([]service.WelfareRecord, 0, len(records))
	for _, rec := range records {
		results = append(results, *r.mapEntWelfareRecord(rec))
	}

	return results, paginationResultFromTotal(int64(total), params), nil
}

func (r *welfareRepository) mapEntWelfareRecord(rec *dbent.WelfareRecord) *service.WelfareRecord {
	if rec == nil {
		return nil
	}
	return &service.WelfareRecord{
		ID:        rec.ID,
		UserID:    rec.UserID,
		UserEmail: rec.UserEmail,
		Amount:    rec.Amount,
		Remarks:   rec.Remarks,
		Status:    rec.Status,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}
}
