package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type tokenNameOptionsResponse struct {
	Items []string `json:"items"`
}

func seedLogRecord(t *testing.T, userID int, tokenName string) {
	t.Helper()

	log := &model.Log{
		UserId:    userID,
		Username:  "tester",
		CreatedAt: 1,
		Type:      model.LogTypeConsume,
		TokenName: tokenName,
		ModelName: "gpt-5.4",
		Quota:     1,
		Group:     "default",
	}
	if err := model.LOG_DB.Create(log).Error; err != nil {
		t.Fatalf("failed to create log record: %v", err)
	}
}

func TestGetUserLogTokenNamesReturnsDistinctCurrentUserTokenNames(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	if err := db.AutoMigrate(&model.Log{}); err != nil {
		t.Fatalf("failed to migrate log table: %v", err)
	}

	seedLogRecord(t, 1, "codex")
	seedLogRecord(t, 1, "codex")
	seedLogRecord(t, 1, "alpha")
	seedLogRecord(t, 2, "openclaw")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/self/token_names?keyword=co&size=20", nil, 1)
	GetUserLogTokenNames(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var data tokenNameOptionsResponse
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode token name options response: %v", err)
	}

	if len(data.Items) != 1 || data.Items[0] != "codex" {
		t.Fatalf("expected only current user's matching token name, got %#v", data.Items)
	}
}

func TestGetLogTokenNamesReturnsDistinctTokenNamesForAdmin(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	if err := db.AutoMigrate(&model.Log{}); err != nil {
		t.Fatalf("failed to migrate log table: %v", err)
	}

	seedLogRecord(t, 1, "codex")
	seedLogRecord(t, 2, "openclaw")
	seedLogRecord(t, 3, "codex")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/token_names?size=20", nil, 1)
	GetLogTokenNames(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var data tokenNameOptionsResponse
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode token name options response: %v", err)
	}

	if len(data.Items) != 2 {
		t.Fatalf("expected two distinct token names, got %#v", data.Items)
	}
	if data.Items[0] != "codex" || data.Items[1] != "openclaw" {
		t.Fatalf("expected sorted distinct token names, got %#v", data.Items)
	}
}
