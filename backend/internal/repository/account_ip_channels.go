package repository

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	dbaccountgroup "github.com/Wei-Shaw/sub2api/ent/accountgroup"
	"github.com/Wei-Shaw/sub2api/ent/predicate"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const accountIPChannelMutationLock int64 = 736282018

// Scheduler retries only need IDs, never the credentials/proxies/group details
// returned by the management endpoint.
func (r *accountRepository) AccountIPChannelSiblingIDs(ctx context.Context, id int64) ([]int64, error) {
	rows, err := r.sql.QueryContext(ctx, `SELECT sibling.account_id FROM account_ip_channels selected JOIN account_ip_channels sibling ON sibling.logical_account_id=selected.logical_account_id JOIN accounts a ON a.id=sibling.account_id WHERE selected.account_id=$1 AND sibling.retired_at IS NULL AND a.deleted_at IS NULL ORDER BY sibling.account_id`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := []int64{}
	for rows.Next() {
		var memberID int64
		if err = rows.Scan(&memberID); err != nil {
			return nil, err
		}
		ids = append(ids, memberID)
	}
	return ids, rows.Err()
}

func (r *accountRepository) ListIPChannelRoutingAccounts(ctx context.Context, platform, typ, status, search string, groupID int64, privacy string) ([]service.Account, error) {
	q := r.accountListFilteredQuery(platform, typ, status, search, groupID, privacy)
	q.Where(predicate.Account(func(s *sql.Selector) {
		s.Where(sql.ExprP("NOT EXISTS (SELECT 1 FROM account_ip_channels c WHERE c.account_id=" + s.C("id") + " AND c.retired_at IS NOT NULL)"))
	}))
	entities, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	return r.accountsToService(ctx, entities)
}

// Only management queries collapse routing records. Gateway and import queries
// intentionally continue returning every independently schedulable channel.
func (r *accountRepository) ListLogicalAccounts(ctx context.Context, params pagination.PaginationParams, platform, typ, status, search string, groupID int64, privacy string) ([]service.Account, *pagination.PaginationResult, error) {
	q := r.accountListFilteredQuery(platform, typ, status, search, groupID, privacy)
	q.Where(predicate.Account(func(s *sql.Selector) {
		s.Where(sql.ExprP("NOT EXISTS (SELECT 1 FROM account_ip_channels retired WHERE retired.account_id = " + s.C("id") + " AND retired.retired_at IS NOT NULL)"))
	}))
	ids, err := q.IDs(ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(ids) == 0 {
		return []service.Account{}, paginationResultFromTotal(0, params), nil
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT DISTINCT COALESCE(c.logical_account_id,a.id) FROM accounts a LEFT JOIN account_ip_channels c ON c.account_id=a.id WHERE a.id=ANY($1)`, pq.Array(ids))
	if err != nil {
		return nil, nil, err
	}
	logicalIDs := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, nil, err
		}
		logicalIDs = append(logicalIDs, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, nil, err
	}
	accountsQuery := r.client.Account.Query().Where(dbaccount.IDIn(logicalIDs...)).Offset(params.Offset()).Limit(params.Limit())
	for _, order := range accountListOrder(params) {
		accountsQuery.Order(order)
	}
	entities, err := accountsQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}
	accounts, err := r.accountsToService(ctx, entities)
	return accounts, paginationResultFromTotal(int64(len(logicalIDs)), params), err
}

func (r *accountRepository) GetAccountIPChannels(ctx context.Context, ids []int64) (map[int64][]service.AccountIPChannel, error) {
	result := map[int64][]service.AccountIPChannel{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT c.account_id,c.logical_account_id,c.enabled,c.disabled_at,l.enabled FROM account_ip_channels c JOIN accounts a ON a.id=c.account_id JOIN account_ip_logical_accounts l ON l.account_id=c.logical_account_id WHERE c.retired_at IS NULL AND a.deleted_at IS NULL AND c.logical_account_id IN (SELECT account_id FROM account_ip_logical_accounts WHERE account_id=ANY($1) UNION SELECT logical_account_id FROM account_ip_channels WHERE account_id=ANY($1)) ORDER BY a.priority,c.account_id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	channels := []service.AccountIPChannel{}
	memberIDs := []int64{}
	for rows.Next() {
		var c service.AccountIPChannel
		var id int64
		if err = rows.Scan(&id, &c.LogicalAccountID, &c.Enabled, &c.DisabledAt, &c.LogicalEnabled); err != nil {
			_ = rows.Close()
			return nil, err
		}
		c.Account = &service.Account{ID: id}
		channels = append(channels, c)
		memberIDs = append(memberIDs, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	accounts, err := r.GetByIDs(ctx, memberIDs)
	if err != nil {
		return nil, err
	}
	byID := map[int64]*service.Account{}
	for _, a := range accounts {
		byID[a.ID] = a
	}
	byRoot := map[int64][]service.AccountIPChannel{}
	for _, c := range channels {
		c.Account = byID[c.Account.ID]
		if c.Account != nil {
			byRoot[c.LogicalAccountID] = append(byRoot[c.LogicalAccountID], c)
		}
	}
	for _, id := range ids {
		if cs := byRoot[id]; len(cs) > 0 {
			result[id] = cs
			continue
		}
		for _, cs := range byRoot {
			for _, c := range cs {
				if c.Account.ID == id {
					result[id] = cs
					break
				}
			}
		}
	}
	return result, nil
}

func (r *accountRepository) WithAccountIPChannelTransaction(ctx context.Context, id int64, operation func(context.Context, service.AccountRepository, []service.AccountIPChannel) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		if _, err := tx.Client().ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", accountIPChannelMutationLock); err != nil {
			return err
		}
		repo := newAccountRepositoryWithSQL(tx.Client(), tx.Client(), nil)
		channels, err := repo.GetAccountIPChannels(ctx, []int64{id})
		if err != nil {
			return err
		}
		if len(channels[id]) > 0 && channels[id][0].LogicalAccountID != id {
			return infraerrors.BadRequest("IP_CHANNEL_USE_PARENT", "请从逻辑账号操作共享配置")
		}
		return operation(ctx, repo, channels[id])
	}
	baseCtx := ctx
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	ctx = dbent.NewTxContext(ctx, tx)
	if _, err = tx.Client().ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", accountIPChannelMutationLock); err != nil {
		return err
	}
	txRepo := newAccountRepositoryWithSQL(tx.Client(), tx.Client(), nil)
	channels, err := txRepo.GetAccountIPChannels(ctx, []int64{id})
	if err != nil {
		return err
	}
	if len(channels[id]) > 0 && channels[id][0].LogicalAccountID != id {
		return infraerrors.BadRequest("IP_CHANNEL_USE_PARENT", "请从逻辑账号操作共享配置")
	}
	if err = operation(ctx, txRepo, channels[id]); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	changed := map[int64]bool{}
	if id > 0 {
		changed[id] = true
	}
	for _, c := range channels[id] {
		changed[c.Account.ID] = true
	}
	if current, err := r.GetAccountIPChannels(baseCtx, []int64{id}); err == nil {
		for _, c := range current[id] {
			changed[c.Account.ID] = true
		}
	}
	for channelID := range changed {
		r.syncSchedulerAccountSnapshotDetached(baseCtx, channelID)
	}
	return nil
}

func compatibleIPChannelAccounts(a, b *service.Account) error {
	if a.Platform != service.PlatformOpenAI || b.Platform != a.Platform || a.Type != service.AccountTypeOAuth || b.Type != a.Type || a.IsShadow() || b.IsShadow() || a.IsRandomProxy() || b.IsRandomProxy() || a.ProxyID == nil || b.ProxyID == nil {
		return errors.New("仅支持使用固定代理的 OpenAI OAuth 账号")
	}
	if err := validateOpenAICredentialIdentity(a.Credentials, b.Credentials); err != nil {
		return err
	}
	group := a.GetCredential(service.OpenAIOAuthCredentialGroupKey)
	sameGroup := group != "" && group == b.GetCredential(service.OpenAIOAuthCredentialGroupKey)
	sameToken := a.GetCredential("access_token") != "" && a.GetCredential("access_token") == b.GetCredential("access_token")
	if !sameGroup && !sameToken {
		return errors.New("凭据身份无法确认一致，不能仅凭相同邮箱合并")
	}
	if a.GetCredential("chatgpt_account_id") != b.GetCredential("chatgpt_account_id") || a.GetCredential("chatgpt_user_id") != b.GetCredential("chatgpt_user_id") {
		return errors.New("工作区或用户身份不一致")
	}
	groupPairs := func(v *service.Account) map[int64]int {
		m := map[int64]int{}
		for _, g := range v.AccountGroups {
			m[g.GroupID] = g.Priority
		}
		return m
	}
	if !reflect.DeepEqual(groupPairs(a), groupPairs(b)) || a.BillingRateMultiplier() != b.BillingRateMultiplier() || !reflect.DeepEqual(a.ExpiresAt, b.ExpiresAt) || a.AutoPauseOnExpired != b.AutoPauseOnExpired {
		return errors.New("所属分组、倍率或有效期等共享配置不同，请先统一配置")
	}
	config := func(v *service.Account) map[string]any {
		m := copyJSONMap(v.Credentials)
		for _, k := range service.OpenAIOAuthSharedCredentialKeys {
			delete(m, k)
		}
		return m
	}
	if !reflect.DeepEqual(config(a), config(b)) {
		return errors.New("模型映射或其他凭据配置不同，请先统一配置")
	}
	extra := func(v *service.Account) map[string]any {
		m := copyJSONMap(v.Extra)
		// Import/protection timestamps describe when a physical record was
		// written. Crossing a clock second must not split identical IP channels.
		delete(m, "imported_at")
		if marker, ok := m[service.AntiDegradeMarkerExtraKey].(map[string]any); ok {
			marker = copyJSONMap(marker)
			delete(marker, "applied_at")
			m[service.AntiDegradeMarkerExtraKey] = marker
		}
		for k := range m {
			if service.IsOpenAICodexTicketPrivateExtraKey(k) || k == "codex_fingerprint_seed" || k == "codex_usage_updated_at" || strings.HasPrefix(k, "codex_primary_") || strings.HasPrefix(k, "codex_secondary_") || strings.HasPrefix(k, "codex_5h_") || strings.HasPrefix(k, "codex_7d_") || strings.HasPrefix(k, "passive_usage_") || k == "model_rate_limits" || k == "session_window_utilization" || k == service.AccountTrafficPolicyKey {
				delete(m, k)
			}
		}
		return m
	}
	if !reflect.DeepEqual(extra(a), extra(b)) {
		return errors.New("账号共享设置不同，请先统一配置")
	}
	return nil
}

func (r *accountRepository) JoinAccountIPChannels(ctx context.Context, ids []int64) (int64, error) {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) == 0 {
		return 0, nil
	}
	var rootID int64
	err := r.WithAccountIPChannelTransaction(ctx, 0, func(ctx context.Context, repo service.AccountRepository, _ []service.AccountIPChannel) error {
		txRepo, ok := repo.(*accountRepository)
		if !ok {
			return errors.New("account IP channel transaction repository type mismatch")
		}
		// Expand existing memberships so a second import cannot split a logical account.
		members, err := txRepo.GetAccountIPChannels(ctx, ids)
		if err != nil {
			return err
		}
		allIDs := slices.Clone(ids)
		for _, cs := range members {
			for _, c := range cs {
				allIDs = append(allIDs, c.Account.ID)
			}
		}
		slices.Sort(allIDs)
		allIDs = slices.Compact(allIDs)
		if len(allIDs) > 50 {
			return infraerrors.BadRequest("IP_CHANNEL_LIMIT", "每个账号最多 50 个 IP 通道")
		}
		accounts, err := repo.GetByIDs(ctx, allIDs)
		if err != nil {
			return err
		}
		if len(accounts) != len(allIDs) {
			return service.ErrAccountNotFound
		}
		root := accounts[0]
		rootID = root.ID
		existingRoot := int64(0)
		for _, cs := range members {
			if len(cs) > 0 {
				if existingRoot != 0 && existingRoot != cs[0].LogicalAccountID {
					return infraerrors.Conflict("IP_CHANNEL_MERGE_CONFLICT", "两个已有逻辑账号不能自动合并，请保留各自历史关联")
				}
				existingRoot = cs[0].LogicalAccountID
			}
		}
		if existingRoot != 0 {
			rootID = existingRoot
		}
		for _, a := range accounts {
			if a.ID == rootID {
				root = a
				break
			}
		}
		proxies := map[int64]bool{}
		for _, a := range accounts {
			if a.IsModelMismatchQuarantined() {
				return infraerrors.Conflict("MODEL_MISMATCH_QUARANTINED", "请先确认恢复降智账号，再合并 IP 通道")
			}
			if shadows, err := repo.ListShadowsByParent(ctx, a.ID); err != nil {
				return err
			} else if len(shadows) > 0 {
				return infraerrors.Conflict("IP_CHANNEL_SPARK_PARENT", "含 Spark 影子账号的父账号暂不能加入 IP 通道")
			}
			if err = compatibleIPChannelAccounts(root, a); err != nil {
				return infraerrors.Conflict("IP_CHANNEL_MERGE_CONFLICT", err.Error())
			}
			if proxies[*a.ProxyID] {
				return infraerrors.Conflict("IP_CHANNEL_DUPLICATE_PROXY", "同一账号不能重复绑定同一个 IP")
			}
			proxies[*a.ProxyID] = true
		}
		if _, err = txRepo.sql.ExecContext(ctx, `INSERT INTO account_ip_logical_accounts(account_id) VALUES($1) ON CONFLICT DO NOTHING`, rootID); err != nil {
			return err
		}
		for _, a := range accounts {
			if _, err = txRepo.sql.ExecContext(ctx, `INSERT INTO account_ip_channels(account_id,logical_account_id,enabled,disabled_at) VALUES($1,$2,$3,CASE WHEN NOT $3 THEN NOW() END) ON CONFLICT(account_id) DO UPDATE SET logical_account_id=EXCLUDED.logical_account_id`, a.ID, rootID, a.Schedulable); err != nil {
				return err
			}
		}
		_, err = txRepo.client.Account.UpdateOneID(rootID).SetName(service.LogicalAccountDisplayName(root.Name)).Save(ctx)
		return err
	})
	return rootID, err
}

func (r *accountRepository) AddAccountIPChannels(ctx context.Context, id int64, proxyIDs []int64, concurrency, priority *int) error {
	_, err := r.AddAccountIPChannelsWithTicketDefaults(ctx, id, proxyIDs, concurrency, priority, nil)
	return err
}

func (r *accountRepository) AddAccountIPChannelsWithTicketDefaults(ctx context.Context, id int64, proxyIDs []int64, concurrency, priority *int, defaults *service.GroupCodexTicketDefaults) ([]int64, error) {
	var createdIDs []int64
	err := r.WithAccountIPChannelTransaction(ctx, id, func(ctx context.Context, repo service.AccountRepository, members []service.AccountIPChannel) error {
		txRepo, ok := repo.(*accountRepository)
		if !ok {
			return errors.New("account IP channel transaction repository type mismatch")
		}
		root, err := repo.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if err = compatibleIPChannelAccounts(root, root); err != nil {
			return infraerrors.BadRequest("IP_CHANNEL_ACCOUNT", err.Error())
		}
		if shadows, err := repo.ListShadowsByParent(ctx, id); err != nil {
			return err
		} else if len(shadows) > 0 {
			return infraerrors.Conflict("IP_CHANNEL_SPARK_PARENT", "含 Spark 影子账号的父账号暂不能加入 IP 通道")
		}
		// Enrol before calling this operation when refresh credentials are present.
		if root.GetCredential("refresh_token") != "" && root.GetCredential(service.OpenAIOAuthCredentialGroupKey) == "" {
			return infraerrors.Conflict("IP_CHANNEL_REIMPORT_REQUIRED", "请先通过多 IP 导入建立共享凭据关联")
		}
		logicalEnabled := root.Schedulable
		if len(members) > 0 {
			logicalEnabled = members[0].LogicalEnabled
		}
		if len(members) == 0 {
			if _, err = txRepo.sql.ExecContext(ctx, `INSERT INTO account_ip_logical_accounts(account_id,enabled) VALUES($1,$2)`, id, logicalEnabled); err != nil {
				return err
			}
			if _, err = txRepo.sql.ExecContext(ctx, `INSERT INTO account_ip_channels(account_id,logical_account_id,enabled,disabled_at) VALUES($1,$1,$2,CASE WHEN NOT $2 THEN NOW() END)`, id, root.Schedulable); err != nil {
				return err
			}
			members = []service.AccountIPChannel{{Account: root, LogicalAccountID: id, Enabled: root.Schedulable}}
		}
		if len(members)+len(proxyIDs) > 50 {
			return infraerrors.BadRequest("IP_CHANNEL_LIMIT", "每个账号最多 50 个 IP 通道")
		}
		seen := map[int64]bool{}
		for _, c := range members {
			if c.Account.ProxyID != nil {
				seen[*c.Account.ProxyID] = true
			}
		}
		// Hold OAuth lock while inheriting the latest generation and adding every channel.
		if err = lockOpenAICredentials(ctx, txRepo.client, id, ""); err != nil {
			return err
		}
		root, err = repo.GetByID(ctx, id)
		if err != nil {
			return err
		}
		for _, proxyID := range proxyIDs {
			if seen[proxyID] {
				return infraerrors.Conflict("IP_CHANNEL_DUPLICATE_PROXY", "同一账号不能重复绑定同一个 IP")
			}
			seen[proxyID] = true
			clone := *root
			clone.ID = 0
			clone.InitialCodexTicketDefaults = defaults
			clone.ProxyID = &proxyID
			clone.Proxy = nil
			clone.Name = service.LogicalAccountDisplayName(root.Name)
			clone.Extra = service.RedactOpenAICodexTicketExtra(root.Extra)
			delete(clone.Extra, "codex_fingerprint_seed")
			clone.ErrorMessage = ""
			clone.LastUsedAt = nil
			clone.RateLimitedAt = nil
			clone.RateLimitResetAt = nil
			clone.OverloadUntil = nil
			clone.TempUnschedulableUntil = nil
			clone.Schedulable = logicalEnabled
			clone.Concurrency = service.DefaultIPChannelConcurrency
			if concurrency != nil {
				clone.Concurrency = *concurrency
			}
			if priority != nil {
				clone.Priority = *priority
			}
			if err = createAccountRecord(ctx, txRepo.client, &clone); err != nil {
				return err
			}
			for _, g := range root.AccountGroups {
				if _, err = txRepo.client.AccountGroup.Create().SetAccountID(clone.ID).SetGroupID(g.GroupID).SetPriority(g.Priority).Save(ctx); err != nil {
					return err
				}
			}
			if _, err = txRepo.sql.ExecContext(ctx, `INSERT INTO account_ip_channels(account_id,logical_account_id,enabled) VALUES($1,$2,TRUE)`, clone.ID, id); err != nil {
				return err
			}
			if err = enqueueSchedulerOutbox(ctx, txRepo.sql, service.SchedulerOutboxEventAccountChanged, &clone.ID, nil, buildSchedulerGroupPayload(root.GroupIDs)); err != nil {
				return err
			}
			createdIDs = append(createdIDs, clone.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return createdIDs, nil
}

func requireAccountIPChannel(members []service.AccountIPChannel, channelID int64) (*service.AccountIPChannel, error) {
	for i := range members {
		if members[i].Account.ID == channelID {
			return &members[i], nil
		}
	}
	return nil, infraerrors.NotFound("IP_CHANNEL_NOT_FOUND", "IP 通道不属于此账号")
}

// The grace period allows scheduler snapshots already handed to requests to
// drain. HTTP handlers additionally fail closed if live concurrency is unknown.
func requireDrainedIPChannel(c *service.AccountIPChannel) error {
	if c.Enabled || c.Account.Schedulable || c.DisabledAt == nil || time.Since(*c.DisabledAt) < 2*time.Minute {
		return infraerrors.Conflict("IP_CHANNEL_DRAINING", "请先停用通道，等待至少两分钟且请求结束后再删除或更换 IP")
	}
	return nil
}

func (r *accountRepository) PatchAccountIPChannel(ctx context.Context, id, channelID int64, patch service.AccountIPChannelPatch) error {
	return r.WithAccountIPChannelTransaction(ctx, id, func(ctx context.Context, repo service.AccountRepository, members []service.AccountIPChannel) error {
		if len(members) == 0 && channelID == id {
			a, err := repo.GetByID(ctx, id)
			if err != nil {
				return err
			}
			if err = compatibleIPChannelAccounts(a, a); err != nil {
				return infraerrors.BadRequest("IP_CHANNEL_ACCOUNT", err.Error())
			}
			if shadows, err := repo.ListShadowsByParent(ctx, id); err != nil {
				return err
			} else if len(shadows) > 0 {
				return infraerrors.Conflict("IP_CHANNEL_SPARK_PARENT", "含 Spark 影子账号的父账号暂不能加入 IP 通道")
			}
			txRepo, ok := repo.(*accountRepository)
			if !ok {
				return errors.New("account IP channel transaction repository type mismatch")
			}
			if _, err = txRepo.sql.ExecContext(ctx, `INSERT INTO account_ip_logical_accounts(account_id,enabled) VALUES($1,$2)`, id, a.Schedulable); err != nil {
				return err
			}
			if _, err = txRepo.sql.ExecContext(ctx, `INSERT INTO account_ip_channels(account_id,logical_account_id,enabled,disabled_at) VALUES($1,$1,$2,CASE WHEN NOT $2 THEN NOW() END)`, id, a.Schedulable); err != nil {
				return err
			}
			members = []service.AccountIPChannel{{Account: a, LogicalAccountID: id, Enabled: a.Schedulable, LogicalEnabled: a.Schedulable}}
		}
		c, err := requireAccountIPChannel(members, channelID)
		if err != nil {
			return err
		}
		txRepo, ok := repo.(*accountRepository)
		if !ok {
			return errors.New("account IP channel transaction repository type mismatch")
		}
		if patch.ProxyID != nil {
			if err = requireDrainedIPChannel(c); err != nil {
				return err
			}
			for _, sibling := range members {
				if sibling.Account.ID != channelID && sibling.Account.ProxyID != nil && *sibling.Account.ProxyID == *patch.ProxyID {
					return infraerrors.Conflict("IP_CHANNEL_DUPLICATE_PROXY", "该 IP 已绑定此账号")
				}
			}
			c.Account.ProxyID = patch.ProxyID
			c.Account.Proxy = nil
		}
		if patch.Concurrency != nil {
			c.Account.Concurrency = *patch.Concurrency
		}
		if patch.Priority != nil {
			c.Account.Priority = *patch.Priority
		}
		if patch.LoadFactor != nil {
			if *patch.LoadFactor == 0 {
				c.Account.LoadFactor = nil
			} else {
				c.Account.LoadFactor = patch.LoadFactor
			}
		}
		if patch.Schedulable != nil {
			if *patch.Schedulable && (c.Account.Proxy == nil || !c.Account.Proxy.IsActive() || c.Account.Proxy.IsExpired(time.Now())) {
				return infraerrors.Conflict("IP_CHANNEL_PROXY", "固定代理不可用，请先修复代理")
			}
			if _, err = txRepo.sql.ExecContext(ctx, `UPDATE account_ip_channels SET enabled=$2,disabled_at=CASE WHEN $2 THEN NULL WHEN enabled THEN NOW() ELSE COALESCE(disabled_at,NOW()) END WHERE account_id=$1`, channelID, *patch.Schedulable); err != nil {
				return err
			}
			rows, err := txRepo.sql.QueryContext(ctx, `SELECT enabled FROM account_ip_logical_accounts WHERE account_id=$1`, id)
			if err != nil {
				return err
			}
			var globalEnabled bool
			if !rows.Next() {
				_ = rows.Close()
				return service.ErrAccountNotFound
			}
			err = rows.Scan(&globalEnabled)
			_ = rows.Close()
			if err != nil {
				return err
			}
			c.Account.Schedulable = *patch.Schedulable && globalEnabled
		}
		return repo.Update(ctx, c.Account)
	})
}

func (r *accountRepository) RemoveAccountIPChannel(ctx context.Context, id, channelID int64) error {
	return r.WithAccountIPChannelTransaction(ctx, id, func(ctx context.Context, repo service.AccountRepository, members []service.AccountIPChannel) error {
		if len(members) <= 1 {
			return infraerrors.Conflict("IP_CHANNEL_LAST", "不能移除最后一个 IP 通道，请使用删除账号")
		}
		c, err := requireAccountIPChannel(members, channelID)
		if err != nil {
			return err
		}
		if err = requireDrainedIPChannel(c); err != nil {
			return err
		}
		txRepo, ok := repo.(*accountRepository)
		if !ok {
			return errors.New("account IP channel transaction repository type mismatch")
		}
		if _, err = txRepo.sql.ExecContext(ctx, `UPDATE account_ip_channels SET retired_at=NOW() WHERE account_id=$1`, channelID); err != nil {
			return err
		}
		if channelID != id {
			if _, err = txRepo.client.AccountGroup.Delete().Where(dbaccountgroup.AccountIDEQ(channelID)).Exec(ctx); err != nil {
				return err
			}
			if _, err = txRepo.client.Account.UpdateOneID(channelID).SetDeletedAt(time.Now()).SetSchedulable(false).Save(ctx); err != nil {
				return err
			}
		}
		if _, err = txRepo.sql.ExecContext(ctx, `DELETE FROM scheduled_test_plans WHERE account_id=$1`, channelID); err != nil {
			return err
		}
		return enqueueSchedulerOutbox(ctx, txRepo.sql, service.SchedulerOutboxEventAccountChanged, &channelID, nil, buildSchedulerGroupPayload(c.Account.GroupIDs))
	})
}

func (r *accountRepository) SetLogicalAccountSchedulable(ctx context.Context, id int64, enabled bool) (bool, error) {
	channels, err := r.GetAccountIPChannels(ctx, []int64{id})
	if err != nil {
		return true, err
	}
	if len(channels[id]) == 0 {
		return false, nil
	}
	err = r.WithAccountIPChannelTransaction(ctx, id, func(ctx context.Context, repo service.AccountRepository, members []service.AccountIPChannel) error {
		txRepo, ok := repo.(*accountRepository)
		if !ok {
			return errors.New("account IP channel transaction repository type mismatch")
		}
		if enabled {
			for _, c := range members {
				if c.Account.IsModelMismatchQuarantined() {
					return infraerrors.Conflict("MODEL_MISMATCH_CONFIRM_REQUIRED", "账号因上游模型不一致已停止调度，请确认后单独恢复")
				}
			}
		}
		if _, err := txRepo.sql.ExecContext(ctx, `UPDATE account_ip_logical_accounts SET enabled=$2 WHERE account_id=$1`, id, enabled); err != nil {
			return err
		}
		for _, c := range members {
			proxyReady := c.Account.Proxy != nil && c.Account.Proxy.IsActive() && !c.Account.Proxy.IsExpired(time.Now())
			if err := repo.SetSchedulable(ctx, c.Account.ID, enabled && c.Enabled && proxyReady); err != nil {
				return err
			}
		}
		return nil
	})
	return true, err
}

var _ service.AccountIPChannelRepository = (*accountRepository)(nil)

func (r *accountRepository) AccountIPChannelHistoryIDs(ctx context.Context, ids []int64) (map[int64][]int64, error) {
	result := map[int64][]int64{}
	for _, id := range ids {
		result[id] = []int64{id}
	}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT logical_account_id,account_id FROM account_ip_channels WHERE logical_account_id=ANY($1) ORDER BY account_id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	grouped := map[int64][]int64{}
	for rows.Next() {
		var id, member int64
		if err = rows.Scan(&id, &member); err != nil {
			return nil, err
		}
		grouped[id] = append(grouped[id], member)
	}
	for id, members := range grouped {
		result[id] = members
	}
	return result, rows.Err()
}
