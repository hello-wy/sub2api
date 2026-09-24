package service

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/lib/pq"
)

// Public DTOs intentionally contain no operational topology, request volume,
// account identity, configuration, or raw upstream errors.
type GroupStatusMetrics struct {
	SuccessRate           *float64 `json:"success_rate"`
	OutputTokensPerSecond *float64 `json:"output_tokens_per_second"`
	TTFTMs                *float64 `json:"ttft_ms"`
}
type GroupStatusBucket struct {
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Status      string    `json:"status"`
	SuccessRate *float64  `json:"success_rate"`
}
type GroupProbeSummary struct {
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
	LatencyMs *int64    `json:"latency_ms,omitempty"`
	Model     string    `json:"model"`
}
type GroupStatusCard struct {
	GroupID   int64               `json:"group_id"`
	GroupName string              `json:"group_name"`
	Platform  string              `json:"platform"`
	Status    string              `json:"status"`
	Metrics   GroupStatusMetrics  `json:"metrics"`
	Buckets   []GroupStatusBucket `json:"buckets"`
	LastProbe *GroupProbeSummary  `json:"last_probe,omitempty"`
}
type GroupStatusResponse struct {
	Enabled   bool              `json:"enabled"`
	UpdatedAt time.Time         `json:"updated_at"`
	Models    []string          `json:"models"`
	Groups    []GroupStatusCard `json:"groups"`
}

type GroupStatusService struct {
	monitor      *ChannelMonitorV2Service
	db           *sql.DB
	groups       GroupRepository
	users        UserRepository
	accounts     AccountRepository
	concurrency  *ConcurrencyService
	settings     channelMonitorRuntimeReader
	runner       GroupProbeRunner
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	startOnce    sync.Once
	stopOnce     sync.Once
	stopped      bool
	activeProbes int
	probeCancels map[string]context.CancelCauseFunc
	outcomes     chan GroupRequestOutcome
	outcomeDone  chan struct{}
}

func NewGroupStatusService(monitor *ChannelMonitorV2Service, db *sql.DB, groups GroupRepository, users UserRepository, accounts AccountRepository, concurrency *ConcurrencyService, settings *SettingService) *GroupStatusService {
	ctx, cancel := context.WithCancel(context.Background())
	s := &GroupStatusService{monitor: monitor, db: db, groups: groups, users: users, accounts: accounts, concurrency: concurrency, ctx: ctx, cancel: cancel, probeCancels: make(map[string]context.CancelCauseFunc), outcomes: make(chan GroupRequestOutcome, 4096), outcomeDone: make(chan struct{})}
	if settings != nil {
		s.settings = settings
	}
	go s.outcomeLoop()
	return s
}

