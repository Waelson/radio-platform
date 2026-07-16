package streaming

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const metadataTimeout = 5 * time.Second

// UpdateMetadata sends the current now-playing title and artist to the
// Icecast or SHOUTcast server of the given target via its HTTP admin API.
//
// For Icecast (and SHOUTcast v2): GET /admin/metadata with Basic Auth
// (user "source", password from cfg).
// For SHOUTcast v1: GET /admin.cgi with the password as a query parameter.
//
// The song string sent to the server is "Artist - Title" when both are
// non-empty, or whichever is available.
func UpdateMetadata(ctx context.Context, cfg TargetConfig, title, artist string) error {
	song := buildSong(title, artist)
	switch cfg.Type {
	case "shoutcast_v1":
		return updateMetadataSHOUTcast(ctx, cfg, song)
	default: // "icecast", "shoutcast_v2"
		return updateMetadataIcecast(ctx, cfg, song)
	}
}

// buildSong formats the song string from title and artist.
func buildSong(title, artist string) string {
	switch {
	case artist != "" && title != "":
		return artist + " - " + title
	case artist != "":
		return artist
	default:
		return title
	}
}

// updateMetadataIcecast sends a metadata update to an Icecast 2 (or
// SHOUTcast v2) server using Basic Auth with the "source" user.
func updateMetadataIcecast(ctx context.Context, cfg TargetConfig, song string) error {
	mount := cfg.Mount
	if mount == "" {
		mount = "/stream"
	}

	u := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   "/admin/metadata",
		RawQuery: url.Values{
			"mount": {mount},
			"mode":  {"updinfo"},
			"song":  {song},
		}.Encode(),
	}

	reqCtx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("streaming: metadata: build request: %w", err)
	}
	req.SetBasicAuth("source", cfg.Password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("streaming: metadata: request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return fmt.Errorf("streaming: metadata: icecast returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// updateMetadataSHOUTcast sends a metadata update to a SHOUTcast v1 server.
// SHOUTcast v1 uses GET /admin.cgi with the password as a query parameter
// (no Basic Auth).
func updateMetadataSHOUTcast(ctx context.Context, cfg TargetConfig, song string) error {
	u := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   "/admin.cgi",
		RawQuery: url.Values{
			"pass": {cfg.Password},
			"mode": {"updinfo"},
			"song": {song},
		}.Encode(),
	}

	reqCtx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("streaming: metadata: shoutcast build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("streaming: metadata: shoutcast request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return fmt.Errorf("streaming: metadata: shoutcast returned HTTP %d", resp.StatusCode)
	}
	return nil
}
