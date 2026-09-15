package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"time"
)

const defaultMaxBodyBytes int64 = 1 << 20 // 1 MiB

type Config struct {
	Port           int
	N8NWebhookURL  string
	HMACSecret     string
	RequireHMAC    bool
	ForwardTimeout time.Duration
	MaxBodyBytes   int64
	LogLevel       slog.Level
}

func Load() (Config, error) {
	port, err := intEnv("PORT", 8080)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PORT must be an integer between 1 and 65535")
	}

	webhookURL := os.Getenv("N8N_WEBHOOK_URL")
	parsedURL, err := url.Parse(webhookURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return Config{}, fmt.Errorf("N8N_WEBHOOK_URL must be an absolute http or https URL")
	}

	requireHMAC, err := boolEnv("REQUIRE_HMAC", true)
	if err != nil {
		return Config{}, fmt.Errorf("REQUIRE_HMAC: %w", err)
	}
	secret := os.Getenv("WUZAPI_HMAC_SECRET")
	if requireHMAC && secret == "" {
		return Config{}, fmt.Errorf("WUZAPI_HMAC_SECRET is required when REQUIRE_HMAC=true")
	}

	forwardTimeout, err := durationEnv("N8N_FORWARD_TIMEOUT", 8*time.Second)
	if err != nil || forwardTimeout <= 0 {
		return Config{}, fmt.Errorf("N8N_FORWARD_TIMEOUT must be a positive duration")
	}
	maxBodyBytes, err := int64Env("MAX_BODY_BYTES", defaultMaxBodyBytes)
	if err != nil || maxBodyBytes <= 0 {
		return Config{}, fmt.Errorf("MAX_BODY_BYTES must be a positive integer")
	}
	logLevel, err := parseLogLevel(envOrDefault("LOG_LEVEL", "info"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		Port:           port,
		N8NWebhookURL:  webhookURL,
		HMACSecret:     secret,
		RequireHMAC:    requireHMAC,
		ForwardTimeout: forwardTimeout,
		MaxBodyBytes:   maxBodyBytes,
		LogLevel:       logLevel,
	}, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func intEnv(name string, fallback int) (int, error) {
	return strconv.Atoi(envOrDefault(name, strconv.Itoa(fallback)))
}

func int64Env(name string, fallback int64) (int64, error) {
	return strconv.ParseInt(envOrDefault(name, strconv.FormatInt(fallback, 10)), 10, 64)
}

func boolEnv(name string, fallback bool) (bool, error) {
	return strconv.ParseBool(envOrDefault(name, strconv.FormatBool(fallback)))
}

func durationEnv(name string, fallback time.Duration) (time.Duration, error) {
	return time.ParseDuration(envOrDefault(name, fallback.String()))
}

func parseLogLevel(value string) (slog.Level, error) {
	switch value {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error")
	}
}