func (s *GroupStatusService) Status(ctx context.Context, filter ChannelMonitorV2Filter) (*GroupStatusResponse, error) {
	cfg, err := s.enabledConfig(ctx)
	if errors.Is(err, ErrChannelMonitorDisabled) {
		return &GroupStatusResponse{Models: []string{}, Groups: []GroupStatusCard{}}, nil
	}
	if err != nil {
		return nil, err
	}
	// The service-status inventory is all active callable groups; the old
	// monitor's hand-picked group/platform selection is not an access policy.
	copyConfig := *cfg
	cfg = &copyConfig
	cfg.GroupIDs = nil
	cfg.IgnoredErrorCategories = []string{"invalid_request", "context_limit", "content_policy", "client_cancelled", "group_access"}
	cfg.Platforms = append([]ChannelMonitorV2PlatformConfig(nil), cfg.Platforms...)
	active, err := s.groups.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	platforms := map[string]bool{}
	for _, g := range active {
		platforms[g.Platform] = true
	}
	for i := range cfg.Platforms {
		cfg.Platforms[i].Enabled = true
		cfg.Platforms[i].Models = nil
		delete(platforms, cfg.Platforms[i].Platform)
	}
	for platform := range platforms {
		cfg.Platforms = append(cfg.Platforms, ChannelMonitorV2PlatformConfig{Platform: platform, Enabled: true})
	}
	// Read the full internal matrix only after the handler has applied server-
	// derived permissions. Construct an independent allow-listed public DTO.
	matrix, err := s.monitor.repo.GetMatrix(ctx, filter, *cfg, ChannelMonitorV2GroupByPlatformGroupModel, false)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	recent := filter
	recent.Start = now.Truncate(time.Minute).Add(-15 * time.Minute)
	recent.End = now.Truncate(time.Minute)
	recent.Bucket = 5 * time.Minute
	current, err := s.monitor.repo.GetMatrix(ctx, recent, *cfg, ChannelMonitorV2GroupByPlatformGroupModel, false)
	if err != nil {
		return nil, err
	}
	fresh := !current.Coverage.DataThrough.IsZero() && now.Sub(current.Coverage.DataThrough) < 10*time.Minute
	result := &GroupStatusResponse{Enabled: true, UpdatedAt: matrix.Coverage.ComputedAt, Models: []string{}, Groups: []GroupStatusCard{}}
	names := map[int64]Group{}
	for _, g := range active {
		names[g.ID] = g
	}
	cards := map[int64]*GroupStatusCard{}
	totals := map[int64]*groupStatusAccumulator{}
	buckets := map[int64]map[int64]*groupStatusAccumulator{}
	models := map[string]bool{}
	for _, g := range active {
		if !groupStatusVisible(g, filter, *cfg) {
			continue
		}
		cards[g.ID] = &GroupStatusCard{GroupID: g.ID, GroupName: g.Name, Platform: g.Platform, Status: "unknown", Buckets: []GroupStatusBucket{}}
		totals[g.ID] = &groupStatusAccumulator{}
		buckets[g.ID] = map[int64]*groupStatusAccumulator{}
	}
	modelFilter := filter
	modelFilter.Models = nil
	dimensions, err := s.monitor.repo.GetDimensions(ctx, modelFilter, *cfg)
	if err != nil {
		return nil, err
	}
	for _, model := range dimensions.Models {
		if model.Value != "" && model.Value != ChannelMonitorV2OtherModel {
			models[model.Value] = true
		}
	}
	for _, row := range matrix.Items {
		if row.GroupID == nil {
			continue
		}
		id := *row.GroupID
		g, exists := names[id]
		if !exists || !groupStatusVisible(g, filter, *cfg) {
			continue
		}
		if cards[id] == nil {
			cards[id] = &GroupStatusCard{GroupID: id, GroupName: g.Name, Platform: g.Platform, Status: "unknown", Buckets: []GroupStatusBucket{}}
			totals[id] = &groupStatusAccumulator{}
			buckets[id] = map[int64]*groupStatusAccumulator{}
		}
		if row.Model != "" && row.Model != ChannelMonitorV2OtherModel {
			models[row.Model] = true
		}
		for _, b := range row.Buckets {
			end := b.BucketStart.Add(filter.Bucket)
			if !end.After(now) && !end.After(matrix.Coverage.DataThrough) && !b.BucketStart.Before(matrix.Coverage.CoverageStart) {
				totals[id].add(b.Metrics)
			}
			key := b.BucketStart.Unix()
			if buckets[id][key] == nil {
				buckets[id][key] = &groupStatusAccumulator{}
			}
			buckets[id][key].add(b.Metrics)
		}
	}
	currents := map[int64]*groupStatusAccumulator{}
	for _, row := range current.Items {
		if row.GroupID == nil {
			continue
		}
		id := *row.GroupID
		if currents[id] == nil {
			currents[id] = &groupStatusAccumulator{}
		}
		currents[id].add(row.Metrics)
	}
	groupIDs := make([]int64, 0, len(cards))
	for id := range cards {
		groupIDs = append(groupIDs, id)
	}
	probes, err := s.latestProbes(ctx, groupIDs, filter.Models)
	if err != nil {
		return nil, err
	}
	for id, card := range cards {
		card.Metrics = totals[id].metrics()
		if fresh && currents[id] != nil {
			card.Status = currents[id].status(cfg.HealthThresholds)
		}
		for start := filter.Start; start.Before(filter.End); start = start.Add(filter.Bucket) {
			b := GroupStatusBucket{Start: start, End: start.Add(filter.Bucket), Status: "unknown"}
			// Only completed and fully aggregated intervals receive a status.
			if !b.End.After(now) && !b.End.After(matrix.Coverage.DataThrough) && !start.Before(matrix.Coverage.CoverageStart) {
				if acc := buckets[id][start.Unix()]; acc != nil {
					b.Status = acc.status(cfg.HealthThresholds)
					b.SuccessRate = acc.metrics().SuccessRate
				}
			}
			card.Buckets = append(card.Buckets, b)
		}
		card.LastProbe = probes[id]
		result.Groups = append(result.Groups, *card)
	}
	for model := range models {
		result.Models = append(result.Models, model)
	}
	sort.Strings(result.Models)
	sort.Slice(result.Groups, func(i, j int) bool { return result.Groups[i].GroupID < result.Groups[j].GroupID })
	return result, nil
}

