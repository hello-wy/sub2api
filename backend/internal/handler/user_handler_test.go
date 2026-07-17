//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userHandlerRepoStub struct {
	user           *service.User
	identities     []service.UserAuthIdentityRecord
	hasQQ          bool
	checkinRecords []service.DailyCheckinRecord
	unbound        []string
}

func (s *userHandlerRepoStub) Create(context.Context, *service.User) error { return nil }
func (s *userHandlerRepoStub) GetByID(context.Context, int64) (*service.User, error) {
	cloned := *s.user
	return &cloned, nil
}
func (s *userHandlerRepoStub) GetByEmail(context.Context, string) (*service.User, error) {
	cloned := *s.user
	return &cloned, nil
}
func (s *userHandlerRepoStub) GetFirstAdmin(context.Context) (*service.User, error) {
	cloned := *s.user
	return &cloned, nil
}
func (s *userHandlerRepoStub) Update(_ context.Context, user *service.User) error {
	cloned := *user
	s.user = &cloned
	return nil
}
func (s *userHandlerRepoStub) Delete(context.Context, int64) error { return nil }
func (s *userHandlerRepoStub) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	if s.user == nil || s.user.AvatarURL == "" {
		return nil, nil
	}
	return &service.UserAvatar{
		StorageProvider: s.user.AvatarSource,
		URL:             s.user.AvatarURL,
		ContentType:     s.user.AvatarMIME,
		ByteSize:        s.user.AvatarByteSize,
		SHA256:          s.user.AvatarSHA256,
	}, nil
}
func (s *userHandlerRepoStub) UpsertUserAvatar(_ context.Context, _ int64, input service.UpsertUserAvatarInput) (*service.UserAvatar, error) {
	s.user.AvatarURL = input.URL
	s.user.AvatarSource = input.StorageProvider
	s.user.AvatarMIME = input.ContentType
	s.user.AvatarByteSize = input.ByteSize
	s.user.AvatarSHA256 = input.SHA256
	return &service.UserAvatar{
		StorageProvider: input.StorageProvider,
		URL:             input.URL,
		ContentType:     input.ContentType,
		ByteSize:        input.ByteSize,
		SHA256:          input.SHA256,
	}, nil
}
func (s *userHandlerRepoStub) DeleteUserAvatar(context.Context, int64) error {
	s.user.AvatarURL = ""
	s.user.AvatarSource = ""
	s.user.AvatarMIME = ""
	s.user.AvatarByteSize = 0
	s.user.AvatarSHA256 = ""
	return nil
}
func (s *userHandlerRepoStub) List(context.Context, pagination.PaginationParams) ([]service.User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *userHandlerRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, service.UserListFilters) ([]service.User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *userHandlerRepoStub) UpdateBalance(context.Context, int64, float64) error { return nil }
func (s *userHandlerRepoStub) DeductBalance(context.Context, int64, float64) error { return nil }
func (s *userHandlerRepoStub) UpdateConcurrency(context.Context, int64, int) error { return nil }
func (s *userHandlerRepoStub) BatchSetConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}
func (s *userHandlerRepoStub) BatchAddConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}
func (s *userHandlerRepoStub) BatchUpdateLimits(context.Context, []int64, *int, *int) (int, error) {
	return 0, nil
}
func (s *userHandlerRepoStub) ExistsByEmail(context.Context, string) (bool, error) { return false, nil }
func (s *userHandlerRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	return 0, nil
}
func (s *userHandlerRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (s *userHandlerRepoStub) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	return map[int64]*time.Time{}, nil
}
func (s *userHandlerRepoStub) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	return nil, nil
}
func (s *userHandlerRepoStub) UpdateUserLastActiveAt(_ context.Context, _ int64, activeAt time.Time) error {
	if s.user != nil {
		s.user.LastActiveAt = &activeAt
	}
	return nil
}
func (s *userHandlerRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (s *userHandlerRepoStub) UpdateTotpSecret(context.Context, int64, *string) error { return nil }
func (s *userHandlerRepoStub) EnableTotp(context.Context, int64) error                { return nil }
func (s *userHandlerRepoStub) DisableTotp(context.Context, int64) error               { return nil }
func (s *userHandlerRepoStub) WithDailyCheckinTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}
func (s *userHandlerRepoStub) ListRecentDailyCheckinRecords(context.Context, int64, int) ([]service.DailyCheckinRecord, error) {
	out := make([]service.DailyCheckinRecord, len(s.checkinRecords))
	copy(out, s.checkinRecords)
	return out, nil
}
func (s *userHandlerRepoStub) ListDailyCheckinRecords(_ context.Context, _ int64, page, pageSize int) ([]service.DailyCheckinRecord, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	total := int64(len(s.checkinRecords))
	start := (page - 1) * pageSize
	if start >= len(s.checkinRecords) {
		return []service.DailyCheckinRecord{}, total, nil
	}
	end := start + pageSize
	if end > len(s.checkinRecords) {
		end = len(s.checkinRecords)
	}
	out := make([]service.DailyCheckinRecord, end-start)
	copy(out, s.checkinRecords[start:end])
	return out, total, nil
}
func (s *userHandlerRepoStub) GetDailyCheckinRecordByDate(context.Context, int64, time.Time) (*service.DailyCheckinRecord, error) {
	return nil, nil
}
func (s *userHandlerRepoStub) CreateDailyCheckinRecord(_ context.Context, record *service.DailyCheckinRecord) error {
	if s != nil {
		if record != nil {
			s.checkinRecords = append([]service.DailyCheckinRecord{*record}, s.checkinRecords...)
		}
	}
	return nil
}
func (s *userHandlerRepoStub) GetByIDIncludeDeleted(ctx context.Context, id int64) (*service.User, error) {
	return s.GetByID(ctx, id)
}
func (s *userHandlerRepoStub) ListUserAuthIdentities(context.Context, int64) ([]service.UserAuthIdentityRecord, error) {
	out := make([]service.UserAuthIdentityRecord, len(s.identities))
	copy(out, s.identities)
	return out, nil
}
func (s *userHandlerRepoStub) HasUserQQ(context.Context, int64) (bool, error) { return s.hasQQ, nil }
func (s *userHandlerRepoStub) UnbindUserAuthProvider(_ context.Context, _ int64, provider string) error {
	s.unbound = append(s.unbound, provider)
	filtered := s.identities[:0]
	for _, identity := range s.identities {
		if identity.ProviderType == provider {
			continue
		}
		filtered = append(filtered, identity)
	}
	s.identities = append([]service.UserAuthIdentityRecord(nil), filtered...)
	return nil
}

