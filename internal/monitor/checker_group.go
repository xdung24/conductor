package monitor

import (
	"context"
	"fmt"

	"github.com/xdung24/service-monitor/internal/models"
)

// GroupChildrenStatusLookup is an optional callback that returns the last_status
// values (1=UP, 0=DOWN) for all monitors whose parent_id equals parentID.
// Set this when starting the per-user scheduler so the GroupChecker can derive
// the aggregate status from its children.
var GroupChildrenStatusLookup func(parentID int64) []int

// GroupChecker derives its status from the last-known status of all child
// monitors (monitors that have parent_id = this monitor's ID).
//
// Status rules:
//   - All children UP   → group is UP
//   - Any child DOWN    → group is DOWN
//   - No children found → group is UP (treated as empty / no-op)
type GroupChecker struct{}

// Check returns UP if all children are UP, DOWN if any child is DOWN.
func (c *GroupChecker) Check(_ context.Context, m *models.Monitor) Result {
	if GroupChildrenStatusLookup == nil {
		return Result{Status: 1, Message: "group (children lookup not wired)"}
	}

	statuses := GroupChildrenStatusLookup(m.ID)
	if len(statuses) == 0 {
		return Result{Status: 1, Message: "group: no child monitors"}
	}

	down := 0
	for _, s := range statuses {
		if s != 1 {
			down++
		}
	}
	total := len(statuses)
	if down == 0 {
		return Result{Status: 1, Message: fmt.Sprintf("group: %d/%d monitors UP", total, total)}
	}
	return Result{Status: 0, Message: fmt.Sprintf("group: %d/%d monitors DOWN", down, total)}
}
