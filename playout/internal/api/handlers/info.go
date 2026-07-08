package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"
)

type infoResponse struct {
	EngineID    string    `json:"engine_id"`
	PID         int       `json:"pid"`
	Version     string    `json:"version"`
	StartTime   time.Time `json:"start_time"`
	LocalIP     string    `json:"local_ip"`
	OS          string    `json:"os"`
	AudioDriver string    `json:"audio_driver"`
}

// Info returns a handler for GET /v1/info that exposes engine identity,
// PID, version, start time, local network IP and the compiled audio driver.
// Used by the client-side status SPA to compute uptime and display process information.
func Info(engineID, version string, startTime time.Time, audioDriver string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(infoResponse{
			EngineID:    engineID,
			PID:         os.Getpid(),
			OS:          runtime.GOOS + "/" + runtime.GOARCH,
			Version:     version,
			StartTime:   startTime,
			LocalIP:     localNetworkIP(),
			AudioDriver: audioDriver,
		})
	}
}

// localNetworkIP returns the machine's preferred outbound IP address by
// dialing a UDP connection (no data is actually sent).
func localNetworkIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}
