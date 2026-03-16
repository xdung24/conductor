package monitor

import (
	"context"

	"github.com/xdung24/service-monitor/internal/models"
)

// ManualChecker returns the user-set status stored in m.ManualStatus.
// Status 1 = UP, 0 = DOWN.  The user controls the status via the UI.
type ManualChecker struct{}

// Check returns the stored manual status without performing any network check.
func (c *ManualChecker) Check(_ context.Context, m *models.Monitor) Result {
	if m.ManualStatus == 1 {
		return Result{Status: 1, Message: "manual: UP (user-set)"}
	}
	return Result{Status: 0, Message: "manual: DOWN (user-set)"}
}
