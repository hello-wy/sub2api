package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *accountRepository) RestoreAccountIPChannels(ctx context.Context, root *service.Account, channels []service.AccountIPChannelBackup, logicalEnabled, rootRetired bool) error {
	return r.WithAccountIPChannelTransaction(ctx, 0, func(ctx context.Context, repository service.AccountRepository, _ []service.AccountIPChannel) error {
		repo, ok := repository.(*accountRepository)
		if !ok {
			return errors.New("account IP channel transaction repository type mismatch")
		}
		if len(channels) == 0 || len(channels) > 50 {
			return errors.New("invalid IP channel count")
		}
		// A saved mismatch on any physical channel quarantines the restored
		// logical account, including when a retired root has only metadata.
		var marker any
		if root.IsModelMismatchQuarantined() {
			marker = root.Extra[service.AccountModelMismatchExtraKey]
		} else {
			for _, c := range channels {
				if (&service.Account{Extra: c.Extra}).IsModelMismatchQuarantined() {
					marker = c.Extra[service.AccountModelMismatchExtraKey]
					break
				}
			}
		}
		if marker != nil {
			root.Extra = copyJSONMap(root.Extra)
			if root.Extra == nil {
				root.Extra = map[string]any{}
			}
			root.Extra[service.AccountModelMismatchExtraKey] = marker
			logicalEnabled = false
		}
		if err := lockOpenAICredentials(ctx, repo.client, 0, root.GetCredential(service.OpenAIOAuthCredentialGroupKey)); err != nil {
			return err
		}
		if group := root.GetCredential(service.OpenAIOAuthCredentialGroupKey); group != "" {
			accounts, err := openAIGroupAccounts(ctx, repo.client)
			if err != nil {
				return err
			}
			for _, member := range accounts {
				if credentialString(member.Credentials, service.OpenAIOAuthCredentialGroupKey) == group {
					if err = validateOpenAICredentialIdentity(root.Credentials, member.Credentials); err != nil {
						return err
					}
					root.Credentials = mergeOpenAICredentialPatch(root.Credentials, member.Credentials)
					break
				}
			}
		}
		root.Schedulable = false
		// A retired root is metadata only; it must not resurrect its former IP.
		if rootRetired {
			root.ProxyID = &channels[0].ProxyID
			root.Proxy = nil
		}
		if err := createAccountRecord(ctx, repo.client, root); err != nil {
			return err
		}
		if err := repo.BindGroups(ctx, root.ID, root.GroupIDs); err != nil {
			return err
		}
		if _, err := repo.sql.ExecContext(ctx, `INSERT INTO account_ip_logical_accounts(account_id,enabled) VALUES($1,$2)`, root.ID, logicalEnabled); err != nil {
			return err
		}
		if rootRetired {
			if _, err := repo.sql.ExecContext(ctx, `INSERT INTO account_ip_channels(account_id,logical_account_id,enabled,disabled_at,retired_at) VALUES($1,$1,FALSE,NOW(),NOW())`, root.ID); err != nil {
				return err
			}
		}
		seen := map[int64]bool{}
		roots := 0
		for _, c := range channels {
			if seen[c.ProxyID] || c.ProxyID <= 0 {
				return errors.New("duplicate or invalid channel proxy")
			}
			seen[c.ProxyID] = true
			account := *root
			account.ID = 0
			account.ProxyID = &c.ProxyID
			account.Concurrency = c.Concurrency
			account.Priority = c.Priority
			account.LoadFactor = c.LoadFactor
			account.Status = c.Status
			account.Extra = copyJSONMap(c.Extra)
			if marker != nil {
				if account.Extra == nil {
					account.Extra = map[string]any{}
				}
				account.Extra[service.AccountModelMismatchExtraKey] = marker
			}
			account.Schedulable = false
			if c.IsRoot {
				roots++
				if rootRetired || roots > 1 {
					return errors.New("invalid root channel")
				}
				account.ID = root.ID
				if err := repo.Update(ctx, &account); err != nil {
					return err
				}
			} else {
				if err := createAccountRecord(ctx, repo.client, &account); err != nil {
					return err
				}
				if err := repo.BindGroups(ctx, account.ID, root.GroupIDs); err != nil {
					return err
				}
			}
			if _, err := repo.sql.ExecContext(ctx, `INSERT INTO account_ip_channels(account_id,logical_account_id,enabled,disabled_at) VALUES($1,$2,$3,CASE WHEN NOT $3 THEN NOW() END)`, account.ID, root.ID, c.Enabled); err != nil {
				return err
			}
			proxy, err := repo.client.Proxy.Get(ctx, c.ProxyID)
			if err != nil {
				return err
			}
			ready := proxy.Status == service.StatusActive && (proxy.ExpiresAt == nil || proxy.ExpiresAt.After(time.Now()))
			if err = repo.SetSchedulable(ctx, account.ID, logicalEnabled && c.Enabled && ready); err != nil {
				return err
			}
			if err = enqueueSchedulerOutbox(ctx, repo.sql, service.SchedulerOutboxEventAccountChanged, &account.ID, nil, buildSchedulerGroupPayload(root.GroupIDs)); err != nil {
				return err
			}
		}
		if !rootRetired && roots != 1 {
			return errors.New("missing root channel")
		}
		return enqueueSchedulerOutbox(ctx, repo.sql, service.SchedulerOutboxEventAccountChanged, &root.ID, nil, buildSchedulerGroupPayload(root.GroupIDs))
	})
}
