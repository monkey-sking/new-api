package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestStoredStatusReasonTriggersAutoDisable(t *testing.T) {
	origAutoDisable := common.AutomaticDisableChannelEnabled
	origRanges := operation_setting.AutomaticDisableStatusCodeRanges
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = origAutoDisable
		operation_setting.AutomaticDisableStatusCodeRanges = origRanges
	})

	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableStatusCodeRanges = []operation_setting.StatusCodeRange{
		{Start: 401, End: 401},
		{Start: 502, End: 504},
		{Start: 530, End: 530},
	}

	require.True(t, ShouldDisableChannelFromStoredReason(constant.ChannelTypeOpenAI, "status_code=503, No available channel for model gpt-5.4 under group default (distributor)"))
	require.True(t, ShouldDisableChannelFromStoredReason(constant.ChannelTypeOpenAI, "status_code=530, The host is configured as a Cloudflare Tunnel, but Cloudflare is currently unable to reach it."))
	require.False(t, ShouldDisableChannelFromStoredReason(constant.ChannelTypeOpenAI, "status_code=400, invalid request body"))
}

func TestDisableStaleChannelsByStoredReason(t *testing.T) {
	truncate(t)

	origAutoDisable := common.AutomaticDisableChannelEnabled
	origRanges := operation_setting.AutomaticDisableStatusCodeRanges
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = origAutoDisable
		operation_setting.AutomaticDisableStatusCodeRanges = origRanges
	})

	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableStatusCodeRanges = []operation_setting.StatusCodeRange{
		{Start: 401, End: 401},
		{Start: 502, End: 504},
		{Start: 530, End: 530},
	}

	autoBan := 1
	channel := &model.Channel{
		Id:        360,
		Type:      constant.ChannelTypeOpenAI,
		Name:      "stale-bad-channel",
		Key:       "sk-test",
		Status:    common.ChannelStatusEnabled,
		AutoBan:   &autoBan,
		OtherInfo: `{"status_reason":"status_code=503, No available channel for model gpt-5.4 under group default (distributor)"}`,
	}
	require.NoError(t, model.DB.Create(channel).Error)

	disabledCount := DisableStaleChannelsByStoredReason()
	require.Equal(t, 1, disabledCount)

	var updated model.Channel
	require.NoError(t, model.DB.First(&updated, "id = ?", channel.Id).Error)
	require.Equal(t, common.ChannelStatusAutoDisabled, updated.Status)
	require.Contains(t, updated.OtherInfo, "No available channel for model gpt-5.4 under group default")
}

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

func TestShouldDisableChannelOnDefaultHardFailureStatusCodes(t *testing.T) {
	origAutoDisable := common.AutomaticDisableChannelEnabled
	origRanges := operation_setting.AutomaticDisableStatusCodeRanges
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = origAutoDisable
		operation_setting.AutomaticDisableStatusCodeRanges = origRanges
	})

	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableStatusCodeRanges = []operation_setting.StatusCodeRange{
		{Start: 401, End: 401},
		{Start: 502, End: 504},
		{Start: 530, End: 530},
	}

	tests := []struct {
		name       string
		statusCode int
	}{
		{name: "bad gateway", statusCode: http.StatusBadGateway},
		{name: "service unavailable", statusCode: http.StatusServiceUnavailable},
		{name: "gateway timeout", statusCode: http.StatusGatewayTimeout},
		{name: "cloudflare tunnel unreachable", statusCode: 530},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := types.NewOpenAIError(
				errors.New("upstream hard failure"),
				types.ErrorCodeBadResponseStatusCode,
				tt.statusCode,
			)
			require.True(t, ShouldDisableChannel(constant.ChannelTypeOpenAI, err))
		})
	}
}

func TestShouldDisableChannelOnBuiltinHardFailureSignals(t *testing.T) {
	origAutoDisable := common.AutomaticDisableChannelEnabled
	origRanges := operation_setting.AutomaticDisableStatusCodeRanges
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = origAutoDisable
		operation_setting.AutomaticDisableStatusCodeRanges = origRanges
	})

	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableStatusCodeRanges = nil

	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "cloudflare tunnel unreachable",
			message: "The host is configured as a Cloudflare Tunnel, but Cloudflare is currently unable to reach it.",
		},
		{
			name:    "malformed upstream url",
			message: "new request failed: parse \" https://yyds.215.im/v1/responses\": first path segment in URL cannot contain colon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := types.NewOpenAIError(
				errors.New(tt.message),
				types.ErrorCodeBadResponseStatusCode,
				http.StatusInternalServerError,
			)
			require.True(t, ShouldDisableChannel(constant.ChannelTypeOpenAI, err))
		})
	}
}

func TestShouldDisableChannelOnUpstreamPoolExhaustionSignals(t *testing.T) {
	origAutoDisable := common.AutomaticDisableChannelEnabled
	origRanges := operation_setting.AutomaticDisableStatusCodeRanges
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = origAutoDisable
		operation_setting.AutomaticDisableStatusCodeRanges = origRanges
	})

	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 401, End: 401}}

	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "account pool exhausted",
			message: "No available accounts: no available accounts",
		},
		{
			name:    "model pool exhausted",
			message: "No available channel for model gpt-5.4 under group default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := types.NewOpenAIError(
				errors.New(tt.message),
				types.ErrorCodeBadResponseStatusCode,
				http.StatusServiceUnavailable,
			)
			require.True(t, ShouldDisableChannel(constant.ChannelTypeOpenAI, err))
		})
	}
}
