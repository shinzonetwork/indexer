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
	Enabled        bool     `yaml:"enabled"`
}

// DefraDBStoreConfig represents store configuration for DefraDB
type DefraDBStoreConfig struct {
	Path string `yaml:"path"`
}

// DefraDBConfig represents DefraDB configuration
type DefraDBConfig struct {
	Url           string             `yaml:"url"`
	KeyringSecret string             `yaml:"keyring_secret"`
	Playground    bool               `yaml:"playground"`
	P2P           DefraDBP2PConfig   `yaml:"p2p"`
	Store         DefraDBStoreConfig `yaml:"store"`
}

// Host returns the DefraDB host URL for backward compatibility
func (d *DefraDBConfig) Host() string {
	return d.Url
}

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

	// Apply environment variable overrides
	applyEnvOverrides(&cfg)

	// Validate configuration
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// validateConfig validates the configuration
func validateConfig(cfg *Config) error {
	if cfg.Indexer.StartHeight < 0 {
		return fmt.Errorf("start_height must be >= 0")
	}
	return nil
}

// applyEnvOverrides applies environment variable overrides to configuration
func applyEnvOverrides(cfg *Config) {
	// DefraDB configuration
	if defraUrl := os.Getenv("DEFRADB_URL"); defraUrl != "" {
		cfg.DefraDB.Url = defraUrl
	} else if host := os.Getenv("DEFRADB_HOST"); host != "" {
		if port := os.Getenv("DEFRADB_PORT"); port != "" {
			cfg.DefraDB.Url = fmt.Sprintf("http://%s:%s", host, port)
		} else {
			cfg.DefraDB.Url = fmt.Sprintf("http://%s:9181", host)
		}
	}

	if keyringSecret := os.Getenv("DEFRADB_KEYRING_SECRET"); keyringSecret != "" {
		cfg.DefraDB.KeyringSecret = keyringSecret
	}

	if playground := os.Getenv("DEFRADB_PLAYGROUND"); playground != "" {
		if parsed, err := strconv.ParseBool(playground); err == nil {
			cfg.DefraDB.Playground = parsed
		}
		// If parsing fails, keep the YAML default value
	}

	if p2pEnabled := os.Getenv("DEFRADB_P2P_ENABLED"); p2pEnabled != "" {
		if parsed, err := strconv.ParseBool(p2pEnabled); err == nil {
			cfg.DefraDB.P2P.Enabled = parsed
		}
	}

	if listenAddr := os.Getenv("DEFRADB_P2P_LISTEN_ADDR"); listenAddr != "" {
		cfg.DefraDB.P2P.ListenAddr = listenAddr
	}

	if storePath := os.Getenv("DEFRADB_STORE_PATH"); storePath != "" {
		cfg.DefraDB.Store.Path = storePath
	}

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

	// Logger configuration
	if loggerDebug := os.Getenv("LOGGER_DEBUG"); loggerDebug != "" {
		if debug, err := strconv.ParseBool(loggerDebug); err == nil {
			cfg.Logger.Development = debug
		}
	}
}
