package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Waelson/radio-library-service/internal/api/handlers"
	"github.com/Waelson/radio-library-service/internal/config"
)

// Server is the HTTP API server for the Library Service.
type Server struct {
	cfg  config.APIConfig
	ts   handlers.TrackStore
	ps   handlers.PlaylistStore
	bs   handlers.BreakStore
	hs   handlers.HotkeyStore
	ix   handlers.IndexService
	cs   handlers.CategoryStore
	cls  handlers.ClockStore
	ss   handlers.SeparationRuleStore
	rls  handlers.RotationLogStore
	svc  handlers.SchedulerService
	log  *slog.Logger
	http *http.Server
}

// New creates a Server ready to be started.
func New(
	cfg config.APIConfig,
	ts handlers.TrackStore,
	ps handlers.PlaylistStore,
	bs handlers.BreakStore,
	hs handlers.HotkeyStore,
	ix handlers.IndexService,
	cs handlers.CategoryStore,
	cls handlers.ClockStore,
	ss handlers.SeparationRuleStore,
	rls handlers.RotationLogStore,
	svc handlers.SchedulerService,
	log *slog.Logger,
) *Server {
	s := &Server{cfg: cfg, ts: ts, ps: ps, bs: bs, hs: hs, ix: ix,
		cs: cs, cls: cls, ss: ss, rls: rls, svc: svc, log: log}
	s.http = &http.Server{
		Addr:         net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Handler:      s.routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return s
}

// Start begins serving requests. It blocks until ctx is cancelled, then shuts
// down gracefully (10 s timeout). A nil error means clean shutdown.
func (s *Server) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.http.Shutdown(shutCtx); err != nil {
			s.log.Error("server shutdown error", "error", err)
		}
	}()

	s.log.Info("API server listening", "addr", s.http.Addr)
	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("api server: %w", err)
	}
	return nil
}

