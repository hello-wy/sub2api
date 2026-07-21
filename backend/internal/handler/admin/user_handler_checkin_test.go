//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminCheckinRepoStub struct {
	service.UserRepository

	user                *service.User
	hasQQ               bool
	checkinRecords      []service.DailyCheckinRecord
	updatedBalanceUser  int64
	updatedBalanceDelta float64
	createdRecord       *service.DailyCheckinRecord
}

func (s *adminCheckinRepoStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	if s.user == nil {
		return &service.User{ID: id}, nil
	}
	cloned := *s.user
	return &cloned, nil
}

func (s *adminCheckinRepoStub) HasUserQQ(context.Context, int64) (bool, error) {
	return s.hasQQ, nil
}

func (s *adminCheckinRepoStub) UpdateBalance(_ context.Context, userID int64, amount float64) error {
	s.updatedBalanceUser = userID
	s.updatedBalanceDelta = amount
	return nil
}

func (s *adminCheckinRepoStub) ListRecentDailyCheckinRecords(context.Context, int64, int) ([]service.DailyCheckinRecord, error) {
	out := make([]service.DailyCheckinRecord, len(s.checkinRecords))
	copy(out, s.checkinRecords)
	return out, nil
}

func (s *adminCheckinRepoStub) ListDailyCheckinRecords(context.Context, int64, int, int) ([]service.DailyCheckinRecord, int64, error) {
	out := make([]service.DailyCheckinRecord, len(s.checkinRecords))
	copy(out, s.checkinRecords)
	return out, int64(len(out)), nil
}

func (s *adminCheckinRepoStub) GetDailyCheckinRecordByDate(context.Context, int64, time.Time) (*service.DailyCheckinRecord, error) {
	return nil, nil
}

func (s *adminCheckinRepoStub) CreateDailyCheckinRecord(_ context.Context, record *service.DailyCheckinRecord) error {
	cloned := *record
	cloned.ID = 101
	*record = cloned
	s.createdRecord = &cloned
	s.checkinRecords = append([]service.DailyCheckinRecord{cloned}, s.checkinRecords...)
	return nil
}

func TestUserHandlerCheckInUserRequiresQQBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &adminCheckinRepoStub{
		user:  &service.User{ID: 31, Balance: 10},
		hasQQ: false,
	}
	handler := NewUserHandler(newStubAdminService(), nil, nil, nil, nil, service.NewUserService(repo, nil, nil, nil), nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "31"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/31/checkin", nil)

	handler.CheckInUser(c)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Nil(t, repo.createdRecord)
	require.Zero(t, repo.updatedBalanceDelta)

	var resp struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "DAILY_CHECKIN_QQ_REQUIRED", resp.Reason)
}

func TestUserHandlerCheckInUserByID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &adminCheckinRepoStub{
		user:  &service.User{ID: 31, Balance: 10},
		hasQQ: true,
	}
	handler := NewUserHandler(newStubAdminService(), nil, nil, nil, nil, service.NewUserService(repo, nil, nil, nil), nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "31"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/31/checkin?timezone=Asia/Shanghai", nil)

	handler.CheckInUser(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, repo.createdRecord)
	require.Equal(t, int64(31), repo.updatedBalanceUser)
	require.Equal(t, int64(31), repo.createdRecord.UserID)
	require.GreaterOrEqual(t, repo.createdRecord.BaseReward, 1.0)
	require.LessOrEqual(t, repo.createdRecord.BaseReward, 3.0)
	require.Equal(t, repo.createdRecord.TotalReward, repo.updatedBalanceDelta)

	var resp struct {
		Code int                  `json:"code"`
		Data DailyCheckinResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.True(t, resp.Data.Summary.QQBound)
	require.False(t, resp.Data.Summary.CanCheckIn)
	require.True(t, resp.Data.Summary.CheckedInToday)
	require.Equal(t, 10+repo.createdRecord.TotalReward, resp.Data.Balance)
	require.Equal(t, "Asia/Shanghai", resp.Data.Summary.Timezone)
}
