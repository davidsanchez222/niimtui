package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Server   ServerConfig     `json:"server"`
	Printers []PrinterProfile `json:"printers"`
	Presets  []LabelPreset    `json:"presets"`
}

const appName = "niimcli"

type ServerConfig struct {
	Listen         string     `json:"listen"`
	AuthToken      string     `json:"auth_token"`
	AllowedOrigins []string   `json:"allowed_origins,omitempty"`
	TLS            *TLSConfig `json:"tls,omitempty"`
}

type TLSConfig struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

type PrinterProfile struct {
	Name          string          `json:"name"`
	Model         string          `json:"model"`
	Transport     string          `json:"transport"`
	DeviceName    string          `json:"device_name"`
	Identifier    string          `json:"identifier,omitempty"`
	Address       string          `json:"address,omitempty"`
	DefaultPreset string          `json:"default_preset"`
	Defaults      PrinterDefaults `json:"defaults,omitempty"`
}

type PrinterDefaults struct {
	Density int `json:"density,omitempty"`
	Rotate  int `json:"rotate,omitempty"`
}

type LabelPreset struct {
	Name      string  `json:"name"`
	WidthMM   float64 `json:"width_mm"`
	HeightMM  float64 `json:"height_mm"`
	Shape     string  `json:"shape"`
	Layout    string  `json:"layout"`
	MarginsMM float64 `json:"margins_mm"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func LoadDefault() (Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return Config{}, err
	}
	cfg, err := Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("default config not found at %s; run `niimcli setup` or pass --config", path)
		}
		return Config{}, err
	}
	return cfg, nil
}

func LoadOptional(path string) (Config, error) {
	if path == "" {
		return LoadDefault()
	}
	return Load(path)
}

func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func DefaultPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, appName, "config.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", appName, "config.json"), nil
}

func DefaultConfig(printer PrinterProfile, presets []LabelPreset) (Config, error) {
	token, err := randomToken()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Server: ServerConfig{
			Listen:    "127.0.0.1:8443",
			AuthToken: token,
		},
		Printers: []PrinterProfile{printer},
		Presets:  presets,
	}, nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate auth token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (c Config) Validate() error {
	if c.Server.Listen == "" {
		return errors.New("config.server.listen is required")
	}
	if c.Server.AuthToken == "" {
		return errors.New("config.server.auth_token is required")
	}
	if len(c.Printers) == 0 {
		return errors.New("config.printers must include at least one printer profile")
	}
	if len(c.Presets) == 0 {
		return errors.New("config.presets must include at least one label preset")
	}

	printerNames := make(map[string]struct{}, len(c.Printers))
	for _, printer := range c.Printers {
		if printer.Name == "" {
			return errors.New("config.printers[].name is required")
		}
		if _, exists := printerNames[printer.Name]; exists {
			return fmt.Errorf("duplicate printer profile %q", printer.Name)
		}
		printerNames[printer.Name] = struct{}{}
		if printer.Model == "" {
			return fmt.Errorf("config.printers[%q].model is required", printer.Name)
		}
		if printer.Transport == "" {
			return fmt.Errorf("config.printers[%q].transport is required", printer.Name)
		}
		if printer.DeviceName == "" && printer.Identifier == "" && printer.Address == "" {
			return fmt.Errorf("config.printers[%q] requires device_name, identifier, or address", printer.Name)
		}
		if printer.DefaultPreset == "" {
			return fmt.Errorf("config.printers[%q].default_preset is required", printer.Name)
		}
	}

	presetNames := make(map[string]struct{}, len(c.Presets))
	for _, preset := range c.Presets {
		if preset.Name == "" {
			return errors.New("config.presets[].name is required")
		}
		if _, exists := presetNames[preset.Name]; exists {
			return fmt.Errorf("duplicate preset %q", preset.Name)
		}
		presetNames[preset.Name] = struct{}{}
		if preset.WidthMM <= 0 || preset.HeightMM <= 0 {
			return fmt.Errorf("config.presets[%q] must have positive dimensions", preset.Name)
		}
		if preset.Shape == "" {
			return fmt.Errorf("config.presets[%q].shape is required", preset.Name)
		}
		if preset.Layout == "" {
			return fmt.Errorf("config.presets[%q].layout is required", preset.Name)
		}
		if preset.MarginsMM < 0 {
			return fmt.Errorf("config.presets[%q].margins_mm must be >= 0", preset.Name)
		}
	}

	for _, printer := range c.Printers {
		if _, ok := presetNames[printer.DefaultPreset]; !ok {
			return fmt.Errorf("config.printers[%q] references unknown default preset %q", printer.Name, printer.DefaultPreset)
		}
	}

	if c.Server.TLS != nil && c.Server.TLS.Enabled {
		if c.Server.TLS.CertFile == "" || c.Server.TLS.KeyFile == "" {
			return errors.New("config.server.tls requires cert_file and key_file when enabled")
		}
	}

	return nil
}
