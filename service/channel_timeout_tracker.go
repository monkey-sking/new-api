package service

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

var channelTimeoutTracker = newChannelWindowTracker()

func channelTimeoutTrackerKey(channelError types.ChannelError) string {
	if channelError.UsingKey != "" {
		return fmt.Sprintf("%d:%s", channelError.ChannelId, channelError.UsingKey)
	}
	return fmt.Sprintf("%d", channelError.ChannelId)
}

func isTimeoutLikeError(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	switch err.StatusCode {
	case http.StatusRequestTimeout, http.StatusGatewayTimeout, 524:
		return true
	}
	lowerMessage := strings.ToLower(err.Error())
	timeoutSignals := []string{
		"timeout",
		"timed out",
		"deadline exceeded",
		"i/o timeout",
		"client.timeout exceeded",
		"streaming timeout",
		"ping data send timeout",
	}
	for _, signal := range timeoutSignals {
		if strings.Contains(lowerMessage, signal) {
			return true
		}
	}
	return false
}

func clearChannelTimeoutWindow(channelError types.ChannelError) {
	key := channelTimeoutTrackerKey(channelError)
	channelTimeoutTracker.clear(key)
}

func pruneChannelTimeoutWindow(channelError types.ChannelError) {
	key := channelTimeoutTrackerKey(channelError)
	channelTimeoutTracker.prune(key, channelDisableThresholdWindow())
}

func ObserveTimeoutThreshold(channelError types.ChannelError, err *types.NewAPIError) *types.NewAPIError {
	if err == nil {
		pruneChannelTimeoutWindow(channelError)
		return nil
	}

	if !common.AutomaticDisableChannelEnabled || !channelError.AutoBan || common.AutomaticDisableConsecutiveTimeoutCount <= 0 {
		return err
	}

	if ShouldDisableChannel(channelError.ChannelType, err) {
		clearChannelTimeoutWindow(channelError)
		return err
	}

	if !isTimeoutLikeError(err) {
		pruneChannelTimeoutWindow(channelError)
		return err
	}

	key := channelTimeoutTrackerKey(channelError)
	count := channelTimeoutTracker.observe(key, channelDisableThresholdWindow())

	if count < common.AutomaticDisableConsecutiveTimeoutCount {
		return err
	}

	channelTimeoutTracker.clear(key)
	reason := fmt.Sprintf("channel timed out %d times within the configured window", count)
	return types.NewOpenAIError(
		fmt.Errorf("%s", reason),
		types.ErrorCodeChannelTimeoutThresholdExceeded,
		http.StatusRequestTimeout,
	)
}
