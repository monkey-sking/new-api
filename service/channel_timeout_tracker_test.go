package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestObserveTimeoutThreshold(t *testing.T) {
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = false
		common.AutomaticDisableConsecutiveTimeoutCount = 0
		common.AutomaticDisableThresholdWindowSeconds = 300
		nowFunc = time.Now
		channelTimeoutTracker.Lock()
		channelTimeoutTracker.events = map[string][]time.Time{}
		channelTimeoutTracker.Unlock()
	})

	common.AutomaticDisableChannelEnabled = true
	common.AutomaticDisableConsecutiveTimeoutCount = 2
	common.AutomaticDisableThresholdWindowSeconds = 60
	currentTime := time.Unix(1_700_000_000, 0)
	nowFunc = func() time.Time {
		return currentTime
	}

	channelError := types.ChannelError{
		ChannelId:   7,
		ChannelName: "test-channel",
		AutoBan:     true,
	}

	timeoutErr := types.NewOpenAIError(
		http.ErrHandlerTimeout,
		types.ErrorCodeBadResponseStatusCode,
		http.StatusGatewayTimeout,
	)

	first := ObserveTimeoutThreshold(channelError, timeoutErr)
	require.Equal(t, timeoutErr, first)

	currentTime = currentTime.Add(30 * time.Second)
	second := ObserveTimeoutThreshold(channelError, timeoutErr)
	require.NotNil(t, second)
	require.Equal(t, types.ErrorCodeChannelTimeoutThresholdExceeded, second.GetErrorCode())
	require.Equal(t, http.StatusRequestTimeout, second.StatusCode)
}

func TestObserveTimeoutThresholdKeepsWindowAcrossSuccessAndNonTimeout(t *testing.T) {
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = false
		common.AutomaticDisableConsecutiveTimeoutCount = 0
		common.AutomaticDisableThresholdWindowSeconds = 300
		nowFunc = time.Now
		channelTimeoutTracker.Lock()
		channelTimeoutTracker.events = map[string][]time.Time{}
		channelTimeoutTracker.Unlock()
	})

	common.AutomaticDisableChannelEnabled = true
	common.AutomaticDisableConsecutiveTimeoutCount = 2
	common.AutomaticDisableThresholdWindowSeconds = 60
	currentTime := time.Unix(1_700_000_100, 0)
	nowFunc = func() time.Time {
		return currentTime
	}

	channelError := types.ChannelError{
		ChannelId:   9,
		ChannelName: "test-channel",
		AutoBan:     true,
	}

	timeoutErr := types.NewOpenAIError(
		http.ErrHandlerTimeout,
		types.ErrorCodeBadResponseStatusCode,
		http.StatusGatewayTimeout,
	)
	nonTimeoutErr := types.NewOpenAIError(
		http.ErrNotSupported,
		types.ErrorCodeBadResponseBody,
		http.StatusBadGateway,
	)

	ObserveTimeoutThreshold(channelError, timeoutErr)
	ObserveTimeoutThreshold(channelError, nonTimeoutErr)
	currentTime = currentTime.Add(20 * time.Second)
	ObserveTimeoutThreshold(channelError, nil)
	currentTime = currentTime.Add(20 * time.Second)

	second := ObserveTimeoutThreshold(channelError, timeoutErr)
	require.NotNil(t, second)
	require.Equal(t, types.ErrorCodeChannelTimeoutThresholdExceeded, second.GetErrorCode())

	currentTime = currentTime.Add(61 * time.Second)
	require.Equal(t, timeoutErr, ObserveTimeoutThreshold(channelError, timeoutErr))
}
