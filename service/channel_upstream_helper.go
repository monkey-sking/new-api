package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

const (
	UpstreamPlatformNewAPI    = "new-api"
	UpstreamPlatformOneAPI    = "one-api"
	UpstreamPlatformOneHub    = "one-hub"
	UpstreamPlatformDoneHub   = "done-hub"
	UpstreamPlatformVeloera   = "veloera"
	UpstreamPlatformAnyRouter = "anyrouter"
	UpstreamPlatformSub2API   = "sub2api"
	UpstreamPlatformCLIProxy  = "cliproxyapi"
	UpstreamPlatformOpenAI    = "openai"
	UpstreamPlatformClaude    = "claude"
	UpstreamPlatformGemini    = "gemini"
	UpstreamPlatformOpenRouter = "openrouter"
)

type UpstreamChannelPreset struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Description          string   `json:"description,omitempty"`
	Platform             string   `json:"platform"`
	SuggestedChannelType int      `json:"suggested_channel_type"`
	BaseURLHint          string   `json:"base_url_hint,omitempty"`
	CheckinSupported     bool     `json:"checkin_supported"`
	HeaderAliases        []string `json:"header_aliases,omitempty"`

	matchKeywords []string
	matchHosts    []string
}

type UpstreamChannelDetectionResult struct {
	Matched              bool                   `json:"matched"`
	Platform             string                 `json:"platform,omitempty"`
	SuggestedChannelType int                    `json:"suggested_channel_type,omitempty"`
	NormalizedBaseURL    string                 `json:"normalized_base_url,omitempty"`
	Preset               *UpstreamChannelPreset `json:"preset,omitempty"`
}

var upstreamChannelPresets = []UpstreamChannelPreset{
	{
		ID:                   "new-api",
		Name:                 "New API 面板",
		Description:          "New API 家族站点，支持 access token + user id 签到",
		Platform:             UpstreamPlatformNewAPI,
		SuggestedChannelType: constant.ChannelTypeOpenAI,
		CheckinSupported:     true,
		HeaderAliases:        []string{"New-Api-User", "New-API-User", "Veloera-User", "voapi-user", "User-id", "Rix-Api-User", "neo-api-user"},
		matchKeywords:        []string{"new-api", "newapi", "neo-api", "rix-api", "super-api", "voapi", "vo-api"},
	},
	{
		ID:                   "one-api",
		Name:                 "One API 面板",
		Description:          "经典 OpenAI 聚合面板",
		Platform:             UpstreamPlatformOneAPI,
		SuggestedChannelType: constant.ChannelTypeOpenAI,
		CheckinSupported:     true,
		matchKeywords:        []string{"one-api", "oneapi"},
	},
	{
		ID:                   "done-hub",
		Name:                 "DoneHub 面板",
		Description:          "通常不暴露签到接口",
		Platform:             UpstreamPlatformDoneHub,
		SuggestedChannelType: constant.ChannelTypeOpenAI,
		CheckinSupported:     false,
		matchKeywords:        []string{"done-hub", "donehub"},
	},
	{
		ID:                   "one-hub",
		Name:                 "OneHub 面板",
		Description:          "OneHub 风格面板",
		Platform:             UpstreamPlatformOneHub,
		SuggestedChannelType: constant.ChannelTypeOpenAI,
		CheckinSupported:     true,
		matchKeywords:        []string{"one-hub", "onehub"},
	},
	{
		ID:                   "veloera",
		Name:                 "Veloera 面板",
		Description:          "Veloera 风格面板，签到走 /api/user/checkin",
		Platform:             UpstreamPlatformVeloera,
		SuggestedChannelType: constant.ChannelTypeOpenAI,
		CheckinSupported:     true,
		HeaderAliases:        []string{"Veloera-User", "New-API-User", "User-id"},
		matchKeywords:        []string{"veloera"},
	},
	{
		ID:                   "anyrouter",
		Name:                 "AnyRouter 面板",
		Description:          "AnyRouter / NewAPI 家族站点",
		Platform:             UpstreamPlatformAnyRouter,
		SuggestedChannelType: constant.ChannelTypeOpenAI,
		CheckinSupported:     true,
		matchKeywords:        []string{"anyrouter"},
	},
	{
		ID:                   "sub2api",
		Name:                 "Sub2API 面板",
		Description:          "订阅制中转平台，通常不支持签到",
		Platform:             UpstreamPlatformSub2API,
		SuggestedChannelType: constant.ChannelTypeOpenAI,
		CheckinSupported:     false,
		matchKeywords:        []string{"sub2api"},
	},
	{
		ID:                   "cliproxyapi",
		Name:                 "CLIProxyAPI / CPA",
		Description:          "推荐按 API Key 使用，不当作可签到面板",
		Platform:             UpstreamPlatformCLIProxy,
		SuggestedChannelType: constant.ChannelTypeOpenAI,
		CheckinSupported:     false,
		matchKeywords:        []string{"cliproxyapi", "cpa"},
	},
	{
		ID:                   "openai-official",
		Name:                 "OpenAI 官方",
		Description:          "官方 OpenAI 兼容接口",
		Platform:             UpstreamPlatformOpenAI,
		SuggestedChannelType: constant.ChannelTypeOpenAI,
		BaseURLHint:          "https://api.openai.com",
		CheckinSupported:     false,
		matchHosts:           []string{"api.openai.com"},
	},
	{
		ID:                   "claude-official",
		Name:                 "Anthropic 官方",
		Description:          "Anthropic Claude 官方接口",
		Platform:             UpstreamPlatformClaude,
		SuggestedChannelType: constant.ChannelTypeAnthropic,
		BaseURLHint:          "https://api.anthropic.com",
		CheckinSupported:     false,
		matchHosts:           []string{"api.anthropic.com"},
	},
	{
		ID:                   "gemini-official",
		Name:                 "Gemini 官方",
		Description:          "Google Gemini 官方接口",
		Platform:             UpstreamPlatformGemini,
		SuggestedChannelType: constant.ChannelTypeGemini,
		BaseURLHint:          "https://generativelanguage.googleapis.com",
		CheckinSupported:     false,
		matchHosts:           []string{"generativelanguage.googleapis.com"},
	},
	{
		ID:                   "openrouter-official",
		Name:                 "OpenRouter 官方",
		Description:          "OpenRouter 官方兼容接口",
		Platform:             UpstreamPlatformOpenRouter,
		SuggestedChannelType: constant.ChannelTypeOpenRouter,
		BaseURLHint:          "https://openrouter.ai/api",
		CheckinSupported:     false,
		matchHosts:           []string{"openrouter.ai"},
	},
}

