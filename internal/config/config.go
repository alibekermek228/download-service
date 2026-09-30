package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	APIAddress             string
	DatabaseURL            string
	TemporalAddress        string
	TemporalTaskQueue      string
	MaxConcurrentDownloads int
	MaxFileSizeBytes       int64
	ShutdownTimeout        time.Duration
}

func Load() (Config, error) {
	maxConcurrent, err := intFromEnv("MAX_CONCURRENT_DOWNLOADS", 5)
	if err != nil {
		return Config{}, err
	}

	maxFileSize, err := int64FromEnv("MAX_FILE_SIZE_BYTES", 10*1024*1024)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := time.ParseDuration(env("SHUTDOWN_TIMEOUT", "1m"))
	if err != nil {
		return Config{}, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
	}

	return Config{
		APIAddress:             env("API_ADDR", ":8081"),
		DatabaseURL:            env("DATABASE_URL", "postgres://internship:internship@localhost:5432/downloads?sslmode=disable"),
		TemporalAddress:        env("TEMPORAL_ADDRESS", "localhost:7233"),
		TemporalTaskQueue:      env("TEMPORAL_TASK_QUEUE", "download-service"),
		MaxConcurrentDownloads: maxConcurrent,
		MaxFileSizeBytes:       maxFileSize,
		ShutdownTimeout:        shutdownTimeout,
	}, nil
}

func env(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func intFromEnv(name string, fallback int) (int, error) {
	value := env(name, strconv.Itoa(fallback))
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func int64FromEnv(name string, fallback int64) (int64, error) {
	value := env(name, strconv.FormatInt(fallback, 10))
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}
