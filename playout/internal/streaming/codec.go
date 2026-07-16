package streaming

import (
	"fmt"
	"os/exec"
	"strings"
)

// supportedFormats is the set of valid TargetConfig.Format values.
var supportedFormats = map[string]struct{}{
	"mp3":       {},
	"ogg_vorbis": {},
	"ogg_opus":  {},
	"aac":       {},
}

// formatCodec maps a format to the FFmpeg encoder name it requires.
// The "aac" encoder is built into FFmpeg and always available.
var formatCodec = map[string]string{
	"mp3":       "libmp3lame",
	"ogg_vorbis": "libvorbis",
	"ogg_opus":  "libopus",
	"aac":       "aac",
}

// ValidateFormat returns an error if format is not one of the supported values.
// An empty format is treated as "mp3" (the default).
func ValidateFormat(format string) error {
	if format == "" {
		return nil // default (mp3) is always valid
	}
	if _, ok := supportedFormats[format]; !ok {
		return fmt.Errorf("streaming: unsupported format %q (supported: mp3, ogg_vorbis, ogg_opus, aac)", format)
	}
	return nil
}

// CheckCodecAvailable returns an error if the FFmpeg encoder required by
// format is not compiled into the installed ffmpeg binary.
// "aac" is always available (built-in encoder); other codecs require
// FFmpeg to be compiled with the corresponding third-party library.
func CheckCodecAvailable(format string) error {
	if format == "" {
		format = "mp3"
	}
	codec, ok := formatCodec[format]
	if !ok {
		return fmt.Errorf("streaming: unsupported format %q", format)
	}
	// aac is a native FFmpeg encoder — no external lib required.
	if codec == "aac" {
		return nil
	}
	out, err := exec.Command(ffmpegBin(), "-encoders", "-hide_banner").Output()
	if err != nil {
		return fmt.Errorf("streaming: could not query ffmpeg encoders: %w", err)
	}
	if !strings.Contains(string(out), codec) {
		return fmt.Errorf("streaming: FFmpeg encoder %q not available (required for format %q); reinstall FFmpeg with %s support",
			codec, format, codec)
	}
	return nil
}