func ListUpstreamChannelPresets() []UpstreamChannelPreset {
	result := make([]UpstreamChannelPreset, 0, len(upstreamChannelPresets))
	for _, preset := range upstreamChannelPresets {
		result = append(result, sanitizeUpstreamPreset(preset))
	}
	return result
}

func sanitizeUpstreamPreset(preset UpstreamChannelPreset) UpstreamChannelPreset {
	preset.matchHosts = nil
	preset.matchKeywords = nil
	return preset
}

func DetectUpstreamChannelPreset(rawBaseURL string, channelType int) UpstreamChannelDetectionResult {
	normalizedBaseURL := normalizeUpstreamBaseURL(rawBaseURL)
	if normalizedBaseURL == "" {
		return UpstreamChannelDetectionResult{}
	}

	host, normalizedLower := upstreamBaseURLMatchInput(normalizedBaseURL)
	if preset := matchUpstreamPreset(host, normalizedLower); preset != nil {
		return buildUpstreamDetectionResult(*preset, normalizedBaseURL)
	}

	if preset := probeUpstreamPresetFromStatus(normalizedBaseURL); preset != nil {
		return buildUpstreamDetectionResult(*preset, normalizedBaseURL)
	}

	switch channelType {
	case constant.ChannelTypeOpenAI:
		preset := sanitizeUpstreamPreset(upstreamChannelPresets[8])
		return UpstreamChannelDetectionResult{
			Matched:              true,
			Platform:             preset.Platform,
			SuggestedChannelType: preset.SuggestedChannelType,
			NormalizedBaseURL:    normalizedBaseURL,
			Preset:               &preset,
		}
	case constant.ChannelTypeAnthropic:
		preset := sanitizeUpstreamPreset(upstreamChannelPresets[9])
		return UpstreamChannelDetectionResult{
			Matched:              true,
			Platform:             preset.Platform,
			SuggestedChannelType: preset.SuggestedChannelType,
			NormalizedBaseURL:    normalizedBaseURL,
			Preset:               &preset,
		}
	case constant.ChannelTypeGemini:
		preset := sanitizeUpstreamPreset(upstreamChannelPresets[10])
		return UpstreamChannelDetectionResult{
			Matched:              true,
			Platform:             preset.Platform,
			SuggestedChannelType: preset.SuggestedChannelType,
			NormalizedBaseURL:    normalizedBaseURL,
			Preset:               &preset,
		}
	case constant.ChannelTypeOpenRouter:
		preset := sanitizeUpstreamPreset(upstreamChannelPresets[11])
		return UpstreamChannelDetectionResult{
			Matched:              true,
			Platform:             preset.Platform,
			SuggestedChannelType: preset.SuggestedChannelType,
			NormalizedBaseURL:    normalizedBaseURL,
			Preset:               &preset,
		}
	default:
		return UpstreamChannelDetectionResult{NormalizedBaseURL: normalizedBaseURL}
	}
}

