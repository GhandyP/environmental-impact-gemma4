package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"github.com/GhandyP/environmental-impact-gemma4/service/internal/config"
	"github.com/GhandyP/environmental-impact-gemma4/service/internal/eia"
	"github.com/GhandyP/environmental-impact-gemma4/service/internal/llm"
	"github.com/GhandyP/environmental-impact-gemma4/service/web"
)

type handler struct {
	cfg       config.Config
	analyzers []llm.Analyzer
}

func NewHandler(cfg config.Config, analyzers []llm.Analyzer) http.Handler {
	h := &handler{cfg, analyzers}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /analyze", h.analyze)
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /", h.home)
	mux.HandleFunc("GET /favicon.ico", h.home)
	return mux
}
func (h *handler) analyze(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	imageData, mime, lang, err := h.input(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if int64(len(imageData)) > h.cfg.MaxImageBytes {
		writeErr(w, http.StatusBadRequest, errors.New("image is too large"))
		return
	}
	if _, _, err = image.DecodeConfig(bytes.NewReader(imageData)); err != nil {
		writeErr(w, http.StatusBadRequest, errors.New("invalid image"))
		return
	}
	l, err := eia.ParseLang(lang)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	ctx := r.Context()
	if h.cfg.ProviderTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.cfg.ProviderTimeout)
		defer cancel()
	}
	provider := llm.NewHybrid(h.analyzers)
	raw, err := provider.Generate(ctx, eia.BuildPrompt(l), imageData, mime)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeErr(w, http.StatusGatewayTimeout, err)
		} else if !provider.Available(r.Context()) {
			writeErr(w, http.StatusServiceUnavailable, errors.New("no model provider available"))
		} else {
			writeErr(w, http.StatusBadGateway, err)
		}
		return
	}
	analysis, err := eia.Extract(raw)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	w.Header().Set("X-EIA-Provider", provider.Name())
	writeJSON(w, http.StatusOK, map[string]any{"provider": provider.Name(), "model": provider.Model(), "lang": string(l), "analysis": analysis})
}
func (h *handler) input(w http.ResponseWriter, r *http.Request) ([]byte, string, string, error) {
	lang := r.FormValue("lang")
	if lang == "" {
		lang = "en"
	}
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(h.cfg.MaxImageBytes + 1024); err != nil {
			return nil, "", lang, err
		}
		f, _, err := r.FormFile("image")
		if err != nil {
			return nil, "", lang, errors.New("image file is required")
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, h.cfg.MaxImageBytes+1))
		mime := "image/jpeg"
		if len(b) > 0 {
			mime = http.DetectContentType(b)
		}
		return b, mime, lang, err
	}
	var in struct {
		ImageB64 string `json:"image_b64"`
		Lang     string `json:"lang"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxImageBytes+1024)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		return nil, "", lang, errors.New("invalid JSON body")
	}
	if in.Lang != "" {
		lang = in.Lang
	}
	b, err := base64.StdEncoding.DecodeString(in.ImageB64)
	return b, "image/jpeg", lang, err
}
func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	items := make([]llm.ProviderInfo, 0, len(h.analyzers))
	for _, a := range h.analyzers {
		items = append(items, a.Info())
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "providers": items})
}
func (h *handler) home(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, web.IndexHTML())
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
