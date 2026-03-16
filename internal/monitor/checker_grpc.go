package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/xdung24/service-monitor/internal/models"
)

// ---------------------------------------------------------------------------
// gRPC Keyword checker
// ---------------------------------------------------------------------------

// GRPCChecker performs a gRPC health check against the standard
// grpc.health.v1.Health/Check protocol. The target service name can be
// specified via GRPCServiceName (empty = default unnamed service).
//
// If HTTPKeyword is set the response's ServingStatus string is also checked
// for keyword presence (or absence when HTTPKeywordInvert is true).
//
// Monitor field usage:
//
//	URL              — gRPC target, e.g. "host:50051"
//	GRPCServiceName  — service name for the health check request; empty = ""
//	GRPCEnableTLS    — use TLS when connecting
//	HTTPIgnoreTLS    — skip TLS certificate verification (only when GRPCEnableTLS=true)
//	HTTPKeyword      — optional keyword to assert in the status string
//	HTTPKeywordInvert — invert keyword assertion
type GRPCChecker struct{}

// Check connects to the gRPC endpoint and calls the standard Health/Check RPC.
func (c *GRPCChecker) Check(ctx context.Context, m *models.Monitor) Result {
	start := time.Now()

	target := m.URL
	// Strip ws:// or http:// schemes users may accidentally include.
	for _, prefix := range []string{"http://", "https://", "grpc://"} {
		target = strings.TrimPrefix(target, prefix)
	}

	var creds credentials.TransportCredentials
	if m.GRPCEnableTLS {
		tlsCfg := &tls.Config{
			InsecureSkipVerify: m.HTTPIgnoreTLS, // #nosec G402 -- user opt-in
		}
		creds = credentials.NewTLS(tlsCfg)
	} else {
		creds = insecure.NewCredentials()
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, time.Duration(m.TimeoutSeconds)*time.Second)
	defer dialCancel()

	conn, err := grpc.DialContext(dialCtx, target, //nolint:staticcheck // DialContext deprecated in 1.69 but still works
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
	)
	if err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("connect: %v", err)}
	}
	defer conn.Close() //nolint:errcheck

	client := grpc_health_v1.NewHealthClient(conn)
	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: m.GRPCServiceName,
	})
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return Result{Status: 0, LatencyMs: latency, Message: fmt.Sprintf("health check: %v", err)}
	}

	statusStr := resp.GetStatus().String()

	// Keyword assertion against the status string.
	if m.HTTPKeyword != "" {
		contains := strings.Contains(statusStr, m.HTTPKeyword)
		if contains == m.HTTPKeywordInvert {
			msg := fmt.Sprintf("keyword %q", m.HTTPKeyword)
			if m.HTTPKeywordInvert {
				msg += " found but should be absent"
			} else {
				msg += " not found in response"
			}
			return Result{Status: 0, LatencyMs: latency, Message: msg}
		}
	}

	if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		return Result{
			Status:    0,
			LatencyMs: latency,
			Message:   fmt.Sprintf("service not serving: %s", statusStr),
		}
	}

	return Result{
		Status:    1,
		LatencyMs: latency,
		Message:   fmt.Sprintf("gRPC OK: %s", statusStr),
	}
}