func buildUpstreamDetectionResult(preset UpstreamChannelPreset, normalizedBaseURL string) UpstreamChannelDetectionResult {
	sanitized := sanitizeUpstreamPreset(preset)
	return UpstreamChannelDetectionResult{
		Matched:              true,
		Platform:             sanitized.Platform,
		SuggestedChannelType: sanitized.SuggestedChannelType,
		NormalizedBaseURL:    normalizedBaseURL,
		Preset:               &sanitized,
	}
}

func matchUpstreamPreset(inputs ...string) *UpstreamChannelPreset {
	normalizedInputs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		trimmed := strings.ToLower(strings.TrimSpace(input))
		if trimmed == "" {
			continue
		}
		normalizedInputs = append(normalizedInputs, trimmed)
	}
	if len(normalizedInputs) == 0 {
		return nil
	}

	for _, preset := range upstreamChannelPresets {
		for _, candidate := range preset.matchHosts {
			for _, input := range normalizedInputs {
				if input == candidate || strings.Contains(input, candidate) {
					sanitized := sanitizeUpstreamPreset(preset)
					return &sanitized
				}
			}
		}
		for _, keyword := range preset.matchKeywords {
			for _, input := range normalizedInputs {
				if strings.Contains(input, keyword) {
					sanitized := sanitizeUpstreamPreset(preset)
					return &sanitized
				}
			}
		}
	}

	return nil
}

func probeUpstreamPresetFromStatus(normalizedBaseURL string) *UpstreamChannelPreset {
	req, err := http.NewRequest(http.MethodGet, normalizedBaseURL+"/api/status", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		return nil
	}

	payload := map[string]any{}
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil
	}

	return matchUpstreamPreset(extractUpstreamStatusHints(payload)...)
}

func extractUpstreamStatusHints(payload map[string]any) []string {
	if payload == nil {
		return nil
	}

	hints := make([]string, 0, 8)
	appendHint := func(value any) {
		trimmed := strings.TrimSpace(fmt.Sprintf("%v", value))
		if trimmed == "" || trimmed == "<nil>" {
			return
		}
		hints = append(hints, trimmed)
	}

	for _, key := range []string{"system_name", "systemName", "name", "brand", "message"} {
		appendHint(payload[key])
	}

	if data, ok := payload["data"].(map[string]any); ok {
		for _, key := range []string{"system_name", "systemName", "name", "brand", "title", "description"} {
			appendHint(data[key])
		}
	}

	return hints
}

func normalizeUpstreamBaseURL(rawBaseURL string) string {
	trimmed := strings.TrimSpace(rawBaseURL)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		trimmed = "https://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	parsed.Fragment = ""
	parsed.RawQuery = ""
	path := strings.TrimSuffix(parsed.Path, "/")
	switch path {
	case "/v1", "/api", "/api/v1":
		parsed.Path = ""
	default:
		parsed.Path = path
	}
	return strings.TrimSuffix(parsed.String(), "/")
}

func upstreamBaseURLMatchInput(normalizedBaseURL string) (string, string) {
	host := normalizedBaseURL
	if parsed, err := url.Parse(normalizedBaseURL); err == nil {
		host = strings.ToLower(parsed.Host)
	}
	return host, strings.ToLower(normalizedBaseURL)
}

func buildUpstreamCheckinHeaders(platform string, accessToken string, userID int) http.Header {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	if userID <= 0 {
		return headers
	}
	value := fmt.Sprintf("%d", userID)
	switch platform {
	case UpstreamPlatformNewAPI, UpstreamPlatformAnyRouter, UpstreamPlatformVeloera:
		headers.Set("New-Api-User", value)
		headers.Set("New-API-User", value)
		headers.Set("Veloera-User", value)
		headers.Set("voapi-user", value)
		headers.Set("User-id", value)
		headers.Set("Rix-Api-User", value)
		headers.Set("neo-api-user", value)
	default:
		headers.Set("New-Api-User", value)
		headers.Set("New-API-User", value)
		headers.Set("User-id", value)
	}
	return headers
}

