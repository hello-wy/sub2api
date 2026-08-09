package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/dailycheckinrecord"
	"github.com/Wei-Shaw/sub2api/ent/welfarerecord"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type welfareRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewWelfareRepository(client *dbent.Client, sqlDB *sql.DB) service.WelfareRepository {
	return &welfareRepository{client: client, sql: sqlDB}
}

func (r *welfareRepository) Create(ctx context.Context, userID int64, email string, amount float64, remarks string) (*service.WelfareRecord, error) {
	record, err := clientFromContext(ctx, r.client).WelfareRecord.Create().
		SetUserID(userID).
		SetUserEmail(email).
		SetAmount(amount).
		SetRemarks(remarks).
		SetStatus(service.WelfareStatusSuccess).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.mapEntWelfareRecord(record), nil
}

func (r *welfareRepository) GetByID(ctx context.Context, id int64, benefitType string) (*service.WelfareRecord, error) {
	if benefitType == service.WelfareBenefitTypeCheckin {
		return r.getCheckinByID(ctx, id)
	}
	record, err := clientFromContext(ctx, r.client).WelfareRecord.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.mapEntWelfareRecord(record), nil
}

func (r *welfareRepository) MarkRevoked(ctx context.Context, id int64, benefitType string) (bool, error) {
	if benefitType == service.WelfareBenefitTypeCheckin {
		return r.markCheckinRevoked(ctx, id)
	}
	n, err := clientFromContext(ctx, r.client).WelfareRecord.Update().
		Where(welfarerecord.IDEQ(id), welfarerecord.StatusEQ(service.WelfareStatusSuccess)).
		SetStatus(service.WelfareStatusRevoked).
		Save(ctx)
	return n > 0, err
}

func (r *welfareRepository) ExistsSuccessByRemarks(ctx context.Context, remarks string) (bool, error) {
	return clientFromContext(ctx, r.client).WelfareRecord.Query().
		Where(welfarerecord.RemarksEQ(remarks), welfarerecord.StatusEQ(service.WelfareStatusSuccess)).
		Exist(ctx)
}

func (r *welfareRepository) List(ctx context.Context, params pagination.PaginationParams, filter service.WelfareListFilter) ([]service.WelfareRecord, *service.WelfareSummary, *pagination.PaginationResult, error) {
	exec := txAwareSQLExecutor(ctx, r.sql, r.client)
	if exec == nil {
		return nil, nil, nil, fmt.Errorf("sql executor is not configured")
	}
	baseQuery, args := buildWelfareUnionQuery(filter)
	summary, err := r.getWelfareSummary(ctx, exec, baseQuery, args)
	if err != nil {
		return nil, nil, nil, err
	}
	records, err := r.listWelfareRows(ctx, exec, baseQuery, args, params)
	if err != nil {
		return nil, nil, nil, err
	}
	return records, summary, paginationResultFromTotal(summary.TotalCount, params), nil
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
		Type:      service.WelfareBenefitTypeLeaderboard,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}
}

func (r *welfareRepository) getCheckinByID(ctx context.Context, id int64) (*service.WelfareRecord, error) {
	exec := txAwareSQLExecutor(ctx, r.sql, r.client)
	if exec == nil {
		return nil, fmt.Errorf("sql executor is not configured")
	}
	query := `SELECT d.id, d.user_id, u.email, d.total_reward,
TO_CHAR(d.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD HH24:MI') || ' 签到',
d.status, d.created_at, d.updated_at
FROM daily_checkin_records d
JOIN users u ON u.id = d.user_id
WHERE d.id = $1`
	record := &service.WelfareRecord{Type: service.WelfareBenefitTypeCheckin}
	err := scanSingleRow(ctx, exec, query, []any{id}, &record.ID, &record.UserID, &record.UserEmail, &record.Amount, &record.Remarks, &record.Status, &record.CreatedAt, &record.UpdatedAt)
	return record, err
}

func (r *welfareRepository) markCheckinRevoked(ctx context.Context, id int64) (bool, error) {
	n, err := clientFromContext(ctx, r.client).DailyCheckinRecord.Update().
		Where(dailycheckinrecord.IDEQ(id), dailycheckinrecord.StatusEQ(service.WelfareStatusSuccess)).
		SetStatus(service.WelfareStatusRevoked).
		Save(ctx)
	return n > 0, err
}

func buildWelfareUnionQuery(filter service.WelfareListFilter) (string, []any) {
	args := make([]any, 0)
	parts := welfareUnionParts(filter)
	where := buildWelfareWhere(filter, &args)
	unionQuery := strings.Join(parts, "\nUNION ALL\n")
	return "SELECT * FROM (" + unionQuery + ") welfare_source" + where, args
}

