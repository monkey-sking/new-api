package service

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestObserveFailureThreshold(t *testing.T) {
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = false
		common.AutomaticDisableConsecutiveFailureCount = 0
		common.AutomaticDisableThresholdWindowSeconds = 300
		nowFunc = time.Now
		channelFailureTracker.Lock()
		channelFailureTracker.events = map[string][]time.Time{}
		channelFailureTracker.Unlock()
	})

	common.AutomaticDisableChannelEnabled = true
	common.AutomaticDisableConsecutiveFailureCount = 2
	common.AutomaticDisableThresholdWindowSeconds = 60
	currentTime := time.Unix(1_700_000_000, 0)
	nowFunc = func() time.Time {
		return currentTime
	}

	channelError := types.ChannelError{
		ChannelId:   11,
		ChannelName: "retry-channel",
		AutoBan:     true,
	}

	retryableErr := types.NewOpenAIError(
		errors.New("upstream temporarily unavailable"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)

	first := ObserveFailureThreshold(channelError, retryableErr)
	require.Equal(t, retryableErr, first)

	currentTime = currentTime.Add(30 * time.Second)
	second := ObserveFailureThreshold(channelError, retryableErr)
	require.NotNil(t, second)
	require.Equal(t, types.ErrorCodeChannelFailureThresholdExceeded, second.GetErrorCode())
	require.Equal(t, http.StatusBadGateway, second.StatusCode)
	require.Equal(t, retryableErr.Error(), second.Error())
}

func TestObserveFailureThresholdKeepsWindowAcrossSuccessAndExpiresOutsideWindow(t *testing.T) {
	origDisableRanges := operation_setting.AutomaticDisableStatusCodeRanges
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = false
		common.AutomaticDisableConsecutiveFailureCount = 0
		common.AutomaticDisableThresholdWindowSeconds = 300
		operation_setting.AutomaticDisableStatusCodeRanges = origDisableRanges
		nowFunc = time.Now
		channelFailureTracker.Lock()
		channelFailureTracker.events = map[string][]time.Time{}
		channelFailureTracker.Unlock()
	})

	common.AutomaticDisableChannelEnabled = true
	common.AutomaticDisableConsecutiveFailureCount = 2
	common.AutomaticDisableThresholdWindowSeconds = 60
	operation_setting.AutomaticDisableStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 401, End: 401}}
	currentTime := time.Unix(1_700_000_100, 0)
	nowFunc = func() time.Time {
		return currentTime
	}

	channelError := types.ChannelError{
		ChannelId:   12,
		ChannelName: "retry-channel",
		AutoBan:     true,
	}

	retryableErr := types.NewOpenAIError(
		errors.New("upstream temporarily unavailable"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)
	unauthorizedErr := types.NewOpenAIError(
		errors.New("invalid api key"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusUnauthorized,
	)

	require.Equal(t, retryableErr, ObserveFailureThreshold(channelError, retryableErr))
	currentTime = currentTime.Add(20 * time.Second)
	require.Nil(t, ObserveFailureThreshold(channelError, nil))
	currentTime = currentTime.Add(20 * time.Second)
	thresholdErr := ObserveFailureThreshold(channelError, retryableErr)
	require.NotNil(t, thresholdErr)
	require.Equal(t, types.ErrorCodeChannelFailureThresholdExceeded, thresholdErr.GetErrorCode())

	require.Equal(t, unauthorizedErr, ObserveFailureThreshold(channelError, unauthorizedErr))
	currentTime = currentTime.Add(61 * time.Second)
	require.Equal(t, retryableErr, ObserveFailureThreshold(channelError, retryableErr))
}
