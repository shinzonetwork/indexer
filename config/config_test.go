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
  host: "localhost"
  port: 9181
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

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Test DefraDB config
	if cfg.DefraDB.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", cfg.DefraDB.Host)
	}
	if cfg.DefraDB.Port != 9181 {
		t.Errorf("Expected port 9181, got %d", cfg.DefraDB.Port)
	}
	if cfg.DefraDB.KeyringSecret != "" {
		t.Errorf("Expected default keyring_secret '', got '%s'", cfg.DefraDB.KeyringSecret)
	}

	// Test P2P config
	if !cfg.DefraDB.P2P.Enabled {
		t.Error("Expected P2P enabled to be true")
	}
	if len(cfg.DefraDB.P2P.BootstrapPeers) != 2 {
		t.Errorf("Expected 2 bootstrap peers, got %d", len(cfg.DefraDB.P2P.BootstrapPeers))
	}

	// Test Geth defaults (should be empty, set by env vars)
	if cfg.Geth.NodeURL != "" {
		t.Errorf("Expected default node_url '', got '%s'", cfg.Geth.NodeURL)
	}
	if cfg.Geth.WsURL != "" {
		t.Errorf("Expected default ws_url '', got '%s'", cfg.Geth.WsURL)
	}
	if cfg.Geth.APIKey != "" {
		t.Errorf("Expected default api_key '', got '%s'", cfg.Geth.APIKey)
	}

	// Test Indexer defaults
	if cfg.Indexer.BlockPollingInterval != 12.0 {
		t.Errorf("Expected default block_polling_interval 12.0, got %f", cfg.Indexer.BlockPollingInterval)
	}
	if cfg.Indexer.BatchSize != 100 {
		t.Errorf("Expected default batch_size 100, got %d", cfg.Indexer.BatchSize)
	}
	if cfg.Indexer.StartHeight != 1800000 {
		t.Errorf("Expected default start_height 1800000, got %d", cfg.Indexer.StartHeight)
	}

	// Test Logger defaults
	if cfg.Logger.Development {
		t.Error("Expected default development to be false")
	}
}

func TestLoadConfig_EnvironmentOverrides(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
defradb:
  host: "localhost"
  port: 9181
  keyring_secret: "original_secret"

indexer:
  start_height: 1000
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set environment variables
	os.Setenv("GCP_RPC_URL", "https://test-rpc.example.com")
	os.Setenv("GCP_WS_URL", "wss://test-ws.example.com")
	os.Setenv("GCP_API_KEY", "test_api_key_123")
	os.Setenv("DEFRA_KEYRING_SECRET", "env_secret")
	os.Setenv("INDEXER_START_HEIGHT", "2000")

	// Clean up environment variables after test
	defer func() {
		os.Unsetenv("GCP_RPC_URL")
		os.Unsetenv("GCP_WS_URL")
		os.Unsetenv("GCP_API_KEY")
		os.Unsetenv("DEFRA_KEYRING_SECRET")
		os.Unsetenv("INDEXER_START_HEIGHT")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify GCP environment overrides work
	if cfg.Geth.NodeURL != "https://test-rpc.example.com" {
		t.Errorf("Expected node_url 'https://test-rpc.example.com', got '%s'", cfg.Geth.NodeURL)
	}
	if cfg.Geth.WsURL != "wss://test-ws.example.com" {
		t.Errorf("Expected ws_url 'wss://test-ws.example.com', got '%s'", cfg.Geth.WsURL)
	}
	if cfg.Geth.APIKey != "test_api_key_123" {
		t.Errorf("Expected api_key 'test_api_key_123', got '%s'", cfg.Geth.APIKey)
	}

	// Verify other environment overrides work
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
  host: "localhost
  port: [invalid yaml
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
defradb:
  host: "localhost"
  port: 9181

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

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Should keep original values when env vars are invalid
	if cfg.DefraDB.Port != 9181 {
		t.Errorf("Expected port 9181 (original), got %d", cfg.DefraDB.Port)
	}
	if cfg.Indexer.StartHeight != 1000 {
		t.Errorf("Expected start_height 1000 (original), got %d", cfg.Indexer.StartHeight)
	}
}
