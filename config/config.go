package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

const CollectionName = "shinzo"

type Config struct {
	DefraDB struct {
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
	} `yaml:"defradb"`

	Geth struct {
		NodeURL string `yaml:"node_url"`
		WsURL   string `yaml:"ws_url"`
		APIKey  string `yaml:"api_key"`
	} `yaml:"geth"`

	Indexer struct {
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
	} `yaml:"indexer"`

	Source struct {
		NodeURL   string `yaml:"node_url"`
		ChainID   string `yaml:"chain_id"`
		Consensus struct {
			Enabled    bool     `yaml:"enabled"`
			Validators []string `yaml:"validators"`
			P2PPort    int      `yaml:"p2p_port"`
			RPCPort    int      `yaml:"rpc_port"`
		} `yaml:"consensus"`
	} `yaml:"source"`
	Logger struct {
		Development bool `yaml:"development"`
	} `yaml:"logger"`
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

	if port := os.Getenv("DEFRA_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.DefraDB.Port = p
		}
	}

	if host := os.Getenv("DEFRA_HOST"); host != "" {
		cfg.DefraDB.Host = host
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
