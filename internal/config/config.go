package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddress            = ":8080"
	defaultHTTPReadHeaderTimeout  = 5 * time.Second
	defaultHTTPShutdownTimeout    = 10 * time.Second
	defaultDatabaseConnectTimeout = 5 * time.Second
	defaultDatabasePingTimeout    = 2 * time.Second
	defaultDatabaseMaxConnections = int32(10)
)

type Config struct {
	HTTPAddress            string
	HTTPReadHeaderTimeout  time.Duration
	HTTPShutdownTimeout    time.Duration
	DatabaseURL            string
	DatabaseConnectTimeout time.Duration
	DatabasePingTimeout    time.Duration
	DatabaseMaxConnections int32
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddress:            envOrDefault("APP_ADDR", defaultHTTPAddress),
		DatabaseURL:            strings.TrimSpace(os.Getenv("DATABASE_URL")),
		DatabaseMaxConnections: defaultDatabaseMaxConnections,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	var err error
	if cfg.HTTPReadHeaderTimeout, err = durationEnv("HTTP_READ_HEADER_TIMEOUT", defaultHTTPReadHeaderTimeout); err != nil {
		return Config{}, err
	}
	if cfg.HTTPShutdownTimeout, err = durationEnv("HTTP_SHUTDOWN_TIMEOUT", defaultHTTPShutdownTimeout); err != nil {
		return Config{}, err
	}
	if cfg.DatabaseConnectTimeout, err = durationEnv("DB_CONNECT_TIMEOUT", defaultDatabaseConnectTimeout); err != nil {
		return Config{}, err
	}
	if cfg.DatabasePingTimeout, err = durationEnv("DB_PING_TIMEOUT", defaultDatabasePingTimeout); err != nil {
		return Config{}, err
	}
	if cfg.DatabaseMaxConnections, err = positiveInt32Env("DB_MAX_CONNECTIONS", defaultDatabaseMaxConnections); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func envOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func durationEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return duration, nil
}

func positiveInt32Env(name string, fallback int32) (int32, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	number, err := strconv.ParseInt(value, 10, 32)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return int32(number), nil
}
