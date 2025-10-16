package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

const CollectionName = "shinzo"

// DefraDBP2PConfig represents P2P configuration for DefraDB
type DefraDBP2PConfig struct {
	BootstrapPeers []string `yaml:"bootstrap_peers"`
	ListenAddr     string   `yaml:"listen_addr"`
}

// DefraDBStoreConfig represents store configuration for DefraDB
type DefraDBStoreConfig struct {
	Path string `yaml:"path"`
}

// DefraDBConfig represents DefraDB configuration
type DefraDBConfig struct {
	Url           string             `yaml:"url"`
	KeyringSecret string             `yaml:"keyring_secret"`
	P2P           DefraDBP2PConfig   `yaml:"p2p"`
	Store         DefraDBStoreConfig `yaml:"store"`
}

// GethConfig represents Geth node configuration
type GethConfig struct {
	NodeURL string `yaml:"node_url"`
	WsURL   string `yaml:"ws_url"`
	APIKey  string `yaml:"api_key"`
}

// PipelineStageConfig represents configuration for a pipeline stage
type PipelineStageConfig struct {
	Workers    int `yaml:"workers"`
	BufferSize int `yaml:"buffer_size"`
}

// IndexerPipelineConfig represents the indexer pipeline configuration
type IndexerPipelineConfig struct {
	FetchBlocks         PipelineStageConfig `yaml:"fetch_blocks"`
	ProcessTransactions PipelineStageConfig `yaml:"process_transactions"`
	StoreData           PipelineStageConfig `yaml:"store_data"`
}

// IndexerConfig represents indexer configuration
type IndexerConfig struct {
	BlockPollingInterval float64               `yaml:"block_polling_interval"`
	BatchSize            int                   `yaml:"batch_size"`
	StartHeight          int                   `yaml:"start_height"`
	Pipeline             IndexerPipelineConfig `yaml:"pipeline"`
}

// LoggerConfig represents logger configuration
type LoggerConfig struct {
	Development bool `yaml:"development"`
}

// Config represents the main configuration structure
type Config struct {
	DefraDB DefraDBConfig `yaml:"defradb"`
	Geth    GethConfig    `yaml:"geth"`
	Indexer IndexerConfig `yaml:"indexer"`
	Logger  LoggerConfig  `yaml:"logger"`
}

// LoadConfig loads configuration from a YAML file and environment variables
func LoadConfig(path string) (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Load YAML config
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Host() method now returns Url directly for backward compatibility

	// Override with environment variables
	if keyringSecret := os.Getenv("DEFRA_KEYRING_SECRET"); keyringSecret != "" {
		cfg.DefraDB.KeyringSecret = keyringSecret
	}

	if startHeight := os.Getenv("INDEXER_START_HEIGHT"); startHeight != "" {
		if h, err := strconv.Atoi(startHeight); err == nil {
			cfg.Indexer.StartHeight = h
		}
	}

	// Override GCP configuration with environment variables
	if gcpRpcUrl := os.Getenv("GCP_RPC_URL"); gcpRpcUrl != "" {
		cfg.Geth.NodeURL = gcpRpcUrl
	}

	if gcpWsUrl := os.Getenv("GCP_WS_URL"); gcpWsUrl != "" {
		cfg.Geth.WsURL = gcpWsUrl
	}

	if gcpApiKey := os.Getenv("GCP_API_KEY"); gcpApiKey != "" {
		cfg.Geth.APIKey = gcpApiKey
	}

	return &cfg, nil
}
