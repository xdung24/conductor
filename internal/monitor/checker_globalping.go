package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xdung24/service-monitor/internal/models"
)

const globalpingAPIBase = "https://api.globalping.io/v1"

// GlobalpingChecker measures reachability of a host via the Globalping public
// distributed network.
//
// The monitor URL is used as the measurement target (hostname or IP).
// The check always performs a ping measurement and polls for the result.
type GlobalpingChecker struct{}

type globalpingCreateReq struct {
	Type   string                 `json:"type"`
	Target string                 `json:"target"`
	Limit  int                    `json:"limit"`
	Opts   map[string]interface{} `json:"measurementOptions,omitempty"`
}

type globalpingCreateResp struct {
	ID string `json:"id"`
}

type globalpingResult struct {
	Status  string `json:"status"`
	Results []struct {
		Result struct {
			Status    string `json:"status"`
			RawOutput string `json:"rawOutput"`
			Stats     *struct {
				Loss float64 `json:"loss"`
				Avg  float64 `json:"avg"`
			} `json:"stats"`
		} `json:"result"`
	} `json:"results"`
}

// Check submits a Globalping ping measurement and waits for a result.
func (c *GlobalpingChecker) Check(ctx context.Context, m *models.Monitor) Result {
	target := m.URL
	if target == "" {
		return Result{Status: 0, Message: "url (target hostname) is required"}
	}

	client := &http.Client{}

	// Submit measurement.
	body, _ := json.Marshal(globalpingCreateReq{
		Type:   "ping",
		Target: target,
		Limit:  1,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		globalpingAPIBase+"/measurements", bytes.NewReader(body))
	if err != nil {
		return Result{Status: 0, Message: "build request: " + err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return Result{Status: 0, Message: "submit: " + err.Error()}
	}
	resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return Result{Status: 0, Message: fmt.Sprintf("Globalping API returned HTTP %d", resp.StatusCode)}
	}

	respBody, _ := io.ReadAll(resp.Body)
	var created globalpingCreateResp
	if err := json.Unmarshal(respBody, &created); err != nil || created.ID == "" {
		// Re-read from the already-closed body by re-issuing request is not
		// possible; the ID is in the Location header as well.
		loc := resp.Header.Get("Location")
		if loc != "" {
			parts := strings.Split(strings.TrimRight(loc, "/"), "/")
			created.ID = parts[len(parts)-1]
		}
	}

	if created.ID == "" {
		return Result{Status: 0, Message: "could not obtain measurement ID from Globalping"}
	}

	// Poll for completion (up to ~10 s).
	pollURL := fmt.Sprintf("%s/measurements/%s", globalpingAPIBase, created.ID)
	for attempts := 0; attempts < 20; attempts++ {
		select {
		case <-ctx.Done():
			return Result{Status: 0, Message: "context deadline exceeded waiting for Globalping result"}
		case <-time.After(500 * time.Millisecond):
		}

		pollReq, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
		if err != nil {
			continue
		}
		pollResp, err := client.Do(pollReq)
		if err != nil {
			continue
		}
		pollBody, _ := io.ReadAll(pollResp.Body)
		pollResp.Body.Close() //nolint:errcheck

		var gr globalpingResult
		if err := json.Unmarshal(pollBody, &gr); err != nil {
			continue
		}
		if gr.Status != "finished" && gr.Status != "failed" {
			continue
		}

		// Measurement done.
		if len(gr.Results) == 0 || gr.Status == "failed" {
			return Result{Status: 0, Message: "Globalping measurement failed — no results"}
		}

		r0 := gr.Results[0].Result
		if r0.Status != "finished" {
			return Result{Status: 0, Message: "probe result status: " + r0.Status}
		}
		if r0.Stats != nil && r0.Stats.Loss >= 100 {
			return Result{Status: 0, Message: fmt.Sprintf("100%% packet loss to %s", target)}
		}
		msg := fmt.Sprintf("Globalping ping OK — target: %s", target)
		if r0.Stats != nil {
			msg += fmt.Sprintf(", avg %.2f ms, loss %.0f%%", r0.Stats.Avg, r0.Stats.Loss)
		}
		return Result{Status: 1, Message: msg}
	}

	return Result{Status: 0, Message: "timed out waiting for Globalping result"}
}
