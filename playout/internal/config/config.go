// Package config defines and loads the Engine configuration.
package config

// Config is the root configuration for the Playout Engine.
// Precedence (highest to lowest): CLI flags > env vars > YAML file > defaults.
type Config struct {
	Engine          EngineConfig          `yaml:"engine"           json:"engine"`
	API             APIConfig             `yaml:"api"              json:"api"`
	Audio           AudioConfig           `yaml:"audio"            json:"audio"`
	Playback        PlaybackConfig        `yaml:"playback"         json:"playback"`
	Health          HealthConfig          `yaml:"health"           json:"health"`
	Panic           PanicConfig           `yaml:"panic"            json:"panic"`
	Logging         LoggingConfig         `yaml:"logging"          json:"logging"`
	Queue           QueueConfig           `yaml:"queue"            json:"queue"`
	HoraCerta       HoraCertaConfig       `yaml:"hora_certa"       json:"hora_certa"`
	Preview         PreviewConfig         `yaml:"preview"          json:"preview"`
	Scheduler       SchedulerConfig       `yaml:"scheduler"        json:"scheduler"`
	HotKeys         HotKeysConfig         `yaml:"hotkeys"          json:"hotkeys"`
	TransmissionLog TransmissionLogConfig `yaml:"transmission_log" json:"transmission_log"`
}

// EngineConfig holds process-level settings.
type EngineConfig struct {
	ID           string `yaml:"id"            json:"id"`
	InstanceLock bool   `yaml:"instance_lock" json:"instance_lock"`
}

// APIConfig holds HTTP API settings.
type APIConfig struct {
	Host string     `yaml:"host" json:"host"`
	Port int        `yaml:"port" json:"port"`
	CORS CORSConfig `yaml:"cors" json:"cors"`
}

