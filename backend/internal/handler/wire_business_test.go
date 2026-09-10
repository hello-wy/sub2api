package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBusinessProvidersReachRequestValidation(t *testing.T) {
	lottery := &service.LotteryService{}
	user := ProvideUserHandler(UserHandlerDependencies{LotteryService: lottery})
	adminUser := ProvideAdminUserHandler(AdminUserHandlerDependencies{
		LotteryService: lottery, QQBindingService: &service.QQBindingService{},
	})
	settings := ProvideAdminSettingHandler(AdminSettingHandlerDependencies{LotteryService: lottery})
	for _, tc := range []struct {
		name   string
		handle gin.HandlerFunc
		status int
	}{
		{"lottery authentication", user.GetLotteryStatus, http.StatusUnauthorized},
		{"ticket user ID", adminUser.AdjustLotteryTickets, http.StatusBadRequest},
		{"QQ binding request", adminUser.ConfirmQQBinding, http.StatusBadRequest},
		{"prize pool request", settings.UpdateLotteryPrizePool, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			tc.handle(ctx)
			require.Equal(t, tc.status, response.Code, response.Body.String())
		})
	}
}

func TestAdminHandlersRetainWelfareHandler(t *testing.T) {
	welfare := admin.NewWelfareHandler(&service.WelfareService{})
	handlers := ProvideAdminHandlers(AdminHandlersDependencies{
		AccountHandler: &admin.AccountHandler{}, WelfareHandler: welfare,
	})
	require.Same(t, welfare, handlers.Welfare)
}
