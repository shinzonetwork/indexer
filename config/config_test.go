package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidYAML(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
defradb:
  url: "http://localhost:9181"
  keyring_secret: "test_secret"
  p2p:
    enabled: true
    bootstrap_peers: ["peer1", "peer2"]
    listen_addr: "/ip4/0.0.0.0/tcp/9171"
  store:
    path: "/tmp/defra"

geth:
  node_url: "http://localhost:8545"

indexer:
  block_polling_interval: 3.0
  batch_size: 100
  start_height: 1000
  pipeline:
    fetch_blocks:
      workers: 4
      buffer_size: 100
    process_transactions:
      workers: 8
      buffer_size: 200
    store_data:
      workers: 2
      buffer_size: 50
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Test DefraDB config
	if cfg.DefraDB.Url != "http://localhost:9181" {
		t.Errorf("Expected url 'http://localhost:9181', got '%s'", cfg.DefraDB.Url)
	}
	if cfg.DefraDB.KeyringSecret != "test_secret" {
		t.Errorf("Expected keyring_secret 'test_secret', got '%s'", cfg.DefraDB.KeyringSecret)
	}

	// Test P2P config
	if !cfg.DefraDB.P2P.Enabled {
		t.Error("Expected P2P enabled to be true")
	}
	if len(cfg.DefraDB.P2P.BootstrapPeers) != 2 {
		t.Errorf("Expected 2 bootstrap peers, got %d", len(cfg.DefraDB.P2P.BootstrapPeers))
	}

	// Test Geth config
	if cfg.Geth.NodeURL != "http://localhost:8545" {
		t.Errorf("Expected node_url 'http://localhost:8545', got '%s'", cfg.Geth.NodeURL)
	}

	// Test Indexer config
	if cfg.Indexer.BlockPollingInterval != 3.0 {
		t.Errorf("Expected block_polling_interval 3.0, got %f", cfg.Indexer.BlockPollingInterval)
	}
	if cfg.Indexer.BatchSize != 100 {
		t.Errorf("Expected batch_size 100, got %d", cfg.Indexer.BatchSize)
	}
	if cfg.Indexer.StartHeight != 1000 {
		t.Errorf("Expected start_height 1000, got %d", cfg.Indexer.StartHeight)
	}
}

func TestLoadConfig_EnvironmentOverrides(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
defradb:
  url: "http://localhost:9181"
  keyring_secret: "original_secret"

indexer:
  start_height: 1000
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set environment variables
	os.Setenv("DEFRA_KEYRING_SECRET", "env_secret")
	os.Setenv("INDEXER_START_HEIGHT", "2000")

	// Clean up environment variables after test
	defer func() {
		os.Unsetenv("DEFRA_KEYRING_SECRET")
		os.Unsetenv("INDEXER_START_HEIGHT")
	}()

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify environment overrides work
	if cfg.DefraDB.KeyringSecret != "env_secret" {
		t.Errorf("Expected keyring_secret 'env_secret', got '%s'", cfg.DefraDB.KeyringSecret)
	}
	if cfg.DefraDB.Url != "http://localhost:9181" {
		t.Errorf("Expected url 'http://localhost:9181', got '%s'", cfg.DefraDB.Url)
	}
	if cfg.Indexer.StartHeight != 2000 {
		t.Errorf("Expected start_height 2000, got %d", cfg.Indexer.StartHeight)
	}
}

func TestLoadConfig_InvalidPath(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent config file, got nil")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	// Create a temporary config file with invalid YAML
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid_config.yaml")

	invalidContent := `
defradb:
  url: "invalid yaml
`

	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	_, err = LoadConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestLoadConfig_InvalidEnvironmentValues(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
indexer:
  start_height: 1000
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set invalid environment variables (should be ignored)
	os.Setenv("INDEXER_START_HEIGHT", "also_not_a_number")

	// Clean up environment variables after test
	defer func() {
		os.Unsetenv("INDEXER_START_HEIGHT")
	}()

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Should keep original values when env vars are invalid
	if cfg.Indexer.StartHeight != 1000 {
		t.Errorf("Expected start_height 1000 (original), got %d", cfg.Indexer.StartHeight)
	}
}
