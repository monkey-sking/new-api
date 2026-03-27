package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

var channelFailureTracker = newChannelWindowTracker()

func clearChannelFailureWindow(channelError types.ChannelError) {
	key := channelTimeoutTrackerKey(channelError)
	channelFailureTracker.clear(key)
}

func pruneChannelFailureWindow(channelError types.ChannelError) {
	key := channelTimeoutTrackerKey(channelError)
	channelFailureTracker.prune(key, channelDisableThresholdWindow())
}

func shouldTrackWindowFailure(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if types.IsChannelError(err) || types.IsSkipRetryError(err) {
		return false
	}
	code := err.StatusCode
	if code >= 200 && code < 300 {
		return false
	}
	if operation_setting.IsAlwaysSkipRetryCode(err.GetErrorCode()) {
		return false
	}
	if code < 100 || code > 599 {
		return true
	}
	return operation_setting.ShouldRetryByStatusCode(code)
}

func ObserveFailureThreshold(channelError types.ChannelError, err *types.NewAPIError) *types.NewAPIError {
	if err == nil {
		pruneChannelFailureWindow(channelError)
		return nil
	}

	if !common.AutomaticDisableChannelEnabled || !channelError.AutoBan || common.AutomaticDisableConsecutiveFailureCount <= 0 {
		return err
	}

	if ShouldDisableChannel(channelError.ChannelType, err) {
		clearChannelFailureWindow(channelError)
		return err
	}

	if !shouldTrackWindowFailure(err) {
		pruneChannelFailureWindow(channelError)
		return err
	}

	key := channelTimeoutTrackerKey(channelError)
	count := channelFailureTracker.observe(key, channelDisableThresholdWindow())

	if count < common.AutomaticDisableConsecutiveFailureCount {
		return err
	}

	channelFailureTracker.clear(key)
	return types.NewOpenAIError(
		errors.New(err.Error()),
		types.ErrorCodeChannelFailureThresholdExceeded,
		err.StatusCode,
	)
}
