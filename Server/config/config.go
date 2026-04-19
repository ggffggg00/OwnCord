// Package config provides configuration loading for the OwnCord server.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	goyaml "go.yaml.in/yaml/v3"
)

// Config holds the full server configuration.
type Config struct {
	Server   ServerConfig   `koanf:"server"`
	Database DatabaseConfig `koanf:"database"`
	TLS      TLSConfig      `koanf:"tls"`
	Upload   UploadConfig   `koanf:"upload"`
	Voice    VoiceConfig    `koanf:"voice"`
	GitHub   GitHubConfig   `koanf:"github"`
}

// GitHubConfig holds GitHub API settings for update checking.
type GitHubConfig struct {
	Token string `koanf:"token"`
}

// VoiceConfig holds LiveKit server connection and voice quality settings.
type VoiceConfig struct {
	LiveKitAPIKey     string `koanf:"livekit_api_key"`    // LiveKit API key
	LiveKitAPISecret  string `koanf:"livekit_api_secret"` // LiveKit API secret
	LiveKitURL        string `koanf:"livekit_url"`        // LiveKit server WebSocket URL (e.g. ws://localhost:7880)
	LiveKitBinaryPath string `koanf:"livekit_binary"`     // path to livekit-server binary; empty = don't auto-start
	// LiveKitRTC UDP port range (inclusive). Must match firewall / Docker published ports.
	LiveKitRTCPortRangeStart int `koanf:"livekit_rtc_port_range_start"`
	LiveKitRTCPortRangeEnd   int `koanf:"livekit_rtc_port_range_end"`
	// LiveKitUseExternalIP controls rtc.use_external_ip in generated livekit.yaml.
	// nil = default true (WAN discovery); explicit false helps localhost + fixed node_ip.
	LiveKitUseExternalIP *bool  `koanf:"livekit_use_external_ip"`
	NodeIP               string `koanf:"node_ip"` // public IP for WebRTC ICE candidates; empty = auto-detect
	Quality              string `koanf:"quality"` // low | medium | high
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port              int      `koanf:"port"`
	Name              string   `koanf:"name"`
	DataDir           string   `koanf:"data_dir"`
	AllowedOrigins    []string `koanf:"allowed_origins"`
	TrustedProxies    []string `koanf:"trusted_proxies"`
	AdminAllowedCIDRs []string `koanf:"admin_allowed_cidrs"`
	WAFEnabled        bool     `koanf:"waf_enabled"`        // Enable Coraza WAF (default: false)
	WAFParanoiaLevel  int      `koanf:"waf_paranoia_level"` // OWASP CRS paranoia level 1-4 (default: 2)
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	Path string `koanf:"path"`
}

// TLSConfig holds TLS/certificate settings.
type TLSConfig struct {
	Mode         string `koanf:"mode"`
	CertFile     string `koanf:"cert_file"`
	KeyFile      string `koanf:"key_file"`
	Domain       string `koanf:"domain"`
	AcmeCacheDir string `koanf:"acme_cache_dir"`
}

// UploadConfig holds file upload settings.
type UploadConfig struct {
	MaxSizeMB  int    `koanf:"max_size_mb"`
	StorageDir string `koanf:"storage_dir"`
}

// defaults returns the default configuration.
func defaults() Config {
	return Config{
		Server: ServerConfig{
			Port:           8443,
			Name:           "OwnCord Server",
			DataDir:        "data",
			AllowedOrigins: []string{},
			TrustedProxies: []string{},
			AdminAllowedCIDRs: []string{
				"127.0.0.0/8",    // localhost IPv4
				"::1/128",        // localhost IPv6
				"10.0.0.0/8",     // private class A
				"172.16.0.0/12",  // private class B
				"192.168.0.0/16", // private class C
				"fc00::/7",       // IPv6 unique local
			},
		},
		Database: DatabaseConfig{
			Path: "data/chatserver.db",
		},
		TLS: TLSConfig{
			Mode:         "self_signed",
			CertFile:     "data/cert.pem",
			KeyFile:      "data/key.pem",
			AcmeCacheDir: "data/acme_certs",
		},
		Upload: UploadConfig{
			MaxSizeMB:  100,
			StorageDir: "data/uploads",
		},
		Voice: func() VoiceConfig {
			useExt := true
			return VoiceConfig{
				LiveKitURL:               "ws://localhost:7880",
				LiveKitRTCPortRangeStart: 50000,
				LiveKitRTCPortRangeEnd:   60000,
				LiveKitUseExternalIP:     &useExt,
				Quality:                  "medium",
			}
		}(),
		GitHub: GitHubConfig{},
	}
}