func groupStatusVisible(g Group, f ChannelMonitorV2Filter, c ChannelMonitorV2Config) bool {
	contains := func(ids []int64, id int64) bool {
		for _, v := range ids {
			if v == id {
				return true
			}
		}
		return false
	}
	if f.RestrictGroups && !contains(f.AllowedGroupIDs, g.ID) {
		return false
	}
	if len(f.GroupIDs) > 0 && !contains(f.GroupIDs, g.ID) {
		return false
	}
	if len(f.Platforms) > 0 {
		match := false
		for _, p := range f.Platforms {
			if p == g.Platform {
				match = true
			}
		}
		if !match {
			return false
		}
	}
	return true
}

type groupStatusAccumulator struct {
	success, errors int64
	ttftSum         float64
	ttftCount       int64
	speedSum        float64
	speedCount      int64
}

func (a *groupStatusAccumulator) add(m ChannelMonitorV2Metric) {
	a.success += m.SuccessRequests
	a.errors += m.EvaluatedErrorRequests
	if m.TTFT.AvgMs != nil {
		a.ttftSum += *m.TTFT.AvgMs * float64(m.TTFT.SampleCount)
		a.ttftCount += m.TTFT.SampleCount
	}
	if m.GenerationTokensPerSecond != nil {
		a.speedSum += *m.GenerationTokensPerSecond * float64(m.GenerationSampleCount)
		a.speedCount += m.GenerationSampleCount
	}
}
func (a *groupStatusAccumulator) metrics() GroupStatusMetrics {
	m := GroupStatusMetrics{}
	if n := a.success + a.errors; n > 0 {
		v := float64(a.success) / float64(n)
		m.SuccessRate = &v
	}
	if a.ttftCount > 0 {
		v := a.ttftSum / float64(a.ttftCount)
		m.TTFTMs = &v
	}
	if a.speedCount > 0 {
		v := a.speedSum / float64(a.speedCount)
		m.OutputTokensPerSecond = &v
	}
	return m
}
func (a *groupStatusAccumulator) status(t ChannelMonitorV2HealthThresholds) string {
	n := a.success + a.errors
	if n == 0 {
		return "unknown"
	}
	if a.success == 0 {
		return "unavailable"
	}
	if n < t.MinimumSample {
		return "unknown"
	}
	if float64(a.errors)/float64(n) >= t.WarningErrorRate {
		return "degraded"
	}
	if a.ttftCount > 0 && a.ttftSum/float64(a.ttftCount) > float64(t.WarningTTFTMs) {
		return "degraded"
	}
	return "operational"
}

func (s *GroupStatusService) latestProbes(ctx context.Context, groupIDs []int64, models []string) (map[int64]*GroupProbeSummary, error) {
	result := make(map[int64]*GroupProbeSummary, len(groupIDs))
	if len(groupIDs) == 0 {
		return result, nil
	}
	// One round trip, with an indexed LIMIT 1 lookup per visible group. A
	// DISTINCT ON over all 90 days would scan every historical probe row.
	query := `SELECT requested.group_id,probe.status,probe.checked_at,probe.latency_ms,probe.model
	 FROM unnest($1::bigint[]) AS requested(group_id)
	 JOIN LATERAL (SELECT status,checked_at,latency_ms,model FROM group_probe_runs WHERE group_id=requested.group_id`
	args := []any{pq.Array(groupIDs)}
	if len(models) > 0 {
		query += ` AND model=ANY($2)`
		args = append(args, pq.Array(models))
	}
	query += ` ORDER BY checked_at DESC,id DESC LIMIT 1) probe ON true`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		v := &GroupProbeSummary{}
		if err = rows.Scan(&id, &v.Status, &v.CheckedAt, &v.LatencyMs, &v.Model); err != nil {
			return nil, err
		}
		result[id] = v
	}
	return result, rows.Err()
}

func (s *GroupStatusService) enabledConfig(ctx context.Context) (*ChannelMonitorV2Config, error) {
	if s.settings != nil {
		runtime := s.settings.GetChannelMonitorRuntime(ctx)
		if !runtime.Enabled || runtime.Mode != "v2" {
			return nil, ErrChannelMonitorDisabled
		}
	}
	if s.monitor == nil {
		return nil, ErrGroupProbeUnavailable
	}
	return s.monitor.getEnabledConfig(ctx)
}

type GroupRequestOutcome struct {
	RequestID        string
	GatewayRequestID string
	GroupID, UserID  int64
	Platform, Model  string
	Success          bool
	ErrorCategory    string
	CompletedAt      time.Time
}

func (s *GroupStatusService) RecordOutcome(ctx context.Context, o GroupRequestOutcome) error {
	if o.RequestID == "" || o.GroupID <= 0 {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.stopped {
		return errors.New("group status is stopping")
	}
	select {
	case s.outcomes <- o:
		return nil
	default:
		return errors.New("group status completion queue is full")
	}
}