func welfareUnionParts(filter service.WelfareListFilter) []string {
	if filter.BenefitType == service.WelfareBenefitTypeLeaderboard {
		return []string{leaderboardWelfareSelect()}
	}
	if filter.BenefitType == service.WelfareBenefitTypeCheckin {
		return []string{checkinWelfareSelect()}
	}
	if filter.BenefitType == service.WelfareBenefitTypeLottery {
		return []string{lotteryWelfareSelect()}
	}
	return []string{leaderboardWelfareSelect(), checkinWelfareSelect(), lotteryWelfareSelect()}
}

func leaderboardWelfareSelect() string {
	return `SELECT id, user_id, user_email, amount, remarks, status, '` +
		service.WelfareBenefitTypeLeaderboard + `' AS type, created_at, updated_at
FROM welfare_records
WHERE COALESCE(source_type, '') <> 'lottery_draw'`
}

func checkinWelfareSelect() string {
	return `SELECT d.id, d.user_id, u.email AS user_email, d.total_reward AS amount,
       TO_CHAR(d.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD HH24:MI') || ' 签到' AS remarks,
       d.status, '` + service.WelfareBenefitTypeCheckin + `' AS type, d.created_at, d.updated_at
FROM daily_checkin_records d
JOIN users u ON u.id = d.user_id`
}

func lotteryWelfareSelect() string {
	return `SELECT id, user_id, user_email, amount, remarks, status, '` +
		service.WelfareBenefitTypeLottery + `' AS type, created_at, updated_at
FROM welfare_records
WHERE source_type = 'lottery_draw'`
}

func buildWelfareWhere(filter service.WelfareListFilter, args *[]any) string {
	conditions := make([]string, 0)
	if filter.SearchEmail != "" {
		conditions = append(conditions, fmt.Sprintf("user_email ILIKE $%d", appendWelfareArg(args, "%"+filter.SearchEmail+"%")))
	}
	if isKnownWelfareStatus(filter.Status) {
		conditions = append(conditions, fmt.Sprintf("status = $%d", appendWelfareArg(args, filter.Status)))
	}
	if !filter.StartTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", appendWelfareArg(args, filter.StartTime)))
	}
	if !filter.EndTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", appendWelfareArg(args, filter.EndTime)))
	}
	if len(conditions) == 0 {
		return ""
	}
	return "\nWHERE " + strings.Join(conditions, " AND ")
}

func isKnownWelfareStatus(status string) bool {
	return status == service.WelfareStatusSuccess || status == service.WelfareStatusRevoked
}

func appendWelfareArg(args *[]any, value any) int {
	*args = append(*args, value)
	return len(*args)
}

func (r *welfareRepository) getWelfareSummary(ctx context.Context, exec sqlQueryExecutor, baseQuery string, args []any) (*service.WelfareSummary, error) {
	query := `SELECT COUNT(*), COALESCE(SUM(amount), 0),
COALESCE(SUM(CASE WHEN type = 'checkin' THEN amount ELSE 0 END), 0),
COALESCE(SUM(CASE WHEN type = 'leaderboard' THEN amount ELSE 0 END), 0),
COALESCE(SUM(CASE WHEN type = 'lottery' THEN amount ELSE 0 END), 0)
FROM (` + baseQuery + `) welfare_union`
	summary := &service.WelfareSummary{}
	err := scanSingleRow(ctx, exec, query, args, &summary.TotalCount, &summary.TotalAmount, &summary.CheckinAmount, &summary.LeaderboardAmount, &summary.LotteryAmount)
	return summary, err
}

func (r *welfareRepository) listWelfareRows(ctx context.Context, exec sqlQueryExecutor, baseQuery string, args []any, params pagination.PaginationParams) (records []service.WelfareRecord, err error) {
	query := `SELECT id, user_id, user_email, amount, remarks, status, type, created_at, updated_at
FROM (` + baseQuery + `) welfare_union
ORDER BY created_at DESC, id DESC
OFFSET $` + fmt.Sprint(len(args)+1) + ` LIMIT $` + fmt.Sprint(len(args)+2)
	rows, err := exec.QueryContext(ctx, query, append(args, params.Offset(), params.Limit())...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	return scanWelfareRows(rows)
}

func scanWelfareRows(rows *sql.Rows) ([]service.WelfareRecord, error) {
	records := make([]service.WelfareRecord, 0)
	for rows.Next() {
		var record service.WelfareRecord
		if err := rows.Scan(&record.ID, &record.UserID, &record.UserEmail, &record.Amount, &record.Remarks, &record.Status, &record.Type, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
