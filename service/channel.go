package service

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

var builtinInsufficientBalanceSignals = []string{
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

var builtinUnavailablePoolSignals = []string{
	"no available account",
	"no available accounts",
	"no available channel",
	"no available channels",
}

var builtinHardFailureSignals = []string{
	"cloudflare tunnel",
	"unable to reach it",
	"first path segment in url cannot contain colon",
	"missing protocol scheme",
}

func parseStoredStatusReason(reason string) *types.NewAPIError {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return nil
	}

	statusCode := 0
	message := trimmed
	if strings.HasPrefix(trimmed, "status_code=") {
		rest := strings.TrimPrefix(trimmed, "status_code=")
		parts := strings.SplitN(rest, ",", 2)
		if code, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
			statusCode = code
		}
		if len(parts) == 2 {
			message = strings.TrimSpace(parts[1])
		} else if statusCode != 0 {
			message = http.StatusText(statusCode)
		}
	}

	if message == "" {
		message = trimmed
	}

	return types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponseStatusCode, statusCode)
}

func ShouldDisableChannelFromStoredReason(channelType int, reason string) bool {
	return ShouldDisableChannel(channelType, parseStoredStatusReason(reason))
}

func DisableStaleChannelsByStoredReason() int {
	if !common.AutomaticDisableChannelEnabled {
		return 0
	}

	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to load channels for stale auto-disable sweep: %v", err))
		return 0
	}

	disabledCount := 0
	for _, channel := range channels {
		if channel == nil || channel.Status != common.ChannelStatusEnabled || !channel.GetAutoBan() {
			continue
		}

		info := channel.GetOtherInfo()
		reason, _ := info["status_reason"].(string)
		if !ShouldDisableChannelFromStoredReason(channel.Type, reason) {
			continue
		}

		if model.UpdateChannelStatus(channel.Id, "", common.ChannelStatusAutoDisabled, reason) {
			disabledCount++
			common.SysLog(fmt.Sprintf("stale auto-disable sweep disabled channel #%d (%s): %s", channel.Id, channel.Name, reason))
		}
	}

	return disabledCount
}

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
	}
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

func ShouldDisableChannel(channelType int, err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if err.StatusCode == http.StatusPaymentRequired {
		return true
	}
	if err.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}
	//if err.StatusCode == http.StatusUnauthorized {
	//	return true
	//}
	if err.StatusCode == http.StatusForbidden {
		switch channelType {
		case constant.ChannelTypeGemini:
			return true
		}
	}
	oaiErr := err.ToOpenAIError()
	switch oaiErr.Code {
	case "invalid_api_key":
		return true
	case "account_deactivated":
		return true
	case "billing_not_active":
		return true
	case "pre_consume_token_quota_failed":
		return true
	case "Arrearage":
		return true
	}
	switch oaiErr.Type {
	case "insufficient_quota":
		return true
	case "insufficient_user_quota":
		return true
	// https://docs.anthropic.com/claude/reference/errors
	case "authentication_error":
		return true
	case "permission_error":
		return true
	case "forbidden":
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	for _, signal := range builtinInsufficientBalanceSignals {
		if strings.Contains(lowerMessage, signal) {
			return true
		}
	}
	for _, signal := range builtinUnavailablePoolSignals {
		if strings.Contains(lowerMessage, signal) {
			return true
		}
	}
	for _, signal := range builtinHardFailureSignals {
		if strings.Contains(lowerMessage, signal) {
			return true
		}
	}
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	return search
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
