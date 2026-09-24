package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
)

var ErrGroupProbeBusy = errors.New("group probe already running")
var ErrGroupProbeBudget = errors.New("group probe daily token budget exhausted")
var ErrGroupProbeNotConfigured = errors.New("group probe is not configured")
var ErrGroupProbeUnavailable = errors.New("group probe is unavailable")

type GroupProbeConfig struct {
	GroupID          int64     `json:"group_id"`
	Enabled          bool      `json:"enabled"`
	Model            string    `json:"model"`
	ReasoningEffort  string    `json:"reasoning_effort"`
	IntervalSeconds  int       `json:"interval_seconds"`
	TimeoutSeconds   int       `json:"timeout_seconds"`
	MaxOutputTokens  int       `json:"max_output_tokens"`
	DailyTokenBudget int64     `json:"daily_token_budget"`
	UpdatedAt        time.Time `json:"updated_at"`
	NextRunAt        time.Time `json:"next_run_at"`
	UpdatedBy        int64     `json:"-"`
}
type GroupProbeRun struct {
	ID               string     `json:"id"`
	GroupID          int64      `json:"group_id"`
	Status           string     `json:"status"`
	Model            string     `json:"model"`
	CheckedAt        time.Time  `json:"checked_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	LatencyMs        *int64     `json:"latency_ms,omitempty"`
	InputTokens      int64      `json:"input_tokens"`
	OutputTokens     int64      `json:"output_tokens"`
	EstimatedCostUSD *float64   `json:"estimated_cost_usd,omitempty"`
	UsageRecorded    bool       `json:"usage_recorded"`
	ErrorCode        string     `json:"error_code,omitempty"`
	Scheduled        bool       `json:"scheduled"`
}
type GroupProbeExecution struct {
	Config GroupProbeConfig
	Key    *APIKey
}
type GroupProbeResult struct {
	Success   bool
	ErrorCode string
	LatencyMs int64
}
type GroupProbeRunner func(context.Context, GroupProbeExecution) GroupProbeResult

type groupProbeContextKey struct{}
type groupProbeCapture struct {
	mu            sync.Mutex
	slotHeld      bool
	input, output int64
	cost          float64
	seen          bool
}

// This capability only exists on an internal context. No HTTP header, payload,
// stored API key, or client-controlled ID can opt into probe privileges.
func GroupProbeKeyFromContext(ctx context.Context) *APIKey {
	key, _ := ctx.Value(groupProbeContextKey{}).(*APIKey)
	if key == nil || key.groupProbe == nil {
		return nil
	}
	return key
}
func IsGroupProbe(ctx context.Context) bool { return GroupProbeKeyFromContext(ctx) != nil }

// Each admitted probe owns a private one-slot gateway lease. The service's
// two-probe limit and durable per-group claim bound these leases; they never
// acquire a real user's Redis concurrency/waiting counters.
func AcquireGroupProbeUserSlot(ctx context.Context) (func(), bool) {
	key := GroupProbeKeyFromContext(ctx)
	if key == nil || ctx.Err() != nil {
		return nil, false
	}
	p := key.groupProbe
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.slotHeld {
		return nil, false
	}
	p.slotHeld = true
	var once sync.Once
	return func() { once.Do(func() { p.mu.Lock(); p.slotHeld = false; p.mu.Unlock() }) }, true
}
func CaptureGroupProbeUsage(key *APIKey, log *UsageLog) bool {
	if key == nil || key.groupProbe == nil {
		return false
	}
	p := key.groupProbe
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = true
	p.input += int64(log.InputTokens + log.CacheReadTokens + log.CacheCreationTokens)
	p.output += int64(log.OutputTokens)
	p.cost += log.TotalCost
	return true
}

func ValidateGroupProbeConfig(c *GroupProbeConfig) error {
	c.Model = strings.TrimSpace(c.Model)
	c.ReasoningEffort = strings.TrimSpace(c.ReasoningEffort)
	if c.GroupID <= 0 || c.Model == "" || len(c.Model) > 200 {
		return errors.New("group_id and a valid model are required")
	}
	if c.IntervalSeconds == 0 {
		c.IntervalSeconds = 300
	}
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = 30
	}
	if c.MaxOutputTokens == 0 {
		c.MaxOutputTokens = 64
	}
	if c.DailyTokenBudget == 0 {
		c.DailyTokenBudget = 10000
	}
	if c.IntervalSeconds < 60 || c.IntervalSeconds > 86400 || c.TimeoutSeconds < 5 || c.TimeoutSeconds > 120 || c.MaxOutputTokens < 16 || c.MaxOutputTokens > 1024 || c.DailyTokenBudget < 256 || c.DailyTokenBudget > 1000000 {
		return errors.New("probe limits are outside their allowed range")
	}
	if c.DailyTokenBudget < int64(c.MaxOutputTokens+256) {
		return errors.New("daily token budget must cover one output limit plus 256 input tokens")
	}
	switch c.ReasoningEffort {
	case "", "minimal", "low", "medium", "high", "xhigh":
	default:
		return errors.New("invalid reasoning effort")
	}
	return validateIntelligentTextModel(c.Model)
}

const groupProbeConfigColumns = `group_id,enabled,model,reasoning_effort,interval_seconds,timeout_seconds,max_output_tokens,daily_token_budget,updated_at,next_run_at,updated_by`

func scanGroupProbeConfig(row interface{ Scan(...any) error }) (GroupProbeConfig, error) {
	var c GroupProbeConfig
	e := row.Scan(&c.GroupID, &c.Enabled, &c.Model, &c.ReasoningEffort, &c.IntervalSeconds, &c.TimeoutSeconds, &c.MaxOutputTokens, &c.DailyTokenBudget, &c.UpdatedAt, &c.NextRunAt, &c.UpdatedBy)
	return c, e
}
func (s *GroupStatusService) ProbeConfigs(ctx context.Context) ([]GroupProbeConfig, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT `+groupProbeConfigColumns+` FROM group_probe_configs ORDER BY group_id`)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	items := []GroupProbeConfig{}
	for rows.Next() {
		c, e := scanGroupProbeConfig(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
func (s *GroupStatusService) SaveProbeConfig(ctx context.Context, c GroupProbeConfig, actorID int64) (*GroupProbeConfig, error) {
	if e := ValidateGroupProbeConfig(&c); e != nil {
		return nil, e
	}
	group, e := s.groups.GetByID(ctx, c.GroupID)
	if e != nil {
		return nil, e
	}
	if group.Status != StatusActive {
		return nil, errors.New("group is not active")
	}
	actor, e := s.users.GetByID(ctx, actorID)
	if e != nil || actor == nil || !actor.IsActive() || !actor.IsAdmin() {
		return nil, errors.New("active administrator is required")
	}
	c.UpdatedBy = actorID
	// Deterministic jitter distributes different groups across the first minute.
	next := time.Now().UTC().Add(time.Duration(10+c.GroupID%50) * time.Second)
	out, e := scanGroupProbeConfig(s.db.QueryRowContext(ctx, `INSERT INTO group_probe_configs(group_id,enabled,model,reasoning_effort,interval_seconds,timeout_seconds,max_output_tokens,daily_token_budget,updated_by,next_run_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(group_id) DO UPDATE SET enabled=EXCLUDED.enabled,model=EXCLUDED.model,reasoning_effort=EXCLUDED.reasoning_effort,interval_seconds=EXCLUDED.interval_seconds,timeout_seconds=EXCLUDED.timeout_seconds,max_output_tokens=EXCLUDED.max_output_tokens,daily_token_budget=EXCLUDED.daily_token_budget,updated_by=EXCLUDED.updated_by,next_run_at=EXCLUDED.next_run_at,updated_at=now() RETURNING `+groupProbeConfigColumns, c.GroupID, c.Enabled, c.Model, c.ReasoningEffort, c.IntervalSeconds, c.TimeoutSeconds, c.MaxOutputTokens, c.DailyTokenBudget, actorID, next))
	return &out, e
}
func (s *GroupStatusService) SetProbeRunner(runner GroupProbeRunner) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.runner = runner
	s.startOnce.Do(func() { s.wg.Add(1); go s.probeLoop() })
}
func (s *GroupStatusService) Stop() {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.stopped = true
		s.cancel()
		close(s.outcomes)
		s.mu.Unlock()
		s.wg.Wait()
		<-s.outcomeDone
	})
}
func (s *GroupStatusService) probeLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	lastMaintenance := time.Time{}
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.probeTick()
			if time.Since(lastMaintenance) > time.Minute {
				s.probeMaintenance()
				lastMaintenance = time.Now()
			}
		}
	}
}

