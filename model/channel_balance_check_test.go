package model

import "testing"

func TestChannelGetBalanceCheckDefaultsToUnchecked(t *testing.T) {
	channel := &Channel{}

	check := channel.GetBalanceCheck()

	if check == nil {
		t.Fatal("expected balance check")
	}
	if check.Status != ChannelBalanceCheckStatusUnchecked {
		t.Fatalf("expected unchecked status, got %s", check.Status)
	}
	if check.CheckedAt != 0 {
		t.Fatalf("expected zero checked_at, got %d", check.CheckedAt)
	}
}

func TestChannelGetBalanceCheckFallsBackToSuccessFromBalanceTimestamp(t *testing.T) {
	channel := &Channel{
		BalanceUpdatedTime: 1234567890,
	}

	check := channel.GetBalanceCheck()

	if check == nil {
		t.Fatal("expected balance check")
	}
	if check.Status != ChannelBalanceCheckStatusSuccess {
		t.Fatalf("expected success status, got %s", check.Status)
	}
	if check.CheckedAt != 1234567890 {
		t.Fatalf("expected checked_at to use balance_updated_time, got %d", check.CheckedAt)
	}
}

func TestChannelGetBalanceCheckReadsOtherInfo(t *testing.T) {
	channel := &Channel{}
	channel.SetOtherInfo(map[string]interface{}{
		channelOtherInfoBalanceCheckStatus:  ChannelBalanceCheckStatusFailed,
		channelOtherInfoBalanceCheckTime:    int64(987654321),
		channelOtherInfoBalanceCheckTrigger: "auto",
		channelOtherInfoBalanceCheckReason:  "尚未实现",
	})

	check := channel.GetBalanceCheck()

	if check == nil {
		t.Fatal("expected balance check")
	}
	if check.Status != ChannelBalanceCheckStatusFailed {
		t.Fatalf("expected failed status, got %s", check.Status)
	}
	if check.CheckedAt != 987654321 {
		t.Fatalf("expected checked_at 987654321, got %d", check.CheckedAt)
	}
	if check.Trigger != "auto" {
		t.Fatalf("expected trigger auto, got %s", check.Trigger)
	}
	if check.Reason != "尚未实现" {
		t.Fatalf("expected reason 尚未实现, got %s", check.Reason)
	}
}
