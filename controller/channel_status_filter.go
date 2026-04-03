package controller

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	channelStatusFilterAll                 = "all"
	channelStatusFilterEnabled             = "enabled"
	channelStatusFilterDisabled            = "disabled"
	channelStatusFilterManualDisabled      = "manual_disabled"
	channelStatusFilterInsufficientBalance = "insufficient_balance"
	channelStatusFilterServerError         = "server_error"
	channelStatusFilterBalanceCheckFailed  = "balance_check_failed"
)

var insufficientBalanceSignals = []string{
	"insufficient balance",
	"insufficient balances",
	"insufficient credit",
	"insufficient credits",
	"credit balance is too low",
	"your credit balance is too low",
	"out of credits",
	"no credits available",
	"not enough balance",
	"quota not enough",
	"payment required",
	"billing quota exceeded",
	"billing hard limit",
	"余额不足",
	"额度不足",
	"余额已用尽",
	"额度已用尽",
	"欠费",
}

func parseStatusFilter(statusParam string) string {
	switch strings.ToLower(strings.TrimSpace(statusParam)) {
	case "", channelStatusFilterAll:
		return channelStatusFilterAll
	case "1", channelStatusFilterEnabled:
		return channelStatusFilterEnabled
	case "0", "2", "3", channelStatusFilterDisabled:
		return channelStatusFilterDisabled
	case channelStatusFilterManualDisabled:
		return channelStatusFilterManualDisabled
	case channelStatusFilterInsufficientBalance:
		return channelStatusFilterInsufficientBalance
	case channelStatusFilterServerError:
		return channelStatusFilterServerError
	case channelStatusFilterBalanceCheckFailed:
		return channelStatusFilterBalanceCheckFailed
	default:
		return channelStatusFilterAll
	}
}

func isDerivedStatusFilter(statusFilter string) bool {
	switch statusFilter {
	case channelStatusFilterInsufficientBalance, channelStatusFilterServerError,
		channelStatusFilterBalanceCheckFailed:
		return true
	default:
		return false
	}
}

func filterChannelsByStatus(channels []*model.Channel, statusFilter string) []*model.Channel {
	if statusFilter == channelStatusFilterAll {
		return channels
	}

	filtered := make([]*model.Channel, 0, len(channels))
	for _, ch := range channels {
		if matchesChannelStatusFilter(ch, statusFilter) {
			filtered = append(filtered, ch)
		}
	}
	return filtered
}

func matchesChannelStatusFilter(ch *model.Channel, statusFilter string) bool {
	if ch == nil {
		return false
	}

	switch statusFilter {
	case channelStatusFilterEnabled:
		return ch.Status == common.ChannelStatusEnabled
	case channelStatusFilterDisabled:
		return ch.Status != common.ChannelStatusEnabled
	case channelStatusFilterManualDisabled:
		return ch.Status == common.ChannelStatusManuallyDisabled
	case channelStatusFilterInsufficientBalance:
		return ch.Status == common.ChannelStatusAutoDisabled &&
			isInsufficientBalanceReason(extractChannelStatusReason(ch))
	case channelStatusFilterServerError:
		return ch.Status == common.ChannelStatusAutoDisabled &&
			!isInsufficientBalanceReason(extractChannelStatusReason(ch))
	case channelStatusFilterBalanceCheckFailed:
		return ch.GetBalanceCheck().Status == model.ChannelBalanceCheckStatusFailed
	default:
		return true
	}
}

func extractChannelStatusReason(ch *model.Channel) string {
	if ch == nil || ch.OtherInfo == "" {
		return ""
	}

	info := map[string]any{}
	err := json.Unmarshal([]byte(ch.OtherInfo), &info)
	if err != nil {
		return ""
	}

	reason, _ := info["status_reason"].(string)
	return reason
}

func isInsufficientBalanceReason(reason string) bool {
	lowerReason := strings.ToLower(strings.TrimSpace(reason))
	if lowerReason == "" {
		return false
	}

	for _, signal := range insufficientBalanceSignals {
		if strings.Contains(lowerReason, signal) {
			return true
		}
	}
	return false
}
