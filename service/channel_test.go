package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestShouldDisableChannelOnTooManyRequests(t *testing.T) {
	origAutoDisable := common.AutomaticDisableChannelEnabled
	origRanges := operation_setting.AutomaticDisableStatusCodeRanges
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = origAutoDisable
		operation_setting.AutomaticDisableStatusCodeRanges = origRanges
	})

	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 401, End: 401}}

	err := types.NewOpenAIError(errors.New("rate limit exceeded"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	require.True(t, ShouldDisableChannel(constant.ChannelTypeOpenAI, err))
}

func TestShouldNotDisableChannelOnBadRequestWithoutMatchingRules(t *testing.T) {
	origAutoDisable := common.AutomaticDisableChannelEnabled
	origRanges := operation_setting.AutomaticDisableStatusCodeRanges
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = origAutoDisable
		operation_setting.AutomaticDisableStatusCodeRanges = origRanges
	})

	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 401, End: 401}}

	err := types.NewOpenAIError(errors.New("invalid request"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
	require.False(t, ShouldDisableChannel(constant.ChannelTypeOpenAI, err))
}

func TestShouldDisableChannelOnThresholdErrors(t *testing.T) {
	origAutoDisable := common.AutomaticDisableChannelEnabled
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = origAutoDisable
	})

	common.AutomaticDisableChannelEnabled = true

	err := types.NewOpenAIError(
		errors.New("channel reached failure threshold"),
		types.ErrorCodeChannelFailureThresholdExceeded,
		http.StatusBadGateway,
	)
	require.True(t, ShouldDisableChannel(constant.ChannelTypeOpenAI, err))
}

func TestShouldDisableChannelOnPaymentRequired(t *testing.T) {
	origAutoDisable := common.AutomaticDisableChannelEnabled
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = origAutoDisable
	})

	common.AutomaticDisableChannelEnabled = true

	err := types.NewOpenAIError(
		errors.New("payment required"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusPaymentRequired,
	)
	require.True(t, ShouldDisableChannel(constant.ChannelTypeOpenAI, err))
}

func TestShouldDisableChannelOnBuiltinInsufficientBalanceSignals(t *testing.T) {
	origAutoDisable := common.AutomaticDisableChannelEnabled
	origRanges := operation_setting.AutomaticDisableStatusCodeRanges
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = origAutoDisable
		operation_setting.AutomaticDisableStatusCodeRanges = origRanges
	})

	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableStatusCodeRanges = nil

	err := types.NewOpenAIError(
		errors.New("insufficient balance for this account"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
	)
	require.True(t, ShouldDisableChannel(constant.ChannelTypeOpenAI, err))
}
