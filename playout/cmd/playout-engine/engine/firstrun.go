//go:build !cli

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// RadioflowDir returns the platform-appropriate RadioFlow home directory.
//
//   - macOS / Linux : ~/RadioFlow
//   - Windows       : %APPDATA%\RadioFlow  (falls back to ~/RadioFlow)
func RadioflowDir() (string, error) {
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "RadioFlow"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("firstrun: user home dir: %w", err)
	}
	return filepath.Join(home, "RadioFlow"), nil
}

// EnsureFirstRun creates the RadioFlow directory tree and playout-engine.yaml
// on the first launch. Subsequent calls are no-ops (config already exists).
// Returns the absolute path to the config file.
func EnsureFirstRun() (string, error) {
	dir, err := RadioflowDir()
	if err != nil {
		return "", err
	}

	configPath := filepath.Join(dir, "playout-engine.yaml")

	if _, err := os.Stat(configPath); err == nil {
		return configPath, nil
	}

	for _, sub := range []string{
		filepath.Join("media", "musicas"),
		filepath.Join("media", "spots"),
		filepath.Join("media", "jingles"),
		filepath.Join("media", "vinhetas"),
		filepath.Join("media", "hora_certa", "hours_dir"),
		filepath.Join("media", "hora_certa", "minutes_dir"),
		"logs",
		"transmission-logs",
	} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			return "", fmt.Errorf("firstrun: mkdir %s: %w", sub, err)
		}
	}

	cfg := defaultConfig(dir)
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		return "", fmt.Errorf("firstrun: write config: %w", err)
	}

	return configPath, nil
}

