package model

type ChannelSelectOptions struct {
	AdjustPriority func(channelID int, basePriority int64) int64
}

func (o *ChannelSelectOptions) EffectivePriority(channelID int, basePriority int64) int64 {
	if o == nil || o.AdjustPriority == nil {
		return basePriority
	}
	return o.AdjustPriority(channelID, basePriority)
}