type userAttributeDefRepoStub struct {
	defs []service.UserAttributeDefinition
}

func (s *userAttributeDefRepoStub) Create(context.Context, *service.UserAttributeDefinition) error {
	return nil
}

func (s *userAttributeDefRepoStub) GetByID(_ context.Context, id int64) (*service.UserAttributeDefinition, error) {
	for i := range s.defs {
		if s.defs[i].ID == id {
			def := s.defs[i]
			return &def, nil
		}
	}
	return nil, service.ErrAttributeDefinitionNotFound
}

func (s *userAttributeDefRepoStub) GetByKey(_ context.Context, key string) (*service.UserAttributeDefinition, error) {
	for i := range s.defs {
		if s.defs[i].Key == key {
			def := s.defs[i]
			return &def, nil
		}
	}
	return nil, service.ErrAttributeDefinitionNotFound
}

func (s *userAttributeDefRepoStub) Update(context.Context, *service.UserAttributeDefinition) error {
	return nil
}

func (s *userAttributeDefRepoStub) Delete(context.Context, int64) error {
	return nil
}

func (s *userAttributeDefRepoStub) List(_ context.Context, enabledOnly bool) ([]service.UserAttributeDefinition, error) {
	out := make([]service.UserAttributeDefinition, 0, len(s.defs))
	for _, def := range s.defs {
		if enabledOnly && !def.Enabled {
			continue
		}
		out = append(out, def)
	}
	return out, nil
}

func (s *userAttributeDefRepoStub) UpdateDisplayOrders(context.Context, map[int64]int) error {
	return nil
}

func (s *userAttributeDefRepoStub) ExistsByKey(_ context.Context, key string) (bool, error) {
	for _, def := range s.defs {
		if def.Key == key {
			return true, nil
		}
	}
	return false, nil
}

type userAttributeValueRepoStub struct {
	values []service.UserAttributeValue
}

