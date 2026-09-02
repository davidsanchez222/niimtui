package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"niimcli/internal/api"
	"niimcli/internal/config"
	"niimcli/internal/service"
)

type Server struct {
	cfg config.Config
	svc *service.Service

	httpServer *http.Server
}

func New(cfg config.Config, svc *service.Service) *Server {
	s := &Server{cfg: cfg, svc: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("OPTIONS /health", s.handleHealth)
	mux.HandleFunc("GET /printers", s.handlePrinters)
	mux.HandleFunc("OPTIONS /printers", s.handlePrinters)
	mux.HandleFunc("GET /presets", s.handlePresets)
	mux.HandleFunc("OPTIONS /presets", s.handlePresets)
	mux.HandleFunc("POST /probe", s.handleProbe)
	mux.HandleFunc("OPTIONS /probe", s.handleProbe)
	mux.HandleFunc("POST /render-preview", s.handleRenderPreview)
	mux.HandleFunc("OPTIONS /render-preview", s.handleRenderPreview)
	mux.HandleFunc("POST /print", s.handlePrint)
	mux.HandleFunc("OPTIONS /print", s.handlePrint)

	s.httpServer = &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		log.Printf("niimcli listening on %s", s.cfg.Server.Listen)
		var err error
		if s.cfg.Server.TLS != nil && s.cfg.Server.TLS.Enabled {
			err = s.httpServer.ListenAndServeTLS(s.cfg.Server.TLS.CertFile, s.cfg.Server.TLS.KeyFile)
		} else {
			err = s.httpServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeOrigin(w, r) {
		return
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "niimcli",
		"status":  "ready",
	})
}

func (s *Server) handlePrinters(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeOrigin(w, r) {
		return
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.authorized(r) {
		writeUnauthorized(w)
		return
	}
	writeJSON(w, http.StatusOK, s.svc.Printers())
}

func (s *Server) handlePresets(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeOrigin(w, r) {
		return
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.authorized(r) {
		writeUnauthorized(w)
		return
	}
	writeJSON(w, http.StatusOK, s.svc.Presets())
}

func (s *Server) handlePrint(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeOrigin(w, r) {
		return
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.authorized(r) {
		writeUnauthorized(w)
		return
	}

	req, ok := s.decodePrintRequest(w, r)
	if !ok {
		return
	}

	resp := s.svc.Print(r.Context(), req)
	status := http.StatusOK
	if !resp.OK {
		status = http.StatusBadRequest
		if resp.Error != nil && resp.Error.Code == service.ErrPrintNotImplemented {
			status = http.StatusNotImplemented
		}
		if resp.Error != nil && resp.Error.Code == service.ErrUnauthorized {
			status = http.StatusUnauthorized
		}
		if resp.Error != nil && resp.Error.Code == service.ErrPrinterNotFound {
			status = http.StatusNotFound
		}
		if resp.Error != nil && resp.Error.Code == service.ErrPresetNotFound {
			status = http.StatusNotFound
		}
	}

	writeJSON(w, status, resp)
}

func (s *Server) handleRenderPreview(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeOrigin(w, r) {
		return
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.authorized(r) {
		writeUnauthorized(w)
		return
	}

	req, ok := s.decodePrintRequest(w, r)
	if !ok {
		return
	}

	preview, errResp := s.svc.PreparePreview(r.Context(), req)
	if errResp != nil {
		status := http.StatusBadRequest
		if errResp.Error != nil && errResp.Error.Code == service.ErrPrinterNotFound {
			status = http.StatusNotFound
		}
		if errResp.Error != nil && errResp.Error.Code == service.ErrPresetNotFound {
			status = http.StatusNotFound
		}
		writeJSON(w, status, *errResp)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(preview)
}

func (s *Server) handleProbe(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeOrigin(w, r) {
		return
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.authorized(r) {
		writeUnauthorized(w)
		return
	}

	req, ok := s.decodeProbeRequest(w, r)
	if !ok {
		return
	}

	meta, errResp := s.svc.Probe(r.Context(), req.Printer.Selector)
	if errResp != nil {
		status := http.StatusBadRequest
		if errResp.Error != nil && errResp.Error.Code == service.ErrPrinterNotFound {
			status = http.StatusNotFound
		}
		writeJSON(w, status, *errResp)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"meta": meta,
	})
}

func (s *Server) decodePrintRequest(w http.ResponseWriter, r *http.Request) (api.PrintRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	defer r.Body.Close()

	var req api.PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.PrintResponse{
			OK: false,
			Error: &api.ErrorBody{
				Code:    service.ErrInvalidRequest,
				Message: "invalid request body",
			},
		})
		return api.PrintRequest{}, false
	}

	return req, true
}

func (s *Server) decodeProbeRequest(w http.ResponseWriter, r *http.Request) (api.PrintRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var req api.PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.PrintResponse{
			OK: false,
			Error: &api.ErrorBody{
				Code:    service.ErrInvalidRequest,
				Message: "invalid request body",
			},
		})
		return api.PrintRequest{}, false
	}

	return req, true
}

func (s *Server) authorized(r *http.Request) bool {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return false
	}
	return strings.TrimPrefix(auth, prefix) == s.cfg.Server.AuthToken
}

func (s *Server) authorizeOrigin(w http.ResponseWriter, r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	if !slices.Contains(s.cfg.Server.AllowedOrigins, origin) {
		writeJSON(w, http.StatusForbidden, api.PrintResponse{
			OK: false,
			Error: &api.ErrorBody{
				Code:    service.ErrUnauthorized,
				Message: "origin not allowed",
			},
		})
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Vary", "Origin")
	return true
}

func writeUnauthorized(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, api.PrintResponse{
		OK: false,
		Error: &api.ErrorBody{
			Code:    service.ErrUnauthorized,
			Message: "unauthorized",
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
