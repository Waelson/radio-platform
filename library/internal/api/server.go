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
	"github.com/Waelson/radio-library-service/internal/api/middleware"
	"github.com/Waelson/radio-library-service/internal/config"
)

// Server is the HTTP API server for the Library Service.
type Server struct {
	cfg  config.APIConfig
	auth config.AuthConfig
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
	tls  handlers.TransmissionLogStore
	ils  handlers.TransmissionImportLogStore
	stg  handlers.SettingsStore
	srw  handlers.SettingsReadWriter
	lw   handlers.LoudnessWorker
	lts  handlers.LoudnessTrackStore
	ciw  handlers.CueInWorker
	cits handlers.CueInTrackStore
	nr   handlers.NormalizationReader
	sts  handlers.StreamingStore
	us   handlers.AuthUserStore
	rcs  handlers.ResetCodeStore
	ml   handlers.Mailer
	log  *slog.Logger
	http *http.Server
}

// SetLoudnessWorker attaches the loudness worker and its backing store so
// the server can register the /v1/loudness/* routes. Must be called before
// Start.
func (s *Server) SetLoudnessWorker(lw handlers.LoudnessWorker, lts handlers.LoudnessTrackStore) {
	s.lw = lw
	s.lts = lts
}

// SetCueInWorker attaches the cue_in worker and its backing store so the
// server can register the /v1/tracks/reanalyze-cuepoints routes. Must be
// called before Start.
func (s *Server) SetCueInWorker(cw handlers.CueInWorker, cits handlers.CueInTrackStore) {
	s.ciw = cw
	s.cits = cits
}

// SetNormalizationReader attaches the normalization settings reader used by
// track and break handlers to compute gain_db. Must be called before Start.
func (s *Server) SetNormalizationReader(nr handlers.NormalizationReader) {
	s.nr = nr
}

