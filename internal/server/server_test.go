package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"niimtui/internal/config"
	"niimtui/internal/service"
)

func TestHealthAllowsConfiguredOriginWithoutAuth(t *testing.T) {
	srv := mustServer(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://homebox.example")
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://homebox.example" {
		t.Fatalf("allow origin = %q, want %q", got, "https://homebox.example")
	}
}

func TestHealthRejectsDisallowedOrigin(t *testing.T) {
	srv := mustServer(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://bad.example")
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestPrintersPreflightReturnsNoContent(t *testing.T) {
	srv := mustServer(t)
	req := httptest.NewRequest(http.MethodOptions, "/printers", nil)
	req.Header.Set("Origin", "https://homebox.example")
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("allow methods = %q", got)
	}
}

func TestPrintersAllowsNoOriginCurlStyleRequests(t *testing.T) {
	srv := mustServer(t)
	req := httptest.NewRequest(http.MethodGet, "/printers", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
}

func mustServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.Config{
		Server: config.ServerConfig{
			Listen:         "127.0.0.1:8443",
			AuthToken:      "test-token",
			AllowedOrigins: []string{"https://homebox.example"},
		},
		Printers: []config.PrinterProfile{{
			Name:          "b1-round",
			Model:         "B1",
			Transport:     "ble",
			DeviceName:    "B1-I427031488",
			Identifier:    "e6bc3bef-5a60-3bc7-ffab-50edc0e9f122",
			DefaultPreset: "b1-50x50-round",
		}},
		Presets: []config.LabelPreset{{
			Name:      "b1-50x50-round",
			WidthMM:   50,
			HeightMM:  50,
			Shape:     "round",
			Layout:    "qr-title",
			MarginsMM: 2,
		}},
	}
	svc, err := service.New(cfg)
	if err != nil {
		t.Fatalf("service.New() error = %v", err)
	}
	return New(cfg, svc)
}
