package service

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

var channelSoftDegradeTracker = newChannelWindowTracker()

const maxChannelSoftDegradePenalty = 5

func channelSoftDegradePath(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if fullPath := strings.TrimSpace(c.FullPath()); fullPath != "" {
		return fullPath
	}
	if c.Request == nil || c.Request.URL == nil {
		return ""
	}
	return strings.TrimSpace(c.Request.URL.Path)
}

func channelSoftDegradeKey(channelID int, modelName string, requestPath string) string {
	return fmt.Sprintf("%d\n%s\n%s", channelID, strings.TrimSpace(modelName), strings.TrimSpace(requestPath))
}

func parseChannelSoftDegradeKey(key string) (int, string, string, bool) {
	parts := strings.SplitN(key, "\n", 3)
	if len(parts) != 3 {
		return 0, "", "", false
	}
	channelID, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || channelID <= 0 {
		return 0, "", "", false
	}
	return channelID, strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2]), true
}

func effectiveChannelSoftDegradePenalty(rawPenalty int) int {
	if rawPenalty <= 0 {
		return 0
	}
	if rawPenalty > maxChannelSoftDegradePenalty {
		return maxChannelSoftDegradePenalty
	}
	return rawPenalty
}

func scoreChannelSoftDegrade(rawPenalty int) int {
	score := 100 - effectiveChannelSoftDegradePenalty(rawPenalty)*20
	if score < 0 {
		return 0
	}
	return score
}

func defaultChannelRuntimeHealth() *model.ChannelRuntimeHealth {
	return &model.ChannelRuntimeHealth{
		Score:            100,
		ActiveScopeCount: 0,
		MaxPenalty:       0,
		EffectivePenalty: 0,
		WindowSeconds:    int(channelSoftDegradeWindow() / time.Second),
		Scope:            "local_node",
	}
}

func cloneChannelRuntimeHealth(health *model.ChannelRuntimeHealth) *model.ChannelRuntimeHealth {
	if health == nil {
		return defaultChannelRuntimeHealth()
	}
	cloned := *health
	return &cloned
}

func BuildChannelRuntimeHealthMap(channels []*model.Channel) map[int]*model.ChannelRuntimeHealth {
	healthByChannel := make(map[int]*model.ChannelRuntimeHealth, len(channels))
	if len(channels) == 0 {
		return healthByChannel
	}

	channelIDs := make(map[int]struct{}, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.Id <= 0 {
			continue
		}
		channelIDs[channel.Id] = struct{}{}
		healthByChannel[channel.Id] = defaultChannelRuntimeHealth()
	}

	if len(channelIDs) == 0 {
		return healthByChannel
	}

	for key, count := range channelSoftDegradeTracker.snapshot(channelSoftDegradeWindow()) {
		channelID, modelName, requestPath, ok := parseChannelSoftDegradeKey(key)
		if !ok {
			continue
		}
		if _, exists := channelIDs[channelID]; !exists {
			continue
		}

		health := healthByChannel[channelID]
		if health == nil {
			health = defaultChannelRuntimeHealth()
			healthByChannel[channelID] = health
		}

		health.ActiveScopeCount++
		if count > health.MaxPenalty {
			health.MaxPenalty = count
			health.EffectivePenalty = effectiveChannelSoftDegradePenalty(count)
			health.Score = scoreChannelSoftDegrade(count)
			health.WorstModel = modelName
			health.WorstPath = requestPath
		}
	}

	return healthByChannel
}

func PopulateChannelRuntimeHealth(channels []*model.Channel) {
	healthByChannel := BuildChannelRuntimeHealthMap(channels)
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		if health, ok := healthByChannel[channel.Id]; ok {
			channel.RuntimeHealth = cloneChannelRuntimeHealth(health)
			continue
		}
		channel.RuntimeHealth = defaultChannelRuntimeHealth()
	}
}

func shouldManageSoftDegrade(channelError types.ChannelError, modelName string, requestPath string) bool {
	if !common.AutomaticDisableChannelEnabled || !channelError.AutoBan {
		return false
	}
	if channelError.ChannelId <= 0 || strings.TrimSpace(modelName) == "" || strings.TrimSpace(requestPath) == "" {
		return false
	}
	return true
}

func isCapabilityMismatchSoftDegrade(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	switch err.StatusCode {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusUnprocessableEntity, http.StatusNotImplemented:
	default:
		return false
	}
	lowerMessage := strings.ToLower(err.Error())
	signals := []string{
		"responses/compact",
		"unsupported endpoint",
		"does not support",
		"not support",
		"unsupported",
		"unknown url",
		"invalid endpoint",
		"not implemented",
	}
	for _, signal := range signals {
		if strings.Contains(lowerMessage, signal) {
			return true
		}
	}
	return false
}

func shouldTrackSoftDegrade(channelError types.ChannelError, err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if ShouldDisableChannel(channelError.ChannelType, err) {
		return false
	}
	if types.IsSkipRetryError(err) || types.IsChannelError(err) {
		return false
	}
	if isTimeoutLikeError(err) || shouldTrackWindowFailure(err) || isCapabilityMismatchSoftDegrade(err) {
		return true
	}
	return false
}

func ObserveChannelSoftDegrade(c *gin.Context, channelError types.ChannelError, modelName string, err *types.NewAPIError) {
	requestPath := channelSoftDegradePath(c)
	if !shouldManageSoftDegrade(channelError, modelName, requestPath) {
		return
	}
	key := channelSoftDegradeKey(channelError.ChannelId, modelName, requestPath)
	window := channelSoftDegradeWindow()

	if err == nil {
		channelSoftDegradeTracker.relieve(key, window)
		return
	}
	if !shouldTrackSoftDegrade(channelError, err) {
		channelSoftDegradeTracker.prune(key, window)
		return
	}
	channelSoftDegradeTracker.observe(key, window)
}

func GetChannelSoftDegradePenalty(c *gin.Context, channelID int, modelName string) int {
	requestPath := channelSoftDegradePath(c)
	if channelID <= 0 || strings.TrimSpace(modelName) == "" || strings.TrimSpace(requestPath) == "" {
		return 0
	}
	return channelSoftDegradeTracker.count(channelSoftDegradeKey(channelID, modelName, requestPath), channelSoftDegradeWindow())
}

func buildChannelSelectOptions(param *RetryParam) *model.ChannelSelectOptions {
	if param == nil || param.Ctx == nil || strings.TrimSpace(param.ModelName) == "" {
		return nil
	}
	return &model.ChannelSelectOptions{
		AdjustPriority: func(channelID int, basePriority int64) int64 {
			penalty := effectiveChannelSoftDegradePenalty(GetChannelSoftDegradePenalty(param.Ctx, channelID, param.ModelName))
			if penalty <= 0 {
				return basePriority
			}
			return basePriority - int64(penalty)
		},
	}
}