func (s *userAttributeValueRepoStub) GetByUserID(_ context.Context, userID int64) ([]service.UserAttributeValue, error) {
	out := make([]service.UserAttributeValue, 0, len(s.values))
	for _, value := range s.values {
		if value.UserID == userID {
			out = append(out, value)
		}
	}
	return out, nil
}

func (s *userAttributeValueRepoStub) GetByUserIDs(_ context.Context, userIDs []int64) ([]service.UserAttributeValue, error) {
	allowed := make(map[int64]struct{}, len(userIDs))
	for _, id := range userIDs {
		allowed[id] = struct{}{}
	}
	out := make([]service.UserAttributeValue, 0, len(s.values))
	for _, value := range s.values {
		if _, ok := allowed[value.UserID]; ok {
			out = append(out, value)
		}
	}
	return out, nil
}

func (s *userAttributeValueRepoStub) UpsertBatch(context.Context, int64, []service.UpdateUserAttributeInput) error {
	return nil
}

func (s *userAttributeValueRepoStub) DeleteByAttributeID(context.Context, int64) error {
	return nil
}

func (s *userAttributeValueRepoStub) DeleteByUserID(context.Context, int64) error {
	return nil
}

func TestUserHandlerUpdateProfileReturnsAvatarURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:       11,
			Email:    "handler-avatar@example.com",
			Username: "handler-avatar",
			Role:     service.RoleUser,
			Status:   service.StatusActive,
		},
	}
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), nil, nil, nil, nil, nil, nil)

	body := []byte(`{"avatar_url":"https://cdn.example.com/avatar.png"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/user", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 11})

	handler.UpdateProfile(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			AvatarURL string `json:"avatar_url"`
			Username  string `json:"username"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, "https://cdn.example.com/avatar.png", resp.Data.AvatarURL)
	require.Equal(t, "handler-avatar", resp.Data.Username)
}

func TestUserHandlerGetAttributesReturnsEnabledDefinitionsAndOwnValues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	attrService := service.NewUserAttributeService(
		&userAttributeDefRepoStub{defs: []service.UserAttributeDefinition{
			{
				ID:        10,
				Key:       "loyalty_weekly_points",
				Name:      "周积分",
				Type:      service.AttributeTypeNumber,
				Enabled:   true,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        11,
				Key:       "internal_note",
				Name:      "内部备注",
				Type:      service.AttributeTypeText,
				Enabled:   false,
				CreatedAt: now,
				UpdatedAt: now,
			},
		}},
		&userAttributeValueRepoStub{values: []service.UserAttributeValue{
			{ID: 101, UserID: 42, AttributeID: 10, Value: "880", CreatedAt: now, UpdatedAt: now},
			{ID: 102, UserID: 42, AttributeID: 11, Value: "hidden", CreatedAt: now, UpdatedAt: now},
			{ID: 103, UserID: 43, AttributeID: 10, Value: "9999", CreatedAt: now, UpdatedAt: now},
		}},
	)
	handler := NewUserHandler(nil, nil, nil, nil, nil, nil, attrService)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/attributes", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

	handler.GetAttributes(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Definitions []UserAttributeDefinitionResponse `json:"definitions"`
			Values      []UserAttributeValueResponse      `json:"values"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Definitions, 1)
	require.Equal(t, "loyalty_weekly_points", resp.Data.Definitions[0].Key)
	require.Len(t, resp.Data.Values, 1)
	require.Equal(t, int64(42), resp.Data.Values[0].UserID)
	require.Equal(t, int64(10), resp.Data.Values[0].AttributeID)
	require.Equal(t, "880", resp.Data.Values[0].Value)
}

func TestUserHandlerGetAttributesReturnsEmptyArraysWhenNoAttributes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	attrService := service.NewUserAttributeService(
		&userAttributeDefRepoStub{},
		&userAttributeValueRepoStub{},
	)
	handler := NewUserHandler(nil, nil, nil, nil, nil, nil, attrService)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/attributes", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

	handler.GetAttributes(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Definitions []UserAttributeDefinitionResponse `json:"definitions"`
			Values      []UserAttributeValueResponse      `json:"values"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Empty(t, resp.Data.Definitions)
	require.Empty(t, resp.Data.Values)
}

func TestUserHandlerGetAttributesRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewUserHandler(nil, nil, nil, nil, nil, nil, nil)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/attributes", nil)

	handler.GetAttributes(c)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestUserHandlerGetProfileReturnsIdentitySummaries(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifiedAt := time.Date(2026, 4, 20, 8, 30, 0, 0, time.UTC)
	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:       11,
			Email:    "identity@example.com",
			Username: "identity-user",
			Role:     service.RoleUser,
			Status:   service.StatusActive,
		},
		identities: []service.UserAuthIdentityRecord{
			{
				ProviderType:    "linuxdo",
				ProviderKey:     "linuxdo",
				ProviderSubject: "linuxdo-subject-123456",
				VerifiedAt:      &verifiedAt,
				Metadata: map[string]any{
					"username": "linuxdo-handle",
				},
			},
			{
				ProviderType:    "oidc",
				ProviderKey:     "https://issuer.example.com",
				ProviderSubject: "oidc-user-abc",
				Metadata: map[string]any{
					"suggested_display_name": "OIDC Display",
				},
			},
		},
	}
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/profile", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 11})

	handler.GetProfile(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Identities struct {
				Email struct {
					Bound       bool   `json:"bound"`
					BoundCount  int    `json:"bound_count"`
					DisplayName string `json:"display_name"`
				} `json:"email"`
				LinuxDo struct {
					Bound       bool   `json:"bound"`
					BoundCount  int    `json:"bound_count"`
					DisplayName string `json:"display_name"`
					ProviderKey string `json:"provider_key"`
				} `json:"linuxdo"`
				OIDC struct {
					Bound       bool   `json:"bound"`
					DisplayName string `json:"display_name"`
					ProviderKey string `json:"provider_key"`
				} `json:"oidc"`
				WeChat struct {
					Bound         bool   `json:"bound"`
					CanBind       bool   `json:"can_bind"`
					BindStartPath string `json:"bind_start_path"`
				} `json:"wechat"`
			} `json:"identities"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.True(t, resp.Data.Identities.Email.Bound)
	require.Equal(t, 1, resp.Data.Identities.Email.BoundCount)
	require.Equal(t, "identity@example.com", resp.Data.Identities.Email.DisplayName)
	require.True(t, resp.Data.Identities.LinuxDo.Bound)
	require.Equal(t, 1, resp.Data.Identities.LinuxDo.BoundCount)
	require.Equal(t, "linuxdo-handle", resp.Data.Identities.LinuxDo.DisplayName)
	require.Equal(t, "linuxdo", resp.Data.Identities.LinuxDo.ProviderKey)
	require.True(t, resp.Data.Identities.OIDC.Bound)
	require.Equal(t, "OIDC Display", resp.Data.Identities.OIDC.DisplayName)
	require.Equal(t, "https://issuer.example.com", resp.Data.Identities.OIDC.ProviderKey)
	require.False(t, resp.Data.Identities.WeChat.Bound)
	require.True(t, resp.Data.Identities.WeChat.CanBind)
	require.Contains(t, resp.Data.Identities.WeChat.BindStartPath, "/api/v1/auth/oauth/wechat/bind/start")
}

func TestUserHandlerGetProfileReturnsLegacyCompatibilityFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifiedAt := time.Date(2026, 4, 20, 8, 30, 0, 0, time.UTC)
	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           21,
			Email:        "legacy-profile@example.com",
			Username:     "linuxdo-handle",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			AvatarURL:    "https://cdn.example.com/linuxdo.png",
			AvatarSource: "remote_url",
		},
		identities: []service.UserAuthIdentityRecord{
			{
				ProviderType:    "linuxdo",
				ProviderKey:     "linuxdo",
				ProviderSubject: "linuxdo-subject-21",
				VerifiedAt:      &verifiedAt,
				Metadata: map[string]any{
					"username":   "linuxdo-handle",
					"avatar_url": "https://cdn.example.com/linuxdo.png",
				},
			},
		},
	}
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/profile", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 21})

	handler.GetProfile(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, true, resp.Data["email_bound"])
	require.Equal(t, true, resp.Data["linuxdo_bound"])
	require.Equal(t, false, resp.Data["oidc_bound"])
	require.Equal(t, false, resp.Data["wechat_bound"])
	require.Equal(t, "https://cdn.example.com/linuxdo.png", resp.Data["avatar_url"])

	avatarSource, ok := resp.Data["avatar_source"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "linuxdo", avatarSource["provider"])
	require.Equal(t, "linuxdo", avatarSource["source"])

	authBindings, ok := resp.Data["auth_bindings"].(map[string]any)
	require.True(t, ok)
	linuxdoBinding, ok := authBindings["linuxdo"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, linuxdoBinding["bound"])
	require.Equal(t, "linuxdo", linuxdoBinding["provider"])

	identityBindings, ok := resp.Data["identity_bindings"].(map[string]any)
	require.True(t, ok)
	emailBinding, ok := identityBindings["email"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, emailBinding["bound"])
	require.Equal(t, "profile.authBindings.notes.emailManagedFromProfile", emailBinding["note_key"])

	linuxdoCompatBinding, ok := identityBindings["linuxdo"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "profile.authBindings.notes.canUnbind", linuxdoCompatBinding["note_key"])

	profileSources, ok := resp.Data["profile_sources"].(map[string]any)
	require.True(t, ok)
	usernameSource, ok := profileSources["username"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "linuxdo", usernameSource["provider"])
	require.Equal(t, "linuxdo", usernameSource["source"])
}

func TestUserHandlerGetProfileDoesNotInferEditedProfileSourcesWithoutMatchingIdentityMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           22,
			Email:        "edited-profile@example.com",
			Username:     "custom-name",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			AvatarURL:    "https://cdn.example.com/custom.png",
			AvatarSource: "remote_url",
		},
		identities: []service.UserAuthIdentityRecord{
			{
				ProviderType:    "linuxdo",
				ProviderKey:     "linuxdo",
				ProviderSubject: "linuxdo-subject-22",
				Metadata: map[string]any{
					"username":   "linuxdo-handle",
					"avatar_url": "https://cdn.example.com/linuxdo.png",
				},
			},
		},
	}
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/profile", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 22})

	handler.GetProfile(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.NotContains(t, resp.Data, "avatar_source")
	require.NotContains(t, resp.Data, "username_source")
	require.NotContains(t, resp.Data, "profile_sources")
}

func TestUserHandlerCheckInDailySuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:       31,
			Email:    "checkin@example.com",
			Username: "checkin-user",
			Role:     service.RoleUser,
			Status:   service.StatusActive,
			Balance:  10,
		},
		hasQQ: true,
	}
	repo.checkinRecords = []service.DailyCheckinRecord{}
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/checkin?timezone=Asia/Shanghai", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 31})

	handler.CheckInDaily(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Balance float64 `json:"balance"`
			Summary struct {
				QQBound        bool    `json:"qq_bound"`
				CheckedInToday bool    `json:"checked_in_today"`
				BaseReward     float64 `json:"base_reward"`
				TodayReward    float64 `json:"today_reward"`
				StreakDays     int     `json:"streak_days"`
			} `json:"summary"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.GreaterOrEqual(t, resp.Data.Balance, 11.0)
	require.LessOrEqual(t, resp.Data.Balance, 13.0)
	require.True(t, resp.Data.Summary.QQBound)
	require.True(t, resp.Data.Summary.CheckedInToday)
	require.GreaterOrEqual(t, resp.Data.Summary.BaseReward, 1.0)
	require.LessOrEqual(t, resp.Data.Summary.BaseReward, 3.0)
	require.Equal(t, resp.Data.Summary.BaseReward, resp.Data.Summary.TodayReward)
	require.Equal(t, 1, resp.Data.Summary.StreakDays)
}