// CORSConfig controls cross-origin behaviour.
type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"         json:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins"`
}

// AudioConfig holds audio pipeline settings.
type AudioConfig struct {
	SampleRate   int          `yaml:"sample_rate"   json:"sample_rate"`
	Channels     int          `yaml:"channels"      json:"channels"`
	BufferFrames int          `yaml:"buffer_frames" json:"buffer_frames"`
	Output       OutputConfig `yaml:"output"        json:"output"`
}

// OutputConfig configures the audio output adapter.
// The driver is determined at compile-time via build tag (-tags coreaudio, -tags portaudio, etc.).
type OutputConfig struct {
	DeviceID string `yaml:"device_id" json:"device_id"` // "default" or platform-specific ID
}

// PlaybackConfig holds queue and playback behaviour settings.
type PlaybackConfig struct {
	DefaultCrossfadeMS         int `yaml:"default_crossfade_ms"          json:"default_crossfade_ms"`
	DefaultStopFadeMS          int `yaml:"default_stop_fade_ms"          json:"default_stop_fade_ms"`
	PreloadNextMS              int `yaml:"preload_next_ms"               json:"preload_next_ms"`
	MaxConsecutiveItemFailures int `yaml:"max_consecutive_item_failures" json:"max_consecutive_item_failures"`

	// Auto crossfade by energy analysis.
	AutoCrossfadeEnabled          bool    `yaml:"auto_crossfade_enabled"              json:"auto_crossfade_enabled"`
	AutoCrossfadeEnergyThreshDBFS float64 `yaml:"auto_crossfade_energy_threshold_dbfs" json:"auto_crossfade_energy_threshold_dbfs"`
	AutoCrossfadeMinBeforeEndMS   int     `yaml:"auto_crossfade_min_before_end_ms"    json:"auto_crossfade_min_before_end_ms"`
	AutoCrossfadeMaxBeforeEndMS   int     `yaml:"auto_crossfade_max_before_end_ms"    json:"auto_crossfade_max_before_end_ms"`
	AutoCrossfadeHoldFrames       int     `yaml:"auto_crossfade_hold_frames"          json:"auto_crossfade_hold_frames"`
}

// HealthConfig holds monitoring and alert thresholds.
type HealthConfig struct {
	ProgressIntervalMS    int     `yaml:"progress_interval_ms"    json:"progress_interval_ms"`
	AudioHealthIntervalMS int     `yaml:"audio_health_interval_ms" json:"audio_health_interval_ms"`
	SilenceThresholdDBFS  float64 `yaml:"silence_threshold_dbfs"  json:"silence_threshold_dbfs"`
	SilenceDurationMS     int     `yaml:"silence_duration_ms"     json:"silence_duration_ms"`
	VUMeterEnabled        bool    `yaml:"vu_meter_enabled"        json:"vu_meter_enabled"`
	VUMeterIntervalMS     int     `yaml:"vu_meter_interval_ms"    json:"vu_meter_interval_ms"`
	PeakHoldMS            int     `yaml:"peak_hold_ms"            json:"peak_hold_ms"`
}

// PanicConfig holds panic-mode settings.
type PanicConfig struct {
	Enabled              bool    `yaml:"enabled"               json:"enabled"`
	BedPath              string  `yaml:"bed_path"              json:"bed_path"`
	AutoOnSilence        bool    `yaml:"auto_on_silence"       json:"auto_on_silence"`
	SilenceThresholdDBFS float64 `yaml:"silence_threshold_dbfs" json:"silence_threshold_dbfs"`
	SilenceDurationMS    int     `yaml:"silence_duration_ms"   json:"silence_duration_ms"`
}

// LoggingConfig holds structured logger settings.
type LoggingConfig struct {
	Level  string `yaml:"level"  json:"level"`  // debug | info | warn | error
	Format string `yaml:"format" json:"format"` // json | text
	Dir    string `yaml:"dir"    json:"dir"`    // directory for engine.log; empty = ~/RadioFlow/logs
}

// QueueConfig holds queue behaviour settings.
type QueueConfig struct {
	Persistence PersistenceConfig `yaml:"persistence" json:"persistence"`
}

// PersistenceConfig controls optional queue persistence across restarts.
type PersistenceConfig struct {
	Enabled        bool   `yaml:"enabled"         json:"enabled"`          // false = NopStore (default)
	Path           string `yaml:"path"            json:"path"`             // defaults to /tmp/playout-<engine-id>-queue.json
	RestoreOnStart bool   `yaml:"restore_on_start" json:"restore_on_start"` // restore queue at startup (default true when enabled)
	ClearOnStop    bool   `yaml:"clear_on_stop"   json:"clear_on_stop"`    // delete snapshot on clean shutdown
}

// HoraCertaConfig configures the Hora Certa time-announcement feature.
// The engine resolves the current HH:MM to pre-recorded MP3 files and plays
// them in sequence (hour file → minute file) when a HORA_CERTA queue item is dequeued.
type HoraCertaConfig struct {
	// HoursDir is the directory containing hour announcement files.
	HoursDir string `yaml:"hours_dir" json:"hours_dir"`

	// MinutesDir is the directory containing minute announcement files.
	// May be the same as HoursDir.
	MinutesDir string `yaml:"minutes_dir" json:"minutes_dir"`

	// HourPattern is the filename template for hour files.
	// {HH} is substituted with the zero-padded hour (00–23).
	// Default: "HRS{HH}.mp3"
	HourPattern string `yaml:"hour_pattern" json:"hour_pattern"`

	// MinutePattern is the filename template for minute files.
	// {MM} is substituted with the zero-padded minute (00–59).
	// Default: "MIN{MM}.mp3"
	MinutePattern string `yaml:"minute_pattern" json:"minute_pattern"`

	// GainDB is the default volume gain applied to hora certa audio.
	// 0 = unity gain. Individual HORA_CERTA queue items may override this.
	GainDB float64 `yaml:"gain_db" json:"gain_db"`
}

// SchedulerConfig configures the timed playback scheduler.
type SchedulerConfig struct {
	// Enabled controls whether the scheduler goroutine is started.
	// When false, no entries are evaluated even if they are registered.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Timezone is the IANA timezone name used to evaluate cron expressions.
	// Leave empty to use the local timezone of the operating system.
	// Examples: "America/Sao_Paulo", "America/Manaus", "UTC"
	Timezone string `yaml:"timezone" json:"timezone"`

	// StorePath is the path to the JSON file used to persist scheduled entries
	// across restarts. Leave empty to use ~/RadioFlow/schedule.json.
	StorePath string `yaml:"store_path" json:"store_path"`

	// MissedThresholdMS defines how late (in milliseconds) a FireAt entry can
	// be before it is considered MISSED instead of fired. This prevents
	// stale one-shot entries from firing unexpectedly after an engine restart.
	// Default: 5000 (5 seconds).
	MissedThresholdMS int `yaml:"missed_threshold_ms" json:"missed_threshold_ms"`
}

// HotKeysConfig configures the hot keys player — a dedicated audio channel for
// hotkey-triggered playback, completely isolated from the main pipeline and
// the preview/CUE channel. The hot keys player uses its own output device and
// supports one active cart at a time.
// The audio driver is determined at compile-time via build tag, same as the main output.
type HotKeysConfig struct {
	// Output configures the dedicated output device for cart playback.
	// device_id should differ from the main output and preview output devices.
	// Leave device_id empty to use the driver's default device.
	Output OutputConfig `yaml:"output" json:"output"`
}

// PreviewConfig configures the audio preview (cue) player.
// The preview player is completely isolated from the main playback pipeline —
// it uses a dedicated output device so the presenter can monitor audio
// without affecting the on-air signal.
// The audio driver is determined at compile-time via build tag, same as the main output.
type PreviewConfig struct {
	// Output configures the dedicated output device for preview playback.
	// device_id should differ from the main output and hot keys output devices.
	// Leave device_id empty to use the driver's default device.
	Output OutputConfig `yaml:"output" json:"output"`
}

// TransmissionLogConfig configures the append-only JSONL log writer that records
// every played track for ECAD compliance and spot audit purposes.
// The Writer is only started when Enabled = true; when false there is zero
// overhead — no goroutine, no file handles, no event subscriptions.
type TransmissionLogConfig struct {
	// Enabled controls whether the LogWriter goroutine is started.
	// Set to true in production to activate transmission logging.
	// Default: false (opt-in).
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Dir is the directory where hourly JSONL files are written.
	// Must be readable by the Library Service importer (shared volume in production).
	// Default: "./transmission-logs"
	Dir string `yaml:"dir" json:"dir"`

	// FileNameTemplate is the filename pattern for log files.
	// Placeholders:
	//   {date} → yyyyMMdd  (UTC)
	//   {hour} → HH        (UTC, zero-padded)
	// The Library Service importer uses this same template to derive the
	// glob pattern for file discovery.
	// Default: "transmission_{date}_{hour}.jsonl"
	FileNameTemplate string `yaml:"file_name_template" json:"file_name_template"`
}