// defaultYAML is the content written when no config file is present.
const defaultYAML = `# OwnCord Server Configuration
server:
  port: 8443
  name: "OwnCord Server"
  data_dir: "data"
  # allowed_origins: []       # empty = deny cross-origin; set to ["*"] for dev or specific origins for prod
  # trusted_proxies: []       # CIDRs of trusted reverse proxies, e.g. ["10.0.0.0/8"]
  # admin_allowed_cidrs:      # CIDRs allowed to access /admin (default: private networks only)
  #   - "127.0.0.0/8"
  #   - "::1/128"
  #   - "10.0.0.0/8"
  #   - "172.16.0.0/12"
  #   - "192.168.0.0/16"

database:
  path: "data/chatserver.db"

tls:
  mode: "self_signed"  # self_signed, acme, manual, off
  cert_file: "data/cert.pem"
  key_file: "data/key.pem"
  domain: ""              # required for acme mode (e.g. "chat.example.com")
  acme_cache_dir: "data/acme_certs"  # where Let's Encrypt certs are cached

upload:
  max_size_mb: 100
  storage_dir: "data/uploads"

voice:
  # livekit_api_key: ""       # LiveKit API key (REQUIRED for voice — generate a unique key)
  # livekit_api_secret: ""    # LiveKit API secret (REQUIRED, min 32 chars — generate a unique secret)
  livekit_url: "ws://localhost:7880"  # LiveKit server WebSocket URL
  # livekit_binary: ""             # path to livekit-server binary; empty = don't auto-start
  # livekit_rtc_port_range_start: 50000  # UDP range for WebRTC (must match firewall / Docker)
  # livekit_rtc_port_range_end: 60000
  # livekit_use_external_ip: true   # LiveKit rtc.use_external_ip (default true; false with node_ip for local/Docker)
  # node_ip: ""                    # public IP for WebRTC media (required for remote users behind NAT)
  # quality: "medium"              # low | medium | high

# github:
#   token: ""  # optional: GitHub API token for higher rate limits (5000 req/hr vs 60)
`