func TestUserHandlerCheckInDailyRequiresQQ(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:       32,
			Email:    "no-qq@example.com",
			Username: "no-qq",
			Role:     service.RoleUser,
			Status:   service.StatusActive,
		},
	}
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/checkin", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 32})

	handler.CheckInDaily(c)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	var resp struct {
		Code int `json:"code"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.NotEqual(t, 0, resp.Code)
}

func TestUserHandlerGetCheckInStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	yesterday, err := time.Parse("2006-01-02", time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02"))
	require.NoError(t, err)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:       33,
			Email:    "status@example.com",
			Username: "status-user",
			Role:     service.RoleUser,
			Status:   service.StatusActive,
			Balance:  20,
		},
		hasQQ: true,
		checkinRecords: []service.DailyCheckinRecord{
			{UserID: 33, CheckinDate: yesterday, Timezone: "UTC", TotalReward: 3, StreakDays: 1},
		},
	}
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/checkin/status?timezone=UTC", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 33})

	handler.GetCheckInStatus(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Balance float64 `json:"balance"`
			Summary struct {
				QQBound        bool `json:"qq_bound"`
				CanCheckIn     bool `json:"can_check_in"`
				CheckedInToday bool `json:"checked_in_today"`
				StreakDays     int  `json:"streak_days"`
				RecentRecords  []struct {
					UserID int64 `json:"user_id"`
				} `json:"recent_records"`
			} `json:"summary"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, 20.0, resp.Data.Balance)
	require.True(t, resp.Data.Summary.QQBound)
	require.False(t, resp.Data.Summary.CheckedInToday)
	require.True(t, resp.Data.Summary.CanCheckIn)
	require.Equal(t, 2, resp.Data.Summary.StreakDays)
	require.Len(t, resp.Data.Summary.RecentRecords, 1)
}

func TestUserHandlerListCheckInHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:       34,
			Email:    "history@example.com",
			Username: "history-user",
			Role:     service.RoleUser,
			Status:   service.StatusActive,
		},
		checkinRecords: []service.DailyCheckinRecord{
			{UserID: 34, CheckinDate: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC), Timezone: "UTC", TotalReward: 3, StreakDays: 1},
			{UserID: 34, CheckinDate: time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC), Timezone: "UTC", TotalReward: 3, StreakDays: 1},
		},
	}
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/checkin/history?page=1&page_size=1", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 34})

	handler.ListCheckInHistory(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items    []service.DailyCheckinRecord `json:"items"`
			Total    int64                        `json:"total"`
			Page     int                          `json:"page"`
			PageSize int                          `json:"page_size"`
			Pages    int                          `json:"pages"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, int64(2), resp.Data.Total)
	require.Equal(t, 1, resp.Data.Page)
	require.Equal(t, 1, resp.Data.PageSize)
	require.Equal(t, 2, resp.Data.Pages)
	require.Len(t, resp.Data.Items, 1)
	require.Equal(t, int64(34), resp.Data.Items[0].UserID)
}

type userHandlerEmailCacheStub struct {
	data *service.VerificationCodeData
}

type userHandlerRefreshTokenCacheStub struct {
	revokedUserIDs []int64
}

func (s *userHandlerRefreshTokenCacheStub) StoreRefreshToken(context.Context, string, *service.RefreshTokenData, time.Duration) error {
	return nil
}

func (s *userHandlerRefreshTokenCacheStub) GetRefreshToken(context.Context, string) (*service.RefreshTokenData, error) {
	return nil, service.ErrRefreshTokenNotFound
}

func (s *userHandlerRefreshTokenCacheStub) DeleteRefreshToken(context.Context, string) error {
	return nil
}

func (s *userHandlerRefreshTokenCacheStub) DeleteUserRefreshTokens(_ context.Context, userID int64) error {
	s.revokedUserIDs = append(s.revokedUserIDs, userID)
	return nil
}

func (s *userHandlerRefreshTokenCacheStub) DeleteTokenFamily(context.Context, string) error {
	return nil
}

func (s *userHandlerRefreshTokenCacheStub) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}

func (s *userHandlerRefreshTokenCacheStub) AddToFamilyTokenSet(context.Context, string, string, time.Duration) error {
	return nil
}

func (s *userHandlerRefreshTokenCacheStub) GetUserTokenHashes(context.Context, int64) ([]string, error) {
	return nil, nil
}

func (s *userHandlerRefreshTokenCacheStub) GetFamilyTokenHashes(context.Context, string) ([]string, error) {
	return nil, nil
}

func (s *userHandlerRefreshTokenCacheStub) IsTokenInFamily(context.Context, string, string) (bool, error) {
	return false, nil
}

func (s *userHandlerEmailCacheStub) GetVerificationCode(context.Context, string) (*service.VerificationCodeData, error) {
	return s.data, nil
}

func (s *userHandlerEmailCacheStub) SetVerificationCode(context.Context, string, *service.VerificationCodeData, time.Duration) error {
	return nil
}

func (s *userHandlerEmailCacheStub) DeleteVerificationCode(context.Context, string) error {
	return nil
}

func (s *userHandlerEmailCacheStub) GetNotifyVerifyCode(context.Context, string) (*service.VerificationCodeData, error) {
	return nil, nil
}

func (s *userHandlerEmailCacheStub) SetNotifyVerifyCode(context.Context, string, *service.VerificationCodeData, time.Duration) error {
	return nil
}

func (s *userHandlerEmailCacheStub) DeleteNotifyVerifyCode(context.Context, string) error {
	return nil
}

func (s *userHandlerEmailCacheStub) GetPasswordResetToken(context.Context, string) (*service.PasswordResetTokenData, error) {
	return nil, nil
}

func (s *userHandlerEmailCacheStub) SetPasswordResetToken(context.Context, string, *service.PasswordResetTokenData, time.Duration) error {
	return nil
}

func (s *userHandlerEmailCacheStub) DeletePasswordResetToken(context.Context, string) error {
	return nil
}

func (s *userHandlerEmailCacheStub) IsPasswordResetEmailInCooldown(context.Context, string) bool {
	return false
}

func (s *userHandlerEmailCacheStub) SetPasswordResetEmailCooldown(context.Context, string, time.Duration) error {
	return nil
}

func (s *userHandlerEmailCacheStub) GetNotifyCodeUserRate(context.Context, int64) (int64, error) {
	return 0, nil
}

func (s *userHandlerEmailCacheStub) IncrNotifyCodeUserRate(context.Context, int64, time.Duration) (int64, error) {
	return 0, nil
}

func TestUserHandlerBindEmailIdentityReturnsProfileResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:       11,
			Email:    "legacy-user" + service.LinuxDoConnectSyntheticEmailDomain,
			Username: "legacy-user",
			Role:     service.RoleUser,
			Status:   service.StatusActive,
		},
	}
	emailCache := &userHandlerEmailCacheStub{
		data: &service.VerificationCodeData{
			Code:      "123456",
			CreatedAt: time.Now().UTC(),
			ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
		},
	}
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:     "test-secret",
			ExpireHour: 1,
		},
	}
	emailService := service.NewEmailService(nil, emailCache)
	authService := service.NewAuthService(nil, repo, nil, nil, cfg, nil, emailService, nil, nil, nil, nil, nil, nil)
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), authService, nil, nil, nil, nil, nil)

	body := []byte(`{"email":"new@example.com","verify_code":"123456","password":"new-password"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/account-bindings/email", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "provider", Value: "email"}}
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 11})

	handler.BindEmailIdentity(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Email      string `json:"email"`
			EmailBound bool   `json:"email_bound"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, "new@example.com", resp.Data.Email)
	require.True(t, resp.Data.EmailBound)
}

func TestUserHandlerUnbindIdentityReturnsUpdatedProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:       21,
			Email:    "identity@example.com",
			Username: "identity-user",
			Role:     service.RoleUser,
			Status:   service.StatusActive,
		},
		identities: []service.UserAuthIdentityRecord{
			{
				ProviderType:    "email",
				ProviderKey:     "email",
				ProviderSubject: "identity@example.com",
			},
			{
				ProviderType:    "linuxdo",
				ProviderKey:     "linuxdo",
				ProviderSubject: "linuxdo-subject-21",
				Metadata: map[string]any{
					"username": "linuxdo-handle",
				},
			},
		},
	}
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/user/account-bindings/linuxdo", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 21})
	c.Params = gin.Params{{Key: "provider", Value: "linuxdo"}}

	handler.UnbindIdentity(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []string{"linuxdo"}, repo.unbound)

	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)

	authBindings, ok := resp.Data["auth_bindings"].(map[string]any)
	require.True(t, ok)
	linuxdoBinding, ok := authBindings["linuxdo"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, linuxdoBinding["bound"])
}

func TestUserHandlerUnbindIdentityRevokesAllUserSessionsWhenAuthServiceConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           23,
			Email:        "identity@example.com",
			Username:     "identity-user",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			TokenVersion: 4,
		},
		identities: []service.UserAuthIdentityRecord{
			{
				ProviderType:    "email",
				ProviderKey:     "email",
				ProviderSubject: "identity@example.com",
			},
			{
				ProviderType:    "linuxdo",
				ProviderKey:     "linuxdo",
				ProviderSubject: "linuxdo-subject-23",
			},
		},
	}
	refreshTokenCache := &userHandlerRefreshTokenCacheStub{}
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:     "test-secret",
			ExpireHour: 1,
		},
	}
	authService := service.NewAuthService(nil, repo, nil, refreshTokenCache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), authService, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/user/account-bindings/linuxdo", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 23})
	c.Params = gin.Params{{Key: "provider", Value: "linuxdo"}}

	handler.UnbindIdentity(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []int64{23}, refreshTokenCache.revokedUserIDs)
	require.Equal(t, int64(5), repo.user.TokenVersion)
}

func TestUserHandlerUnbindIdentityDoesNotRevokeSessionsWhenNothingWasUnbound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           24,
			Email:        "identity@example.com",
			Username:     "identity-user",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			TokenVersion: 4,
		},
		identities: []service.UserAuthIdentityRecord{
			{
				ProviderType:    "email",
				ProviderKey:     "email",
				ProviderSubject: "identity@example.com",
			},
		},
	}
	refreshTokenCache := &userHandlerRefreshTokenCacheStub{}
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:     "test-secret",
			ExpireHour: 1,
		},
	}
	authService := service.NewAuthService(nil, repo, nil, refreshTokenCache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), authService, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/user/account-bindings/linuxdo", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 24})
	c.Params = gin.Params{{Key: "provider", Value: "linuxdo"}}

	handler.UnbindIdentity(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, repo.unbound)
	require.Empty(t, refreshTokenCache.revokedUserIDs)
	require.Equal(t, int64(4), repo.user.TokenVersion)
}

func TestUserHandlerBindEmailIdentityRejectsWrongCurrentPasswordForBoundEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &service.User{
		ID:       11,
		Email:    "current@example.com",
		Username: "bound-user",
		Role:     service.RoleUser,
		Status:   service.StatusActive,
	}
	require.NoError(t, user.SetPassword("current-password"))

	repo := &userHandlerRepoStub{user: user}
	emailCache := &userHandlerEmailCacheStub{
		data: &service.VerificationCodeData{
			Code:      "123456",
			CreatedAt: time.Now().UTC(),
			ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
		},
	}
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:     "test-secret",
			ExpireHour: 1,
		},
	}
	emailService := service.NewEmailService(nil, emailCache)
	authService := service.NewAuthService(nil, repo, nil, nil, cfg, nil, emailService, nil, nil, nil, nil, nil, nil)
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), authService, nil, nil, nil, nil, nil)

	body := []byte(`{"email":"new@example.com","verify_code":"123456","password":"wrong-password"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/account-bindings/email", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 11})

	handler.BindEmailIdentity(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Equal(t, "PASSWORD_INCORRECT", resp.Reason)
	require.Equal(t, "current password is incorrect", resp.Message)
	require.Equal(t, "current@example.com", repo.user.Email)
}

func TestUserHandlerStartIdentityBindingReturnsAuthorizeURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:       11,
			Email:    "identity@example.com",
			Username: "identity-user",
			Role:     service.RoleUser,
			Status:   service.StatusActive,
		},
	}
	handler := NewUserHandler(service.NewUserService(repo, nil, nil, nil), nil, nil, nil, nil, nil, nil)

	body := []byte(`{"provider":"wechat","redirect_to":"/settings/profile"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/auth-identities/bind/start", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 11})

	handler.StartIdentityBinding(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Provider           string `json:"provider"`
			AuthorizeURL       string `json:"authorize_url"`
			Method             string `json:"method"`
			UseBrowserRedirect bool   `json:"use_browser_redirect"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, "wechat", resp.Data.Provider)
	require.Equal(t, "GET", resp.Data.Method)
	require.True(t, resp.Data.UseBrowserRedirect)
	require.Contains(t, resp.Data.AuthorizeURL, "/api/v1/auth/oauth/wechat/bind/start")
	require.Contains(t, resp.Data.AuthorizeURL, "intent=bind_current_user")
	require.Contains(t, resp.Data.AuthorizeURL, "redirect=%2Fsettings%2Fprofile")
}
