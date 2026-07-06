// Package config defines and loads the Engine configuration.
package config

// Config is the root configuration for the Playout Engine.
// Precedence (highest to lowest): CLI flags > env vars > YAML file > defaults.
type Config struct {
	Engine    EngineConfig    `yaml:"engine"`
	API       APIConfig       `yaml:"api"`
	Audio     AudioConfig     `yaml:"audio"`
	Playback  PlaybackConfig  `yaml:"playback"`
	Health    HealthConfig    `yaml:"health"`
	Panic     PanicConfig     `yaml:"panic"`
	Logging   LoggingConfig   `yaml:"logging"`
	Security  SecurityConfig  `yaml:"security"`
	Admin     AdminConfig     `yaml:"admin"`
	Queue     QueueConfig     `yaml:"queue"`
	HoraCerta HoraCertaConfig `yaml:"hora_certa"`
	Preview   PreviewConfig   `yaml:"preview"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
}

// EngineConfig holds process-level settings.
type EngineConfig struct {
	ID           string `yaml:"id"`
	InstanceLock bool   `yaml:"instance_lock"`
}

// APIConfig holds HTTP API settings.
type APIConfig struct {
	Host string     `yaml:"host"`
	Port int        `yaml:"port"`
	CORS CORSConfig `yaml:"cors"`
}

// CORSConfig controls cross-origin behaviour.
type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// AudioConfig holds audio pipeline settings.
type AudioConfig struct {
	SampleRate   int          `yaml:"sample_rate"`
	Channels     int          `yaml:"channels"`
	BufferFrames int          `yaml:"buffer_frames"`
	Output       OutputConfig `yaml:"output"`
}

// OutputConfig selects and configures the audio output adapter.
type OutputConfig struct {
	Driver          string `yaml:"driver"`           // null | portaudio | file
	DeviceID        string `yaml:"device_id"`        // "default" or platform-specific ID
	AllowNullOutput bool   `yaml:"allow_null_output"` // degrade gracefully when device fails
}

// PlaybackConfig holds queue and playback behaviour settings.
type PlaybackConfig struct {
	DefaultCrossfadeMS         int `yaml:"default_crossfade_ms"`
	DefaultStopFadeMS          int `yaml:"default_stop_fade_ms"`
	PreloadNextMS              int `yaml:"preload_next_ms"`
	MaxConsecutiveItemFailures int `yaml:"max_consecutive_item_failures"`

	// Auto crossfade by energy analysis.
	AutoCrossfadeEnabled          bool    `yaml:"auto_crossfade_enabled"`
	AutoCrossfadeEnergyThreshDBFS float64 `yaml:"auto_crossfade_energy_threshold_dbfs"`
	AutoCrossfadeMinBeforeEndMS   int     `yaml:"auto_crossfade_min_before_end_ms"`
	AutoCrossfadeMaxBeforeEndMS   int     `yaml:"auto_crossfade_max_before_end_ms"`
	AutoCrossfadeHoldFrames       int     `yaml:"auto_crossfade_hold_frames"`
}

// HealthConfig holds monitoring and alert thresholds.
type HealthConfig struct {
	ProgressIntervalMS    int     `yaml:"progress_interval_ms"`
	AudioHealthIntervalMS int     `yaml:"audio_health_interval_ms"`
	SilenceThresholdDBFS  float64 `yaml:"silence_threshold_dbfs"`
	SilenceDurationMS     int     `yaml:"silence_duration_ms"`
	VUMeterEnabled        bool    `yaml:"vu_meter_enabled"`
	VUMeterIntervalMS     int     `yaml:"vu_meter_interval_ms"`
	PeakHoldMS            int     `yaml:"peak_hold_ms"`
}

// PanicConfig holds panic-mode settings.
type PanicConfig struct {
	Enabled              bool    `yaml:"enabled"`
	BedPath              string  `yaml:"bed_path"`
	AutoOnSilence        bool    `yaml:"auto_on_silence"`
	SilenceThresholdDBFS float64 `yaml:"silence_threshold_dbfs"`
	SilenceDurationMS    int     `yaml:"silence_duration_ms"`
}

// LoggingConfig holds structured logger settings.
type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug | info | warn | error
	Format string `yaml:"format"` // json | text
}

// SecurityConfig holds path validation and access control settings.
type SecurityConfig struct {
	AllowedRoots []string `yaml:"allowed_roots"`
}

// AdminConfig controls administrative endpoints.
type AdminConfig struct {
	ShutdownEnabled bool `yaml:"shutdown_enabled"`
}

// QueueConfig holds queue behaviour settings.
type QueueConfig struct {
	Persistence PersistenceConfig `yaml:"persistence"`
}

// PersistenceConfig controls optional queue persistence across restarts.
type PersistenceConfig struct {
	Enabled        bool   `yaml:"enabled"`          // false = NopStore (default)
	Path           string `yaml:"path"`             // defaults to /tmp/playout-<engine-id>-queue.json
	RestoreOnStart bool   `yaml:"restore_on_start"` // restore queue at startup (default true when enabled)
	ClearOnStop    bool   `yaml:"clear_on_stop"`    // delete snapshot on clean shutdown
}

// HoraCertaConfig configures the Hora Certa time-announcement feature.
// The engine resolves the current HH:MM to pre-recorded MP3 files and plays
// them in sequence (hour file → minute file) when a HORA_CERTA queue item is dequeued.
type HoraCertaConfig struct {
	// HoursDir is the directory containing hour announcement files.
	HoursDir string `yaml:"hours_dir"`

	// MinutesDir is the directory containing minute announcement files.
	// May be the same as HoursDir.
	MinutesDir string `yaml:"minutes_dir"`

	// HourPattern is the filename template for hour files.
	// {HH} is substituted with the zero-padded hour (00–23).
	// Default: "HRS{HH}.mp3"
	HourPattern string `yaml:"hour_pattern"`

	// MinutePattern is the filename template for minute files.
	// {MM} is substituted with the zero-padded minute (00–59).
	// Default: "MIN{MM}.mp3"
	MinutePattern string `yaml:"minute_pattern"`

	// GainDB is the default volume gain applied to hora certa audio.
	// 0 = unity gain. Individual HORA_CERTA queue items may override this.
	GainDB float64 `yaml:"gain_db"`
}

// SchedulerConfig configures the timed playback scheduler.
type SchedulerConfig struct {
	// Enabled controls whether the scheduler goroutine is started.
	// When false, no entries are evaluated even if they are registered.
	Enabled bool `yaml:"enabled"`

	// Timezone is the IANA timezone name used to evaluate cron expressions.
	// Leave empty to use the local timezone of the operating system.
	// Examples: "America/Sao_Paulo", "America/Manaus", "UTC"
	Timezone string `yaml:"timezone"`

	// StorePath is the path to the JSON file used to persist scheduled entries
	// across restarts. Leave empty to use ~/RadioFlow/schedule.json.
	StorePath string `yaml:"store_path"`

	// MissedThresholdMS defines how late (in milliseconds) a FireAt entry can
	// be before it is considered MISSED instead of fired. This prevents
	// stale one-shot entries from firing unexpectedly after an engine restart.
	// Default: 5000 (5 seconds).
	MissedThresholdMS int `yaml:"missed_threshold_ms"`
}

// PreviewConfig configures the audio preview (cue) player.
// The preview player is completely isolated from the main playback pipeline —
// it uses a dedicated output device so the presenter can monitor audio
// without affecting the on-air signal.
type PreviewConfig struct {
	// Enabled controls whether the preview feature is available.
	// When false, all /v1/preview/* endpoints return 503.
	Enabled bool `yaml:"enabled"`

	// OutputDriver selects the audio backend for preview playback.
	// Valid values: "null" | "coreaudio" | "portaudio" | "file"
	// Defaults to "null" (silent — useful for testing without a second device).
	OutputDriver string `yaml:"output_driver"`

	// OutputDevice is the platform-specific device identifier for preview output.
	// Leave empty to use the driver's default device.
	// Examples: "BlackHole 2ch" (macOS), "hw:1,0" (ALSA/Linux).
	OutputDevice string `yaml:"output_device"`
}
