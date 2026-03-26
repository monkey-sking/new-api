package model

import "github.com/QuantumNous/new-api/common"

// PriorityRetryIndex converts the global retry count into a priority-bucket index.
// When SamePriorityRetryTimes is 2:
// retry 0,1 -> priority 0
// retry 2,3 -> priority 1
func PriorityRetryIndex(retry int) int {
	attemptsPerPriority := common.SamePriorityRetryTimes
	if attemptsPerPriority <= 0 {
		attemptsPerPriority = 1
	}
	if retry <= 0 {
		return 0
	}
	return retry / attemptsPerPriority
}
