package service

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var nowFunc = time.Now

type channelWindowTracker struct {
	sync.Mutex
	events map[string][]time.Time
}

func newChannelWindowTracker() channelWindowTracker {
	return channelWindowTracker{
		events: map[string][]time.Time{},
	}
}

func (t *channelWindowTracker) observe(key string, window time.Duration) int {
	t.Lock()
	defer t.Unlock()

	now := nowFunc()
	events := pruneChannelWindowEvents(t.events[key], now, window)
	events = append(events, now)
	t.events[key] = events
	return len(events)
}

func (t *channelWindowTracker) prune(key string, window time.Duration) {
	t.Lock()
	defer t.Unlock()

	events := pruneChannelWindowEvents(t.events[key], nowFunc(), window)
	if len(events) == 0 {
		delete(t.events, key)
		return
	}
	t.events[key] = events
}

func (t *channelWindowTracker) clear(key string) {
	t.Lock()
	delete(t.events, key)
	t.Unlock()
}

func (t *channelWindowTracker) count(key string, window time.Duration) int {
	t.Lock()
	defer t.Unlock()

	events := pruneChannelWindowEvents(t.events[key], nowFunc(), window)
	if len(events) == 0 {
		delete(t.events, key)
		return 0
	}
	t.events[key] = events
	return len(events)
}

func (t *channelWindowTracker) snapshot(window time.Duration) map[string]int {
	t.Lock()
	defer t.Unlock()

	now := nowFunc()
	snapshot := make(map[string]int, len(t.events))
	for key, events := range t.events {
		pruned := pruneChannelWindowEvents(events, now, window)
		if len(pruned) == 0 {
			delete(t.events, key)
			continue
		}
		t.events[key] = pruned
		snapshot[key] = len(pruned)
	}
	return snapshot
}

func (t *channelWindowTracker) relieve(key string, window time.Duration) int {
	t.Lock()
	defer t.Unlock()

	events := pruneChannelWindowEvents(t.events[key], nowFunc(), window)
	if len(events) == 0 {
		delete(t.events, key)
		return 0
	}
	if len(events) == 1 {
		delete(t.events, key)
		return 0
	}
	events = events[1:]
	t.events[key] = events
	return len(events)
}

func pruneChannelWindowEvents(events []time.Time, now time.Time, window time.Duration) []time.Time {
	if len(events) == 0 {
		return nil
	}
	if window <= 0 {
		return nil
	}

	cutoff := now.Add(-window)
	kept := make([]time.Time, 0, len(events))
	for _, eventTime := range events {
		if eventTime.Before(cutoff) {
			continue
		}
		kept = append(kept, eventTime)
	}
	return kept
}

func channelDisableThresholdWindow() time.Duration {
	windowSeconds := common.AutomaticDisableThresholdWindowSeconds
	if windowSeconds <= 0 {
		windowSeconds = 300
	}
	return time.Duration(windowSeconds) * time.Second
}

func channelSoftDegradeWindow() time.Duration {
	windowSeconds := common.ChannelSoftDegradeWindowSeconds
	if windowSeconds <= 0 {
		windowSeconds = 86400
	}
	return time.Duration(windowSeconds) * time.Second
}
