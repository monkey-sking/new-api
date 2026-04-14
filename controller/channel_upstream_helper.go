package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	channelUpstreamCheckinTaskDefaultIntervalMinutes = 30
	channelUpstreamCheckinTaskBatchSize              = 100
	channelUpstreamCheckinDefaultIntervalHours       = 24
)

var (
	channelUpstreamCheckinTaskOnce    sync.Once
	channelUpstreamCheckinTaskRunning atomic.Bool
)

type detectUpstreamChannelPresetRequest struct {
	BaseURL string `json:"base_url"`
	Type    int    `json:"type"`
}

func ListUpstreamChannelPresets(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    service.ListUpstreamChannelPresets(),
	})
}

func DetectUpstreamChannelPreset(c *gin.Context) {
	var req detectUpstreamChannelPresetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	result := service.DetectUpstreamChannelPreset(req.BaseURL, req.Type)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"detection": result,
			"presets":   service.ListUpstreamChannelPresets(),
		},
	})
}

func RunChannelUpstreamCheckin(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	result, runErr := service.RunChannelUpstreamCheckin(channel, "manual")
	if runErr != nil {
		result = &model.ChannelUpstreamCheckin{
			Status:    model.ChannelUpstreamCheckinStatusFailed,
			Trigger:   "manual",
			Reason:    runErr.Error(),
			CheckedAt: common.GetTimestamp(),
		}
	}

	channel.SetUpstreamCheckinResult(result)
	if err := model.DB.Model(channel).Update("other_info", channel.OtherInfo).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	message := result.Message
	if message == "" {
		message = "上游签到已执行"
	}
	c.JSON(http.StatusOK, gin.H{
		"success": result.Status == model.ChannelUpstreamCheckinStatusSuccess,
		"message": message,
		"data":    result,
	})
}

func StartChannelUpstreamCheckinTask() {
	channelUpstreamCheckinTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		if !common.GetEnvOrDefaultBool("CHANNEL_UPSTREAM_CHECKIN_TASK_ENABLED", true) {
			common.SysLog("upstream checkin task disabled by CHANNEL_UPSTREAM_CHECKIN_TASK_ENABLED")
			return
		}

		intervalMinutes := common.GetEnvOrDefault(
			"CHANNEL_UPSTREAM_CHECKIN_TASK_INTERVAL_MINUTES",
			channelUpstreamCheckinTaskDefaultIntervalMinutes,
		)
		if intervalMinutes < 1 {
			intervalMinutes = channelUpstreamCheckinTaskDefaultIntervalMinutes
		}
		interval := time.Duration(intervalMinutes) * time.Minute

		go func() {
			common.SysLog(fmt.Sprintf("upstream checkin task started: interval=%s", interval))
			runChannelUpstreamCheckinTaskOnce()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for range ticker.C {
				runChannelUpstreamCheckinTaskOnce()
			}
		}()
	})
}

func runChannelUpstreamCheckinTaskOnce() {
	if !channelUpstreamCheckinTaskRunning.CompareAndSwap(false, true) {
		return
	}
	defer channelUpstreamCheckinTaskRunning.Store(false)

	now := common.GetTimestamp()
	lastID := 0

	for {
		var channels []*model.Channel
		err := model.DB.
			Where("id > ?", lastID).
			Where("status = ?", common.ChannelStatusEnabled).
			Order("id asc").
			Limit(channelUpstreamCheckinTaskBatchSize).
			Find(&channels).Error
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to load upstream checkin channels: %v", err))
			return
		}
		if len(channels) == 0 {
			return
		}

		for _, channel := range channels {
			lastID = channel.Id
			settings := channel.GetOtherSettings()
			if !settings.UpstreamCheckinEnabled {
				continue
			}
			intervalHours := settings.UpstreamCheckinIntervalHours
			if intervalHours <= 0 {
				intervalHours = channelUpstreamCheckinDefaultIntervalHours
			}
			lastCheck := channel.GetUpstreamCheckin().CheckedAt
			if lastCheck > 0 && now-lastCheck < int64(intervalHours)*3600 {
				continue
			}

			result, runErr := service.RunChannelUpstreamCheckin(channel, "auto")
			if runErr != nil {
				result = &model.ChannelUpstreamCheckin{
					Status:    model.ChannelUpstreamCheckinStatusFailed,
					Trigger:   "auto",
					Reason:    runErr.Error(),
					CheckedAt: common.GetTimestamp(),
				}
			}
			channel.SetUpstreamCheckinResult(result)
			if err := model.DB.Model(channel).Update("other_info", channel.OtherInfo).Error; err != nil {
				common.SysLog(fmt.Sprintf("failed to save upstream checkin result: channel_id=%d, error=%v", channel.Id, err))
				continue
			}
		}

		if len(channels) < channelUpstreamCheckinTaskBatchSize {
			return
		}
	}
}
