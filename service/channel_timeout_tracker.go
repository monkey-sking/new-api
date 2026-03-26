package service

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

var channelTimeoutTracker = struct {
	sync.Mutex
	counts map[string]int
}{
	counts: map[string]int{},
}

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

func resetChannelTimeoutCount(channelError types.ChannelError) {
	key := channelTimeoutTrackerKey(channelError)
	channelTimeoutTracker.Lock()
	delete(channelTimeoutTracker.counts, key)
	channelTimeoutTracker.Unlock()
}

func ObserveTimeoutThreshold(channelError types.ChannelError, err *types.NewAPIError) *types.NewAPIError {
	if err == nil {
		resetChannelTimeoutCount(channelError)
		return nil
	}

	if !common.AutomaticDisableChannelEnabled || !channelError.AutoBan || common.AutomaticDisableConsecutiveTimeoutCount <= 0 {
		return err
	}

	if !isTimeoutLikeError(err) {
		resetChannelTimeoutCount(channelError)
		return err
	}

	key := channelTimeoutTrackerKey(channelError)

	channelTimeoutTracker.Lock()
	channelTimeoutTracker.counts[key]++
	count := channelTimeoutTracker.counts[key]
	if count >= common.AutomaticDisableConsecutiveTimeoutCount {
		delete(channelTimeoutTracker.counts, key)
	}
	channelTimeoutTracker.Unlock()

	if count < common.AutomaticDisableConsecutiveTimeoutCount {
		return err
	}

	reason := fmt.Sprintf("channel timed out %d consecutive times", count)
	return types.NewOpenAIError(
		fmt.Errorf("%s", reason),
		types.ErrorCodeChannelTimeoutThresholdExceeded,
		http.StatusRequestTimeout,
	)
}
