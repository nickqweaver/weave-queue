package queue

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxConcurrency != 10 {
		t.Errorf("MaxConcurrency = %d, want 10", cfg.MaxConcurrency)
	}

	if cfg.MaxQueue != 100 {
		t.Errorf("MaxQueue = %d, want 100", cfg.MaxQueue)
	}

	if cfg.BatchSize != 10 {
		t.Errorf("BatchSize = %d, want 10", cfg.BatchSize)
	}

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}

	if cfg.LeaseTTL != 30*time.Second {
		t.Errorf("LeaseTTL = %v, want 30s", cfg.LeaseTTL)
	}

	if cfg.ExecutionTimeout != 5*time.Minute {
		t.Errorf("ExecutionTimeout = %v, want 5m", cfg.ExecutionTimeout)
	}

	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s", cfg.ShutdownTimeout)
	}

	if cfg.RetryBackoffBaseMS != 500 {
		t.Errorf("RetryBackoffBaseMS = %d, want 500", cfg.RetryBackoffBaseMS)
	}

	if cfg.RetryBackoffMaxMS != 30000 {
		t.Errorf("RetryBackoffMaxMS = %d, want 30000", cfg.RetryBackoffMaxMS)
	}

	if cfg.RetryFetchRatio != 0.2 {
		t.Errorf("RetryFetchRatio = %f, want 0.2", cfg.RetryFetchRatio)
	}

	if cfg.MaxPayloadBytes != 1048576 {
		t.Errorf("MaxPayloadBytes = %d, want 1048576", cfg.MaxPayloadBytes)
	}

	if cfg.MaxMetadataBytes != 65536 {
		t.Errorf("MaxMetadataBytes = %d, want 65536", cfg.MaxMetadataBytes)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "zero values get defaults",
			config: Config{
				MaxConcurrency:     0,
				MaxQueue:           0,
				BatchSize:          0,
				MaxRetries:         0,
				LeaseTTL:           0,
				ExecutionTimeout:   0,
				ShutdownTimeout:    0,
				RetryBackoffBaseMS: 0,
				RetryBackoffMaxMS:  0,
				RetryFetchRatio:    0,
				MaxPayloadBytes:    0,
				MaxMetadataBytes:   0,
			},
			wantErr: false,
		},
		{
			name: "batch size exceeds max queue",
			config: Config{
				MaxQueue:  10,
				BatchSize: 20,
			},
			wantErr: true,
		},
		{
			name: "lease TTL exceeds execution timeout",
			config: Config{
				LeaseTTL:         10 * time.Minute,
				ExecutionTimeout: 5 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "retry backoff max less than base",
			config: Config{
				RetryBackoffBaseMS: 1000,
				RetryBackoffMaxMS:  500,
			},
			wantErr: true,
		},
		{
			name: "retry fetch ratio capped at 1",
			config: Config{
				RetryFetchRatio: 1.5,
			},
			wantErr: false,
		},
		{
			name: "negative max retries defaults to 3",
			config: Config{
				MaxRetries: -1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigValidateAppliesDefaults(t *testing.T) {
	cfg := Config{}
	defaults := DefaultConfig()

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if cfg.MaxConcurrency != defaults.MaxConcurrency {
		t.Errorf("MaxConcurrency = %d, want %d", cfg.MaxConcurrency, defaults.MaxConcurrency)
	}

	if cfg.MaxQueue != defaults.MaxQueue {
		t.Errorf("MaxQueue = %d, want %d", cfg.MaxQueue, defaults.MaxQueue)
	}

	if cfg.BatchSize != defaults.BatchSize {
		t.Errorf("BatchSize = %d, want %d", cfg.BatchSize, defaults.BatchSize)
	}

	if cfg.MaxRetries != 0 {
		t.Errorf("MaxRetries = %d, want 0 (zero value should be preserved)", cfg.MaxRetries)
	}

	if cfg.LeaseTTL != defaults.LeaseTTL {
		t.Errorf("LeaseTTL = %v, want %v", cfg.LeaseTTL, defaults.LeaseTTL)
	}

	if cfg.ExecutionTimeout != defaults.ExecutionTimeout {
		t.Errorf("ExecutionTimeout = %v, want %v", cfg.ExecutionTimeout, defaults.ExecutionTimeout)
	}

	if cfg.ShutdownTimeout != defaults.ShutdownTimeout {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, defaults.ShutdownTimeout)
	}

	if cfg.RetryBackoffBaseMS != defaults.RetryBackoffBaseMS {
		t.Errorf("RetryBackoffBaseMS = %d, want %d", cfg.RetryBackoffBaseMS, defaults.RetryBackoffBaseMS)
	}

	if cfg.RetryBackoffMaxMS != defaults.RetryBackoffMaxMS {
		t.Errorf("RetryBackoffMaxMS = %d, want %d", cfg.RetryBackoffMaxMS, defaults.RetryBackoffMaxMS)
	}

	if cfg.RetryFetchRatio != defaults.RetryFetchRatio {
		t.Errorf("RetryFetchRatio = %f, want %f", cfg.RetryFetchRatio, defaults.RetryFetchRatio)
	}

	if cfg.MaxPayloadBytes != defaults.MaxPayloadBytes {
		t.Errorf("MaxPayloadBytes = %d, want %d", cfg.MaxPayloadBytes, defaults.MaxPayloadBytes)
	}

	if cfg.MaxMetadataBytes != defaults.MaxMetadataBytes {
		t.Errorf("MaxMetadataBytes = %d, want %d", cfg.MaxMetadataBytes, defaults.MaxMetadataBytes)
	}
}

func TestConfigValidatePreservesValidValues(t *testing.T) {
	cfg := Config{
		MaxConcurrency:     25,
		MaxQueue:           500,
		BatchSize:          50,
		MaxRetries:         5,
		LeaseTTL:           45 * time.Second,
		ExecutionTimeout:   10 * time.Minute,
		ShutdownTimeout:    60 * time.Second,
		RetryBackoffBaseMS: 1000,
		RetryBackoffMaxMS:  60000,
		RetryFetchRatio:    0.5,
		MaxPayloadBytes:    2097152, // 2MB
		MaxMetadataBytes:   131072,  // 128KB
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if cfg.MaxConcurrency != 25 {
		t.Errorf("MaxConcurrency = %d, want 25", cfg.MaxConcurrency)
	}

	if cfg.MaxQueue != 500 {
		t.Errorf("MaxQueue = %d, want 500", cfg.MaxQueue)
	}

	if cfg.BatchSize != 50 {
		t.Errorf("BatchSize = %d, want 50", cfg.BatchSize)
	}

	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}

	if cfg.LeaseTTL != 45*time.Second {
		t.Errorf("LeaseTTL = %v, want 45s", cfg.LeaseTTL)
	}

	if cfg.ExecutionTimeout != 10*time.Minute {
		t.Errorf("ExecutionTimeout = %v, want 10m", cfg.ExecutionTimeout)
	}

	if cfg.ShutdownTimeout != 60*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 60s", cfg.ShutdownTimeout)
	}

	if cfg.RetryBackoffBaseMS != 1000 {
		t.Errorf("RetryBackoffBaseMS = %d, want 1000", cfg.RetryBackoffBaseMS)
	}

	if cfg.RetryBackoffMaxMS != 60000 {
		t.Errorf("RetryBackoffMaxMS = %d, want 60000", cfg.RetryBackoffMaxMS)
	}

	if cfg.RetryFetchRatio != 0.5 {
		t.Errorf("RetryFetchRatio = %f, want 0.5", cfg.RetryFetchRatio)
	}

	if cfg.MaxPayloadBytes != 2097152 {
		t.Errorf("MaxPayloadBytes = %d, want 2097152", cfg.MaxPayloadBytes)
	}

	if cfg.MaxMetadataBytes != 131072 {
		t.Errorf("MaxMetadataBytes = %d, want 131072", cfg.MaxMetadataBytes)
	}
}

func TestConfigValidateCapsRetryFetchRatio(t *testing.T) {
	cfg := Config{
		RetryFetchRatio: 2.0,
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if cfg.RetryFetchRatio != 1.0 {
		t.Errorf("RetryFetchRatio = %f, want 1.0 (capped)", cfg.RetryFetchRatio)
	}
}
