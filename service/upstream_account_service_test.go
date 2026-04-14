package service

import (
	"encoding/base64"
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
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUpstreamServiceTestDB(t *testing.T) *gorm.DB {
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

	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.UpstreamSite{}, &model.UpstreamAccount{}, &model.UpstreamCheckinLog{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func fakeJWTWithUserID(userID int) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"id":%d}`, userID)))
	return header + "." + payload + ".signature"
}

func TestRefreshUpstreamAccountSessionAndCheckinForNewAPI(t *testing.T) {
	setupUpstreamServiceTestDB(t)

	token := fakeJWTWithUserID(7)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/login":
			require.Equal(t, http.MethodPost, r.Method)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"success":true,"data":{"access_token":"%s"}}`, token)))
		case "/api/user/self":
			if r.Header.Get("New-Api-User") == "7" {
				_, _ = w.Write([]byte(`{"success":true,"data":{"id":7,"username":"tester"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":false,"message":"missing New-Api-User"}`))
		case "/api/token/":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"name":"default","key":"sk-upstream","status":1}]}`))
		case "/api/user/checkin":
			require.Equal(t, "7", r.Header.Get("New-Api-User"))
			_, _ = w.Write([]byte(`{"success":true,"message":"签到成功","data":{"quota_awarded":66}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	site := &model.UpstreamSite{
		Name:     "test-site",
		BaseURL:  server.URL,
		Platform: UpstreamPlatformNewAPI,
		Status:   model.UpstreamSiteStatusActive,
	}
	require.NoError(t, site.Insert())

	account := &model.UpstreamAccount{
		SiteID:               site.Id,
		Name:                 "primary",
		Username:             "tester",
		Password:             "secret",
		CheckinEnabled:       true,
		CheckinIntervalHours: 24,
		Status:               model.UpstreamAccountStatusActive,
	}
	require.NoError(t, account.Insert())

	session, err := RefreshUpstreamAccountSession(account)
	require.NoError(t, err)
	require.Equal(t, token, session.AccessToken)
	require.Equal(t, 7, session.PlatformUserID)
	require.Equal(t, "sk-upstream", session.APIToken)

	result, err := RunUpstreamAccountCheckin(account, "manual")
	require.NoError(t, err)
	require.Equal(t, model.UpstreamCheckinStatusSuccess, result.Status)
	require.Equal(t, "66", result.Reward)

	reloaded, err := model.GetUpstreamAccountByID(account.Id)
	require.NoError(t, err)
	require.Equal(t, 7, reloaded.PlatformUserID)
	require.Equal(t, "sk-upstream", reloaded.APIToken)
	require.Equal(t, model.UpstreamCheckinStatusSuccess, reloaded.LastCheckinStatus)

	logs, total, err := model.SearchUpstreamCheckinLogs(account.Id, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	require.Equal(t, "66", logs[0].Reward)
}

func TestSyncLegacyChannelUpstreamDataImportsChannelSettings(t *testing.T) {
	setupUpstreamServiceTestDB(t)

	baseURL := "https://legacy.example.com/v1"
	channel := &model.Channel{
		Type:    constant.ChannelTypeOpenAI,
		Name:    "legacy-channel",
		Key:     "proxy-key",
		Status:  common.ChannelStatusEnabled,
		BaseURL: &baseURL,
		Models:  "gpt-4o",
		Group:   "default",
	}
	channel.SetSetting(dto.ChannelSettings{
		Proxy: "http://127.0.0.1:7890",
	})
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		UpstreamPlatform:             UpstreamPlatformNewAPI,
		UpstreamPreset:               "new-api",
		UpstreamCheckinEnabled:       true,
		UpstreamCheckinIntervalHours: 12,
		UpstreamCheckinAccessToken:   "legacy-token",
		UpstreamCheckinUserID:        9,
	})
	channel.SetUpstreamCheckinResult(&model.ChannelUpstreamCheckin{
		Status:    model.ChannelUpstreamCheckinStatusSuccess,
		CheckedAt: 1775617000,
		Trigger:   "manual",
		Reward:    "88",
		Message:   "签到成功",
	})
	require.NoError(t, model.DB.Create(channel).Error)

	stats, err := SyncLegacyChannelUpstreamData(true)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, 1, stats.SitesCreated)
	require.Equal(t, 1, stats.AccountsCreated)
	require.Equal(t, 1, stats.LogsCreated)

	var sites []model.UpstreamSite
	require.NoError(t, model.DB.Order("id asc").Find(&sites).Error)
	require.Len(t, sites, 1)
	require.Equal(t, channel.Id, sites[0].ChannelID)
	require.Equal(t, "https://legacy.example.com", sites[0].BaseURL)
	require.Equal(t, UpstreamPlatformNewAPI, sites[0].Platform)
	require.Equal(t, "new-api", sites[0].Preset)
	require.Equal(t, "http://127.0.0.1:7890", sites[0].Proxy)

	var accounts []model.UpstreamAccount
	require.NoError(t, model.DB.Order("id asc").Find(&accounts).Error)
	require.Len(t, accounts, 1)
	require.Equal(t, sites[0].Id, accounts[0].SiteID)
	require.Equal(t, "legacy-token", accounts[0].AccessToken)
	require.Equal(t, 9, accounts[0].PlatformUserID)
	require.True(t, accounts[0].CheckinEnabled)
	require.Equal(t, 12, accounts[0].CheckinIntervalHours)
	require.Equal(t, int64(1775617000), accounts[0].LastCheckinAt)
	require.Equal(t, model.UpstreamCheckinStatusSuccess, accounts[0].LastCheckinStatus)
	require.Contains(t, accounts[0].ExtraConfig, "\"legacy_channel_id\":")

	var logs []model.UpstreamCheckinLog
	require.NoError(t, model.DB.Order("id asc").Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, accounts[0].Id, logs[0].AccountID)
	require.Equal(t, model.UpstreamCheckinStatusSuccess, logs[0].Status)
	require.Equal(t, "88", logs[0].Reward)
	require.Equal(t, int64(1775617000), logs[0].CreatedTime)

	stats, err = SyncLegacyChannelUpstreamData(true)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, 0, stats.SitesCreated)
	require.Equal(t, 0, stats.AccountsCreated)
	require.Equal(t, 0, stats.LogsCreated)

	var siteCount int64
	require.NoError(t, model.DB.Model(&model.UpstreamSite{}).Count(&siteCount).Error)
	require.Equal(t, int64(1), siteCount)

	var accountCount int64
	require.NoError(t, model.DB.Model(&model.UpstreamAccount{}).Count(&accountCount).Error)
	require.Equal(t, int64(1), accountCount)

	var logCount int64
	require.NoError(t, model.DB.Model(&model.UpstreamCheckinLog{}).Count(&logCount).Error)
	require.Equal(t, int64(1), logCount)
}

