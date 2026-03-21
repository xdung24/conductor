package monitor

import (
	"fmt"
	"testing"

	"github.com/xdung24/conductor/internal/models"
)

func TestCheckerFor(t *testing.T) {
	tests := []struct {
		monitorType models.MonitorType
		wantType    string
	}{
		{models.MonitorTypeDNS, "*monitor.DNSChecker"},
		{models.MonitorTypeTCP, "*monitor.TCPChecker"},
		{models.MonitorTypePing, "*monitor.PingChecker"},
		{models.MonitorTypeHTTP, "*monitor.HTTPChecker"},
		{models.MonitorTypePush, "*monitor.HTTPChecker"}, // push falls to default
		{"unknown", "*monitor.HTTPChecker"},              // unknown type falls to default
	}
	for _, tt := range tests {
		m := &models.Monitor{Type: tt.monitorType}
		got := checkerFor(nil, m)
		gotType := fmt.Sprintf("%T", got)
		if gotType != tt.wantType {
			t.Errorf("checkerFor(%q) = %s, want %s", tt.monitorType, gotType, tt.wantType)
		}
	}
}