// Load reads configuration from the given YAML file path, merging with
// defaults and environment variable overrides. If the file does not exist,
// a default config.yaml is written and defaults are returned.
func Load(cfgPath string) (*Config, error) {
	k := koanf.New(".")

	// Layer 1: built-in defaults via struct provider.
	def := defaults()
	if err := k.Load(structs.Provider(def, "koanf"), nil); err != nil {
		return nil, fmt.Errorf("loading defaults: %w", err)
	}

	// Layer 2: YAML file (create default if missing).
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if writeErr := os.WriteFile(cfgPath, []byte(defaultYAML), 0o600); writeErr != nil {
			return nil, fmt.Errorf("writing default config: %w", writeErr)
		}
	} else {
		// Read the file and try to parse it ourselves to detect invalid YAML.
		raw, readErr := os.ReadFile(cfgPath)
		if readErr != nil {
			return nil, fmt.Errorf("reading config file %s: %w", cfgPath, readErr)
		}
		if parseErr := validateYAML(raw); parseErr != nil {
			return nil, fmt.Errorf("loading config file %s: %w", cfgPath, parseErr)
		}
		if err := k.Load(file.Provider(cfgPath), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("loading config file %s: %w", cfgPath, err)
		}
	}

	// Layer 3: environment variable overrides.
	// OWNCORD_SERVER_PORT -> server.port, OWNCORD_TLS_MODE -> tls.mode, etc.
	envProvider := env.Provider("OWNCORD_", ".", func(s string) string {
		// Strip prefix, lowercase, replace _ with . except within a key segment.
		// OWNCORD_SERVER_PORT -> server.port
		// OWNCORD_DATABASE_PATH -> database.path
		// OWNCORD_UPLOAD_MAX_SIZE_MB -> upload.max_size_mb
		s = strings.TrimPrefix(s, "OWNCORD_")
		s = strings.ToLower(s)
		// Split into at most 2 parts on the first underscore to get
		// section.key. We need smarter splitting because keys can have
		// underscores (e.g. max_size_mb, data_dir, storage_dir).
		return envKeyToKoanf(s)
	})
	if err := k.Load(envProvider, nil); err != nil {
		return nil, fmt.Errorf("loading env vars: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	// Apply voice defaults for zero-value fields (koanf loses defaults when
	// the YAML section is present but fields are commented out / omitted).
	if err := applyVoiceDefaults(&cfg.Voice); err != nil {
		return nil, fmt.Errorf("applying voice defaults: %w", err)
	}

	// Warn if using default dev credentials — these are public and insecure.
	// Clear credentials so downstream consumers (e.g. NewLiveKitClient) see
	// empty values and refuse to start voice.
	if IsDefaultVoiceCredentials(&cfg.Voice) {
		slog.Warn("using default LiveKit dev credentials — voice will be disabled; set voice.livekit_api_key and voice.livekit_api_secret in config.yaml")
		cfg.Voice.LiveKitAPIKey = ""
		cfg.Voice.LiveKitAPISecret = ""
	}

	return &cfg, nil
}

// defaultLiveKitAPIKey and defaultLiveKitAPISecret are the well-known dev
// credentials that ship in the default config. They must never be used in
// production — NewLiveKitClient rejects them.
const (
	DefaultLiveKitAPIKey    = "devkey"
	DefaultLiveKitAPISecret = "owncord-dev-secret-key-min-32chars" //nolint:gosec // G101: false positive — config key name, not a credential
)

// IsDefaultVoiceCredentials returns true when the voice config still uses
// the well-known default dev credentials shipped in the source code.
func IsDefaultVoiceCredentials(v *VoiceConfig) bool {
	return v.LiveKitAPIKey == DefaultLiveKitAPIKey ||
		v.LiveKitAPISecret == DefaultLiveKitAPISecret
}

// generateRandomKey returns a crypto-random hex string of the given byte length.
func generateRandomKey(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// applyVoiceDefaults fills in zero-value voice fields with sensible defaults.
// This guards against the koanf merge behaviour where an empty YAML section
// overwrites struct defaults with Go zero values.
// When API key/secret are empty, unique random credentials are generated
// so voice works out of the box without shipping known-public defaults.
func applyVoiceDefaults(v *VoiceConfig) error {
	if v.LiveKitAPIKey == "" {
		key, err := generateRandomKey(8)
		if err != nil {
			return fmt.Errorf("generating LiveKit API key: %w", err)
		}
		v.LiveKitAPIKey = "key-" + key
		slog.Warn("generated random LiveKit API key — voice tokens will break on restart; set voice.livekit_api_key in config.yaml for stable operation")
	}
	if v.LiveKitAPISecret == "" {
		secret, err := generateRandomKey(32)
		if err != nil {
			return fmt.Errorf("generating LiveKit API secret: %w", err)
		}
		v.LiveKitAPISecret = secret
		slog.Warn("generated random LiveKit API secret — set voice.livekit_api_secret in config.yaml for stable operation")
	}
	if v.LiveKitURL == "" {
		v.LiveKitURL = "ws://localhost:7880"
	}
	if v.Quality == "" {
		v.Quality = "medium"
	}
	if v.LiveKitRTCPortRangeStart == 0 {
		v.LiveKitRTCPortRangeStart = 50000
	}
	if v.LiveKitRTCPortRangeEnd == 0 {
		v.LiveKitRTCPortRangeEnd = 60000
	}
	if v.LiveKitUseExternalIP == nil {
		t := true
		v.LiveKitUseExternalIP = &t
	}
	if v.LiveKitRTCPortRangeStart >= v.LiveKitRTCPortRangeEnd {
		return fmt.Errorf("voice.livekit_rtc_port_range_start (%d) must be less than voice.livekit_rtc_port_range_end (%d)",
			v.LiveKitRTCPortRangeStart, v.LiveKitRTCPortRangeEnd)
	}
	if v.LiveKitRTCPortRangeStart < 1024 || v.LiveKitRTCPortRangeEnd > 65535 {
		return fmt.Errorf("voice LiveKit RTC port range must be within 1024-65535 (got %d-%d)",
			v.LiveKitRTCPortRangeStart, v.LiveKitRTCPortRangeEnd)
	}
	return nil
}

// EffectiveLiveKitRTCPortRange returns the UDP port range written to livekit.yaml.
// Call after Load/applyVoiceDefaults; if values are still zero, defaults match applyVoiceDefaults.
func EffectiveLiveKitRTCPortRange(v *VoiceConfig) (start, end int, err error) {
	if v == nil {
		return 0, 0, fmt.Errorf("voice config is nil")
	}
	start = v.LiveKitRTCPortRangeStart
	end = v.LiveKitRTCPortRangeEnd
	if start == 0 {
		start = 50000
	}
	if end == 0 {
		end = 60000
	}
	if start >= end {
		return 0, 0, fmt.Errorf("voice.livekit_rtc_port_range_start (%d) must be less than voice.livekit_rtc_port_range_end (%d)", start, end)
	}
	if start < 1024 || end > 65535 {
		return 0, 0, fmt.Errorf("voice LiveKit RTC port range must be within 1024-65535 (got %d-%d)", start, end)
	}
	return start, end, nil
}

// EffectiveLiveKitUseExternalIP returns rtc.use_external_ip for generated livekit.yaml.
// Default is true when unset (nil); use explicit false with node_ip for loopback/Docker dev.
func EffectiveLiveKitUseExternalIP(v *VoiceConfig) bool {
	if v == nil || v.LiveKitUseExternalIP == nil {
		return true
	}
	return *v.LiveKitUseExternalIP
}

// validateYAML checks that raw bytes are valid YAML.
func validateYAML(raw []byte) error {
	var v any
	return goyaml.Unmarshal(raw, &v)
}

// envKeyToKoanf converts a lower-case env key (without OWNCORD_ prefix) to a
// koanf dotted path. The first segment (up to the first underscore) is the
// section; the remainder is the key (with underscores preserved).
//
// Examples:
//
//	server_port        -> server.port
//	server_name        -> server.name
//	server_data_dir    -> server.data_dir
//	database_path      -> database.path
//	tls_mode           -> tls.mode
//	tls_cert_file      -> tls.cert_file
//	upload_max_size_mb -> upload.max_size_mb
func envKeyToKoanf(s string) string {
	idx := strings.Index(s, "_")
	if idx < 0 {
		return s
	}
	return s[:idx] + "." + s[idx+1:]
}
