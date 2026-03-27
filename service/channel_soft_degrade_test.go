package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestObserveChannelSoftDegrade_TracksByModelAndPathAndRelievesOnSuccess(t *testing.T) {
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = false
		common.AutomaticDisableThresholdWindowSeconds = 300
		common.ChannelSoftDegradeWindowSeconds = 86400
		nowFunc = time.Now
		channelSoftDegradeTracker.Lock()
		channelSoftDegradeTracker.events = map[string][]time.Time{}
		channelSoftDegradeTracker.Unlock()
	})

	common.AutomaticDisableChannelEnabled = true
	common.AutomaticDisableThresholdWindowSeconds = 60
	common.ChannelSoftDegradeWindowSeconds = 60
	currentTime := time.Unix(1_700_001_000, 0)
	nowFunc = func() time.Time {
		return currentTime
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)

	channelError := types.ChannelError{
		ChannelId:   91,
		ChannelType: constant.ChannelTypeOpenAI,
		ChannelName: "soft-degrade",
		AutoBan:     true,
	}
	capabilityErr := types.NewOpenAIError(
		errors.New("unsupported endpoint /v1/responses/compact"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusNotFound,
	)

	ObserveChannelSoftDegrade(ctx, channelError, "gpt-5.4-openai-compact", capabilityErr)
	require.Equal(t, 1, GetChannelSoftDegradePenalty(ctx, channelError.ChannelId, "gpt-5.4-openai-compact"))

	currentTime = currentTime.Add(10 * time.Second)
	ObserveChannelSoftDegrade(ctx, channelError, "gpt-5.4-openai-compact", capabilityErr)
	require.Equal(t, 2, GetChannelSoftDegradePenalty(ctx, channelError.ChannelId, "gpt-5.4-openai-compact"))

	currentTime = currentTime.Add(10 * time.Second)
	ObserveChannelSoftDegrade(ctx, channelError, "gpt-5.4-openai-compact", nil)
	require.Equal(t, 1, GetChannelSoftDegradePenalty(ctx, channelError.ChannelId, "gpt-5.4-openai-compact"))
	require.Equal(t, 0, GetChannelSoftDegradePenalty(ctx, channelError.ChannelId, "gpt-5.4"))

	otherRec := httptest.NewRecorder()
	otherCtx, _ := gin.CreateTestContext(otherRec)
	otherCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	require.Equal(t, 0, GetChannelSoftDegradePenalty(otherCtx, channelError.ChannelId, "gpt-5.4-openai-compact"))
}

func TestObserveChannelSoftDegrade_HardDisableErrorDoesNotAccumulatePenalty(t *testing.T) {
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = false
		common.AutomaticDisableThresholdWindowSeconds = 300
		common.ChannelSoftDegradeWindowSeconds = 86400
		nowFunc = time.Now
		channelSoftDegradeTracker.Lock()
		channelSoftDegradeTracker.events = map[string][]time.Time{}
		channelSoftDegradeTracker.Unlock()
	})

	common.AutomaticDisableChannelEnabled = true
	common.AutomaticDisableThresholdWindowSeconds = 60
	common.ChannelSoftDegradeWindowSeconds = 60
	nowFunc = func() time.Time {
		return time.Unix(1_700_001_200, 0)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	channelError := types.ChannelError{
		ChannelId:   92,
		ChannelType: constant.ChannelTypeOpenAI,
		ChannelName: "hard-disable",
		AutoBan:     true,
	}
	hardErr := types.NewOpenAIError(
		errors.New("invalid api key"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusUnauthorized,
	)

	ObserveChannelSoftDegrade(ctx, channelError, "gpt-5.4", hardErr)
	require.Equal(t, 0, GetChannelSoftDegradePenalty(ctx, channelError.ChannelId, "gpt-5.4"))
}

func TestObserveChannelSoftDegrade_UsesDedicatedWindow(t *testing.T) {
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = false
		common.AutomaticDisableThresholdWindowSeconds = 300
		common.ChannelSoftDegradeWindowSeconds = 86400
		nowFunc = time.Now
		channelSoftDegradeTracker.Lock()
		channelSoftDegradeTracker.events = map[string][]time.Time{}
		channelSoftDegradeTracker.Unlock()
	})

	common.AutomaticDisableChannelEnabled = true
	common.AutomaticDisableThresholdWindowSeconds = 60
	common.ChannelSoftDegradeWindowSeconds = 86400
	currentTime := time.Unix(1_700_002_000, 0)
	nowFunc = func() time.Time {
		return currentTime
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)

	channelError := types.ChannelError{
		ChannelId:   93,
		ChannelType: constant.ChannelTypeOpenAI,
		ChannelName: "dedicated-window",
		AutoBan:     true,
	}
	capabilityErr := types.NewOpenAIError(
		errors.New("unsupported endpoint /v1/responses/compact"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusNotFound,
	)

	ObserveChannelSoftDegrade(ctx, channelError, "gpt-5.4-openai-compact", capabilityErr)
	currentTime = currentTime.Add(10 * time.Minute)
	require.Equal(t, 1, GetChannelSoftDegradePenalty(ctx, channelError.ChannelId, "gpt-5.4-openai-compact"))
}

func TestBuildChannelRuntimeHealthMap_UsesWorstScopeAndPenaltyCap(t *testing.T) {
	t.Cleanup(func() {
		common.ChannelSoftDegradeWindowSeconds = 86400
		nowFunc = time.Now
		channelSoftDegradeTracker.Lock()
		channelSoftDegradeTracker.events = map[string][]time.Time{}
		channelSoftDegradeTracker.Unlock()
	})

	common.ChannelSoftDegradeWindowSeconds = 86400
	nowFunc = func() time.Time {
		return time.Unix(1_700_003_000, 0)
	}

	channelSoftDegradeTracker.Lock()
	channelSoftDegradeTracker.events = map[string][]time.Time{
		channelSoftDegradeKey(301, "gpt-5.4-openai-compact", "/v1/responses/compact"): {
			nowFunc(),
			nowFunc(),
			nowFunc(),
			nowFunc(),
			nowFunc(),
			nowFunc(),
		},
		channelSoftDegradeKey(301, "gpt-5.4", "/v1/responses"): {
			nowFunc(),
		},
	}
	channelSoftDegradeTracker.Unlock()

	healthMap := BuildChannelRuntimeHealthMap([]*model.Channel{{Id: 301}})
	health := healthMap[301]
	require.NotNil(t, health)
	require.Equal(t, 2, health.ActiveScopeCount)
	require.Equal(t, 6, health.MaxPenalty)
	require.Equal(t, 5, health.EffectivePenalty)
	require.Equal(t, 0, health.Score)
	require.Equal(t, "gpt-5.4-openai-compact", health.WorstModel)
	require.Equal(t, "/v1/responses/compact", health.WorstPath)
	require.Equal(t, 86400, health.WindowSeconds)
}
