package controller

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUpstreamControllerTestDB(t *testing.T) *gorm.DB {
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

	require.NoError(t, db.AutoMigrate(&model.UpstreamSite{}, &model.UpstreamAccount{}, &model.UpstreamCheckinLog{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func fakeControllerJWT(userID int) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"id":%d}`, userID)))
	return header + "." + payload + ".signature"
}

func TestUpstreamSiteAccountSessionAndCheckinEndpoints(t *testing.T) {
	setupUpstreamControllerTestDB(t)
	token := fakeControllerJWT(7)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/login":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"success":true,"data":{"access_token":"%s"}}`, token)))
		case "/api/user/self":
			if r.Header.Get("New-Api-User") == "7" {
				_, _ = w.Write([]byte(`{"success":true,"data":{"id":7,"username":"operator"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":false,"message":"missing New-Api-User"}`))
		case "/api/token/":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"name":"default","key":"sk-account","status":1}]}`))
		case "/api/user/checkin":
			require.Equal(t, "7", r.Header.Get("New-Api-User"))
			_, _ = w.Write([]byte(`{"success":true,"message":"ok","data":{"quota_awarded":88}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	createSiteCtx, createSiteRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/upstream/sites", map[string]any{
		"name":     "demo-site",
		"base_url": server.URL,
		"platform": service.UpstreamPlatformNewAPI,
	}, 1)
	CreateUpstreamSite(createSiteCtx)
	createSiteResp := decodeAPIResponse(t, createSiteRecorder)
	require.True(t, createSiteResp.Success)

	var createdSite model.UpstreamSite
	require.NoError(t, common.Unmarshal(createSiteResp.Data, &createdSite))
	require.NotZero(t, createdSite.Id)

	createAccountCtx, createAccountRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/upstream/accounts", map[string]any{
		"site_id":                createdSite.Id,
		"name":                   "primary",
		"username":               "operator",
		"password":               "secret",
		"checkin_enabled":        true,
		"checkin_interval_hours": 24,
	}, 1)
	CreateUpstreamAccount(createAccountCtx)
	createAccountResp := decodeAPIResponse(t, createAccountRecorder)
	require.True(t, createAccountResp.Success)

	var createdAccount model.UpstreamAccount
	require.NoError(t, common.Unmarshal(createAccountResp.Data, &createdAccount))
	require.NotZero(t, createdAccount.Id)

	refreshCtx, refreshRecorder := newAuthenticatedContext(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/upstream/accounts/%d/refresh_session", createdAccount.Id),
		nil,
		1,
	)
	refreshCtx.Params = append(refreshCtx.Params, gin.Param{Key: "id", Value: fmt.Sprintf("%d", createdAccount.Id)})
	RefreshUpstreamAccountSession(refreshCtx)
	refreshResp := decodeAPIResponse(t, refreshRecorder)
	require.True(t, refreshResp.Success)

	checkinCtx, checkinRecorder := newAuthenticatedContext(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/upstream/accounts/%d/checkin", createdAccount.Id),
		nil,
		1,
	)
	checkinCtx.Params = append(checkinCtx.Params, gin.Param{Key: "id", Value: fmt.Sprintf("%d", createdAccount.Id)})
	RunUpstreamAccountCheckin(checkinCtx)
	checkinResp := decodeAPIResponse(t, checkinRecorder)
	require.True(t, checkinResp.Success)

	logsCtx, logsRecorder := newAuthenticatedContext(t, http.MethodGet, fmt.Sprintf("/api/upstream/checkin_logs?account_id=%d", createdAccount.Id), nil, 1)
	GetUpstreamCheckinLogs(logsCtx)
	logsResp := decodeAPIResponse(t, logsRecorder)
	require.True(t, logsResp.Success)

	var payload struct {
		Items []model.UpstreamCheckinLog `json:"items"`
		Total int                        `json:"total"`
	}
	require.NoError(t, common.Unmarshal(logsResp.Data, &payload))
	require.Equal(t, 1, payload.Total)
	require.Len(t, payload.Items, 1)
	require.Equal(t, model.UpstreamCheckinStatusSuccess, payload.Items[0].Status)
}