func RunChannelUpstreamCheckin(channel *model.Channel, trigger string) (*model.ChannelUpstreamCheckin, error) {
	if channel == nil {
		return nil, errors.New("channel is nil")
	}
	settings := channel.GetOtherSettings()
	if !settings.UpstreamCheckinEnabled && trigger != "manual" {
		return &model.ChannelUpstreamCheckin{
			Status:    model.ChannelUpstreamCheckinStatusSkipped,
			Trigger:   trigger,
			Reason:    "未启用自动签到",
			CheckedAt: common.GetTimestamp(),
		}, nil
	}
	if strings.TrimSpace(settings.UpstreamCheckinAccessToken) == "" {
		return nil, errors.New("upstream checkin access token 未配置")
	}

	detection := UpstreamChannelDetectionResult{
		NormalizedBaseURL: normalizeUpstreamBaseURL(channel.GetBaseURL()),
	}
	platform := strings.TrimSpace(settings.UpstreamPlatform)
	if platform == "" {
		detection = DetectUpstreamChannelPreset(channel.GetBaseURL(), channel.Type)
		platform = detection.Platform
	}
	if platform == "" {
		return nil, errors.New("未识别上游平台，无法执行签到")
	}

	preset := findUpstreamPreset(platform)
	if preset == nil || !preset.CheckinSupported {
		return &model.ChannelUpstreamCheckin{
			Status:    model.ChannelUpstreamCheckinStatusSkipped,
			Trigger:   trigger,
			Reason:    "该上游平台当前不支持签到",
			CheckedAt: common.GetTimestamp(),
		}, nil
	}

	baseURL := detection.NormalizedBaseURL
	if baseURL == "" {
		baseURL = normalizeUpstreamBaseURL(channel.GetBaseURL())
	}
	if baseURL == "" {
		return nil, errors.New("渠道 base_url 不能为空")
	}

	client, err := NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/user/checkin", nil)
	if err != nil {
		return nil, err
	}
	for key, values := range buildUpstreamCheckinHeaders(platform, settings.UpstreamCheckinAccessToken, settings.UpstreamCheckinUserID) {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return &model.ChannelUpstreamCheckin{
			Status:    model.ChannelUpstreamCheckinStatusFailed,
			Trigger:   trigger,
			Reason:    err.Error(),
			CheckedAt: common.GetTimestamp(),
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &model.ChannelUpstreamCheckin{
			Status:    model.ChannelUpstreamCheckinStatusFailed,
			Trigger:   trigger,
			Reason:    fmt.Sprintf("status code: %d", resp.StatusCode),
			Message:   strings.TrimSpace(string(body)),
			CheckedAt: common.GetTimestamp(),
		}, nil
	}

	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			QuotaAwarded any `json:"quota_awarded"`
			Reward       any `json:"reward"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return &model.ChannelUpstreamCheckin{
			Status:    model.ChannelUpstreamCheckinStatusFailed,
			Trigger:   trigger,
			Reason:    "签到响应格式错误",
			Message:   strings.TrimSpace(string(body)),
			CheckedAt: common.GetTimestamp(),
		}, nil
	}

	result := &model.ChannelUpstreamCheckin{
		Trigger:   trigger,
		Message:   payload.Message,
		CheckedAt: common.GetTimestamp(),
	}
	if payload.Success {
		result.Status = model.ChannelUpstreamCheckinStatusSuccess
		result.Reward = stringifyUpstreamReward(payload.Data.QuotaAwarded)
		if result.Reward == "" {
			result.Reward = stringifyUpstreamReward(payload.Data.Reward)
		}
		if result.Message == "" {
			result.Message = "签到成功"
		}
		return result, nil
	}
	result.Status = model.ChannelUpstreamCheckinStatusFailed
	result.Reason = strings.TrimSpace(payload.Message)
	if result.Reason == "" {
		result.Reason = "签到失败"
	}
	return result, nil
}

func findUpstreamPreset(platform string) *UpstreamChannelPreset {
	for _, preset := range upstreamChannelPresets {
		if preset.Platform == platform {
			sanitized := sanitizeUpstreamPreset(preset)
			return &sanitized
		}
	}
	return nil
}

func stringifyUpstreamReward(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", v), "0"), ".")
	case json.Number:
		return v.String()
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func ValidateChannelUpstreamSettings(channel *model.Channel) error {
	if channel == nil {
		return nil
	}
	settings := channel.GetOtherSettings()
	if !settings.UpstreamCheckinEnabled {
		return nil
	}
	if strings.TrimSpace(settings.UpstreamCheckinAccessToken) == "" {
		return errors.New("启用上游签到时必须填写 access token")
	}
	if settings.UpstreamCheckinIntervalHours < 0 {
		return errors.New("上游签到间隔必须大于等于 0")
	}
	return nil
}

func NormalizeChannelUpstreamSettings(channel *model.Channel) {
	if channel == nil {
		return
	}
	settings := channel.GetOtherSettings()
	if settings.UpstreamCheckinIntervalHours <= 0 {
		settings.UpstreamCheckinIntervalHours = 24
	}
	settings.UpstreamCheckinAccessToken = strings.TrimSpace(settings.UpstreamCheckinAccessToken)
	settings.UpstreamPlatform = strings.TrimSpace(settings.UpstreamPlatform)
	settings.UpstreamPreset = strings.TrimSpace(settings.UpstreamPreset)
	channel.SetOtherSettings(settings)
}
