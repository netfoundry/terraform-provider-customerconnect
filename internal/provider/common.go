package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var errNotFound = errors.New("resource not found")

// apiTimestamp represents a timestamp field returned by the NetFoundry API.
// The API normally returns RFC3339 strings, but some endpoints return a
// numeric Unix epoch (seconds or milliseconds) instead. This type accepts
// either encoding and normalizes it to an RFC3339 string.
type apiTimestamp string

func (t *apiTimestamp) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if string(data) == "null" {
		*t = ""
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*t = apiTimestamp(s)
		return nil
	}

	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("apiTimestamp: %w", err)
	}
	i, err := n.Int64()
	if err != nil {
		// Not an integer epoch we recognize; keep the raw numeric text
		// rather than failing the whole response.
		*t = apiTimestamp(n.String())
		return nil
	}
	sec, nsec := i, int64(0)
	if i > 1e12 { // value is in milliseconds, not seconds
		sec = i / 1000
		nsec = (i % 1000) * int64(time.Millisecond)
	}
	*t = apiTimestamp(time.Unix(sec, nsec).UTC().Format(time.RFC3339Nano))
	return nil
}

// doRequest executes an authenticated HTTP request against the NetFoundry API.
// A non-nil body is sent with Content-Type: application/json.
// Returns the raw response body, HTTP status code, and any error.
// 404 responses are mapped to errNotFound; other 4xx/5xx responses return an error.
func doRequest(ctx context.Context, method, url, token string, body []byte) ([]byte, int, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	var req *http.Request
	var err error
	if len(body) > 0 {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	tflog.Debug(ctx, "NetFoundry API request", map[string]any{
		"method": method,
		"url":    url,
		"body":   string(body),
	})

	resp, err := client.Do(req)
	if err != nil {
		tflog.Debug(ctx, "NetFoundry API request error", map[string]any{
			"method": method,
			"url":    url,
			"error":  err.Error(),
		})
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", err)
	}

	tflog.Debug(ctx, "NetFoundry API response", map[string]any{
		"method": method,
		"url":    url,
		"status": resp.StatusCode,
		"body":   string(respBody),
	})

	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, errNotFound
	}
	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
}

// stringValOrNull returns types.StringNull() for an empty string, otherwise types.StringValue(s).
// This ensures optional fields not set by the API are represented as null rather than "",
// which would conflict with a Terraform plan that has those fields as null.
func stringValOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// stringSliceOrNil returns nil for an empty slice, otherwise s unchanged.
// This ensures optional list fields not set by the API are represented as a
// null list rather than an empty one, which would conflict with a Terraform
// plan that has those fields as null.
func stringSliceOrNil(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}
