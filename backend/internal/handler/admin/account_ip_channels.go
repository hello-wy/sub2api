package admin

import (
	"context"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type AccountIPChannelResponse struct {
	RateLimit429Enabled     bool                              `json:"rate_limit_429_enabled"`
	TempUnschedulableReason string                            `json:"temp_unschedulable_reason,omitempty"`
	Upstream429Observation  *account429Observation            `json:"upstream_429_observation,omitempty"`
	CodexTicket             *service.CodexAccountTicketStatus `json:"codex_ticket,omitempty"`
	Recoverable             bool                              `json:"recoverable"`
	ModelMismatch           any                               `json:"model_mismatch,omitempty"`
	ID                      int64                             `json:"id"`
	LogicalAccountID        int64                             `json:"logical_account_id"`
	ProxyID                 *int64                            `json:"proxy_id"`
	Proxy                   *dto.Proxy                        `json:"proxy,omitempty"`
	Concurrency             int                               `json:"concurrency"`
	CurrentConcurrency      *int                              `json:"current_concurrency"`
	Priority                int                               `json:"priority"`
	LoadFactor              *int                              `json:"load_factor"`
	LogicalEnabled          bool                              `json:"logical_enabled"`
	Status                  string                            `json:"status"`
	Healthy                 bool                              `json:"healthy"`
	Schedulable             bool                              `json:"schedulable"`
	Enabled                 bool                              `json:"enabled"`
	DisabledAt              *time.Time                        `json:"disabled_at,omitempty"`
	ErrorMessage            string                            `json:"error_message,omitempty"`
	RateLimitResetAt        *time.Time                        `json:"rate_limit_reset_at,omitempty"`
	OverloadUntil           *time.Time                        `json:"overload_until,omitempty"`
	TempUnschedulableUntil  *time.Time                        `json:"temp_unschedulable_until,omitempty"`
}

type account429Observation struct {
	StatusCode int    `json:"status_code"`
	ObservedAt string `json:"observed_at"`
	ResetAt    string `json:"reset_at,omitempty"`
	Ignored    bool   `json:"ignored"`
}

func channel429Observation(account *service.Account) *account429Observation {
	if account == nil {
		return nil
	}
	value, ok := account.Extra["upstream_429_observation"].(map[string]any)
	if !ok {
		return nil
	}
	observed, _ := value["observed_at"].(string)
	if observed == "" {
		return nil
	}
	reset, _ := value["reset_at"].(string)
	ignored, _ := value["ignored"].(bool)
	return &account429Observation{StatusCode: 429, ObservedAt: observed, ResetAt: reset, Ignored: ignored}
}

func channelModelMismatchEvidence(account *service.Account) any {
	if !account.HasModelMismatch() {
		return nil
	}
	return account.Extra[service.AccountModelMismatchExtraKey]
}

func channelConcurrencyValue(counts map[int64]int, id int64) *int {
	if counts == nil {
		return nil
	}
	v := counts[id]
	return &v
}

// Display uses the already-loaded STATE summary as the authoritative health
// signal for every account that explicitly enabled STATE.
func channelBusinessHealthy(account *service.Account, ticket *service.CodexAccountTicketStatus) bool {
	if account == nil || !account.IsSchedulable() || account.Proxy == nil || !account.Proxy.IsActive() || account.Proxy.IsExpired(time.Now()) {
		return false
	}
	if ticket != nil && ticket.Enabled {
		return ticket.TicketUsable && ticket.GlobalEnabled && !ticket.AuthenticationBlocked && ticket.ExpiresAt != nil && ticket.ExpiresAt.After(time.Now())
	}
	return true
}

func (h *AccountHandler) listCodexRoutingAccounts(ctx context.Context) ([]service.Account, error) {
	if manager, ok := h.adminService.(service.AccountIPChannelAdmin); ok {
		return manager.ListRoutingAccounts(ctx, service.PlatformOpenAI, service.AccountTypeOAuth, "", "", 0, "")
	}
	return h.listAccountsFiltered(ctx, service.PlatformOpenAI, service.AccountTypeOAuth, "", "", 0, "", "created_at", "desc")
}

func (h *AccountHandler) accountIPChannelResponses(ctx context.Context, ids []int64) (map[int64][]AccountIPChannelResponse, error) {
	result := map[int64][]AccountIPChannelResponse{}
	manager, ok := h.adminService.(service.AccountIPChannelAdmin)
	if !ok {
		return result, nil
	}
	channels, err := manager.GetAccountIPChannels(ctx, ids)
	if err != nil {
		// 账号详情中的固定 IP/逻辑通道元数据属于安全门禁的一部分。
		// 读取失败时必须 fail-closed，避免前端把不可验证的账号展示成可编辑、
		// 可调度状态；调用方仍可在写入成功后返回明确的 unavailable 标记。
		return nil, infraerrors.ServiceUnavailable("IP_CHANNEL_METADATA_UNAVAILABLE", "IP channel metadata unavailable")
	}
	all := []int64{}
	seen := map[int64]bool{}
	for _, members := range channels {
		for _, c := range members {
			if !seen[c.Account.ID] {
				all = append(all, c.Account.ID)
				seen[c.Account.ID] = true
			}
		}
	}
	var counts map[int64]int
	if h.concurrencyService != nil && len(all) > 0 {
		if v, e := h.concurrencyService.GetAccountConcurrencyBatchStrict(ctx, all); e == nil {
			counts = v
		}
	}
	for id, members := range channels {
		for _, c := range members {
			a := c.Account
			result[id] = append(result[id], AccountIPChannelResponse{Recoverable: service.AccountHasRecoverableState(a), ID: a.ID, LogicalAccountID: c.LogicalAccountID, ProxyID: a.ProxyID, Proxy: dto.ProxyFromService(a.Proxy), Concurrency: a.Concurrency, CurrentConcurrency: channelConcurrencyValue(counts, a.ID), Priority: a.Priority, LoadFactor: a.LoadFactor, LogicalEnabled: c.LogicalEnabled, Status: a.Status, Schedulable: a.Schedulable, Enabled: c.Enabled, DisabledAt: c.DisabledAt, ErrorMessage: a.ErrorMessage, RateLimitResetAt: a.RateLimitResetAt, OverloadUntil: a.OverloadUntil, TempUnschedulableUntil: a.TempUnschedulableUntil})
			result[id][len(result[id])-1].ModelMismatch = channelModelMismatchEvidence(a)
			result[id][len(result[id])-1].CodexTicket = h.accountTicketSummary(ctx, a)
			result[id][len(result[id])-1].RateLimit429Enabled = service.Context429Enforcement(ctx)
			result[id][len(result[id])-1].TempUnschedulableReason = a.TempUnschedulableReason
			result[id][len(result[id])-1].Upstream429Observation = channel429Observation(a)
			result[id][len(result[id])-1].Healthy = c.Enabled && c.LogicalEnabled && channelBusinessHealthy(a, result[id][len(result[id])-1].CodexTicket)
		}
	}
	return result, nil
}

func (h *AccountHandler) enrichIPChannels(ctx context.Context, items []AccountWithConcurrency) error {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		if item.Account != nil {
			ids = append(ids, item.ID)
		}
	}
	channels, err := h.accountIPChannelResponses(ctx, ids)
	if err != nil {
		return err
	}
	for i := range items {
		cs := channels[items[i].ID]
		if len(cs) == 0 {
			continue
		}
		items[i].IPChannels = cs
		items[i].ChannelCount = len(cs)
		items[i].Schedulable = cs[0].LogicalEnabled
		items[i].CurrentConcurrency = 0
		healthy := 0
		for _, c := range cs {
			if c.CurrentConcurrency != nil {
				items[i].CurrentConcurrency += *c.CurrentConcurrency
			}
			if c.Healthy {
				healthy++
			}
		}
		items[i].ChannelStatus = "unavailable"
		if healthy == len(cs) {
			items[i].ChannelStatus = "normal"
		} else if healthy > 0 {
			items[i].ChannelStatus = "partial"
		}
	}
	return nil
}

