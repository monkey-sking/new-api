package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestCountEnabledChannels(t *testing.T) {
	channels := []*model.Channel{
		{Id: 1, Type: 1, Status: common.ChannelStatusEnabled},
		{Id: 2, Type: 1, Status: common.ChannelStatusAutoDisabled},
		nil,
		{Id: 3, Type: 2, Status: common.ChannelStatusEnabled},
	}

	if got := countEnabledChannels(channels); got != 2 {
		t.Fatalf("expected 2 enabled channels, got %d", got)
	}

	if got := countEnabledChannelsByType(channels, 1); got != 1 {
		t.Fatalf("expected 1 enabled channel for type 1, got %d", got)
	}

	if got := countEnabledChannelsByType(channels, 2); got != 1 {
		t.Fatalf("expected 1 enabled channel for type 2, got %d", got)
	}
}

func TestShouldProtectLastEnabledChannelFromHealthDisable(t *testing.T) {
	lastEnabledOfType := &model.Channel{Id: 1, Type: 1, Status: common.ChannelStatusEnabled}
	anotherEnabledOfSameType := &model.Channel{Id: 2, Type: 1, Status: common.ChannelStatusEnabled}
	lastEnabledOfDifferentType := &model.Channel{Id: 3, Type: 2, Status: common.ChannelStatusEnabled}
	autoDisabled := &model.Channel{Id: 4, Type: 1, Status: common.ChannelStatusAutoDisabled}

	if !shouldProtectLastEnabledChannelFromHealthDisable(lastEnabledOfType, 1) {
		t.Fatal("expected last enabled channel of the same type to be protected")
	}
	if shouldProtectLastEnabledChannelFromHealthDisable(anotherEnabledOfSameType, 2) {
		t.Fatal("did not expect protection when multiple enabled channels of the same type remain")
	}
	if shouldProtectLastEnabledChannelFromHealthDisable(autoDisabled, 1) {
		t.Fatal("did not expect auto-disabled channel to be protected")
	}
	if shouldProtectLastEnabledChannelFromHealthDisable(nil, 1) {
		t.Fatal("did not expect nil channel to be protected")
	}
	if !shouldProtectLastEnabledChannelFromHealthDisable(lastEnabledOfDifferentType, 1) {
		t.Fatal("expected last enabled channel of a different type to be protected independently")
	}
}
