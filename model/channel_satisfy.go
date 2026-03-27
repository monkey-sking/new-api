package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func IsChannelEnabledForGroupModel(group string, modelName string, channelID int) bool {
	if group == "" || modelName == "" || channelID <= 0 {
		return false
	}
	if !common.MemoryCacheEnabled {
		return isChannelEnabledForGroupModelDB(group, modelName, channelID)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return false
	}

	if isChannelIDInList(group2model2channels[group][modelName], channelID) {
		return true
	}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized != "" && normalized != modelName {
		if isChannelIDInList(group2model2channels[group][normalized], channelID) {
			return true
		}
	}
	if isChannelIDInList(getResponsesCompactFallbackChannelIDs(group, modelName), channelID) {
		return true
	}
	return false
}

func IsChannelEnabledForAnyGroupModel(groups []string, modelName string, channelID int) bool {
	if len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if IsChannelEnabledForGroupModel(g, modelName, channelID) {
			return true
		}
	}
	return false
}

func isChannelEnabledForGroupModelDB(group string, modelName string, channelID int) bool {
	var count int64
	err := DB.Model(&Ability{}).
		Joins("left join channels on abilities.channel_id = channels.id").
		Where("abilities."+commonGroupCol+" = ? and abilities.model = ? and abilities.channel_id = ? and abilities.enabled = ?", group, modelName, channelID, true).
		Where("channels.status = ?", common.ChannelStatusEnabled).
		Count(&count).Error
	if err == nil && count > 0 {
		return true
	}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized != "" && normalized != modelName {
		count = 0
		err = DB.Model(&Ability{}).
			Joins("left join channels on abilities.channel_id = channels.id").
			Where("abilities."+commonGroupCol+" = ? and abilities.model = ? and abilities.channel_id = ? and abilities.enabled = ?", group, normalized, channelID, true).
			Where("channels.status = ?", common.ChannelStatusEnabled).
			Count(&count).Error
		if err == nil && count > 0 {
			return true
		}
	}
	fallbackModels := responsesCompactFallbackModels(modelName)
	if len(fallbackModels) == 0 {
		return false
	}
	var ability AbilityWithChannel
	for _, fallbackModel := range fallbackModels {
		err = DB.Table("abilities").
			Select("abilities.*, channels.type as channel_type").
			Joins("left join channels on abilities.channel_id = channels.id").
			Where("abilities."+commonGroupCol+" = ? and abilities.model = ? and abilities.channel_id = ? and abilities.enabled = ?", group, fallbackModel, channelID, true).
			Where("channels.status = ?", common.ChannelStatusEnabled).
			Limit(1).
			Scan(&ability).Error
		if err == nil && ability.ChannelId > 0 && supportsResponsesCompactChannelType(ability.ChannelType) {
			return true
		}
		ability = AbilityWithChannel{}
	}
	return false
}

func isChannelIDInList(list []int, channelID int) bool {
	for _, id := range list {
		if id == channelID {
			return true
		}
	}
	return false
}