func defaultConfig(dir string) string {
	queuePath := filepath.Join(dir, "queue.json")
	schedulePath := filepath.Join(dir, "schedule.json")
	hoursDir := filepath.Join(dir, "media", "hora_certa", "hours_dir")
	minutesDir := filepath.Join(dir, "media", "hora_certa", "minutes_dir")
	transmissionLogsDir := filepath.Join(dir, "transmission-logs")
	driver := DefaultAudioDriver()

	return fmt.Sprintf(`# =============================================================================
# Configuração do Radio Playout Engine
# Precedência (maior para menor): flags CLI > variáveis de ambiente > este arquivo > padrões internos
# =============================================================================

# -----------------------------------------------------------------------------
# Identificação do engine
# -----------------------------------------------------------------------------
engine:
  id: "studio-a-main"
  instance_lock: true

# -----------------------------------------------------------------------------
# Servidor HTTP / API REST
# -----------------------------------------------------------------------------
api:
  host: "127.0.0.1"
  port: 8080
  cors:
    enabled: true
    allowed_origins:
      - "http://localhost:3000"
      - "http://localhost:3333"
      - "http://localhost:5173"
      - "http://localhost:8080"

# -----------------------------------------------------------------------------
# Pipeline de áudio
# -----------------------------------------------------------------------------
audio:
  sample_rate: 48000
  channels: 2
  buffer_frames: 2048
  output:
    driver: %q
    device_id: "default"

# -----------------------------------------------------------------------------
# Comportamento de reprodução
# -----------------------------------------------------------------------------
playback:
  default_crossfade_ms: 8000
  default_stop_fade_ms: 300
  preload_next_ms: 3000
  max_consecutive_item_failures: 3
  auto_crossfade_enabled: true
  auto_crossfade_energy_threshold_dbfs: -18.0
  auto_crossfade_min_before_end_ms: 2000
  auto_crossfade_max_before_end_ms: 20000
  auto_crossfade_hold_frames: 8

# -----------------------------------------------------------------------------
# Monitoramento de saúde do áudio
# -----------------------------------------------------------------------------
health:
  progress_interval_ms: 500
  audio_health_interval_ms: 500
  silence_threshold_dbfs: -60
  silence_duration_ms: 2000
  vu_meter_enabled: true
  vu_meter_interval_ms: 100
  peak_hold_ms: 3000

# -----------------------------------------------------------------------------
# Modo Panic (proteção ao ar)
# -----------------------------------------------------------------------------
panic:
  enabled: true
  bed_path: ""
  auto_on_silence: false
  silence_threshold_dbfs: -60
  silence_duration_ms: 2000

# -----------------------------------------------------------------------------
# Logging
# -----------------------------------------------------------------------------
logging:
  level: "info"
  format: "text"

# -----------------------------------------------------------------------------
# Segurança
# -----------------------------------------------------------------------------
security:
  allowed_roots: []

# -----------------------------------------------------------------------------
# Administração
# -----------------------------------------------------------------------------
admin:
  shutdown_enabled: true

# -----------------------------------------------------------------------------
# Persistência da fila
# -----------------------------------------------------------------------------
queue:
  persistence:
    enabled: true
    path: %q
    restore_on_start: true
    clear_on_stop: false

# -----------------------------------------------------------------------------
# Hora Certa
# -----------------------------------------------------------------------------
hora_certa:
  hours_dir: %q
  minutes_dir: %q
  hour_pattern: "HRS{HH}.mp3"
  minute_pattern: "MIN{MM}.mp3"
  gain_db: 0.0

# -----------------------------------------------------------------------------
# Preview (cue player)
# -----------------------------------------------------------------------------
preview:
  # Habilita o recurso de preview. Quando false, os endpoints /v1/preview/*
  # retornam 503 Service Unavailable.
  enabled: true

  # Driver de saída para o dispositivo de preview.
  # Valores: "null" (silencioso), "coreaudio" (macOS), "portaudio" (multiplataforma)
  # Use "null" para testar sem um segundo dispositivo de áudio.
  output_driver: "null"

  # Dispositivo de saída dedicado para preview.
  # Vazio = dispositivo padrão do driver selecionado.
  # Exemplos: "BlackHole 2ch" (macOS virtual), "hw:1,0" (ALSA/Linux)
  output:
    device_id: ""

# -----------------------------------------------------------------------------
# Scheduler (programação horária)
# -----------------------------------------------------------------------------
scheduler:
  # Habilita o scheduler. Quando false, nenhuma entrada é avaliada.
  enabled: true

  # Timezone para avaliação das expressões cron.
  # Padrão: timezone do sistema operacional.
  # Exemplos: "America/Sao_Paulo", "America/Manaus", "UTC"
  timezone: "America/Sao_Paulo"

  # Caminho do arquivo de persistência do schedule.
  # Vazio = ~/RadioFlow/schedule.json
  store_path: %q

  # Tolerância de atraso: se um entry deveria ter disparado há mais que
  # este tempo (ex: engine foi reiniciado), ele é marcado como MISSED
  # em vez de disparar com atraso.
  missed_threshold_ms: 5000

# -----------------------------------------------------------------------------
# Hot Keys (botoneira)
# -----------------------------------------------------------------------------
hotkeys:
  # Dispositivo de saída dedicado para as hot keys.
  # Deve ser diferente do dispositivo principal e do dispositivo de preview.
  # Vazio = dispositivo padrão do driver.
  # Exemplos: "BlackHole 2ch" (macOS), "hw:1,0" (ALSA/Linux)
  output:
    device_id: ""

# -----------------------------------------------------------------------------
# Log de Transmissão (conformidade ECAD / prova de veiculação)
# -----------------------------------------------------------------------------
transmission_log:
  # Habilita o LogWriter. Quando false, nenhuma goroutine é iniciada e nenhum
  # arquivo é criado — zero overhead no pipeline de áudio.
  # Ative em produção para registrar todas as faixas reproduzidas.
  enabled: false

  # Diretório onde os arquivos JSONL são gravados (um arquivo por hora UTC).
  # Em produção, deve ser visível também pelo Library Service (volume compartilhado).
  dir: %q

  # Template do nome dos arquivos. Placeholders:
  #   {date} → yyyyMMdd (UTC)   {hour} → HH zero-preenchido (UTC)
  # O Library Service usa o mesmo template para descobrir arquivos a importar.
  file_name_template: "transmission_{date}_{hour}.jsonl"
`, driver, queuePath, hoursDir, minutesDir, schedulePath, transmissionLogsDir)
}

// DefaultAudioDriver returns the recommended output driver for the current OS.
func DefaultAudioDriver() string {
	switch runtime.GOOS {
	case "darwin":
		return "coreaudio"
	case "windows":
		return "portaudio"
	default:
		return "portaudio"
	}
}
