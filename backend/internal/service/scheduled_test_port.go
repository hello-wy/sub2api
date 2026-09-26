package service

import (
	"context"
	"time"
)

// PelicanTestConfig stores intelligence test inputs; a missing kind preserves legacy HTML plans.
type PelicanTestConfig struct {
	QuestionKind    string `json:"question_kind,omitempty"`
	Prompt          string `json:"prompt"`
	ReasoningEffort string `json:"reasoning_effort"`
	ParallelCount   int    `json:"parallel_count"`
	// ModelID is recorded with each result so later edits do not relabel history.
	ModelID string `json:"model_id,omitempty"`
}

// ScheduledTestExecution is the latest group run, including bounded automatic retries.
type ScheduledTestExecution struct {
	CancelRequested bool       `json:"cancel_requested,omitempty"`
	Status          string     `json:"status"`
	Phase           string     `json:"phase,omitempty"`
	Attempt         int        `json:"attempt"`
	MaxAttempts     int        `json:"max_attempts"`
	Total           int        `json:"total"`
	Completed       int        `json:"completed"`
	Succeeded       int        `json:"succeeded"`
	Failed          int        `json:"failed"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	RetryAt         *time.Time `json:"retry_at,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
}

// ScheduledTestPlan represents a scheduled test plan domain model.
type ScheduledTestPlan struct {
	Execution *ScheduledTestExecution `json:"execution,omitempty"`

	PelicanConfig  *PelicanTestConfig `json:"pelican_config,omitempty"`
	RunningUntil   *time.Time         `json:"running_until,omitempty"`
	ID             int64              `json:"id"`
	AccountID      int64              `json:"account_id"`
	GroupID        int64              `json:"group_id,omitempty"`
	APIKeyID       int64              `json:"api_key_id,omitempty"`
	ModelID        string             `json:"model_id"`
	CronExpression string             `json:"cron_expression"`
	Enabled        bool               `json:"enabled"`
	MaxResults     int                `json:"max_results"`
	AutoRecover    bool               `json:"auto_recover"`
	LastRunAt      *time.Time         `json:"last_run_at"`
	NextRunAt      *time.Time         `json:"next_run_at"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// ScheduledTestResult represents a single test execution result.
type ScheduledTestResult struct {
	PelicanConfig *PelicanTestConfig `json:"pelican_config,omitempty"`
	ID            int64              `json:"id"`
	PlanID        int64              `json:"plan_id"`
	Status        string             `json:"status"`
	ResponseText  string             `json:"response_text"`
	ErrorMessage  string             `json:"error_message"`
	LatencyMs     int64              `json:"latency_ms"`
	StartedAt     time.Time          `json:"started_at"`
	FinishedAt    time.Time          `json:"finished_at"`
	CreatedAt     time.Time          `json:"created_at"`
}

// ScheduledTestPlanRepository defines the data access interface for test plans.
type ScheduledTestPlanRepository interface {
	ClaimPelican(ctx context.Context, plan *ScheduledTestPlan, now, until, next time.Time, immediate ...bool) (bool, error)
	FinishPelican(ctx context.Context, id int64, until, finished time.Time) error
	Create(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error)
	GetByID(ctx context.Context, id int64) (*ScheduledTestPlan, error)
	ListByAccountID(ctx context.Context, accountID int64) ([]*ScheduledTestPlan, error)
	ListByGroupID(ctx context.Context, groupID int64) ([]*ScheduledTestPlan, error)
	ListGroupTestKeys(ctx context.Context, groupID int64) ([]*GroupTestKey, error)
	UpdatePelicanExecution(ctx context.Context, id int64, until time.Time, state *ScheduledTestExecution) error
	RequestPelicanCancellation(ctx context.Context, id int64, until time.Time) (bool, error)
	ListDue(ctx context.Context, now time.Time) ([]*ScheduledTestPlan, error)
	Update(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error)
	Delete(ctx context.Context, id int64) error
	UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error
}

// GroupTestKey exposes labels for administrators without exposing credentials.
type GroupTestKey struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	UserEmail string `json:"user_email"`
}

// PelicanHistoryResult includes account identity without exposing account credentials.
type PelicanHistoryResult struct {
	ScheduledTestResult
	AccountID   int64  `json:"account_id"`
	AccountName string `json:"account_name"`
}
type PelicanHistoryPage struct {
	Items      []*PelicanHistoryResult `json:"items"`
	NextCursor int64                   `json:"next_cursor"`
}

// ScheduledTestResultRepository defines the data access interface for test results.
type ScheduledTestResultRepository interface {
	ListPelicanHistory(ctx context.Context, beforeID int64, limit int) ([]*PelicanHistoryResult, error)
	PruneExpiredPelican(ctx context.Context, before time.Time) error
	Create(ctx context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error)
	GetResult(ctx context.Context, planID, resultID int64) (*ScheduledTestResult, error)
	ListByPlanID(ctx context.Context, planID int64, limit int, includeContent ...bool) ([]*ScheduledTestResult, error)
	PruneOldResults(ctx context.Context, planID int64, keepCount int) error
}
