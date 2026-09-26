package middleware

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type scheduledGroupRepo struct{ service.GroupRepository }

func (*scheduledGroupRepo) GetByID(context.Context, int64) (*service.Group, error) {
	return &service.Group{ID: 17, Status: service.StatusActive}, nil
}

type scheduledKeyRepo struct{ service.APIKeyRepository }

func (*scheduledKeyRepo) GetByID(context.Context, int64) (*service.APIKey, error) {
	id := int64(17)
	return &service.APIKey{ID: 23, Key: "test-credential", GroupID: &id, Status: service.StatusActive}, nil
}

func TestScheduledPelicanGroupTargetRunsAfterAuthBeforeRouting(t *testing.T) {
	for _, test := range []struct {
		name       string
		keyGroup   int64
		rejectAuth bool
		success    bool
	}{
		{"normal group", 17, false, true},
		{"rebound or stale key", 18, false, false},
		{"auth rejected", 17, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			routed := false
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				require.Equal(t, "Bearer test-credential", c.GetHeader("Authorization"))
				if test.rejectAuth {
					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}
				c.Set(string(ContextKeyAPIKey), &service.APIKey{GroupID: &test.keyGroup})
				c.Next()
			}, ScheduledPelicanGroupTarget(), func(c *gin.Context) {
				routed = true
				fmt.Fprint(c.Writer, "data: {\"choices\":[{\"delta\":{\"content\":\"<svg></svg>\"},\"finish_reason\":\"stop\"}]}\n")
			})
			svc := service.ProvideScheduledTestService(nil, nil, nil, &scheduledGroupRepo{}, &scheduledKeyRepo{})
			svc.SetGroupGateway(router)
			result, err := svc.RunGroupPelican(context.Background(), &service.ScheduledTestPlan{GroupID: 17, APIKeyID: 23, ModelID: "public-model", PelicanConfig: &service.PelicanTestConfig{Prompt: "draw", ReasoningEffort: "medium"}})
			require.NoError(t, err)
			require.Equal(t, test.success, routed)
			require.Equal(t, test.success, result.Status == "success")
		})
	}
}
