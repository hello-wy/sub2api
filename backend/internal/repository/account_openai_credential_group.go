package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// One transaction-scoped lock also covers legacy accounts during enrollment.
// Unlike a Redis lease it cannot expire midway through an upstream rotation.
// This deliberately serializes OAuth mutations (never ordinary API traffic).
const openAICredentialMutationLock int64 = 736282017

func lockOpenAICredentials(ctx context.Context, client *dbent.Client, accountID int64, group string) error {
	if accountID == 0 && group == "" {
		_, err := client.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", openAICredentialMutationLock)
		return err
	}
	if _, err := client.ExecContext(ctx, "SELECT pg_advisory_xact_lock_shared($1)", openAICredentialMutationLock); err != nil {
		return err
	}
	if accountID > 0 {
		account, err := client.Account.Get(ctx, accountID)
		if err != nil {
			return err
		}
		group = credentialString(account.Credentials, service.OpenAIOAuthCredentialGroupKey)
		if group == "" {
			group = "account:" + strconv.FormatInt(accountID, 10)
		}
	}
	_, err := client.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", "openai:credential-group:"+group)
	return err
}

func (r *accountRepository) openAICredentialTransaction(ctx context.Context, accountID int64, group string, operation func(context.Context, *dbent.Client) ([]int64, error)) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		if err := lockOpenAICredentials(ctx, tx.Client(), accountID, group); err != nil {
			return err
		}
		_, err := operation(ctx, tx.Client())
		return err
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockOpenAICredentials(ctx, tx.Client(), accountID, group); err != nil {
		return err
	}
	ids, err := operation(ctx, tx.Client())
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for _, id := range ids {
		r.syncSchedulerAccountSnapshotDetached(ctx, id)
	}
	return nil
}

func openAIGroupAccounts(ctx context.Context, client *dbent.Client) ([]*dbent.Account, error) {
	return client.Account.Query().Where(dbaccount.PlatformEQ(service.PlatformOpenAI), dbaccount.TypeEQ(service.AccountTypeOAuth), dbaccount.ParentAccountIDIsNil()).All(ctx)
}

func mergeOpenAICredentialPatch(base, patch map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(patch))
	for key, value := range base {
		result[key] = value
	}
	for _, key := range service.OpenAIOAuthSharedCredentialKeys {
		delete(result, key)
	}
	for key, value := range service.OpenAIOAuthCredentialPatch(patch) {
		result[key] = value
	}
	return result
}

func saveOpenAICredentialPatch(ctx context.Context, client *dbent.Client, accounts []*dbent.Account, credentials map[string]any) ([]int64, error) {
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		merged := mergeOpenAICredentialPatch(account.Credentials, credentials)
		if _, err := client.Account.UpdateOneID(account.ID).SetCredentials(merged).Save(ctx); err != nil {
			return nil, err
		}
		id := account.ID
		if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountChanged, &id, nil, nil); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func credentialString(credentials map[string]any, key string) string {
	value, _ := credentials[key].(string)
	return value
}

// Retain only one-way fingerprints, including consumed rotation generations.
// Old exports must resolve to the current group even after later reauthorization.
// Never evict old fingerprints: eviction would make an old consumed token appear
// to be a new authorization and allow it to replace the current group token.
func openAISourceFingerprintHistory(credentials ...map[string]any) []string {
	seen := make(map[string]bool)
	for _, creds := range credentials {
		if source := credentialString(creds, service.OpenAIOAuthSourceFingerprintKey); source != "" {
			seen[source] = true
		}
		switch history := creds[service.OpenAIOAuthSourceFingerprintsKey].(type) {
		case []string:
			for _, source := range history {
				if source != "" {
					seen[source] = true
				}
			}
		case []any:
			for _, value := range history {
				if source, ok := value.(string); ok && source != "" {
					seen[source] = true
				}
			}
		}
		if credentialString(creds, "refresh_token") != "" || credentialString(creds, "access_token") != "" {
			seen[service.OpenAIOAuthSourceFingerprint(creds)] = true
		}
		if at := credentialString(creds, "access_token"); at != "" {
			seen[service.OpenAIOAuthSourceFingerprint(map[string]any{"access_token": at})] = true
		}
	}
	history := make([]string, 0, len(seen))
	for source := range seen {
		history = append(history, source)
	}
	slices.Sort(history)
	return history
}

func openAIKnownSource(credentials map[string]any, fingerprint string) bool {
	return slices.Contains(openAISourceFingerprintHistory(credentials), fingerprint)
}

func validateOpenAICredentialIdentity(a, b map[string]any) error {
	for _, key := range []string{"chatgpt_account_id", "chatgpt_user_id", "client_id"} {
		left, right := credentialString(a, key), credentialString(b, key)
		if left != "" && right != "" && left != right {
			return fmt.Errorf("OAuth credential group has conflicting %s", key)
		}
	}
	return nil
}