func (s *GroupStatusService) probeTick() {
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	if _, err := s.enabledConfig(ctx); err != nil {
		if errors.Is(err, ErrChannelMonitorDisabled) {
			s.cancelRunningProbes(err)
		}
		return
	}
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT group_id FROM group_probe_configs WHERE enabled AND next_run_at<=now() ORDER BY next_run_at LIMIT 1`).Scan(&id); err == nil {
		// Keep the loop responsive to configuration shutdown while a scheduled
		// request is running. The shared active limit still bounds both kinds.
		_, _ = s.startProbe(ctx, id, true, true)
	}
}
func (s *GroupStatusService) StartProbe(ctx context.Context, id int64) (*GroupProbeRun, error) {
	return s.startProbe(ctx, id, false, true)
}

func (s *GroupStatusService) claimProbe(ctx context.Context, id int64, scheduled bool) (*GroupProbeRun, GroupProbeConfig, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, GroupProbeConfig{}, e
	}
	defer func() { _ = tx.Rollback() }()
	c, e := scanGroupProbeConfig(tx.QueryRowContext(ctx, `SELECT `+groupProbeConfigColumns+` FROM group_probe_configs WHERE group_id=$1 FOR UPDATE`, id))
	if errors.Is(e, sql.ErrNoRows) {
		return nil, c, ErrGroupProbeNotConfigured
	}
	if e != nil {
		return nil, c, e
	}
	now := time.Now().UTC()
	if scheduled && (!c.Enabled || c.NextRunAt.After(now)) {
		return nil, c, ErrGroupProbeBusy
	}
	// A dead worker cannot block the group forever; its reservation remains
	// charged against today's budget because upstream consumption is unknown.
	_, e = tx.ExecContext(ctx, `UPDATE group_probe_runs SET status='failed',error_code='worker_interrupted',finished_at=now() WHERE group_id=$1 AND status='running' AND lease_until<now()`, id)
	if e != nil {
		return nil, c, e
	}
	var running bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM group_probe_runs WHERE group_id=$1 AND status='running')`, id).Scan(&running)
	if e != nil {
		return nil, c, e
	}
	if running {
		return nil, c, ErrGroupProbeBusy
	}
	var used int64
	e = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(GREATEST(reserved_tokens,input_tokens+output_tokens)),0) FROM group_probe_runs WHERE group_id=$1 AND checked_at >= date_trunc('day',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'`, id).Scan(&used)
	if e != nil {
		return nil, c, e
	}
	next := now.Add(time.Duration(c.IntervalSeconds+int(c.GroupID%11)) * time.Second)
	_, e = tx.ExecContext(ctx, `UPDATE group_probe_configs SET next_run_at=$2 WHERE group_id=$1`, id, next)
	if e != nil {
		return nil, c, e
	}
	reserved := int64(c.MaxOutputTokens + 256)
	if used+reserved > c.DailyTokenBudget {
		if e = tx.Commit(); e != nil {
			return nil, c, e
		}
		return nil, c, ErrGroupProbeBudget
	}
	run := &GroupProbeRun{ID: uuid.NewString(), GroupID: id, Model: c.Model, Status: "running", CheckedAt: now, Scheduled: scheduled}
	_, e = tx.ExecContext(ctx, `INSERT INTO group_probe_runs(id,group_id,model,status,checked_at,lease_until,reserved_tokens,scheduled) VALUES($1,$2,$3,'running',$4,$5,$6,$7)`, run.ID, id, c.Model, now, now.Add(time.Duration(c.TimeoutSeconds+45)*time.Second), reserved, scheduled)
	if e != nil {
		return nil, c, e
	}
	e = tx.Commit()
	return run, c, e
}
func (s *GroupStatusService) startProbe(ctx context.Context, id int64, scheduled, async bool) (*GroupProbeRun, error) {
	if _, err := s.enabledConfig(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	runner := s.runner
	if runner == nil || s.stopped {
		s.mu.Unlock()
		return nil, ErrGroupProbeUnavailable
	}
	if s.activeProbes >= 2 {
		s.mu.Unlock()
		return nil, ErrGroupProbeBusy
	}
	s.activeProbes++
	s.wg.Add(1)
	s.mu.Unlock()
	release := func() { s.mu.Lock(); s.activeProbes--; s.mu.Unlock(); s.wg.Done() }
	run, c, e := s.claimProbe(ctx, id, scheduled)
	if e != nil {
		release()
		return nil, e
	}
	if async {
		go func() { defer release(); s.executeProbe(run, c, runner) }()
	} else {
		defer release()
		s.executeProbe(run, c, runner)
	}
	copy := *run
	copy.Status = "running"
	return &copy, nil
}

func (s *GroupStatusService) executeProbe(run *GroupProbeRun, c GroupProbeConfig, runner GroupProbeRunner) {
	started := time.Now()
	parent, cancelCause := context.WithCancelCause(s.ctx)
	ctx, cancel := context.WithTimeout(parent, time.Duration(c.TimeoutSeconds)*time.Second)
	defer cancel()
	s.mu.Lock()
	s.probeCancels[run.ID] = cancelCause
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.probeCancels, run.ID)
		s.mu.Unlock()
		cancelCause(nil)
	}()
	result := GroupProbeResult{ErrorCode: "internal_error"}
	capture := &groupProbeCapture{}
	dispatched := false
	defer func() {
		if recover() != nil {
			result = GroupProbeResult{ErrorCode: "internal_error"}
		}
		status := "failed"
		if result.Success {
			status = "success"
		}
		if result.ErrorCode == "busy" || result.ErrorCode == "monitoring_disabled" {
			status = "skipped"
		}
		finish := time.Now().UTC()
		latency := result.LatencyMs
		if latency <= 0 {
			latency = time.Since(started).Milliseconds()
		}
		capture.mu.Lock()
		input, output, cost, seen := capture.input, capture.output, capture.cost, capture.seen
		capture.mu.Unlock()
		writeCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		reserved := int64(c.MaxOutputTokens + 256)
		if (result.Success && seen) || !dispatched {
			reserved = 0
		}
		var reportedCost *float64
		if seen {
			reportedCost = &cost
		}
		if _, err := s.db.ExecContext(writeCtx, `UPDATE group_probe_runs SET status=$2,finished_at=$3,latency_ms=$4,input_tokens=$5,output_tokens=$6,estimated_cost_usd=$7,error_code=$8,reserved_tokens=$9,usage_recorded=$10 WHERE id=$1 AND status='running'`, run.ID, status, finish, latency, input, output, reportedCost, result.ErrorCode, reserved, seen); err != nil {
			logger.LegacyPrintf("group_status", "probe result persistence failed: %v", err)
		}
	}()
	if _, err := s.enabledConfig(ctx); err != nil {
		result.ErrorCode = "monitoring_disabled"
		return
	}
	group, e := s.groups.GetByID(ctx, c.GroupID)
	if e != nil || group == nil || group.Status != StatusActive {
		result.ErrorCode = "group_unavailable"
		return
	}
	actor, e := s.users.GetByID(ctx, c.UpdatedBy)
	if e != nil || actor == nil || !actor.IsActive() || !actor.IsAdmin() {
		result.ErrorCode = "administrator_unavailable"
		return
	}
	// Conservatively yield if any group member is serving requests or waiting.
	// This keeps scheduled diagnostics out of the normal user capacity budget.
	if s.accounts != nil && s.concurrency != nil {
		accounts, e := s.accounts.ListSchedulableByGroupID(ctx, c.GroupID)
		if e != nil {
			result.ErrorCode = "busy"
			return
		}
		ids := make([]int64, 0, len(accounts))
		for _, a := range accounts {
			ids = append(ids, a.ID)
		}
		loads, e := s.concurrency.GetAccountConcurrencyBatch(ctx, ids)
		if e != nil {
			result.ErrorCode = "busy"
			return
		}
		for _, n := range loads {
			if n > 0 {
				result.ErrorCode = "busy"
				return
			}
		}
		for _, id := range ids {
			n, e := s.concurrency.GetAccountWaitingCount(ctx, id)
			if e != nil || n > 0 {
				result.ErrorCode = "busy"
				return
			}
		}
	}
	actorCopy := *actor
	actorCopy.Concurrency = 1
	key := &APIKey{ID: -c.GroupID, UserID: actor.ID, User: &actorCopy, GroupID: &group.ID, Group: group, Status: StatusAPIKeyActive, Name: "internal group probe", groupProbe: capture}
	ctx = context.WithValue(ctx, groupProbeContextKey{}, key)
	// Configuration may have changed while resolving the account pool.
	if _, err := s.enabledConfig(ctx); err != nil {
		result.ErrorCode = "monitoring_disabled"
		return
	}
	dispatched = true
	result = runner(ctx, GroupProbeExecution{Config: c, Key: key})
	if errors.Is(context.Cause(ctx), ErrChannelMonitorDisabled) {
		result.Success = false
		result.ErrorCode = "monitoring_disabled"
	}
}

func (s *GroupStatusService) cancelRunningProbes(cause error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, cancel := range s.probeCancels {
		cancel(cause)
	}
}

func (s *GroupStatusService) ProbeHistory(ctx context.Context, id int64) ([]GroupProbeRun, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid group id")
	}
	rows, e := s.db.QueryContext(ctx, `SELECT id,group_id,status,model,checked_at,finished_at,latency_ms,input_tokens,output_tokens,estimated_cost_usd,error_code,scheduled,usage_recorded FROM group_probe_runs WHERE group_id=$1 ORDER BY checked_at DESC LIMIT 50`, id)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	items := []GroupProbeRun{}
	for rows.Next() {
		var r GroupProbeRun
		if e = rows.Scan(&r.ID, &r.GroupID, &r.Status, &r.Model, &r.CheckedAt, &r.FinishedAt, &r.LatencyMs, &r.InputTokens, &r.OutputTokens, &r.EstimatedCostUSD, &r.ErrorCode, &r.Scheduled, &r.UsageRecorded); e != nil {
			return nil, e
		}
		items = append(items, r)
	}
	return items, rows.Err()
}
