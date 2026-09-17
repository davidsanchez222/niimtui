package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type FontOption struct {
	Name string
	Path string
}

func discoverFonts(currentFontPath string) []FontOption {
	fonts := []FontOption{{Name: "Default", Path: ""}}
	seen := map[string]bool{"": true}

	for _, dir := range fontSearchDirs() {
		_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry == nil {
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			if !isSupportedFontPath(path) {
				return nil
			}
			path = filepath.Clean(path)
			if seen[path] {
				return nil
			}
			seen[path] = true
			fonts = append(fonts, FontOption{Name: fontDisplayName(path), Path: path})
			return nil
		})
	}

	currentFontPath = strings.TrimSpace(currentFontPath)
	if currentFontPath != "" {
		currentFontPath = filepath.Clean(currentFontPath)
		if !seen[currentFontPath] {
			fonts = append(fonts, FontOption{Name: fontDisplayName(currentFontPath), Path: currentFontPath})
		}
	}

	defaultFont := fonts[0]
	rest := append([]FontOption(nil), fonts[1:]...)
	sort.Slice(rest, func(i, j int) bool {
		return strings.ToLower(rest[i].Name) < strings.ToLower(rest[j].Name)
	})
	return append([]FontOption{defaultFont}, rest...)
}

func fontSearchDirs() []string {
	dirs := make([]string, 0, 8)
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		dirs = append(dirs, "/System/Library/Fonts", "/Library/Fonts")
		if home != "" {
			dirs = append(dirs, filepath.Join(home, "Library", "Fonts"))
		}
	case "windows":
		if windir := os.Getenv("WINDIR"); windir != "" {
			dirs = append(dirs, filepath.Join(windir, "Fonts"))
		}
	default:
		dirs = append(dirs, "/usr/share/fonts", "/usr/local/share/fonts")
		if home != "" {
			dirs = append(dirs, filepath.Join(home, ".local", "share", "fonts"), filepath.Join(home, ".fonts"))
		}
	}
	return dirs
}

func isSupportedFontPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf":
		return true
	default:
		return false
	}
}

func fontDisplayName(path string) string {
	base := filepath.Base(strings.TrimSpace(path))
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "Custom"
	}
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if strings.TrimSpace(base) == "" {
		return "Custom"
	}
	return base
}

func fontOptionIndex(fonts []FontOption, path string) int {
	path = strings.TrimSpace(path)
	if path != "" {
		path = filepath.Clean(path)
	}
	for i, font := range fonts {
		fontPath := strings.TrimSpace(font.Path)
		if fontPath != "" {
			fontPath = filepath.Clean(fontPath)
		}
		if fontPath == path {
			return i
		}
	}
	return 0
}
