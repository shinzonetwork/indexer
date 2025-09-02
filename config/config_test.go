package config

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear any existing environment variables
	os.Unsetenv("GCP_RPC_URL")
	os.Unsetenv("GCP_WS_URL")
	os.Unsetenv("GCP_API_KEY")
	os.Unsetenv("DEFRA_HOST")
	os.Unsetenv("DEFRA_PORT")
	os.Unsetenv("DEFRA_KEYRING_SECRET")
	os.Unsetenv("INDEXER_START_HEIGHT")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Test DefraDB defaults
	if cfg.DefraDB.Host != "localhost" {
		t.Errorf("Expected default host 'localhost', got '%s'", cfg.DefraDB.Host)
	}
	if cfg.DefraDB.Port != 9181 {
		t.Errorf("Expected default port 9181, got %d", cfg.DefraDB.Port)
	}
	if cfg.DefraDB.KeyringSecret != "" {
		t.Errorf("Expected default keyring_secret '', got '%s'", cfg.DefraDB.KeyringSecret)
	}
	if cfg.DefraDB.P2P.Enabled {
		t.Error("Expected default P2P enabled to be false")
	}
	if cfg.DefraDB.Store.Path != "./data" {
		t.Errorf("Expected default store path './data', got '%s'", cfg.DefraDB.Store.Path)
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
	// Set environment variables
	os.Setenv("GCP_RPC_URL", "https://test-rpc.example.com")
	os.Setenv("GCP_WS_URL", "wss://test-ws.example.com")
	os.Setenv("GCP_API_KEY", "test_api_key_123")
	os.Setenv("LOGGER_DEBUG", "true")
	os.Setenv("DEFRADB_HOST", "test-defra-host")
	os.Setenv("DEFRADB_PORT", "8888")
	os.Setenv("DEFRADB_KEYRING_SECRET", "test_keyring_secret")
	os.Setenv("DEFRADB_P2P_ENABLED", "true")
	os.Setenv("DEFRADB_P2P_BOOTSTRAP_PEERS", "peer1,peer2,peer3")
	os.Setenv("DEFRADB_P2P_LISTEN_ADDR", "/ip4/0.0.0.0/tcp/9171")
	os.Setenv("DEFRADB_STORE_PATH", "/custom/data/path")
	os.Setenv("INDEXER_START_HEIGHT", "2000")
	
	// Clean up environment variables after test
	defer func() {
		os.Unsetenv("GCP_RPC_URL")
		os.Unsetenv("GCP_WS_URL")
		os.Unsetenv("GCP_API_KEY")
		os.Unsetenv("LOGGER_DEBUG")
		os.Unsetenv("DEFRADB_HOST")
		os.Unsetenv("DEFRADB_PORT")
		os.Unsetenv("DEFRADB_KEYRING_SECRET")
		os.Unsetenv("DEFRADB_P2P_ENABLED")
		os.Unsetenv("DEFRADB_P2P_BOOTSTRAP_PEERS")
		os.Unsetenv("DEFRADB_P2P_LISTEN_ADDR")
		os.Unsetenv("DEFRADB_STORE_PATH")
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

	// Verify Logger environment overrides work
	if !cfg.Logger.Development {
		t.Error("Expected logger development to be true")
	}

	// Verify DefraDB environment overrides work
	if cfg.DefraDB.Host != "test-defra-host" {
		t.Errorf("Expected host 'test-defra-host', got '%s'", cfg.DefraDB.Host)
	}
	if cfg.DefraDB.Port != 8888 {
		t.Errorf("Expected port 8888, got %d", cfg.DefraDB.Port)
	}
	if cfg.DefraDB.KeyringSecret != "test_keyring_secret" {
		t.Errorf("Expected keyring_secret 'test_keyring_secret', got '%s'", cfg.DefraDB.KeyringSecret)
	}
	if !cfg.DefraDB.P2P.Enabled {
		t.Error("Expected P2P enabled to be true")
	}
	if len(cfg.DefraDB.P2P.BootstrapPeers) != 3 {
		t.Errorf("Expected 3 bootstrap peers, got %d", len(cfg.DefraDB.P2P.BootstrapPeers))
	}
	if cfg.DefraDB.P2P.BootstrapPeers[0] != "peer1" {
		t.Errorf("Expected first peer 'peer1', got '%s'", cfg.DefraDB.P2P.BootstrapPeers[0])
	}
	if cfg.DefraDB.P2P.ListenAddr != "/ip4/0.0.0.0/tcp/9171" {
		t.Errorf("Expected listen_addr '/ip4/0.0.0.0/tcp/9171', got '%s'", cfg.DefraDB.P2P.ListenAddr)
	}
	if cfg.DefraDB.Store.Path != "/custom/data/path" {
		t.Errorf("Expected store path '/custom/data/path', got '%s'", cfg.DefraDB.Store.Path)
	}

	// Verify Indexer environment overrides work
	if cfg.Indexer.StartHeight != 2000 {
		t.Errorf("Expected start_height 2000, got %d", cfg.Indexer.StartHeight)
	}
}

func TestLoadConfig_InvalidEnvironmentValues(t *testing.T) {
	// Set invalid environment variables (should be ignored)
	os.Setenv("DEFRA_PORT", "not_a_number")
	os.Setenv("INDEXER_START_HEIGHT", "also_not_a_number")
	
	// Clean up environment variables after test
	defer func() {
		os.Unsetenv("DEFRA_PORT")
		os.Unsetenv("INDEXER_START_HEIGHT")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Should keep default values when env vars are invalid
	if cfg.DefraDB.Port != 9181 {
		t.Errorf("Expected port 9181 (default), got %d", cfg.DefraDB.Port)
	}
	if cfg.Indexer.StartHeight != 1800000 {
		t.Errorf("Expected start_height 1800000 (default), got %d", cfg.Indexer.StartHeight)
	}
}
