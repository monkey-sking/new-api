package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelRequestRateLimitBypassesAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origRedisEnabled := common.RedisEnabled
	origLimiter := inMemoryRateLimiter
	origEnabled := setting.ModelRequestRateLimitEnabled
	origDuration := setting.ModelRequestRateLimitDurationMinutes
	origCount := setting.ModelRequestRateLimitCount
	origSuccessCount := setting.ModelRequestRateLimitSuccessCount
	t.Cleanup(func() {
		common.RedisEnabled = origRedisEnabled
		inMemoryRateLimiter = origLimiter
		setting.ModelRequestRateLimitEnabled = origEnabled
		setting.ModelRequestRateLimitDurationMinutes = origDuration
		setting.ModelRequestRateLimitCount = origCount
		setting.ModelRequestRateLimitSuccessCount = origSuccessCount
	})

	common.RedisEnabled = false
	inMemoryRateLimiter = common.InMemoryRateLimiter{}
	setting.ModelRequestRateLimitEnabled = true
	setting.ModelRequestRateLimitDurationMinutes = 1
	setting.ModelRequestRateLimitCount = 1
	setting.ModelRequestRateLimitSuccessCount = 1

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 1)
		c.Set("role", common.RoleAdminUser)
		c.Next()
	})
	router.Use(ModelRequestRateLimit())
	router.GET("/v1/chat/completions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)
	}
}

func TestModelRequestRateLimitStillAppliesToCommonUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origRedisEnabled := common.RedisEnabled
	origLimiter := inMemoryRateLimiter
	origEnabled := setting.ModelRequestRateLimitEnabled
	origDuration := setting.ModelRequestRateLimitDurationMinutes
	origCount := setting.ModelRequestRateLimitCount
	origSuccessCount := setting.ModelRequestRateLimitSuccessCount
	t.Cleanup(func() {
		common.RedisEnabled = origRedisEnabled
		inMemoryRateLimiter = origLimiter
		setting.ModelRequestRateLimitEnabled = origEnabled
		setting.ModelRequestRateLimitDurationMinutes = origDuration
		setting.ModelRequestRateLimitCount = origCount
		setting.ModelRequestRateLimitSuccessCount = origSuccessCount
	})

	common.RedisEnabled = false
	inMemoryRateLimiter = common.InMemoryRateLimiter{}
	setting.ModelRequestRateLimitEnabled = true
	setting.ModelRequestRateLimitDurationMinutes = 1
	setting.ModelRequestRateLimitCount = 1
	setting.ModelRequestRateLimitSuccessCount = 1

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 2)
		c.Set("role", common.RoleCommonUser)
		c.Next()
	})
	router.Use(ModelRequestRateLimit())
	router.GET("/v1/chat/completions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	first := httptest.NewRecorder()
	router.ServeHTTP(first, req)
	require.Equal(t, http.StatusOK, first.Code)

	secondReq := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	second := httptest.NewRecorder()
	router.ServeHTTP(second, secondReq)
	require.Equal(t, http.StatusTooManyRequests, second.Code)
}