// routes builds the request mux.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/health", s.handleHealth)

	mux.HandleFunc("GET /v1/tracks/artists", handlers.ListArtists(s.ts))
	mux.HandleFunc("GET /v1/tracks/{id}", handlers.GetTrack(s.ts))
	mux.HandleFunc("GET /v1/tracks", handlers.SearchTracks(s.ts))
	mux.HandleFunc("PATCH /v1/tracks/{id}", handlers.PatchTrack(s.ts))

	mux.HandleFunc("GET /v1/playlists", handlers.ListPlaylists(s.ps))
	mux.HandleFunc("POST /v1/playlists", handlers.CreatePlaylist(s.ps))
	mux.HandleFunc("GET /v1/playlists/{id}", handlers.GetPlaylist(s.ps))
	mux.HandleFunc("PUT /v1/playlists/{id}", handlers.UpdatePlaylist(s.ps))
	mux.HandleFunc("DELETE /v1/playlists/{id}", handlers.DeletePlaylist(s.ps))
	mux.HandleFunc("POST /v1/playlists/{id}/items", handlers.AddPlaylistItem(s.ps))
	mux.HandleFunc("DELETE /v1/playlists/{id}/items/{item_id}", handlers.RemovePlaylistItem(s.ps))
	mux.HandleFunc("PUT /v1/playlists/{id}/items/reorder", handlers.ReorderPlaylistItems(s.ps))

	mux.HandleFunc("GET /v1/breaks", handlers.ListBreaks(s.bs))
	mux.HandleFunc("POST /v1/breaks", handlers.CreateBreak(s.bs))
	mux.HandleFunc("GET /v1/breaks/{id}", handlers.GetBreak(s.bs))
	mux.HandleFunc("PUT /v1/breaks/{id}", handlers.UpdateBreak(s.bs))
	mux.HandleFunc("DELETE /v1/breaks/{id}", handlers.DeleteBreak(s.bs))
	mux.HandleFunc("POST /v1/breaks/{id}/items", handlers.AddBreakItem(s.bs))
	mux.HandleFunc("DELETE /v1/breaks/{id}/items/{item_id}", handlers.RemoveBreakItem(s.bs))
	mux.HandleFunc("PUT /v1/breaks/{id}/items/reorder", handlers.ReorderBreakItems(s.bs))

	mux.HandleFunc("GET /v1/index/status", handlers.GetIndexStatus(s.ix))
	mux.HandleFunc("POST /v1/index/scan", handlers.TriggerScan(s.ix))

	mux.HandleFunc("GET /v1/hotkeys/profiles",                          handlers.ListHotkeyProfiles(s.hs))
	mux.HandleFunc("POST /v1/hotkeys/profiles",                         handlers.CreateHotkeyProfile(s.hs))
	mux.HandleFunc("GET /v1/hotkeys/profiles/{id}",                     handlers.GetHotkeyProfile(s.hs))
	mux.HandleFunc("PUT /v1/hotkeys/profiles/{id}",                     handlers.UpdateHotkeyProfile(s.hs))
	mux.HandleFunc("DELETE /v1/hotkeys/profiles/{id}",                  handlers.DeleteHotkeyProfile(s.hs))
	mux.HandleFunc("POST /v1/hotkeys/profiles/{id}/buttons",            handlers.AddHotkeyButton(s.hs))
	mux.HandleFunc("PUT /v1/hotkeys/profiles/{id}/buttons/reorder",     handlers.ReorderHotkeyButtons(s.hs))
	mux.HandleFunc("PATCH /v1/hotkeys/buttons/{id}",                    handlers.PatchHotkeyButton(s.hs))
	mux.HandleFunc("DELETE /v1/hotkeys/buttons/{id}",                   handlers.DeleteHotkeyButton(s.hs))

	// Categories
	mux.HandleFunc("GET /v1/categories",                                handlers.ListCategories(s.cs))
	mux.HandleFunc("POST /v1/categories",                               handlers.CreateCategory(s.cs))
	mux.HandleFunc("GET /v1/categories/{id}",                           handlers.GetCategory(s.cs))
	mux.HandleFunc("PUT /v1/categories/{id}",                           handlers.UpdateCategory(s.cs))
	mux.HandleFunc("DELETE /v1/categories/{id}",                        handlers.DeleteCategory(s.cs))
	mux.HandleFunc("GET /v1/categories/{id}/tracks",                    handlers.ListCategoryTracks(s.cs))
	mux.HandleFunc("POST /v1/categories/{id}/tracks",                   handlers.AddCategoryTracks(s.cs))
	mux.HandleFunc("DELETE /v1/categories/{id}/tracks/{track_id}",      handlers.RemoveCategoryTrack(s.cs))
	mux.HandleFunc("PUT /v1/tracks/{id}/categories",                    handlers.SetTrackCategories(s.cs))

	// Clocks
	mux.HandleFunc("GET /v1/clocks",                                    handlers.ListClocks(s.cls))
	mux.HandleFunc("POST /v1/clocks",                                   handlers.CreateClock(s.cls))
	mux.HandleFunc("GET /v1/clocks/{id}",                               handlers.GetClock(s.cls))
	mux.HandleFunc("PUT /v1/clocks/{id}",                               handlers.UpdateClock(s.cls))
	mux.HandleFunc("DELETE /v1/clocks/{id}",                            handlers.DeleteClock(s.cls))
	mux.HandleFunc("POST /v1/clocks/{id}/slots",                        handlers.AddClockSlot(s.cls))
	mux.HandleFunc("PUT /v1/clocks/{id}/slots/reorder",                 handlers.ReorderClockSlots(s.cls))
	mux.HandleFunc("PUT /v1/clocks/{id}/slots/{slot_id}",               handlers.UpdateClockSlot(s.cls))
	mux.HandleFunc("DELETE /v1/clocks/{id}/slots/{slot_id}",            handlers.DeleteClockSlot(s.cls))

	// Schedule grid
	mux.HandleFunc("GET /v1/schedule/clock-grid",                       handlers.GetClockGrid(s.cls))
	mux.HandleFunc("PUT /v1/schedule/clock-grid",                       handlers.SetClockGrid(s.cls))

	// Separation rules
	mux.HandleFunc("GET /v1/schedule/separation-rules",                 handlers.ListSeparationRules(s.ss))
	mux.HandleFunc("POST /v1/schedule/separation-rules",                handlers.CreateSeparationRule(s.ss))
	mux.HandleFunc("PUT /v1/schedule/separation-rules/{id}",            handlers.UpdateSeparationRule(s.ss))
	mux.HandleFunc("DELETE /v1/schedule/separation-rules/{id}",         handlers.DeleteSeparationRule(s.ss))

	// Playlist generator
	mux.HandleFunc("POST /v1/schedule/generate",                        handlers.GenerateSchedule(s.svc))

	// Rotation log
	mux.HandleFunc("POST /v1/rotation-log",                             handlers.AppendRotationLog(s.rls))
	mux.HandleFunc("GET /v1/rotation-log",                              handlers.GetRotationLog(s.rls))

	return corsMiddleware(s.cfg.CORS.AllowedOrigins, mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// corsMiddleware sets CORS headers for all responses. When allowedOrigins is
// empty, CORS is disabled (same-origin only).
func corsMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		return next
	}
	allowed := make(map[string]struct{}, len(allowedOrigins))
	wildcard := false
	for _, o := range allowedOrigins {
		if o == "*" {
			wildcard = true
		}
		allowed[o] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if wildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, PATCH, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Strip trailing slash for cleaner matching.
		if r.URL.Path != "/" && strings.HasSuffix(r.URL.Path, "/") {
			r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
		}

		next.ServeHTTP(w, r)
	})
}
