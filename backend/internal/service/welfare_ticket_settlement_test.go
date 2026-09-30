package service

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type ticketRebateSettingStub struct {
	SettingRepository
	values map[string]string
}

func (s ticketRebateSettingStub) GetValue(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func TestTicketRebateSwitchDefaultsOff(t *testing.T) {
	svc := &WelfareService{settingRepo: ticketRebateSettingStub{values: map[string]string{}}}
	enabled, rules, err := svc.loadTicketRebateSettings(context.Background())
	require.NoError(t, err)
	require.False(t, enabled)
	require.Nil(t, rules)
}

func newTicketSettlementTestService(t *testing.T) (*WelfareService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return &WelfareService{entClient: client, db: db}, mock
}

func TestDailyTicketRebateDisabledDoesNotIssueTickets(t *testing.T) {
	svc, mock := newTicketSettlementTestService(t)
	day := time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)
	candidate := dailyRewardCandidate{userID: 7, consumption: decimal.NewFromInt(20)}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(amount\\), 0\\), COUNT\\(\\*\\) FROM welfare_records").
		WithArgs(int64(7), "2026-09-30 消费 $%").
		WillReturnRows(sqlmock.NewRows([]string{"amount", "count"}).AddRow(0, 0))
	mock.ExpectQuery("INSERT INTO daily_ticket_rebates").
		WithArgs(int64(7), "2026-09-30", "20", 0, "[]", float64(0)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectExec("UPDATE daily_ticket_rebates SET status = 'success'").
		WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.settleDailyReward(context.Background(), day, candidate, false, nil)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDailyTicketRebateReplayDoesNotIssueTickets(t *testing.T) {
	svc, mock := newTicketSettlementTestService(t)
	day := time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)
	candidate := dailyRewardCandidate{userID: 7, consumption: decimal.NewFromInt(20)}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(amount\\), 0\\), COUNT\\(\\*\\) FROM welfare_records").
		WithArgs(int64(7), "2026-09-30 消费 $%").
		WillReturnRows(sqlmock.NewRows([]string{"amount", "count"}).AddRow(0, 0))
	mock.ExpectQuery("INSERT INTO daily_ticket_rebates").
		WithArgs(int64(7), "2026-09-30", "20", 4, "[]", float64(0)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	err := svc.settleDailyRewardTx(context.Background(), day, candidate, 4, []byte("[]"))
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDailyTicketRebateIssuesLedgerAndBalanceInOneTransaction(t *testing.T) {
	svc, mock := newTicketSettlementTestService(t)
	day := time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)
	candidate := dailyRewardCandidate{userID: 7, consumption: decimal.NewFromInt(20)}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(amount\\), 0\\), COUNT\\(\\*\\) FROM welfare_records").
		WithArgs(int64(7), "2026-09-30 消费 $%").
		WillReturnRows(sqlmock.NewRows([]string{"amount", "count"}).AddRow(0, 0))
	mock.ExpectQuery("INSERT INTO daily_ticket_rebates").
		WithArgs(int64(7), "2026-09-30", "20", 4, "[]", float64(0)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectExec("INSERT INTO lottery_user_states").WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT ticket_debt FROM lottery_user_states").WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"ticket_debt"}).AddRow(1))
	mock.ExpectExec("INSERT INTO lottery_ticket_ledger").
		WithArgs(int64(7), 4, 3, "7:2026-09-30", "2026-09-30").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE lottery_user_states SET ticket_debt").
		WithArgs(int64(7), 0).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE daily_ticket_rebates SET status = 'success'").
		WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.settleDailyRewardTx(context.Background(), day, candidate, 4, []byte("[]"))
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDailyTicketRebatePreservesAlreadyPaidAmount(t *testing.T) {
	svc, mock := newTicketSettlementTestService(t)
	day := time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)
	candidate := dailyRewardCandidate{userID: 7, consumption: decimal.NewFromInt(20), amount: 10}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(amount\\), 0\\), COUNT\\(\\*\\) FROM welfare_records").
		WithArgs(int64(7), "2026-09-30 消费 $%").
		WillReturnRows(sqlmock.NewRows([]string{"amount", "count"}).AddRow(2.5, 1))
	mock.ExpectQuery("INSERT INTO daily_ticket_rebates").
		WithArgs(int64(7), "2026-09-30", "20", 0, "[]", 2.5).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectExec("UPDATE daily_ticket_rebates SET status = 'success'").
		WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.settleDailyRewardTx(context.Background(), day, candidate, 0, []byte("[]"))
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
