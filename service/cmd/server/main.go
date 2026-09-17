// Command server runs the EIA demo service: a REST API plus an embedded
// bilingual web UI that analyze an image and return a structured
// environmental impact assessment.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GhandyP/environmental-impact-gemma4/service/internal/config"
	"github.com/GhandyP/environmental-impact-gemma4/service/internal/httpapi"
	"github.com/GhandyP/environmental-impact-gemma4/service/internal/llm"
)

func newHandler(cfg config.Config) http.Handler {
	var providers []llm.Analyzer
	if cfg.Mock {
		providers = []llm.Analyzer{llm.NewMock("mock-eia")}
	} else {
		client := &http.Client{Timeout: cfg.ProviderTimeout}
		providers = []llm.Analyzer{
			llm.NewLocal(cfg.LLMLocalBaseURL, cfg.LLMLocalModel, client),
			llm.NewGemini(cfg.GeminiAPIKey, cfg.GeminiModel, cfg.ProviderTimeout),
		}
	}
	return httpapi.NewHandler(cfg, providers)
}

func main() {
	cfg := config.Load()
	srv := &http.Server{Addr: cfg.Addr, Handler: newHandler(cfg), ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		log.Printf("eia-service listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Print("eia-service stopped")
}
