package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false

	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.Channel{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedUpstreamCheckinChannel(t *testing.T, db *gorm.DB, baseURL string, settings dto.ChannelOtherSettings) *model.Channel {
	t.Helper()

	channel := &model.Channel{
		Type:    constant.ChannelTypeOpenAI,
		Name:    "test-upstream-channel",
		Key:     "proxy-key",
		Status:  common.ChannelStatusEnabled,
		BaseURL: &baseURL,
		Models:  "gpt-4o",
		Group:   "default",
	}
	channel.SetOtherSettings(settings)
	require.NoError(t, db.Create(channel).Error)
	return channel
}

func TestDetectUpstreamChannelPresetEndpoint(t *testing.T) {
	setupChannelControllerTestDB(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/channel/upstream/detect", map[string]any{
		"base_url": "https://newapi.example.com/v1",
		"type":     constant.ChannelTypeCustom,
	}, 1)

	DetectUpstreamChannelPreset(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success)

	var payload struct {
		Detection struct {
			Matched           bool   `json:"matched"`
			Platform          string `json:"platform"`
			NormalizedBaseURL string `json:"normalized_base_url"`
			Preset            struct {
				ID               string `json:"id"`
				CheckinSupported bool   `json:"checkin_supported"`
			} `json:"preset"`
		} `json:"detection"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &payload))
	require.True(t, payload.Detection.Matched)
	require.Equal(t, "new-api", payload.Detection.Platform)
	require.Equal(t, "https://newapi.example.com", payload.Detection.NormalizedBaseURL)
	require.Equal(t, "new-api", payload.Detection.Preset.ID)
	require.True(t, payload.Detection.Preset.CheckinSupported)
}

func TestRunChannelUpstreamCheckinEndpointPersistsResult(t *testing.T) {
	db := setupChannelControllerTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/user/checkin", r.URL.Path)
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		_, _ = w.Write([]byte(`{"success":true,"message":"签到成功","data":{"quota_awarded":666}}`))
	}))
	defer server.Close()

	channel := seedUpstreamCheckinChannel(t, db, server.URL, dto.ChannelOtherSettings{
		UpstreamPlatform:           "new-api",
		UpstreamCheckinEnabled:     true,
		UpstreamCheckinAccessToken: "test-token",
		UpstreamCheckinUserID:      9,
	})

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/channel/%d/upstream_checkin", channel.Id),
		nil,
		1,
	)
	ctx.Params = append(ctx.Params, gin.Param{Key: "id", Value: fmt.Sprintf("%d", channel.Id)})

	RunChannelUpstreamCheckin(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success)

	var payload model.ChannelUpstreamCheckin
	require.NoError(t, common.Unmarshal(response.Data, &payload))
	require.Equal(t, model.ChannelUpstreamCheckinStatusSuccess, payload.Status)
	require.Equal(t, "666", payload.Reward)

	reloaded, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	checkin := reloaded.GetUpstreamCheckin()
	require.Equal(t, model.ChannelUpstreamCheckinStatusSuccess, checkin.Status)
	require.Equal(t, "666", checkin.Reward)
	require.Equal(t, "签到成功", checkin.Message)
	require.Positive(t, checkin.CheckedAt)
}

func TestRunChannelUpstreamCheckinTaskOnce(t *testing.T) {
	db := setupChannelControllerTestDB(t)

	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		require.Equal(t, "/api/user/checkin", r.URL.Path)
		_, _ = w.Write([]byte(`{"success":true,"message":"ok","data":{"quota_awarded":88}}`))
	}))
	defer server.Close()

	channel := seedUpstreamCheckinChannel(t, db, server.URL, dto.ChannelOtherSettings{
		UpstreamPlatform:             "new-api",
		UpstreamCheckinEnabled:       true,
		UpstreamCheckinIntervalHours: 24,
		UpstreamCheckinAccessToken:   "task-token",
		UpstreamCheckinUserID:        3,
	})

	runChannelUpstreamCheckinTaskOnce()
	require.Equal(t, 1, requestCount)

	reloaded, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, model.ChannelUpstreamCheckinStatusSuccess, reloaded.GetUpstreamCheckin().Status)

	// Recent success should be skipped on the next run because interval has not elapsed.
	runChannelUpstreamCheckinTaskOnce()
	require.Equal(t, 1, requestCount)
}

func TestValidateChannelRejectsMissingUpstreamCheckinToken(t *testing.T) {
	channel := &model.Channel{
		Type:   constant.ChannelTypeOpenAI,
		Key:    "proxy-key",
		Models: "gpt-4o",
		Group:  "default",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		UpstreamPlatform:       "new-api",
		UpstreamCheckinEnabled: true,
	})

	err := validateChannel(channel, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "access token")
}

func TestDecodeCheckinResponseBodyShape(t *testing.T) {
	var payload struct {
		Success bool `json:"success"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"success":true}`), &payload))
	require.True(t, payload.Success)
}
