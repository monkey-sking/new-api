package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestDetectUpstreamChannelPreset(t *testing.T) {
	t.Run("detect new-api family", func(t *testing.T) {
		result := DetectUpstreamChannelPreset("https://newapi.example.com", constant.ChannelTypeCustom)

		require.True(t, result.Matched)
		require.Equal(t, UpstreamPlatformNewAPI, result.Platform)
		require.Equal(t, constant.ChannelTypeOpenAI, result.SuggestedChannelType)
		require.NotNil(t, result.Preset)
		require.True(t, result.Preset.CheckinSupported)
	})

	t.Run("detect official openai", func(t *testing.T) {
		result := DetectUpstreamChannelPreset("https://api.openai.com/v1", constant.ChannelTypeCustom)

		require.True(t, result.Matched)
		require.Equal(t, UpstreamPlatformOpenAI, result.Platform)
		require.Equal(t, constant.ChannelTypeOpenAI, result.SuggestedChannelType)
		require.NotNil(t, result.Preset)
		require.False(t, result.Preset.CheckinSupported)
		require.Equal(t, "https://api.openai.com", result.NormalizedBaseURL)
	})

	t.Run("detect done-hub as no-checkin platform", func(t *testing.T) {
		result := DetectUpstreamChannelPreset("https://done-hub.example.com", constant.ChannelTypeCustom)

		require.True(t, result.Matched)
		require.Equal(t, UpstreamPlatformDoneHub, result.Platform)
		require.NotNil(t, result.Preset)
		require.False(t, result.Preset.CheckinSupported)
	})

	t.Run("unknown host remains unmatched", func(t *testing.T) {
		result := DetectUpstreamChannelPreset("https://gateway.example.com", constant.ChannelTypeCustom)

		require.False(t, result.Matched)
		require.Nil(t, result.Preset)
		require.Equal(t, "", result.Platform)
	})

	t.Run("detect localhost upstream via api status payload", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/api/status", r.URL.Path)
			_, _ = w.Write([]byte(`{"success":true,"data":{"system_name":"new-api mock"}}`))
		}))
		defer server.Close()

		result := DetectUpstreamChannelPreset(server.URL, constant.ChannelTypeCustom)

		require.True(t, result.Matched)
		require.Equal(t, UpstreamPlatformNewAPI, result.Platform)
		require.NotNil(t, result.Preset)
		require.Equal(t, UpstreamPlatformNewAPI, result.Preset.Platform)
	})
}

func TestBuildUpstreamCheckinHeaders(t *testing.T) {
	headers := buildUpstreamCheckinHeaders(UpstreamPlatformNewAPI, "test-access-token", 42)

	require.Equal(t, "Bearer test-access-token", headers.Get("Authorization"))
	require.Equal(t, "42", headers.Get("New-Api-User"))
	require.Equal(t, "42", headers.Get("New-API-User"))
	require.Equal(t, "42", headers.Get("Veloera-User"))
	require.Equal(t, "42", headers.Get("voapi-user"))
	require.Equal(t, "42", headers.Get("User-id"))
	require.Equal(t, "42", headers.Get("Rix-Api-User"))
	require.Equal(t, "42", headers.Get("neo-api-user"))
}

func TestRunChannelUpstreamCheckin(t *testing.T) {
	t.Run("successfully checks in against new-api style endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/api/user/checkin", r.URL.Path)
			require.Equal(t, "Bearer upstream-token", r.Header.Get("Authorization"))
			require.Equal(t, "7", r.Header.Get("New-Api-User"))
			_, _ = w.Write([]byte(`{"success":true,"message":"签到成功","data":{"quota_awarded":123456}}`))
		}))
		defer server.Close()

		channel := &model.Channel{
			Type:    constant.ChannelTypeOpenAI,
			BaseURL: &server.URL,
		}
		channel.SetOtherSettings(dto.ChannelOtherSettings{
			UpstreamPlatform:            UpstreamPlatformNewAPI,
			UpstreamCheckinEnabled:      true,
			UpstreamCheckinAccessToken:  "upstream-token",
			UpstreamCheckinUserID:       7,
			UpstreamCheckinIntervalHours: 24,
		})

		result, err := RunChannelUpstreamCheckin(channel, "manual")
		require.NoError(t, err)
		require.Equal(t, model.ChannelUpstreamCheckinStatusSuccess, result.Status)
		require.Equal(t, "123456", result.Reward)
		require.Equal(t, "签到成功", result.Message)
	})

	t.Run("returns unsupported for sub2api", func(t *testing.T) {
		channel := &model.Channel{}
		channel.SetOtherSettings(dto.ChannelOtherSettings{
			UpstreamPlatform:           UpstreamPlatformSub2API,
			UpstreamCheckinEnabled:     true,
			UpstreamCheckinAccessToken: "jwt-token",
		})

		result, err := RunChannelUpstreamCheckin(channel, "manual")
		require.NoError(t, err)
		require.Equal(t, model.ChannelUpstreamCheckinStatusSkipped, result.Status)
		require.Contains(t, result.Reason, "不支持")
	})

	t.Run("requires access token when checkin enabled", func(t *testing.T) {
		channel := &model.Channel{}
		channel.SetOtherSettings(dto.ChannelOtherSettings{
			UpstreamPlatform:       UpstreamPlatformNewAPI,
			UpstreamCheckinEnabled: true,
		})

		_, err := RunChannelUpstreamCheckin(channel, "manual")
		require.Error(t, err)
		require.Contains(t, err.Error(), "access token")
	})
}
