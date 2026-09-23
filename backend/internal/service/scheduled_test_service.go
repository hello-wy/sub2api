package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var scheduledTestCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// ScheduledTestService provides CRUD operations for scheduled test plans and results.
type ScheduledTestService struct {
	planRepo   ScheduledTestPlanRepository
	resultRepo ScheduledTestResultRepository
}

// NewScheduledTestService creates a new ScheduledTestService.
func NewScheduledTestService(
	planRepo ScheduledTestPlanRepository,
	resultRepo ScheduledTestResultRepository,
) *ScheduledTestService {
	return &ScheduledTestService{
		planRepo:   planRepo,
		resultRepo: resultRepo,
	}
}

// CreatePlan validates the cron expression, computes next_run_at, and persists the plan.
func (s *ScheduledTestService) CreatePlan(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	nextRun, err := nextPlanRun(plan, time.Now())
	if err != nil {
		return nil, fmt.Errorf("invalid test schedule: %w", err)
	}
	plan.NextRunAt = &nextRun
	if plan.PelicanConfig != nil && plan.Enabled {
		expires := time.Now().Add(time.Duration(plan.PelicanConfig.RunForHours) * time.Hour)
		plan.ExpiresAt = &expires
	}

	if plan.MaxResults <= 0 {
		plan.MaxResults = 50
	}

	return s.planRepo.Create(ctx, plan)
}

// GetPlan retrieves a plan by ID.
func (s *ScheduledTestService) GetPlan(ctx context.Context, id int64) (*ScheduledTestPlan, error) {
	return s.planRepo.GetByID(ctx, id)
}

// ListPlansByAccount returns all plans for a given account.
func (s *ScheduledTestService) ListPlansByAccount(ctx context.Context, accountID int64) ([]*ScheduledTestPlan, error) {
	return s.planRepo.ListByAccountID(ctx, accountID)
}

// UpdatePlan validates cron and updates the plan.
func (s *ScheduledTestService) UpdatePlan(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	nextRun, err := nextPlanRun(plan, time.Now())
	if err != nil {
		return nil, fmt.Errorf("invalid test schedule: %w", err)
	}
	plan.NextRunAt = &nextRun
	if plan.PelicanConfig != nil && plan.Enabled {
		expires := time.Now().Add(time.Duration(plan.PelicanConfig.RunForHours) * time.Hour)
		plan.ExpiresAt = &expires
	}

	return s.planRepo.Update(ctx, plan)
}

// DeletePlan removes a plan and its results (via CASCADE).
func (s *ScheduledTestService) DeletePlan(ctx context.Context, id int64) error {
	return s.planRepo.Delete(ctx, id)
}

// ListResults returns the most recent results for a plan.
func (s *ScheduledTestService) ListResults(ctx context.Context, planID int64, limit int, includeContent ...bool) ([]*ScheduledTestResult, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.resultRepo.ListByPlanID(ctx, planID, limit, includeContent...)
}

// SaveResult inserts a result and prunes old entries beyond maxResults.
func (s *ScheduledTestService) SaveResult(ctx context.Context, planID int64, maxResults int, result *ScheduledTestResult) error {
	result.PlanID = planID
	if _, err := s.resultRepo.Create(ctx, result); err != nil {
		return err
	}
	return s.resultRepo.PruneOldResults(ctx, planID, maxResults)
}

func computeNextRun(cronExpr string, from time.Time) (time.Time, error) {
	sched, err := scheduledTestCronParser.Parse(cronExpr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(from), nil
}

func nextPlanRun(plan *ScheduledTestPlan, now time.Time) (time.Time, error) {
	if cfg := plan.PelicanConfig; cfg != nil {
		if cfg.RunForHours == 0 {
			cfg.RunForHours = 24
		}
		if cfg.RunForHours < 1 || cfg.RunForHours > 168 {
			return time.Time{}, fmt.Errorf("run duration must be 1–168 hours")
		}
		if cfg.IntervalMinutes >= cfg.RunForHours*60 {
			return time.Time{}, fmt.Errorf("interval must be shorter than the run duration")
		}
		if strings.TrimSpace(cfg.Prompt) == "" || len(cfg.Prompt) > 32000 || strings.TrimSpace(plan.ModelID) == "" || len(plan.ModelID) > 100 {
			return time.Time{}, fmt.Errorf("pelican prompt and model are required (maximum 32000/100 bytes)")
		}
		if cfg.IntervalMinutes < 1 || cfg.IntervalMinutes > 10080 || cfg.ParallelCount < 1 || cfg.ParallelCount > 8 {
			return time.Time{}, fmt.Errorf("interval must be 1–10080 minutes; parallel count must be 1–8")
		}
		if normalizePelicanReasoningEffort(cfg.ReasoningEffort) == "" {
			return time.Time{}, fmt.Errorf("invalid reasoning effort")
		}
		if plan.MaxResults <= 0 {
			plan.MaxResults = 50
		}
		if plan.MaxResults > 50 {
			return time.Time{}, fmt.Errorf("pelican history retention cannot exceed 50 results")
		}
		cfg.ModelID = plan.ModelID
		plan.AutoRecover = false
		return now.Add(time.Duration(cfg.IntervalMinutes) * time.Minute), nil
	}
	return computeNextRun(plan.CronExpression, now)
}

func (s *ScheduledTestService) GetResult(ctx context.Context, planID, resultID int64) (*ScheduledTestResult, error) {
	return s.resultRepo.GetResult(ctx, planID, resultID)
}