func (r *accountRepository) RegisterOpenAIOAuthCredentialGroup(ctx context.Context, ids []int64, incoming map[string]any) (map[string]any, error) {
	prepared := make(map[string]any, len(incoming)+3)
	for key, value := range incoming {
		prepared[key] = value
	}
	delete(prepared, service.OpenAIOAuthCredentialGroupKey)
	delete(prepared, service.OpenAIOAuthSourceFingerprintKey)
	delete(prepared, service.OpenAIOAuthSourceFingerprintsKey)
	requested := make(map[int64]bool, len(ids))
	for _, id := range ids {
		requested[id] = true
	}
	fingerprint := service.OpenAIOAuthSourceFingerprint(prepared)
	err := r.openAICredentialTransaction(ctx, 0, "", func(ctx context.Context, client *dbent.Client) ([]int64, error) {
		accounts, err := openAIGroupAccounts(ctx, client)
		if err != nil {
			return nil, err
		}
		group := ""
		matched := make(map[int64]bool)
		for _, account := range accounts {
			sameRT := credentialString(prepared, "refresh_token") != "" && credentialString(prepared, "refresh_token") == credentialString(account.Credentials, "refresh_token")
			sameAT := credentialString(prepared, "access_token") != "" && credentialString(prepared, "access_token") == credentialString(account.Credentials, "access_token")
			sameSource := fingerprint != "" && openAIKnownSource(account.Credentials, fingerprint)
			if !requested[account.ID] && !sameRT && !sameAT && !sameSource {
				continue
			}
			if err := validateOpenAICredentialIdentity(prepared, account.Credentials); err != nil {
				return nil, err
			}
			matched[account.ID] = true
			if existing := credentialString(account.Credentials, service.OpenAIOAuthCredentialGroupKey); existing != "" {
				if group != "" && group != existing {
					return nil, errors.New("cannot merge independent OAuth credential groups; reauthorize them separately")
				}
				group = existing
			}
		}
		for id := range requested {
			if !matched[id] {
				return nil, service.ErrAccountNotFound
			}
		}
		if group == "" {
			// Deterministic for a source with no existing row. This closes the
			// concurrent first-import window without persisting a raw token.
			group = "fp:" + fingerprint
			if fingerprint == "" {
				group = rand.Text()
			}
		}
		members := make([]*dbent.Account, 0)
		var canonical *dbent.Account
		historySources := []map[string]any{prepared}
		knownSource := false
		for _, account := range accounts {
			if !matched[account.ID] && credentialString(account.Credentials, service.OpenAIOAuthCredentialGroupKey) != group {
				continue
			}
			if err := validateOpenAICredentialIdentity(prepared, account.Credentials); err != nil {
				return nil, err
			}
			members = append(members, account)
			historySources = append(historySources, account.Credentials)
			knownSource = knownSource || openAIKnownSource(account.Credentials, fingerprint)
			if canonical == nil || account.UpdatedAt.After(canonical.UpdatedAt) {
				canonical = account
			}
		}
		// Reimporting the original JSON must never put an already consumed RT back.
		if canonical != nil && knownSource {
			prepared = mergeOpenAICredentialPatch(prepared, canonical.Credentials)
		} else {
			prepared[service.OpenAIOAuthSourceFingerprintKey] = fingerprint
			prepared["_token_version"] = time.Now().UnixMilli()
		}
		prepared[service.OpenAIOAuthCredentialGroupKey] = group
		prepared[service.OpenAIOAuthSourceFingerprintsKey] = openAISourceFingerprintHistory(historySources...)
		return saveOpenAICredentialPatch(ctx, client, members, prepared)
	})
	return prepared, err
}

