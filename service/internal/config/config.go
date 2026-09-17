// Package config loads service configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the EIA service.
type Config struct {
	// Addr is the HTTP listen address (EIA_ADDR, default ":8080").
	Addr string

	// LLMLocalBaseURL is the llama.cpp llama-server base URL. When empty,
	// the local provider is considered unavailable and requests fall back
	// to Gemini (EIA_LLAMA_BASE_URL, default "" = disabled).
	LLMLocalBaseURL string

	// LLMLocalModel is the model id sent to the local OpenAI-compatible
	// endpoint (EIA_LLAMA_MODEL, default "gemma-4-E2B-it"). The value is
	// informational for llama.cpp but must be present in the request body.
	LLMLocalModel string

	// GeminiAPIKey is the Google API key for the Gemini API. Both
	// GEMINI_API_KEY and GOOGLE_API_KEY are honored (GEMINI_API_KEY wins).
	// Empty means the Gemini provider is unavailable.
	GeminiAPIKey string

	// GeminiModel is the Gemini model id used for analysis
	// (EIA_GEMINI_MODEL, default "gemini-flash-latest").
	GeminiModel string

	// MaxImageBytes caps the accepted image payload size
	// (EIA_MAX_IMAGE_BYTES, default 10 MiB).
	MaxImageBytes int64

	// ProviderTimeout bounds a single provider inference call
	// (EIA_PROVIDER_TIMEOUT, default 120s).
	ProviderTimeout time.Duration

	// Mock enables the deterministic mock provider instead of real models
	// (EIA_MOCK, default false). Useful for demos without GPU or API key.
	Mock bool
}

// Load builds a Config from environment variables, applying defaults.
func Load() Config {
	return Config{
		Addr:            strOr("EIA_ADDR", ":8080"),
		LLMLocalBaseURL: strOr("EIA_LLAMA_BASE_URL", ""),
		LLMLocalModel:   strOr("EIA_LLAMA_MODEL", "gemma-4-E2B-it"),
		GeminiAPIKey:    strOr("GEMINI_API_KEY", strOr("GOOGLE_API_KEY", "")),
		GeminiModel:     strOr("EIA_GEMINI_MODEL", "gemini-flash-latest"),
		MaxImageBytes:   intOr("EIA_MAX_IMAGE_BYTES", 10<<20),
		ProviderTimeout: durOr("EIA_PROVIDER_TIMEOUT", 120*time.Second),
		Mock:            boolOr("EIA_MOCK", false),
	}
}

func strOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func intOr(key string, fallback int64) int64 {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func durOr(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return fallback
}

func boolOr(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