// New creates a Server ready to be started.
func New(
	cfg config.APIConfig,
	authCfg config.AuthConfig,
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
	tls handlers.TransmissionLogStore,
	ils handlers.TransmissionImportLogStore,
	stg handlers.SettingsStore,
	srw handlers.SettingsReadWriter,
	sts handlers.StreamingStore,
	us  handlers.AuthUserStore,
	rcs handlers.ResetCodeStore,
	ml  handlers.Mailer,
	log *slog.Logger,
) *Server {
	s := &Server{cfg: cfg, auth: authCfg, ts: ts, ps: ps, bs: bs, hs: hs, ix: ix,
		cs: cs, cls: cls, ss: ss, rls: rls, svc: svc,
		tls: tls, ils: ils, stg: stg, srw: srw, sts: sts, us: us, rcs: rcs, ml: ml, log: log}
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

	s.http = &http.Server{
		Addr:         net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port)),
		Handler:      s.routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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

	// ── Auth routes (public) ──────────────────────────────────────────────────
	authCfg := handlers.AuthConfig{
		JWTSecret: s.auth.JWTSecret,
		TokenTTL:  time.Duration(s.auth.TokenTTLMinutes) * time.Minute,
	}
	mux.HandleFunc("POST /v1/auth/login",          handlers.Login(s.us, authCfg))
	mux.HandleFunc("POST /v1/auth/reset-request",  handlers.ResetRequest(s.us, s.rcs, s.ml))
	mux.HandleFunc("POST /v1/auth/reset-verify",   handlers.ResetVerify(s.us, s.rcs, authCfg))
	mux.HandleFunc("POST /v1/auth/reset-confirm",  handlers.ResetConfirm(s.us, authCfg))

	// ── Auth routes (require valid JWT) ───────────────────────────────────────
	requireAuth := middleware.RequireAuth(s.auth.JWTSecret)
	mux.Handle("POST /v1/auth/change-password",              requireAuth(handlers.ChangePassword(s.us)))
	mux.Handle("POST /v1/users",                             requireAuth(handlers.CreateUser(s.us)))
	mux.Handle("POST /v1/users/{id}/reset-password",         requireAuth(handlers.AdminResetPassword(s.us, s.auth.DefaultResetPassword)))

	mux.Handle("GET /v1/tracks/artists",           requireAuth(handlers.ListArtists(s.ts)))
	mux.Handle("GET /v1/tracks/{id}",             requireAuth(handlers.GetTrack(s.ts, s.nr)))
	mux.Handle("GET /v1/tracks",                  requireAuth(handlers.SearchTracks(s.ts, s.nr)))
	mux.Handle("PATCH /v1/tracks/{id}",           requireAuth(handlers.PatchTrack(s.ts, s.nr)))
	mux.Handle("PUT /v1/tracks/{id}/cuepoints",   requireAuth(handlers.SaveCuePoints(s.ts)))

	mux.Handle("GET /v1/playlists",                            requireAuth(handlers.ListPlaylists(s.ps)))
	mux.Handle("POST /v1/playlists",                           requireAuth(handlers.CreatePlaylist(s.ps)))
	mux.Handle("GET /v1/playlists/{id}",                       requireAuth(handlers.GetPlaylist(s.ps)))
	mux.Handle("PUT /v1/playlists/{id}",                       requireAuth(handlers.UpdatePlaylist(s.ps)))
	mux.Handle("DELETE /v1/playlists/{id}",                    requireAuth(handlers.DeletePlaylist(s.ps)))
	mux.Handle("POST /v1/playlists/{id}/items",                requireAuth(handlers.AddPlaylistItem(s.ps)))
	mux.Handle("DELETE /v1/playlists/{id}/items/{item_id}",    requireAuth(handlers.RemovePlaylistItem(s.ps)))
	mux.Handle("PUT /v1/playlists/{id}/items/reorder",         requireAuth(handlers.ReorderPlaylistItems(s.ps)))

	mux.Handle("GET /v1/breaks",                               requireAuth(handlers.ListBreaks(s.bs)))
	mux.Handle("POST /v1/breaks",                              requireAuth(handlers.CreateBreak(s.bs)))
	mux.Handle("GET /v1/breaks/{id}",                          requireAuth(handlers.GetBreak(s.bs, s.nr)))
	mux.Handle("PUT /v1/breaks/{id}",                          requireAuth(handlers.UpdateBreak(s.bs)))
	mux.Handle("DELETE /v1/breaks/{id}",                       requireAuth(handlers.DeleteBreak(s.bs)))
	mux.Handle("POST /v1/breaks/{id}/items",                   requireAuth(handlers.AddBreakItem(s.bs)))
	mux.Handle("DELETE /v1/breaks/{id}/items/{item_id}",       requireAuth(handlers.RemoveBreakItem(s.bs)))
	mux.Handle("PUT /v1/breaks/{id}/items/reorder",            requireAuth(handlers.ReorderBreakItems(s.bs)))

	mux.Handle("GET /v1/index/status",              requireAuth(handlers.GetIndexStatus(s.ix)))
	mux.Handle("POST /v1/index/scan",                requireAuth(handlers.TriggerScan(s.ix)))
	mux.Handle("POST /v1/index/sync-categories",     requireAuth(handlers.SyncCategories(s.ix)))

	mux.Handle("GET /v1/hotkeys/profiles",                          requireAuth(handlers.ListHotkeyProfiles(s.hs)))
	mux.Handle("POST /v1/hotkeys/profiles",                         requireAuth(handlers.CreateHotkeyProfile(s.hs)))
	mux.Handle("GET /v1/hotkeys/profiles/{id}",                     requireAuth(handlers.GetHotkeyProfile(s.hs, s.nr)))
	mux.Handle("PUT /v1/hotkeys/profiles/{id}",                     requireAuth(handlers.UpdateHotkeyProfile(s.hs)))
	mux.Handle("DELETE /v1/hotkeys/profiles/{id}",                  requireAuth(handlers.DeleteHotkeyProfile(s.hs)))
	mux.Handle("POST /v1/hotkeys/profiles/{id}/buttons",            requireAuth(handlers.AddHotkeyButton(s.hs)))
	mux.Handle("PUT /v1/hotkeys/profiles/{id}/buttons/reorder",     requireAuth(handlers.ReorderHotkeyButtons(s.hs)))
	mux.Handle("PATCH /v1/hotkeys/buttons/{id}",                    requireAuth(handlers.PatchHotkeyButton(s.hs)))
	mux.Handle("DELETE /v1/hotkeys/buttons/{id}",                   requireAuth(handlers.DeleteHotkeyButton(s.hs)))

	// Categories
	mux.Handle("GET /v1/categories",                                requireAuth(handlers.ListCategories(s.cs)))
	mux.Handle("POST /v1/categories",                               requireAuth(handlers.CreateCategory(s.cs)))
	mux.Handle("GET /v1/categories/{id}",                           requireAuth(handlers.GetCategory(s.cs)))
	mux.Handle("PUT /v1/categories/{id}",                           requireAuth(handlers.UpdateCategory(s.cs)))
	mux.Handle("DELETE /v1/categories/{id}",                        requireAuth(handlers.DeleteCategory(s.cs)))
	mux.Handle("GET /v1/categories/{id}/tracks",                    requireAuth(handlers.ListCategoryTracks(s.cs)))
	mux.Handle("POST /v1/categories/{id}/tracks",                   requireAuth(handlers.AddCategoryTracks(s.cs)))
	mux.Handle("DELETE /v1/categories/{id}/tracks/{track_id}",      requireAuth(handlers.RemoveCategoryTrack(s.cs)))
	mux.Handle("PUT /v1/tracks/{id}/categories",                    requireAuth(handlers.SetTrackCategories(s.cs)))

	// Clocks
	mux.Handle("GET /v1/clocks",                                    requireAuth(handlers.ListClocks(s.cls)))
	mux.Handle("POST /v1/clocks",                                   requireAuth(handlers.CreateClock(s.cls)))
	mux.Handle("GET /v1/clocks/{id}",                               requireAuth(handlers.GetClock(s.cls)))
	mux.Handle("PUT /v1/clocks/{id}",                               requireAuth(handlers.UpdateClock(s.cls)))
	mux.Handle("DELETE /v1/clocks/{id}",                            requireAuth(handlers.DeleteClock(s.cls)))
	mux.Handle("POST /v1/clocks/{id}/slots",                        requireAuth(handlers.AddClockSlot(s.cls)))
	mux.Handle("PUT /v1/clocks/{id}/slots/reorder",                 requireAuth(handlers.ReorderClockSlots(s.cls)))
	mux.Handle("PUT /v1/clocks/{id}/slots/{slot_id}",               requireAuth(handlers.UpdateClockSlot(s.cls)))
	mux.Handle("DELETE /v1/clocks/{id}/slots/{slot_id}",            requireAuth(handlers.DeleteClockSlot(s.cls)))

	// Schedule grid
	mux.Handle("GET /v1/schedule/clock-grid",                       requireAuth(handlers.GetClockGrid(s.cls)))
	mux.Handle("PUT /v1/schedule/clock-grid",                       requireAuth(handlers.SetClockGrid(s.cls)))

	// Separation rules
	mux.Handle("GET /v1/schedule/separation-rules",                 requireAuth(handlers.ListSeparationRules(s.ss)))
	mux.Handle("POST /v1/schedule/separation-rules",                requireAuth(handlers.CreateSeparationRule(s.ss)))
	mux.Handle("PUT /v1/schedule/separation-rules/{id}",            requireAuth(handlers.UpdateSeparationRule(s.ss)))
	mux.Handle("DELETE /v1/schedule/separation-rules/{id}",         requireAuth(handlers.DeleteSeparationRule(s.ss)))

	// Playlist generator
	mux.Handle("POST /v1/schedule/generate",                        requireAuth(handlers.GenerateSchedule(s.svc, s.nr)))

	// Rotation log
	mux.Handle("POST /v1/rotation-log",                             requireAuth(handlers.AppendRotationLog(s.rls)))
	mux.Handle("GET /v1/rotation-log",                              requireAuth(handlers.GetRotationLog(s.rls)))

	// Settings
	mux.Handle("GET /v1/settings",                                  requireAuth(handlers.ListSettings(s.srw)))
	mux.Handle("GET /v1/settings/{key}",                            requireAuth(handlers.GetSetting(s.srw)))
	mux.Handle("PUT /v1/settings/{key}",                            requireAuth(handlers.UpdateSetting(s.srw)))

	// Loudness analysis
	if s.lw != nil {
		mux.Handle("GET /v1/loudness/status",          requireAuth(handlers.GetLoudnessStatus(s.lw)))
		mux.Handle("POST /v1/loudness/analyze/{id}",   requireAuth(handlers.ReanalyzeTrack(s.lw, s.lts)))
		mux.Handle("POST /v1/loudness/analyze",        requireAuth(handlers.ReanalyzeAll(s.lw, s.lts)))
		mux.Handle("DELETE /v1/loudness/analyze",      requireAuth(handlers.CancelLoudness(s.lw)))
	}

	// CueIn reanalysis
	if s.ciw != nil {
		mux.Handle("GET /v1/tracks/reanalyze-cuepoints/status", requireAuth(handlers.GetCueInReanalyzeStatus(s.ciw)))
		mux.Handle("POST /v1/tracks/reanalyze-cuepoints",       requireAuth(handlers.TriggerCueInReanalyze(s.ciw, s.cits)))
		mux.Handle("DELETE /v1/tracks/reanalyze-cuepoints",     requireAuth(handlers.CancelCueInReanalyze(s.ciw)))
	}

	// Streaming targets
	mux.Handle("GET /v1/streaming",          requireAuth(handlers.ListStreamingTargets(s.sts)))
	mux.Handle("POST /v1/streaming",         requireAuth(handlers.CreateStreamingTarget(s.sts)))
	mux.Handle("GET /v1/streaming/{id}",     requireAuth(handlers.GetStreamingTarget(s.sts)))
	mux.Handle("PUT /v1/streaming/{id}",     requireAuth(handlers.UpdateStreamingTarget(s.sts)))
	mux.Handle("DELETE /v1/streaming/{id}",  requireAuth(handlers.DeleteStreamingTarget(s.sts)))

	// Transmission log — order matters: more specific paths first
	mux.Handle("GET /v1/transmission-log/export/ecad",              requireAuth(handlers.ExportECAD(s.tls, s.stg)))
	mux.Handle("GET /v1/transmission-log/export",                   requireAuth(handlers.ExportTransmissionLog(s.tls)))
	mux.Handle("GET /v1/transmission-log/summary",                  requireAuth(handlers.GetTransmissionLogSummary(s.tls)))
	mux.Handle("GET /v1/transmission-log/imports",                  requireAuth(handlers.ListImportLog(s.ils)))
	mux.Handle("GET /v1/transmission-log",                          requireAuth(handlers.ListTransmissionLog(s.tls)))

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
			w.Header().Set("Access-Control-Allow-Methods", "GET, PATCH, POST, PUT, DELETE, OPTIONS")
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
