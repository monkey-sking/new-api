package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestGetRandomSatisfiedChannelWithExcluded_ResponsesCompactFallsBackToBaseChannel(t *testing.T) {
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})

	channelSyncLock.Lock()
	oldGroup2Model2Channels := group2model2channels
	oldChannelsIDM := channelsIDM
	group2model2channels = map[string]map[string][]int{
		"default": {
			"gpt-5.4": {102, 101},
		},
	}
	channelsIDM = map[int]*Channel{
		101: {Id: 101, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Name: "openai-base"},
		102: {Id: 102, Type: constant.ChannelTypeAnthropic, Status: common.ChannelStatusEnabled, Name: "anthropic-base"},
	}
	channelSyncLock.Unlock()
	t.Cleanup(func() {
		channelSyncLock.Lock()
		group2model2channels = oldGroup2Model2Channels
		channelsIDM = oldChannelsIDM
		channelSyncLock.Unlock()
	})

	channel, err := GetRandomSatisfiedChannelWithExcluded("default", ratio_setting.WithCompactModelSuffix("gpt-5.4"), 0, nil)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 101, channel.Id)
}

func TestGetRandomSatisfiedChannelWithSelectionOptions_PriorityPenaltyDownranksChannel(t *testing.T) {
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})

	priority := int64(1)
	channelSyncLock.Lock()
	oldGroup2Model2Channels := group2model2channels
	oldChannelsIDM := channelsIDM
	group2model2channels = map[string]map[string][]int{
		"default": {
			"gpt-5.4": {301, 302},
		},
	}
	channelsIDM = map[int]*Channel{
		301: {Id: 301, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Name: "penalized", Priority: &priority},
		302: {Id: 302, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Name: "healthy", Priority: &priority},
	}
	channelSyncLock.Unlock()
	t.Cleanup(func() {
		channelSyncLock.Lock()
		group2model2channels = oldGroup2Model2Channels
		channelsIDM = oldChannelsIDM
		channelSyncLock.Unlock()
	})

	channel, err := GetRandomSatisfiedChannelWithSelectionOptions("default", "gpt-5.4", 0, nil, &ChannelSelectOptions{
		AdjustPriority: func(channelID int, basePriority int64) int64 {
			if channelID == 301 {
				return basePriority - 1
			}
			return basePriority
		},
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 302, channel.Id)
}

func TestIsChannelEnabledForGroupModel_ResponsesCompactFallbackFiltersUnsupportedChannel(t *testing.T) {
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})

	channelSyncLock.Lock()
	oldGroup2Model2Channels := group2model2channels
	oldChannelsIDM := channelsIDM
	group2model2channels = map[string]map[string][]int{
		"default": {
			"gpt-5.4": {201, 202},
		},
	}
	channelsIDM = map[int]*Channel{
		201: {Id: 201, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Name: "openai-base"},
		202: {Id: 202, Type: constant.ChannelTypeAnthropic, Status: common.ChannelStatusEnabled, Name: "anthropic-base"},
	}
	channelSyncLock.Unlock()
	t.Cleanup(func() {
		channelSyncLock.Lock()
		group2model2channels = oldGroup2Model2Channels
		channelsIDM = oldChannelsIDM
		channelSyncLock.Unlock()
	})

	compactModel := ratio_setting.WithCompactModelSuffix("gpt-5.4")
	require.True(t, IsChannelEnabledForGroupModel("default", compactModel, 201))
	require.False(t, IsChannelEnabledForGroupModel("default", compactModel, 202))
}

func TestGetChannelWithExcluded_ResponsesCompactFallsBackToSupportedBaseChannelDB(t *testing.T) {
	truncateTables(t)

	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
	})

	openAIPriority := int64(5)
	anthropicPriority := int64(10)
	openAIChannel := &Channel{
		Type:     constant.ChannelTypeOpenAI,
		Key:      "openai-key",
		Status:   common.ChannelStatusEnabled,
		Name:     "openai-db",
		Group:    "default",
		Models:   "gpt-5.4",
		Priority: &openAIPriority,
	}
	anthropicChannel := &Channel{
		Type:     constant.ChannelTypeAnthropic,
		Key:      "anthropic-key",
		Status:   common.ChannelStatusEnabled,
		Name:     "anthropic-db",
		Group:    "default",
		Models:   "gpt-5.4",
		Priority: &anthropicPriority,
	}
	require.NoError(t, DB.Create(openAIChannel).Error)
	require.NoError(t, DB.Create(anthropicChannel).Error)

	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     "gpt-5.4",
		ChannelId: anthropicChannel.Id,
		Enabled:   true,
		Priority:  &anthropicPriority,
		Weight:    0,
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     "gpt-5.4",
		ChannelId: openAIChannel.Id,
		Enabled:   true,
		Priority:  &openAIPriority,
		Weight:    0,
	}).Error)

	compactModel := ratio_setting.WithCompactModelSuffix("gpt-5.4")
	channel, err := GetChannelWithExcluded("default", compactModel, 0, nil)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, openAIChannel.Id, channel.Id)

	require.True(t, isChannelEnabledForGroupModelDB("default", compactModel, openAIChannel.Id))
	require.False(t, isChannelEnabledForGroupModelDB("default", compactModel, anthropicChannel.Id))
}
