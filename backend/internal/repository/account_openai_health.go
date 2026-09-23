package repository

import (
	"context"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqljson"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.OpenAIOAuthHealthRepository = (*accountRepository)(nil)

func queryOpenAIHealthGroup(client *dbent.Client, group string) *dbent.AccountQuery {
	return client.Account.Query().Where(dbaccount.PlatformEQ(service.PlatformOpenAI), dbaccount.TypeEQ(service.AccountTypeOAuth), dbaccount.ParentAccountIDIsNil(), dbaccount.DeletedAtIsNil(), func(selector *entsql.Selector) {
		selector.Where(sqljson.ValueEQ(dbaccount.FieldCredentials, group, sqljson.Path(service.OpenAIOAuthCredentialGroupKey)))
	})
}

func sameOpenAITokenGeneration(a, b *service.Account) bool {
	return a.GetCredential("access_token") == b.GetCredential("access_token") &&
		a.GetCredential("refresh_token") == b.GetCredential("refresh_token")
}

// Generic account editors carry complete snapshots. Their name/group changes
// must not overwrite health written since the snapshot was read. Explicit
// recovery uses ClearError/ClearRateLimit/SetSchedulable, not this editor path.
// lockAndMergeAccountProbeExtra already holds this row's write lock.
func preserveLockedOpenAIOAuthHealth(ctx context.Context, client *dbent.Client, incoming *service.Account) error {
	if incoming.Platform != service.PlatformOpenAI || incoming.Type != service.AccountTypeOAuth {
		return nil
	}
	row, err := client.Account.Get(ctx, incoming.ID)
	if err != nil {
		return err
	}
	if row.Platform != service.PlatformOpenAI || row.Type != service.AccountTypeOAuth {
		return nil
	}
	current := accountEntityToService(row)
	// These fields are exclusively managed by upstream health / recovery.
	// Copy even nil values so an old edit cannot resurrect a cleared cooldown.
	incoming.RateLimitedAt, incoming.RateLimitResetAt = current.RateLimitedAt, current.RateLimitResetAt
	incoming.OverloadUntil = current.OverloadUntil
	incoming.TempUnschedulableUntil, incoming.TempUnschedulableReason = current.TempUnschedulableUntil, current.TempUnschedulableReason
	if current.Status == service.StatusError || incoming.UpdatedAt.Before(current.UpdatedAt) {
		incoming.Status, incoming.ErrorMessage, incoming.Schedulable = current.Status, current.ErrorMessage, current.Schedulable
	}
	// A 401 marks expiry on the current credential to make maintenance run.
	// Keep that marker when an old standalone (non-group) editor posts its
	// original expiry, without preventing a deliberate new credential import.
	if service.IsOAuthRefreshCooldown(current.TempUnschedulableReason) && sameOpenAITokenGeneration(incoming, current) {
		incoming.Credentials = copyJSONMap(incoming.Credentials)
		if expiry, exists := current.Credentials["expires_at"]; exists {
			incoming.Credentials["expires_at"] = expiry
		}
	}
	return nil
}

func openAIHealthMembers(ctx context.Context, client *dbent.Client, expected *service.Account) ([]*dbent.Account, error) {
	current, err := client.Account.Get(ctx, expected.ID)
	if err != nil {
		return nil, err
	}
	if current.Platform != service.PlatformOpenAI || current.Type != service.AccountTypeOAuth || current.ParentAccountID != nil || current.DeletedAt != nil {
		return nil, nil
	}
	group := credentialString(current.Credentials, service.OpenAIOAuthCredentialGroupKey)
	if group != "" {
		return queryOpenAIHealthGroup(client, group).Order(dbent.Asc(dbaccount.FieldID)).ForUpdate().All(ctx)
	}
	return client.Account.Query().Where(dbaccount.IDEQ(current.ID), dbaccount.DeletedAtIsNil()).ForUpdate().All(ctx)
}

// The shared-account creator and IP import paths hold the credential group
// lock while creating records. A new IP must inherit the existing upstream
// cooldown instead of becoming a way around the same account's 401/429.
func inheritOpenAIHealthForNewAccount(ctx context.Context, client *dbent.Client, account *service.Account) error {
	if account.Platform != service.PlatformOpenAI || account.Type != service.AccountTypeOAuth || account.IsCredentialShadow() {
		return nil
	}
	group := account.GetCredential(service.OpenAIOAuthCredentialGroupKey)
	if group == "" {
		return nil
	}
	members, err := queryOpenAIHealthGroup(client, group).All(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, member := range members {
		if service.Context429Enforcement(ctx) && member.RateLimitResetAt != nil && member.RateLimitResetAt.After(now) && (account.RateLimitResetAt == nil || member.RateLimitResetAt.After(*account.RateLimitResetAt)) {
			account.RateLimitResetAt, account.RateLimitedAt = member.RateLimitResetAt, member.RateLimitedAt
		}
		fresh := accountEntityToService(member)
		if !sameOpenAITokenGeneration(fresh, account) {
			continue
		}
		if fresh.Status == service.StatusError && (strings.HasPrefix(fresh.ErrorMessage, "OAuth 401:") || strings.HasPrefix(fresh.ErrorMessage, "OAuth refresh rejected;")) {
			account.Status, account.ErrorMessage, account.Schedulable = fresh.Status, fresh.ErrorMessage, false
		}
		if service.IsOAuthRefreshCooldown(fresh.TempUnschedulableReason) && fresh.TempUnschedulableUntil != nil && fresh.TempUnschedulableUntil.After(now) && (account.TempUnschedulableUntil == nil || fresh.TempUnschedulableUntil.After(*account.TempUnschedulableUntil)) {
			account.TempUnschedulableUntil, account.TempUnschedulableReason = fresh.TempUnschedulableUntil, fresh.TempUnschedulableReason
		}
	}
	return nil
}

func (r *accountRepository) ApplyOpenAIOAuthHealth(ctx context.Context, expected *service.Account, mutation service.OpenAIOAuthHealthMutation) ([]*service.Account, error) {
	var changed []*service.Account
	err := r.openAICredentialTransaction(ctx, expected.ID, "", func(ctx context.Context, client *dbent.Client) ([]int64, error) {
		members, err := openAIHealthMembers(ctx, client, expected)
		if err != nil {
			return nil, err
		}
		// A late 401 / invalid_grant from an old token must never quarantine a
		// concurrently refreshed or explicitly reauthorized credential group.
		if mutation.RateLimitUntil == nil {
			matched := false
			for _, member := range members {
				if member.ID == expected.ID {
					matched = credentialString(member.Credentials, "access_token") == expected.GetCredential("access_token") && (!mutation.MatchRefreshToken || sameOpenAITokenGeneration(accountEntityToService(member), expected))
				}
			}
			if !matched {
				return nil, nil
			}
		}
		ids := make([]int64, 0, len(members))
		for _, member := range members {
			account := accountEntityToService(member)
			if mutation.RateLimitUntil == nil && (account.GetCredential("access_token") != expected.GetCredential("access_token") || (mutation.MatchRefreshToken && !sameOpenAITokenGeneration(account, expected))) {
				continue
			}
			update := client.Account.UpdateOneID(member.ID)
			switch {
			case mutation.RateLimitUntil != nil:
				until := *mutation.RateLimitUntil
				if member.RateLimitResetAt != nil && member.RateLimitResetAt.After(until) {
					until = *member.RateLimitResetAt
				}
				update.SetRateLimitedAt(time.Now()).SetRateLimitResetAt(until)
			case mutation.Permanent || (mutation.AuthCooldownUntil != nil && credentialString(member.Credentials, "refresh_token") == ""):
				reason := mutation.Reason
				if !mutation.Permanent {
					reason = "OAuth 401: refresh token unavailable; reauthorization required"
				}
				update.SetStatus(service.StatusError).SetSchedulable(false).SetErrorMessage(reason)
			case mutation.AuthCooldownUntil != nil:
				// Mutate only expiry on the locked CURRENT row. Never write back
				// credentials from the request snapshot (refresh tokens rotate).
				credentials := make(map[string]any, len(member.Credentials))
				for key, value := range member.Credentials {
					credentials[key] = value
				}
				credentials["expires_at"] = time.Unix(0, 0).UTC().Format(time.RFC3339)
				update.SetCredentials(credentials)
				if member.TempUnschedulableUntil == nil || !member.TempUnschedulableUntil.After(time.Now()) || service.IsOAuthRefreshCooldown(account.TempUnschedulableReason) {
					until := *mutation.AuthCooldownUntil
					if member.TempUnschedulableUntil != nil && member.TempUnschedulableUntil.After(until) {
						until = *member.TempUnschedulableUntil
					}
					update.SetTempUnschedulableUntil(until).SetTempUnschedulableReason(mutation.Reason)
				}
			default:
				continue
			}
			updated, err := update.Save(ctx)
			if err != nil {
				return nil, err
			}
			id := member.ID
			if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountChanged, &id, nil, nil); err != nil {
				return nil, err
			}
			ids = append(ids, id)
			changed = append(changed, accountEntityToService(updated))
		}
		return ids, nil
	})
	if err != nil {
		return nil, err
	}
	return changed, nil
}

func (r *accountRepository) ClearOpenAIOAuthRefreshCooldown(ctx context.Context, expected *service.Account) ([]*service.Account, error) {
	var changed []*service.Account
	err := r.openAICredentialTransaction(ctx, expected.ID, "", func(ctx context.Context, client *dbent.Client) ([]int64, error) {
		members, err := openAIHealthMembers(ctx, client, expected)
		if err != nil {
			return nil, err
		}
		var ids []int64
		for _, member := range members {
			account := accountEntityToService(member)
			if !sameOpenAITokenGeneration(account, expected) || !service.IsOAuthRefreshCooldown(account.TempUnschedulableReason) {
				continue
			}
			// Preserve status, schedulable, model quarantine and rate-limit
			// reset. Token maintenance never re-enables a business route.
			updated, err := client.Account.UpdateOneID(member.ID).ClearTempUnschedulableUntil().ClearTempUnschedulableReason().Save(ctx)
			if err != nil {
				return nil, err
			}
			id := member.ID
			if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountChanged, &id, nil, nil); err != nil {
				return nil, err
			}
			ids = append(ids, id)
			changed = append(changed, accountEntityToService(updated))
		}
		return ids, nil
	})
	if err != nil {
		return nil, err
	}
	return changed, nil
}
