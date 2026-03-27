package helper

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWriteResponsesErrorStream_UsesResponsesLifecycleEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/backend-api/codex/responses", nil)
	ctx.Set(common.RequestIdKey, "req_test")

	err := WriteResponsesErrorStream(ctx, types.OpenAIError{
		Message: "upstream failed",
		Type:    "new_api_error",
		Code:    "bad_response",
	}, "gpt-5-codex")
	require.NoError(t, err)

	body := recorder.Body.String()
	require.Contains(t, body, "event: response.created")
	require.Contains(t, body, "event: response.failed")
	require.Contains(t, body, "event: response.completed")
	require.Contains(t, body, "\"status\":\"failed\"")
	require.Contains(t, body, "\"id\":\"resp_req_test\"")
	require.NotContains(t, body, "event: done")
	require.NotContains(t, body, "[DONE]")
	require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	require.Equal(t, 3, strings.Count(body, "event: "))
}
