package streaming

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"time"
)

const statsTimeout = 5 * time.Second

// icestats is the root element of the Icecast /admin/stats XML response.
type icestats struct {
	Sources []icecastSource `xml:"source"`
}

// icecastSource represents a single mount point in the stats XML.
type icecastSource struct {
	Mount     string `xml:"mount,attr"`
	Listeners int    `xml:"listeners"`
}

// FetchIcecastListeners fetches the current listener count for the given
// target's mount from Icecast's /admin/stats XML endpoint.
//
// Authentication: Basic Auth with user "admin" and the target's Password.
// (Many Icecast setups share the same source/admin password. If the server
// returns 401/403 the error is returned and the caller should skip the target.)
//
// SHOUTcast v1 targets are skipped (returns 0, nil) because their stats
// API differs and requires a separate implementation.
func FetchIcecastListeners(ctx context.Context, cfg TargetConfig) (int, error) {
	if cfg.Type == "shoutcast_v1" {
		return 0, nil // SHOUTcast v1 stats API differs; silently skip
	}

	url := fmt.Sprintf("http://%s:%d/admin/stats", cfg.Host, cfg.Port)
	reqCtx, cancel := context.WithTimeout(ctx, statsTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("streaming: stats: build request: %w", err)
	}
	req.SetBasicAuth("admin", cfg.Password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("streaming: stats: request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("streaming: stats: icecast returned HTTP %d", resp.StatusCode)
	}

	var stats icestats
	if err := xml.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return 0, fmt.Errorf("streaming: stats: parse XML: %w", err)
	}

	mount := cfg.Mount
	if mount == "" {
		mount = "/stream"
	}
	for _, src := range stats.Sources {
		if src.Mount == mount {
			return src.Listeners, nil
		}
	}
	// Mount not found in stats — no active listeners yet.
	return 0, nil
}
