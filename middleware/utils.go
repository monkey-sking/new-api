package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func abortWithOpenAiMessage(c *gin.Context, statusCode int, message string, code ...types.ErrorCode) {
	codeStr := ""
	if len(code) > 0 {
		codeStr = string(code[0])
	}
	userId := c.GetInt("id")

	errorObj := gin.H{
		"message": common.MessageWithRequestId(message, c.GetString(common.RequestIdKey)),
		"type":    "new_api_error",
		"code":    codeStr,
	}

	isStream := false
	if strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
		isStream = true
	} else {
		bodyStorage, err := common.GetBodyStorage(c)
		if err == nil && bodyStorage != nil {
			if bodyBytes, err := bodyStorage.Bytes(); err == nil {
				isStream = strings.Contains(string(bodyBytes), `"stream":true`) || strings.Contains(string(bodyBytes), `"stream": true`)
			}
		}
	}

	if isStream {
		if strings.Contains(c.Request.URL.Path, "/responses") {
			_ = helper.WriteResponsesErrorStream(c, types.OpenAIError{
				Message: common.Interface2String(errorObj["message"]),
				Type:    common.Interface2String(errorObj["type"]),
				Code:    errorObj["code"],
			}, "")
		} else {
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			c.Writer.Header().Set("Cache-Control", "no-cache")
			c.Writer.Header().Set("Connection", "keep-alive")
			c.Writer.WriteHeader(http.StatusOK)

			errorData, _ := common.Marshal(gin.H{
				"error": errorObj,
			})
			_, _ = c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", string(errorData))))
			_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
		}
		c.Writer.Flush()
	} else {
		c.JSON(statusCode, gin.H{
			"error": errorObj,
		})
	}

	c.Abort()
	logger.LogError(c.Request.Context(), fmt.Sprintf("user %d | %s", userId, message))
}

func abortWithMidjourneyMessage(c *gin.Context, statusCode int, code int, description string) {
	c.JSON(statusCode, gin.H{
		"description": description,
		"type":        "new_api_error",
		"code":        code,
	})
	c.Abort()
	logger.LogError(c.Request.Context(), description)
}
