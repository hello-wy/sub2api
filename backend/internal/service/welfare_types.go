package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	WelfareBenefitTypeLeaderboard = "leaderboard"
	WelfareBenefitTypeCheckin     = "checkin"
	WelfareBenefitTypeLottery     = "lottery"
)

const (
	WelfareStatusSuccess = "success"
	WelfareStatusRevoked = "revoked"
)

type WelfareRecord struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	UserEmail string    `json:"user_email"`
	Amount    float64   `json:"amount"`
	Remarks   string    `json:"remarks"`
	Status    string    `json:"status"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WelfareSummary struct {
	TotalCount        int64   `json:"total_count"`
	TotalAmount       float64 `json:"total_amount"`
	CheckinAmount     float64 `json:"checkin_amount"`
	LeaderboardAmount float64 `json:"leaderboard_amount"`
	LotteryAmount     float64 `json:"lottery_amount"`
}

type WelfareListFilter struct {
	SearchEmail string
	StartTime   time.Time
	EndTime     time.Time
	BenefitType string
	Status      string
}

type WelfareRepository interface {
	Create(ctx context.Context, userID int64, email string, amount float64, remarks string) (*WelfareRecord, error)
	GetByID(ctx context.Context, id int64, benefitType string) (*WelfareRecord, error)
	MarkRevoked(ctx context.Context, id int64, benefitType string) (bool, error)
	ExistsSuccessByRemarks(ctx context.Context, remarks string) (bool, error)
	List(ctx context.Context, params pagination.PaginationParams, filter WelfareListFilter) ([]WelfareRecord, *WelfareSummary, *pagination.PaginationResult, error)
}
