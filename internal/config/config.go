package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type Config struct {
	Server   ServerConfig     `json:"server"`
	Printers []PrinterProfile `json:"printers"`
	Presets  []LabelPreset    `json:"presets"`
}

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
