package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
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

// LoadConfig loads configuration with sensible defaults and environment variable overrides
func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Initialize config with sensible defaults
	cfg := Config{
		DefraDB: struct {
			Host          string `yaml:"host"`
			Port          int    `yaml:"port"`
			KeyringSecret string `yaml:"keyring_secret"`
			P2P           struct {
				Enabled        bool     `yaml:"enabled"`
				BootstrapPeers []string `yaml:"bootstrap_peers"`
				ListenAddr     string   `yaml:"listen_addr"`
			} `yaml:"p2p"`
			Store struct {
				Path string `yaml:"path"`
			} `yaml:"store"`
		}{
			Host:          "localhost",
			Port:          9181,
			KeyringSecret: "",
			P2P: struct {
				Enabled        bool     `yaml:"enabled"`
				BootstrapPeers []string `yaml:"bootstrap_peers"`
				ListenAddr     string   `yaml:"listen_addr"`
			}{
				Enabled:        false,
				BootstrapPeers: []string{},
				ListenAddr:     "",
			},
			Store: struct {
				Path string `yaml:"path"`
			}{
				Path: "./data",
			},
		},
		Geth: struct {
			NodeURL string `yaml:"node_url"`
			WsURL   string `yaml:"ws_url"`
			APIKey  string `yaml:"api_key"`
		}{
			NodeURL: "", // Will be set from environment variables
			WsURL:   "", // Will be set from environment variables
			APIKey:  "", // Will be set from environment variables
		},
		Indexer: struct {
			BlockPollingInterval float64 `yaml:"block_polling_interval"`
			BatchSize            int     `yaml:"batch_size"`
			StartHeight          int     `yaml:"start_height"`
			Pipeline             struct {
				FetchBlocks struct {
					Workers    int `yaml:"workers"`
					BufferSize int `yaml:"buffer_size"`
				} `yaml:"fetch_blocks"`
				ProcessTransactions struct {
					Workers    int `yaml:"workers"`
					BufferSize int `yaml:"buffer_size"`
				} `yaml:"process_transactions"`
				StoreData struct {
					Workers    int `yaml:"workers"`
					BufferSize int `yaml:"buffer_size"`
				} `yaml:"store_data"`
			} `yaml:"pipeline"`
		}{
			BlockPollingInterval: 12.0,
			BatchSize:            100,
			StartHeight:          1800000,
			Pipeline: struct {
				FetchBlocks struct {
					Workers    int `yaml:"workers"`
					BufferSize int `yaml:"buffer_size"`
				} `yaml:"fetch_blocks"`
				ProcessTransactions struct {
					Workers    int `yaml:"workers"`
					BufferSize int `yaml:"buffer_size"`
				} `yaml:"process_transactions"`
				StoreData struct {
					Workers    int `yaml:"workers"`
					BufferSize int `yaml:"buffer_size"`
				} `yaml:"store_data"`
			}{
				FetchBlocks: struct {
					Workers    int `yaml:"workers"`
					BufferSize int `yaml:"buffer_size"`
				}{
					Workers:    4,
					BufferSize: 100,
				},
				ProcessTransactions: struct {
					Workers    int `yaml:"workers"`
					BufferSize int `yaml:"buffer_size"`
				}{
					Workers:    4,
					BufferSize: 100,
				},
				StoreData: struct {
					Workers    int `yaml:"workers"`
					BufferSize int `yaml:"buffer_size"`
				}{
					Workers:    4,
					BufferSize: 100,
				},
			},
		},
		Logger: struct {
			Development bool `yaml:"development"`
		}{
			Development: false,
		},
	}

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
