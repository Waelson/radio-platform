// Package api provides the HTTP API server for the Playout Engine.
// It must not import mixer, output, or any audio package.
// All state reads go through state.Manager; all mutations go via commands.Bus.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/api/handlers"
	"github.com/Waelson/radio-playout-engine/internal/api/ws"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	appcfg "github.com/Waelson/radio-playout-engine/internal/config"
	"github.com/Waelson/radio-playout-engine/internal/metrics"
	"github.com/Waelson/radio-playout-engine/internal/queue"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// Config holds the API server configuration (subset of the full app config).
type Config struct {
	Host           string
	Port           int
	AllowedOrigins []string
	EngineID       string
	Version        string
	StartTime      time.Time // used by the /status SPA and /v1/info
	AudioDriver    string    // driver compiled into this binary: "coreaudio" | "portaudio" | "wasapi" | "null"
}

// PreviewDeps carries optional preview player dependencies for the API server.
// When Enabled is false all /v1/preview/* endpoints return 503.
type PreviewDeps struct {
	Enabled   bool
	GetStatus func() any // returns preview.Status as any; nil when disabled
}

// DevicesDeps carries the device-listing function for GET /v1/devices.
// When List is nil the endpoint returns an empty list with 200 OK.
type DevicesDeps struct {
	List func() ([]handlers.AudioDevice, error)
}

// ScheduleDeps carries the scheduler manager for the /v1/schedule/* endpoints.
// When Mgr is nil all schedule endpoints return 404.
type ScheduleDeps struct {
	Mgr handlers.ScheduleManager
}

// ConfigDeps carries config-related dependencies for the config endpoints.
// When Snapshot is nil, GET /v1/config/current and PUT /v1/config are not registered.
type ConfigDeps struct {
	Snapshot *appcfg.Config
	Path     string // absolute path to the YAML file; empty = read-only mode
}

// Server wraps an http.Server and owns the routing for the Engine's REST API.
type Server struct {
	cfg            Config
	stateMgr       *state.Manager
	cmdBus         *commands.Bus
	queueMgr       *queue.Manager
	wsHub          *ws.Hub
	metrics        *metrics.Collector
	previewEnabled bool
	previewStatus  func() any
	listDevices    func() ([]handlers.AudioDevice, error)
	scheduleMgr    handlers.ScheduleManager
	configSnapshot *appcfg.Config
	configPath     string
	log            *slog.Logger
	httpSrv        *http.Server
}