func TestSyncLegacyChannelUpstreamDataReusesUnboundSite(t *testing.T) {
	setupUpstreamServiceTestDB(t)

	existingSite := &model.UpstreamSite{
		Name:     "manual-site",
		BaseURL:  "https://legacy.example.com",
		Platform: UpstreamPlatformNewAPI,
		Status:   model.UpstreamSiteStatusActive,
	}
	require.NoError(t, existingSite.Insert())

	baseURL := "https://legacy.example.com"
	channel := &model.Channel{
		Type:    constant.ChannelTypeOpenAI,
		Name:    "legacy-channel",
		Key:     "proxy-key",
		Status:  common.ChannelStatusEnabled,
		BaseURL: &baseURL,
		Models:  "gpt-4o",
		Group:   "default",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		UpstreamPlatform:           UpstreamPlatformNewAPI,
		UpstreamCheckinAccessToken: "legacy-token",
	})
	require.NoError(t, model.DB.Create(channel).Error)

	stats, err := SyncLegacyChannelUpstreamData(true)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, 0, stats.SitesCreated)
	require.Equal(t, 1, stats.SitesUpdated)

	reloadedSite, err := model.GetUpstreamSiteByID(existingSite.Id)
	require.NoError(t, err)
	require.Equal(t, channel.Id, reloadedSite.ChannelID)

	var siteCount int64
	require.NoError(t, model.DB.Model(&model.UpstreamSite{}).Count(&siteCount).Error)
	require.Equal(t, int64(1), siteCount)
}
