package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/a2d2-dev/claude-usage-monitor/internal/auth"
)

// UploadPayload is the JSON body sent to POST /api/upload.
// It contains only aggregated statistics — no prompts or file paths.
type UploadPayload struct {
	// Period is the month in YYYY-MM format.
	Period string `json:"period"`
	// DeviceID uniquely identifies this machine.
	DeviceID string `json:"device_id"`
	// DeviceName is the optional human-readable device label.
	DeviceName string `json:"device_name"`
	// TotalCostUSD is the total spend for the period on this device.
	TotalCostUSD float64 `json:"total_cost_usd"`
	// TotalTokens is the sum of all token types.
	TotalTokens int `json:"total_tokens"`
	// InputTokens is the number of input tokens.
	InputTokens int `json:"input_tokens"`
	// OutputTokens is the number of output tokens.
	OutputTokens int `json:"output_tokens"`
	// CacheReadTokens is the number of cache read tokens.
	CacheReadTokens int `json:"cache_read_tokens"`
	// CacheWriteTokens is the number of cache creation tokens.
	CacheWriteTokens int `json:"cache_write_tokens"`
	// SessionCount is the number of completed 5-hour session blocks.
	SessionCount int `json:"session_count"`
	// ModelBreakdown maps model name to its share of cost and tokens.
	ModelBreakdown map[string]*ModelMonthlyStats `json:"model_breakdown"`
}

// UploadResponse is the JSON response from POST /api/upload.
type UploadResponse struct {
	// Rank is the user's current global rank for the period.
	Rank int `json:"rank"`
	// TotalUsers is the total number of users with data for the period.
	TotalUsers int `json:"total_users"`
	// ShareURL is the permanent shareable URL for the user's stats page.
	ShareURL string `json:"share_url"`
}

// Upload sends monthly stats to the backend and returns the upload response.
// It requires a valid JWT (from auth.LoadAuth) and a known device identity.
//
// Parameters:
//   - ctx: request context
//   - jwt: the bearer token issued by /auth/verify
//   - device: the device identity from auth.EnsureDevice
//   - stats: the monthly aggregated stats from AggregateCurrentMonth
func Upload(ctx context.Context, jwt string, device *auth.DeviceInfo, stats *MonthlyStats) (*UploadResponse, error) {
	payload := UploadPayload{
		Period:           stats.Period,
		DeviceID:         device.DeviceID,
		DeviceName:       device.DeviceName,
		TotalCostUSD:     stats.TotalCostUSD,
		TotalTokens:      stats.TotalTokens(),
		InputTokens:      stats.InputTokens,
		OutputTokens:     stats.OutputTokens,
		CacheReadTokens:  stats.CacheReadTokens,
		CacheWriteTokens: stats.CacheWriteTokens,
		SessionCount:     stats.SessionCount,
		ModelBreakdown:   stats.ModelBreakdown,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal upload payload: %w", err)
	}

	url := auth.APIBase + "/api/upload"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST /api/upload: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upload response: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("JWT 已过期，请重新认证 (u 键)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload failed (HTTP %d): %s", resp.StatusCode, respBody)
	}

	var result UploadResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse upload response: %w", err)
	}
	return &result, nil
}
