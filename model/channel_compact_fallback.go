package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func responsesCompactFallbackBaseModel(modelName string) (string, bool) {
	if !strings.HasSuffix(modelName, ratio_setting.CompactModelSuffix) {
		return "", false
	}
	baseModel := strings.TrimSuffix(modelName, ratio_setting.CompactModelSuffix)
	if baseModel == "" {
		return "", false
	}
	return baseModel, true
}

func responsesCompactFallbackModels(modelName string) []string {
	baseModel, ok := responsesCompactFallbackBaseModel(modelName)
	if !ok {
		return nil
	}
	models := []string{baseModel}
	if normalized := ratio_setting.FormatMatchingModelName(baseModel); normalized != "" && normalized != baseModel {
		models = append(models, normalized)
	}
	return models
}

func supportsResponsesCompactChannelType(channelType int) bool {
	apiType, _ := common.ChannelType2APIType(channelType)
	return apiType == constant.APITypeOpenAI || apiType == constant.APITypeCodex
}

func supportsResponsesCompactChannel(channel *Channel) bool {
	return channel != nil && supportsResponsesCompactChannelType(channel.Type)
}
