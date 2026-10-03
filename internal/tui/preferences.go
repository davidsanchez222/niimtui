package tui

import (
	"errors"
	"os"
	"strings"

	"niimtui/internal/config"
)

func (m Model) preferences() config.Preferences {
	return config.Preferences{AutoInsert: m.AutoInsert, LivePreview: m.LivePreviewMode}
}

// persistPreferences saves menu settings to the config file. Without a config file (an ad-hoc
// size with no setup) settings stay for this session only.
func (m *Model) persistPreferences() {
	prefs := m.preferences()
	m.Print.Preferences = prefs
	if strings.TrimSpace(m.Print.ConfigPath) == "" {
		return
	}
	if err := savePreferences(m.Print.ConfigPath, prefs); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		m.setStatus("Preference not saved: %v", err)
	}
}

func savePreferences(path string, prefs config.Preferences) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	cfg.Preferences = prefs
	return config.Save(path, cfg)
}
