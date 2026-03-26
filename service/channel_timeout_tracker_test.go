package service

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestObserveTimeoutThreshold(t *testing.T) {
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = false
		common.AutomaticDisableConsecutiveTimeoutCount = 0
		channelTimeoutTracker.Lock()
		channelTimeoutTracker.counts = map[string]int{}
		channelTimeoutTracker.Unlock()
	})

	common.AutomaticDisableChannelEnabled = true
	common.AutomaticDisableConsecutiveTimeoutCount = 2

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

	second := ObserveTimeoutThreshold(channelError, timeoutErr)
	require.NotNil(t, second)
	require.Equal(t, types.ErrorCodeChannelTimeoutThresholdExceeded, second.GetErrorCode())
	require.Equal(t, http.StatusRequestTimeout, second.StatusCode)
}

func TestObserveTimeoutThresholdResetsOnSuccessAndNonTimeout(t *testing.T) {
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = false
		common.AutomaticDisableConsecutiveTimeoutCount = 0
		channelTimeoutTracker.Lock()
		channelTimeoutTracker.counts = map[string]int{}
		channelTimeoutTracker.Unlock()
	})

	common.AutomaticDisableChannelEnabled = true
	common.AutomaticDisableConsecutiveTimeoutCount = 2

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
	ObserveTimeoutThreshold(channelError, nil)

	firstAfterReset := ObserveTimeoutThreshold(channelError, timeoutErr)
	require.Equal(t, timeoutErr, firstAfterReset)

	ObserveTimeoutThreshold(channelError, nonTimeoutErr)
	firstAfterNonTimeoutReset := ObserveTimeoutThreshold(channelError, timeoutErr)
	require.Equal(t, timeoutErr, firstAfterNonTimeoutReset)
}
