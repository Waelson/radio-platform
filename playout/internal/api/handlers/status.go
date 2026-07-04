package handlers

import (
	"net/http"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/state"
)

// statusNowPlaying mirrors state.NowPlaying with JSON tags matching the API spec.
type statusNowPlaying struct {
	QueueItemID string            `json:"queue_item_id"`
	AssetID     string            `json:"asset_id"`
	Path        string            `json:"path"`
	Title       string            `json:"title"`
	Artist      string            `json:"artist"`
	Type        string            `json:"type"`
	DurationMS  int64             `json:"duration_ms"`
	PositionMS  int64             `json:"position_ms"`
	Percent     float64           `json:"percent"`
	Transition  *statusTransition `json:"transition,omitempty"`
}

type statusTransition struct {
	Type       string `json:"type"`
	DurationMS int64  `json:"duration_ms"`
}

// statusQueue carries queue metadata in the status response.
type statusQueue struct {
	Size       int    `json:"size"`
	NextItemID string `json:"next_item_id,omitempty"`
}

// statusAudioHealth carries audio health metrics in the status response.
type statusAudioHealth struct {
	LevelDBFS     float64 `json:"level_dbfs"`
	PeakDBFS      float64 `json:"peak_dbfs"`
	Silence       bool    `json:"silence"`
	BufferPct     int     `json:"buffer_pct"`
	UnderrunCount int64   `json:"underrun_count"`
}

// statusLastCommand carries the last command result in the status response.
type statusLastCommand struct {
	Command  string    `json:"command"`
	Status   string    `json:"status"` // "ACCEPTED" | "REJECTED"
	At       time.Time `json:"at"`
}

// statusResponse is the full body for GET /v1/status.
type statusResponse struct {
	EngineID    string             `json:"engine_id"`
	State       string             `json:"state"`
	Mode        string             `json:"mode"`
	Panic       bool               `json:"panic"`
	NowPlaying  *statusNowPlaying  `json:"now_playing,omitempty"`
	Queue       statusQueue        `json:"queue"`
	AudioHealth statusAudioHealth  `json:"audio_health"`
	LastCommand *statusLastCommand `json:"last_command,omitempty"`
	ErrorMsg    string             `json:"error,omitempty"`
}

// Status returns a handler for GET /v1/status.
func Status(stateMgr *state.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap := stateMgr.Snapshot()

		resp := statusResponse{
			EngineID: snap.EngineID,
			State:    string(snap.State),
			Mode:     string(snap.Mode),
			Panic:    snap.Panic,
			Queue: statusQueue{
				Size:       snap.Queue.Size,
				NextItemID: snap.Queue.NextItemID,
			},
			AudioHealth: statusAudioHealth{
				LevelDBFS:     snap.AudioHealth.LevelDBFS,
				PeakDBFS:      snap.AudioHealth.PeakDBFS,
				Silence:       snap.AudioHealth.Silence,
				BufferPct:     snap.AudioHealth.BufferPct,
				UnderrunCount: snap.AudioHealth.UnderrunCount,
			},
			ErrorMsg: snap.ErrorMsg,
		}

		if snap.NowPlaying != nil {
			np := &statusNowPlaying{
				QueueItemID: snap.NowPlaying.QueueItemID,
				AssetID:     snap.NowPlaying.AssetID,
				Path:        snap.NowPlaying.Path,
				Title:       snap.NowPlaying.Title,
				Artist:      snap.NowPlaying.Artist,
				Type:        snap.NowPlaying.Type,
				DurationMS:  snap.NowPlaying.DurationMS,
				PositionMS:  snap.NowPlaying.PositionMS,
				Percent:     snap.NowPlaying.Percent,
			}
			if snap.NowPlaying.Transition != nil {
				np.Transition = &statusTransition{
					Type:       snap.NowPlaying.Transition.Type,
					DurationMS: snap.NowPlaying.Transition.DurationMS,
				}
			}
			resp.NowPlaying = np
		}

		if snap.LastCommand != nil {
			status := "ACCEPTED"
			if !snap.LastCommand.Accepted {
				status = "REJECTED"
			}
			resp.LastCommand = &statusLastCommand{
				Command: snap.LastCommand.Command,
				Status:  status,
				At:      snap.LastCommand.At,
			}
		}

		writeJSON(w, http.StatusOK, resp)
	}
}