func (r *accountRepository) RefreshOpenAIOAuthCredentials(ctx context.Context, expected *service.Account, refresh func(context.Context, *service.Account) (map[string]any, error)) (*service.Account, error) {
	var durable *service.Account
	err := r.openAICredentialTransaction(ctx, expected.ID, "", func(ctx context.Context, client *dbent.Client) ([]int64, error) {
		entity, err := client.Account.Query().Where(dbaccount.IDEQ(expected.ID)).ForUpdate().Only(ctx)
		if err != nil {
			return nil, translatePersistenceError(err, service.ErrAccountNotFound, nil)
		}
		fresh := accountEntityToService(entity)
		if fresh.Platform != service.PlatformOpenAI || fresh.Type != service.AccountTypeOAuth || fresh.IsCredentialShadow() {
			return nil, errors.New("account is no longer an OpenAI OAuth credential owner")
		}
		durable = fresh
		if expected.GetCredential("refresh_token") != fresh.GetCredential("refresh_token") || expected.GetCredential("access_token") != fresh.GetCredential("access_token") {
			return nil, nil
		}
		members := []*dbent.Account{entity}
		if group := fresh.GetCredential(service.OpenAIOAuthCredentialGroupKey); group != "" {
			accounts, err := openAIGroupAccounts(ctx, client)
			if err != nil {
				return nil, err
			}
			members = nil
			for _, account := range accounts {
				if credentialString(account.Credentials, service.OpenAIOAuthCredentialGroupKey) == group {
					members = append(members, account)
				}
			}
			memberIDs := make([]int64, 0, len(members))
			for _, member := range members {
				memberIDs = append(memberIDs, member.ID)
			}
			members, err = client.Account.Query().Where(dbaccount.IDIn(memberIDs...)).Order(dbent.Asc(dbaccount.FieldID)).ForUpdate().All(ctx)
			if err != nil {
				return nil, err
			}
		}
		historySources := make([]map[string]any, 0, len(members))
		for _, member := range members {
			historySources = append(historySources, member.Credentials)
		}
		// Capture before the callback: some refresh adapters mutate their fresh
		// snapshot in place, which must not erase the consumed generation.
		history := openAISourceFingerprintHistory(historySources...)
		credentials, err := refresh(service.WithOpenAIRefreshCoordinator(ctx), fresh)
		if err != nil {
			return nil, err
		}
		credentials["_token_version"] = time.Now().UnixMilli()
		if fresh.GetCredential(service.OpenAIOAuthCredentialGroupKey) != "" {
			credentials[service.OpenAIOAuthSourceFingerprintsKey] = openAISourceFingerprintHistory(map[string]any{service.OpenAIOAuthSourceFingerprintsKey: history}, credentials)
		}
		ids, err := saveOpenAICredentialPatch(ctx, client, members, credentials)
		if err != nil {
			return nil, err
		}
		durable.Credentials = mergeOpenAICredentialPatch(entity.Credentials, credentials)
		return ids, nil
	})
	return durable, err
}

// Protect token fields against delayed post-refresh/admin snapshots. Group
// reauthorization goes through Register, which updates the whole group atomically.
func preserveOpenAISharedCredentials(ctx context.Context, client *dbent.Client, id int64, credentials map[string]any) (map[string]any, error) {
	entity, err := client.Account.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if entity.Platform != service.PlatformOpenAI || entity.Type != service.AccountTypeOAuth {
		return credentials, nil
	}
	group := credentialString(entity.Credentials, service.OpenAIOAuthCredentialGroupKey)
	if group == "" {
		return credentials, nil
	}
	if supplied := credentialString(credentials, service.OpenAIOAuthCredentialGroupKey); supplied != "" && supplied != group {
		return nil, errors.New("OAuth credential group cannot be changed through account editing")
	}
	if service.IsOpenAIRegisteredCredentialSnapshot(ctx) &&
		credentialString(credentials, service.OpenAIOAuthCredentialGroupKey) == group &&
		credentialString(credentials, service.OpenAIOAuthSourceFingerprintKey) != "" &&
		credentialString(credentials, service.OpenAIOAuthSourceFingerprintKey) == credentialString(entity.Credentials, service.OpenAIOAuthSourceFingerprintKey) {
		return mergeOpenAICredentialPatch(credentials, entity.Credentials), nil
	}
	for _, key := range service.OpenAIOAuthSharedCredentialKeys {
		if key == service.OpenAIOAuthCredentialGroupKey || key == service.OpenAIOAuthSourceFingerprintKey || key == service.OpenAIOAuthSourceFingerprintsKey || key == "_token_version" {
			continue
		}
		if supplied, ok := credentials[key]; ok && !reflect.DeepEqual(supplied, entity.Credentials[key]) {
			return nil, errors.New("shared OAuth credentials must be changed through the multi-IP reauthorization/import flow")
		}
	}
	return mergeOpenAICredentialPatch(credentials, entity.Credentials), nil
}

func (r *accountRepository) createOpenAISharedAccount(ctx context.Context, account *service.Account) error {
	return r.openAICredentialTransaction(ctx, 0, account.GetCredential(service.OpenAIOAuthCredentialGroupKey), func(ctx context.Context, client *dbent.Client) ([]int64, error) {
		accounts, err := openAIGroupAccounts(ctx, client)
		if err != nil {
			return nil, err
		}
		for _, member := range accounts {
			if credentialString(member.Credentials, service.OpenAIOAuthCredentialGroupKey) == account.GetCredential(service.OpenAIOAuthCredentialGroupKey) {
				if err := validateOpenAICredentialIdentity(account.Credentials, member.Credentials); err != nil {
					return nil, err
				}
				account.Credentials = mergeOpenAICredentialPatch(account.Credentials, member.Credentials)
				break
			}
		}
		if err := createAccountRecord(ctx, client, account); err != nil {
			return nil, err
		}
		if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountChanged, &account.ID, nil, buildSchedulerGroupPayload(account.GroupIDs)); err != nil {
			return nil, err
		}
		return []int64{account.ID}, nil
	})
}