func ipChannelIDs(c *gin.Context) (int64, int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, 0, infraerrors.BadRequest("ACCOUNT_ID", "账号 ID 无效")
	}
	var cid int64
	if v := c.Param("channelId"); v != "" {
		cid, err = strconv.ParseInt(v, 10, 64)
		if err != nil || cid <= 0 {
			return 0, 0, infraerrors.BadRequest("CHANNEL_ID", "IP 通道 ID 无效")
		}
	}
	return id, cid, nil
}

func (h *AccountHandler) GetIPChannels(c *gin.Context) {
	id, _, err := ipChannelIDs(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items, err := h.accountIPChannelResponses(c.Request.Context(), []int64{id})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if len(items[id]) == 0 {
		a, err := h.adminService.GetAccount(c.Request.Context(), id)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if a.ProxyID != nil {
			var counts map[int64]int
			if h.concurrencyService != nil {
				counts, _ = h.concurrencyService.GetAccountConcurrencyBatchStrict(c.Request.Context(), []int64{id})
			}
			items[id] = []AccountIPChannelResponse{{Recoverable: service.AccountHasRecoverableState(a), ModelMismatch: channelModelMismatchEvidence(a), ID: id, LogicalAccountID: id, ProxyID: a.ProxyID, Proxy: dto.ProxyFromService(a.Proxy), Concurrency: a.Concurrency, CurrentConcurrency: channelConcurrencyValue(counts, id), Priority: a.Priority, LoadFactor: a.LoadFactor, LogicalEnabled: a.Schedulable, Healthy: a.IsSchedulable() && a.Proxy != nil && a.Proxy.IsActive() && !a.Proxy.IsExpired(time.Now()), Status: a.Status, Schedulable: a.Schedulable, Enabled: a.Schedulable, ErrorMessage: a.ErrorMessage, RateLimitResetAt: a.RateLimitResetAt, OverloadUntil: a.OverloadUntil, TempUnschedulableUntil: a.TempUnschedulableUntil}}
			items[id][0].CodexTicket = h.accountTicketSummary(c.Request.Context(), a)
			items[id][0].Healthy = channelBusinessHealthy(a, items[id][0].CodexTicket)
			items[id][0].RateLimit429Enabled = service.Context429Enforcement(c.Request.Context())
			items[id][0].TempUnschedulableReason = a.TempUnschedulableReason
			items[id][0].Upstream429Observation = channel429Observation(a)
		}
	}
	if items[id] == nil {
		items[id] = []AccountIPChannelResponse{}
	}
	response.Success(c, gin.H{"items": items[id]})
}

func (h *AccountHandler) AddIPChannels(c *gin.Context) {
	id, _, err := ipChannelIDs(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req struct {
		ProxyIDs    []int64 `json:"proxy_ids" binding:"required"`
		Concurrency *int    `json:"concurrency"`
		Priority    *int    `json:"priority"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	manager, ok := h.adminService.(service.AccountIPChannelAdmin)
	if !ok {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("IP_CHANNEL_UNAVAILABLE", "IP channel service unavailable"))
		return
	}
	if err = manager.AddAccountIPChannels(c.Request.Context(), id, req.ProxyIDs, req.Concurrency, req.Priority); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.GetIPChannels(c)
}

func (h *AccountHandler) ensureIPChannelIdle(ctx context.Context, id int64) error {
	if h.concurrencyService == nil {
		return infraerrors.ServiceUnavailable("IP_CHANNEL_CONCURRENCY_UNAVAILABLE", "无法确认通道是否已排空，请稍后重试")
	}
	counts, err := h.concurrencyService.GetAccountConcurrencyBatchStrict(ctx, []int64{id})
	if err != nil || counts == nil {
		return infraerrors.ServiceUnavailable("IP_CHANNEL_CONCURRENCY_UNAVAILABLE", "无法读取实时并发，请稍后重试")
	}
	if counts[id] > 0 {
		return infraerrors.Conflict("IP_CHANNEL_BUSY", "通道仍有请求正在执行，请等待完成")
	}
	return nil
}

func (h *AccountHandler) PatchIPChannel(c *gin.Context) {
	id, cid, err := ipChannelIDs(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req service.AccountIPChannelPatch
	if err = c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.ProxyID != nil {
		if err = h.ensureIPChannelIdle(c.Request.Context(), cid); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	manager, ok := h.adminService.(service.AccountIPChannelAdmin)
	if !ok {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("IP_CHANNEL_UNAVAILABLE", "IP channel service unavailable"))
		return
	}
	if err = manager.PatchAccountIPChannel(c.Request.Context(), id, cid, req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": true})
}

func (h *AccountHandler) DeleteIPChannel(c *gin.Context) {
	id, cid, err := ipChannelIDs(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err = h.ensureIPChannelIdle(c.Request.Context(), cid); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	manager, ok := h.adminService.(service.AccountIPChannelAdmin)
	if !ok {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("IP_CHANNEL_UNAVAILABLE", "IP channel service unavailable"))
		return
	}
	if err = manager.RemoveAccountIPChannel(c.Request.Context(), id, cid); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *AccountHandler) GetIPChannelStats(c *gin.Context) {
	id, cid, err := ipChannelIDs(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	manager, ok := h.adminService.(service.AccountIPChannelAdmin)
	if !ok {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("IP_CHANNEL_UNAVAILABLE", "IP channel service unavailable"))
		return
	}
	members, err := manager.AccountIPChannelHistoryIDs(c.Request.Context(), []int64{id})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	found := false
	for _, member := range members[id] {
		if member == cid {
			found = true
		}
	}
	if !found {
		response.ErrorFrom(c, infraerrors.NotFound("IP_CHANNEL_NOT_FOUND", "IP 通道不属于此账号"))
		return
	}
	days := 30
	if d, e := strconv.Atoi(c.Query("days")); e == nil && d > 0 && d <= 90 {
		days = d
	}
	now := timezone.Now()
	end := timezone.StartOfDay(now.AddDate(0, 0, 1))
	start := timezone.StartOfDay(now.AddDate(0, 0, -days+1))
	stats, err := h.accountUsageService.GetAccountUsageStats(c.Request.Context(), cid, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

func (h *AccountHandler) RecoverIPChannelState(c *gin.Context) {
	id, channelID, err := ipChannelIDs(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	manager, ok := h.adminService.(service.AccountIPChannelAdmin)
	if !ok || h.rateLimitService == nil {
		response.Error(c, 503, "IP channel recovery unavailable")
		return
	}
	groups, err := manager.GetAccountIPChannels(c.Request.Context(), []int64{id})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	found := false
	for _, ch := range groups[id] {
		if ch.LogicalAccountID == id && ch.Account.ID == channelID {
			found = true
			break
		}
	}
	if !found && id == channelID && len(groups[id]) == 0 {
		account, getErr := h.adminService.GetAccount(c.Request.Context(), id)
		found = getErr == nil && account != nil && account.Platform == service.PlatformOpenAI && account.Type == service.AccountTypeOAuth && account.ProxyID != nil && !account.IsCredentialShadow() && !account.IsRandomProxy()
	}
	if !found {
		response.NotFound(c, "IP channel not found")
		return
	}
	result, err := h.rateLimitService.RecoverAccountState(c.Request.Context(), channelID, service.AccountRecoveryOptions{InvalidateToken: true})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
