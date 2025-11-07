package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/shinzonetwork/app-sdk/pkg/config"
	"gopkg.in/yaml.v3"
)

const CollectionName = "shinzo"

// GethConfig represents Geth node configuration
type GethConfig struct {
	NodeURL string `yaml:"node_url"`
	WsURL   string `yaml:"ws_url"`
	APIKey  string `yaml:"api_key"`
}

// IndexerConfig represents indexer configuration
type IndexerConfig struct {
	StartHeight int `yaml:"start_height"`
}

// Config represents the main configuration structure
type Config struct {
	ShinzoAppConfig *config.Config  // Embedded app-sdk config for defra
	Geth            GethConfig    `yaml:"geth"`
	Indexer         IndexerConfig `yaml:"indexer"`
}

// DefraDBConfig provides backward compatibility access to defradb config
func (c *Config) DefraDB() config.DefraDBConfig {
	if c.ShinzoAppConfig == nil {
		return config.DefraDBConfig{}
	}
	return c.ShinzoAppConfig.DefraDB
}

// Logger provides backward compatibility access to logger config
func (c *Config) Logger() config.LoggerConfig {
	if c.ShinzoAppConfig == nil {
		return config.LoggerConfig{}
	}
	return c.ShinzoAppConfig.Logger
}

// LoadConfig loads configuration from a YAML file and environment variables
func LoadConfig(path string) (*Config, error) {
	// Load app-sdk config first (handles defradb and logger)
	shinzoAppConfig, err := config.LoadConfig(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load app-sdk config: %w", err)
	}

	// Load YAML config for indexer-specific config
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set the app-sdk config
	cfg.ShinzoAppConfig = shinzoAppConfig

	// Override with environment variables for indexer-specific config
	overrideWithEnvVars(&cfg)

	return &cfg, nil
}

// overrideWithEnvVars overrides configuration with environment variables
// Note: DefraDB and Logger config are handled by app-sdk's config.LoadConfig
func overrideWithEnvVars(cfg *Config) {
	// Geth configuration - prioritize GCP Geth node over managed node
	// If GCP_GETH_RPC_URL is empty, fall back to your GCP node IP
	if gcpGethRpcUrl := os.Getenv("GCP_GETH_RPC_URL"); gcpGethRpcUrl != "" {
		cfg.Geth.NodeURL = gcpGethRpcUrl
	} else if gcpRpcUrl := os.Getenv("GCP_RPC_URL"); gcpRpcUrl != "" {
		cfg.Geth.NodeURL = gcpRpcUrl
	}
	// If GCP_GETH_RPC_URL is empty, use your GCP node IP from config
	if cfg.Geth.NodeURL == "" && os.Getenv("GCP_GETH_RPC_URL") == "" {
		cfg.Geth.NodeURL = "http://34.68.131.15:8545"
	}

	if gcpGethWsUrl := os.Getenv("GCP_GETH_WS_URL"); gcpGethWsUrl != "" {
		cfg.Geth.WsURL = gcpGethWsUrl
	} else if gcpWsUrl := os.Getenv("GCP_WS_URL"); gcpWsUrl != "" {
		cfg.Geth.WsURL = gcpWsUrl
	}
	// If GCP_GETH_WS_URL is empty, use your GCP node IP from config
	if cfg.Geth.WsURL == "" && os.Getenv("GCP_GETH_WS_URL") == "" {
		cfg.Geth.WsURL = "ws://34.68.131.15:8546"
	}

	if gcpGethApiKey := os.Getenv("GCP_GETH_API_KEY"); gcpGethApiKey != "" {
		cfg.Geth.APIKey = gcpGethApiKey
	} else if gcpApiKey := os.Getenv("GCP_API_KEY"); gcpApiKey != "" {
		cfg.Geth.APIKey = gcpApiKey
	}

	// Indexer configuration
	if startHeight := os.Getenv("INDEXER_START_HEIGHT"); startHeight != "" {
		if h, err := strconv.Atoi(startHeight); err == nil {
			cfg.Indexer.StartHeight = h
		}
	}
}
