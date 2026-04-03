package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestParseStatusFilter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty defaults to all", input: "", want: channelStatusFilterAll},
		{name: "enabled numeric", input: "1", want: channelStatusFilterEnabled},
		{name: "disabled numeric", input: "0", want: channelStatusFilterDisabled},
		{name: "manual disabled", input: "manual_disabled", want: channelStatusFilterManualDisabled},
		{name: "insufficient balance", input: "insufficient_balance", want: channelStatusFilterInsufficientBalance},
		{name: "server error", input: "server_error", want: channelStatusFilterServerError},
		{name: "balance check failed", input: "balance_check_failed", want: channelStatusFilterBalanceCheckFailed},
		{name: "unknown falls back", input: "weird", want: channelStatusFilterAll},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseStatusFilter(tt.input); got != tt.want {
				t.Fatalf("parseStatusFilter(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchesChannelStatusFilter(t *testing.T) {
	enabled := &model.Channel{Status: common.ChannelStatusEnabled}
	manualDisabled := &model.Channel{Status: common.ChannelStatusManuallyDisabled}
	autoDisabledLowBalance := &model.Channel{
		Status:    common.ChannelStatusAutoDisabled,
		OtherInfo: `{"status_reason":"余额不足，停止服务"}`,
	}
	autoDisabledServerError := &model.Channel{
		Status:    common.ChannelStatusAutoDisabled,
		OtherInfo: `{"status_reason":"upstream 503 temporarily unavailable"}`,
	}
	balanceCheckFailed := &model.Channel{
		Status: common.ChannelStatusEnabled,
		OtherInfo: `{
			"balance_check_status":"failed",
			"balance_check_reason":"request timeout"
		}`,
	}

	tests := []struct {
		name   string
		filter string
		ch     *model.Channel
		want   bool
	}{
		{name: "enabled matches enabled filter", filter: channelStatusFilterEnabled, ch: enabled, want: true},
		{name: "manual disabled matches manual filter", filter: channelStatusFilterManualDisabled, ch: manualDisabled, want: true},
		{name: "low balance matches insufficient filter", filter: channelStatusFilterInsufficientBalance, ch: autoDisabledLowBalance, want: true},
		{name: "server error does not match insufficient filter", filter: channelStatusFilterInsufficientBalance, ch: autoDisabledServerError, want: false},
		{name: "server error matches server filter", filter: channelStatusFilterServerError, ch: autoDisabledServerError, want: true},
		{name: "low balance does not match server filter", filter: channelStatusFilterServerError, ch: autoDisabledLowBalance, want: false},
		{name: "balance check failed matches balance filter", filter: channelStatusFilterBalanceCheckFailed, ch: balanceCheckFailed, want: true},
		{name: "enabled does not match disabled filter", filter: channelStatusFilterDisabled, ch: enabled, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesChannelStatusFilter(tt.ch, tt.filter); got != tt.want {
				t.Fatalf("matchesChannelStatusFilter(%q) = %v, want %v", tt.filter, got, tt.want)
			}
		})
	}
}