// New creates a Server wired to stateMgr for reads and cmdBus for writes.
// queueMgr may be nil; queue endpoints will be unavailable until it is set.
// wsHub may be nil; the /v1/events endpoint will not be registered.
// col may be nil; the /v1/metrics endpoint will not be registered.
// devicesDeps.List may be nil; GET /v1/devices will return an empty list.
func New(cfg Config, stateMgr *state.Manager, cmdBus *commands.Bus, queueMgr *queue.Manager, wsHub *ws.Hub, col *metrics.Collector, previewDeps PreviewDeps, devicesDeps DevicesDeps, scheduleDeps ScheduleDeps, configDeps ConfigDeps, log *slog.Logger) *Server {
	s := &Server{
		cfg:            cfg,
		stateMgr:       stateMgr,
		cmdBus:         cmdBus,
		queueMgr:       queueMgr,
		wsHub:          wsHub,
		metrics:        col,
		previewEnabled: previewDeps.Enabled,
		previewStatus:  previewDeps.GetStatus,
		listDevices:    devicesDeps.List,
		scheduleMgr:    scheduleDeps.Mgr,
		configSnapshot: configDeps.Snapshot,
		configPath:     configDeps.Path,
		log:            log,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	handler := s.withMiddleware(mux)

	s.httpSrv = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Observability
	mux.HandleFunc("GET /v1/health", handlers.Health(s.stateMgr))
	mux.HandleFunc("GET /v1/ready", handlers.Ready(s.stateMgr))
	mux.HandleFunc("GET /v1/status", handlers.Status(s.stateMgr))
	mux.HandleFunc("GET /v1/build", handlers.Build(s.cfg.Version))
	mux.HandleFunc("GET /v1/info", handlers.Info(s.cfg.EngineID, s.cfg.Version, s.cfg.StartTime, s.cfg.AudioDriver))
	mux.HandleFunc("GET /status", handlers.StatusHTML(s.cfg.Port, s.cfg.Version, s.cfg.StartTime, s.stateMgr))

	// Queue
	if s.queueMgr != nil {
		mux.HandleFunc("GET /v1/queue", handlers.QueueList(s.queueMgr))
		mux.HandleFunc("POST /v1/queue/enqueue", handlers.Enqueue(s.cmdBus, s.queueMgr))
		mux.HandleFunc("POST /v1/queue/enqueue-break", handlers.EnqueueBreak(s.cmdBus, s.queueMgr))
		mux.HandleFunc("POST /v1/queue/insert-next", handlers.InsertNext(s.cmdBus, s.queueMgr))
		mux.HandleFunc("POST /v1/queue/insert-after", handlers.InsertAfter(s.cmdBus, s.queueMgr))
		mux.HandleFunc("POST /v1/queue/clear", handlers.ClearQueue(s.cmdBus))
		mux.HandleFunc("POST /v1/queue/remove-item", handlers.RemoveItem(s.cmdBus))
		mux.HandleFunc("POST /v1/queue/move-item", handlers.MoveItem(s.cmdBus))
		mux.HandleFunc("POST /v1/queue/reorder-item", handlers.ReorderItem(s.cmdBus))
	}

	// Playback control
	mux.HandleFunc("POST /v1/playback/play", handlers.Play(s.cmdBus))
	mux.HandleFunc("POST /v1/playback/pause", handlers.Pause(s.cmdBus))
	mux.HandleFunc("POST /v1/playback/resume", handlers.Resume(s.cmdBus))
	mux.HandleFunc("POST /v1/playback/stop", handlers.Stop(s.cmdBus))
	mux.HandleFunc("POST /v1/playback/skip", handlers.Skip(s.cmdBus))
	mux.HandleFunc("POST /v1/playback/enter-assist", handlers.EnterAssist(s.cmdBus))
	mux.HandleFunc("POST /v1/playback/return-auto", handlers.ReturnAuto(s.cmdBus))

	// Panic mode
	mux.HandleFunc("POST /v1/panic/enter", handlers.EnterPanic(s.cmdBus))
	mux.HandleFunc("POST /v1/panic/exit", handlers.ExitPanic(s.cmdBus))

	// Hot buttons
	mux.HandleFunc("POST /v1/hotbuttons/trigger", handlers.TriggerHotButton(s.cmdBus))

	// Preview (cue) player — always registered; returns 503 when disabled.
	mux.HandleFunc("POST /v1/preview/play",   handlers.PreviewPlay(s.cmdBus, s.previewEnabled))
	mux.HandleFunc("POST /v1/preview/pause",  handlers.PreviewPause(s.cmdBus, s.previewEnabled))
	mux.HandleFunc("POST /v1/preview/resume", handlers.PreviewResume(s.cmdBus, s.previewEnabled))
	mux.HandleFunc("POST /v1/preview/stop",   handlers.PreviewStop(s.cmdBus, s.previewEnabled))
	mux.HandleFunc("POST /v1/preview/seek",   handlers.PreviewSeek(s.cmdBus, s.previewEnabled))
	mux.HandleFunc("GET /v1/preview/status",  handlers.PreviewStatus(s.previewStatus, s.previewEnabled))

	// Devices
	mux.HandleFunc("GET /v1/devices", handlers.Devices(s.listDevices))

	// Schedule — registered only when a manager is injected.
	if s.scheduleMgr != nil {
		mux.HandleFunc("POST /v1/schedule", handlers.ScheduleAdd(s.scheduleMgr))
		mux.HandleFunc("GET /v1/schedule", handlers.ScheduleList(s.scheduleMgr))
		mux.HandleFunc("GET /v1/schedule/{id}", handlers.ScheduleGet(s.scheduleMgr))
		mux.HandleFunc("PUT /v1/schedule/{id}", handlers.ScheduleUpdate(s.scheduleMgr))
		mux.HandleFunc("DELETE /v1/schedule/{id}", handlers.ScheduleDelete(s.scheduleMgr))
		mux.HandleFunc("POST /v1/schedule/{id}/enable", handlers.ScheduleEnable(s.scheduleMgr))
		mux.HandleFunc("POST /v1/schedule/{id}/disable", handlers.ScheduleDisable(s.scheduleMgr))
	}

	// Config
	if s.configSnapshot != nil {
		mux.HandleFunc("GET /config",             handlers.ConfigHTML())
		mux.HandleFunc("GET /v1/config/current",  handlers.GetCurrentConfig(s.configSnapshot))
		mux.HandleFunc("POST /v1/config/browse",  handlers.BrowsePath())
		mux.HandleFunc("PUT /v1/config",          handlers.UpdateConfig(s.configPath))
	}

	// Admin
	mux.HandleFunc("POST /v1/admin/shutdown", handlers.Shutdown())

	// Metrics
	if s.metrics != nil {
		mux.HandleFunc("GET /v1/metrics", handlers.Metrics(s.metrics))
	}

	// WebSocket event stream
	if s.wsHub != nil {
		mux.HandleFunc("GET /v1/events", func(w http.ResponseWriter, r *http.Request) {
			ws.ServeWS(s.wsHub, w, r)
		})
	}
}

// Start begins listening. It blocks until the server is closed.
// Call Shutdown() to stop it gracefully.
func (s *Server) Start() error {
	s.log.Info("api server listening", "addr", s.httpSrv.Addr)
	if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("api server: %w", err)
	}
	return nil
}

// Shutdown gracefully drains in-flight requests within the given context.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("api server shutting down")
	return s.httpSrv.Shutdown(ctx)
}

// Addr returns the TCP address the server is (or will be) listening on.
func (s *Server) Addr() string {
	return s.httpSrv.Addr
}

// withMiddleware wraps the mux with CORS and recovery middleware.
func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return s.recovery(s.cors(next))
}

// cors adds Access-Control-Allow-Origin headers when the request Origin
// matches one of the configured allowed origins.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) isAllowedOrigin(origin string) bool {
	for _, allowed := range s.cfg.AllowedOrigins {
		if strings.EqualFold(allowed, origin) || allowed == "*" {
			return true
		}
	}
	return false
}

// recovery catches panics inside handlers, logs them, and returns 500.
func (s *Server) recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rc := recover(); rc != nil {
				s.log.Error("api handler panic", "path", r.URL.Path, "panic", rc)
				http.Error(w, `{"ok":false,"error":"internal_error","message":"unexpected error"}`,
					http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ListenAddr resolves the actual listening address (useful in tests where
// port 0 is used to get a random free port).
func ListenAddr(host string, port int) (string, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return "", err
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr, nil
}
