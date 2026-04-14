package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelGetUpstreamCheckinReadsOtherInfo(t *testing.T) {
	channel := &Channel{}
	channel.SetOtherInfo(map[string]interface{}{
		channelOtherInfoUpstreamCheckinStatus:  ChannelUpstreamCheckinStatusFailed,
		channelOtherInfoUpstreamCheckinTime:    int64(123456),
		channelOtherInfoUpstreamCheckinTrigger: "auto",
		channelOtherInfoUpstreamCheckinReason:  "签到失败",
		channelOtherInfoUpstreamCheckinReward:  "10",
		channelOtherInfoUpstreamCheckinMessage: "额度不足",
	})

	result := channel.GetUpstreamCheckin()
	require.Equal(t, ChannelUpstreamCheckinStatusFailed, result.Status)
	require.Equal(t, int64(123456), result.CheckedAt)
	require.Equal(t, "auto", result.Trigger)
	require.Equal(t, "签到失败", result.Reason)
	require.Equal(t, "10", result.Reward)
	require.Equal(t, "额度不足", result.Message)
}

func TestChannelSetUpstreamCheckinResult(t *testing.T) {
	channel := &Channel{}

	channel.SetUpstreamCheckinResult(&ChannelUpstreamCheckin{
		Status:    ChannelUpstreamCheckinStatusSuccess,
		Trigger:   "manual",
		Reason:    "",
		Reward:    "88",
		Message:   "签到成功",
		CheckedAt: 222222,
	})

	result := channel.GetUpstreamCheckin()
	require.Equal(t, ChannelUpstreamCheckinStatusSuccess, result.Status)
	require.Equal(t, int64(222222), result.CheckedAt)
	require.Equal(t, "manual", result.Trigger)
	require.Equal(t, "88", result.Reward)
	require.Equal(t, "签到成功", result.Message)
	require.Equal(t, "", result.Reason)
}
