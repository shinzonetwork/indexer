package indexer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shinzonetwork/indexer/config"
	"github.com/shinzonetwork/indexer/pkg/defra"
	"github.com/shinzonetwork/indexer/pkg/errors"
	"github.com/shinzonetwork/indexer/pkg/logger"
	"github.com/shinzonetwork/indexer/pkg/rpc"
	"github.com/shinzonetwork/indexer/pkg/server"
	"github.com/shinzonetwork/indexer/pkg/types"

	"github.com/sourcenetwork/defradb/node"
	
	appsdk "github.com/shinzonetwork/app-sdk/pkg/defra"
	appConfig "github.com/shinzonetwork/app-sdk/pkg/config"
)

const (
	// Default configuration constants - can be made configurable via config file
	DefaultBlocksToIndexAtOnce = 10
	DefaultRetryAttempts       = 3
	DefaultSchemaWaitTimeout   = 15 * time.Second
	DefaultDefraReadyTimeout   = 30 * time.Second
	// DefaultBlockOffset is the number of blocks behind the latest block to process
	// This prevents "transaction type not supported" errors from very recent blocks
	DefaultBlockOffset = 3
)

var requiredPeers []string = []string{} // Here, we can consider adding any "big peers" we need - these requiredPeers can be used as a quick start point to speed up the peer discovery process

const defaultListenAddress string = "/ip4/127.0.0.1/tcp/9171"

type ChainIndexer struct {
	cfg                       *config.Config
	shouldIndex               bool
	isStarted                 bool
	hasIndexedAtLeastOneBlock bool
	defraNode                 *node.Node // Embedded DefraDB node (nil if using external)
	healthServer              *server.HealthServer
	currentBlock              int64
	lastProcessedTime         time.Time
	mutex                     sync.RWMutex
}

func (i *ChainIndexer) IsStarted() bool {
	return i.isStarted
}

func (i *ChainIndexer) HasIndexedAtLeastOneBlock() bool {
	return i.hasIndexedAtLeastOneBlock
}

// GetDefraDBPort returns the port of the embedded DefraDB node, or -1 if using external DefraDB
func (i *ChainIndexer) GetDefraDBPort() int {
	if i.defraNode == nil {
		return -1
	}
	return defra.GetPort(i.defraNode)
}

func CreateIndexer(cfg *config.Config) (*ChainIndexer, error) {
	if cfg == nil {
		return nil, errors.NewConfigurationError(
			"indexer",
			"CreateIndexer",
			"config is nil",
			"host=nil, port=nil",
			nil,
			errors.WithMetadata("host", "nil"),
			errors.WithMetadata("port", "nil"))
	}
	return &ChainIndexer{
		cfg:                       cfg,
		shouldIndex:               false,
		isStarted:                 false,
		hasIndexedAtLeastOneBlock: false,
	}, nil
}

func (i *ChainIndexer) StartIndexing(defraStarted bool) error {
	ctx := context.Background()
	cfg := i.cfg

	if cfg == nil {
		return fmt.Errorf("configuration is required - use config.LoadConfig() to load configuration")
	}
	cfg.DefraDB.P2P.BootstrapPeers = append(cfg.DefraDB.P2P.BootstrapPeers, requiredPeers...)

	// Only initialize logger if it hasn't been initialized yet (e.g., in tests)
	if logger.Sugar == nil {
		logger.Init(cfg.Logger.Development)
	}

	if !defraStarted {
		// Use app-sdk to start DefraDB instance with persistent keys
		// Convert indexer config to app-sdk config
		appCfg := appConfig.Config{
			DefraDB: appConfig.DefraDBConfig{
				Url:           cfg.DefraDB.Url,
				KeyringSecret: cfg.DefraDB.KeyringSecret,
				P2P: appConfig.DefraP2PConfig{
					BootstrapPeers: cfg.DefraDB.P2P.BootstrapPeers,
					ListenAddr:     cfg.DefraDB.P2P.ListenAddr,
				},
				Store: appConfig.DefraStoreConfig{
					Path: cfg.DefraDB.Store.Path,
				},
			},
		}
		// Note: app-sdk P2P config has no Enabled field - P2P should be enabled by ListenAddr
		
		// Debug: Log the P2P configuration being passed to app-sdk
		logger.Sugar.Warnf("=== P2P DEBUG === ListenAddr: '%s', BootstrapPeers: %v", 
			appCfg.DefraDB.P2P.ListenAddr, appCfg.DefraDB.P2P.BootstrapPeers)
		logger.Sugar.Warnf("=== P2P DEBUG === Original config - ListenAddr: '%s', Enabled: %t", 
			cfg.DefraDB.P2P.ListenAddr, cfg.DefraDB.P2P.Enabled)

		defraNode, err := appsdk.StartDefraInstance(&appCfg,
			&appsdk.SchemaApplierFromFile{DefaultPath: "schema/schema.graphql"},
			"Block", "Transaction", "AccessListEntry", "Log")
		if err != nil {
			return fmt.Errorf("Failed to start DefraDB instance with app-sdk: %v", err)
		}

		// Store the defraNode reference for port access
		i.defraNode = defraNode

		// Use the actual DefraDB URL from the started node, not the config URL
		actualDefraURL := defraNode.APIURL
		err = defra.WaitForDefraDB(actualDefraURL)
		if err != nil {
			return err
		}
	} else {
		// Using external DefraDB - wait for it and apply schema via HTTP
		err := defra.WaitForDefraDB(cfg.DefraDB.Url)
		if err != nil {
			return err
		}

		err = applySchemaViaHTTP(cfg.DefraDB.Url)
		if err != nil && !strings.Contains(err.Error(), "collection already exists") {
			return fmt.Errorf("failed to apply schema to external DefraDB: %v", err)
		}
	}

	// Check if defra has any block - use actual DefraDB URL for embedded node
	var defraURL string
	if !defraStarted && i.defraNode != nil {
		// Using embedded DefraDB - use the actual URL from the started node
		defraURL = i.defraNode.APIURL
	} else {
		// Using external DefraDB - use the configured URL
		defraURL = cfg.DefraDB.Url
	}

	blockHandler, err := defra.NewBlockHandler(defraURL)
	if err != nil {
		return fmt.Errorf("failed to create block handler for block check: %v", err)
	}

	startHeight := int64(cfg.Indexer.StartHeight)

	nBlock, err := blockHandler.GetHighestBlockNumber(ctx)
	if err != nil {
		// if error.
		// If no blocks exist, start from configured start height (error is expected)
		logger.Sugar.Info("No existing blocks found, starting from configured height")
	} else if nBlock > 0 && nBlock > startHeight {
		// if nBlock is greater than startHeight; use block from defra
		// if yes increment by 1
		cfg.Indexer.StartHeight = int(nBlock + 1)
		logger.Sugar.Infof("Found existing blocks up to %d, starting from %d", nBlock, cfg.Indexer.StartHeight)
	} else {
		// if nBlock is less than startHeight
		logger.Sugar.Infof("No existing blocks found, starting from configured height")
	}

	// create indexing bool
	i.shouldIndex = true

	// Connect to Ethereum client with WebSocket and HTTP support
	client, err := rpc.NewEthereumClient(cfg.Geth.NodeURL, cfg.Geth.WsURL, cfg.Geth.APIKey)
	if err != nil {
		logCtx := errors.LogContext(err)
		logger.Sugar.With(logCtx).Fatalf("Failed to connect to Ethereum client: %v", err)
	}
	defer client.Close()

	// Reuse the block handler created earlier for processing
	// (blockHandler was already created above for the block check)

	logger.Sugar.Info("Starting indexer - will process latest blocks from Geth ", cfg.Geth.NodeURL)

	// Get starting block number
	nextBlockToProcess := int64(cfg.Indexer.StartHeight)

	// Start health server
	var healthDefraURL string
	if cfg.DefraDB.Url != "" {
		healthDefraURL = cfg.DefraDB.Url
	} else if i.defraNode != nil {
		healthDefraURL = fmt.Sprintf("http://localhost:%d", defra.GetPort(i.defraNode))
	}
	i.healthServer = server.NewHealthServer(8080, i, healthDefraURL)

	// Start health server in background
	go func() {
		if err := i.healthServer.Start(); err != nil {
			logger.Sugar.Errorf("Health server failed: %v", err)
		}
	}()

	for i.shouldIndex {
		i.isStarted = true

		select {
		case <-ctx.Done():
			logger.Sugar.Info("Real-time indexing stopped")
			return nil
		default:
			// Step 2: Process the specific block we want (nextBlockToProcess)
			logger.Sugar.Infof("=== Processing block %d ===", nextBlockToProcess)

			err := i.processBlock(ctx, client, blockHandler, nextBlockToProcess)
			if err != nil {
				if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
					// Step 4: Block doesn't exist yet (we're ahead of the chain) - wait 3 seconds and try again
					logger.Sugar.Infof("Block %d not available yet (ahead of chain), waiting 3s before retry...", nextBlockToProcess)
					time.Sleep(3 * time.Second)
					continue
				} else if strings.Contains(err.Error(), "already exists") {
					// Block already processed, move to next
					logger.Sugar.Infof("Block %d already processed, moving to next", nextBlockToProcess)
					nextBlockToProcess++
					i.hasIndexedAtLeastOneBlock = true
					continue
				} else if strings.Contains(err.Error(), "transaction type not supported") {
					// Skip problematic block
					logger.Sugar.Warnf("Block %d has unsupported transaction types, skipping", nextBlockToProcess)
					nextBlockToProcess++
					i.hasIndexedAtLeastOneBlock = true
					continue
				} else {
					// Other error - retry in 3 seconds
					logger.Sugar.Errorf("Failed to process block %d: %v, retrying in 3s", nextBlockToProcess, err)
					time.Sleep(3 * time.Second)
					continue
				}
			}

			// Success! Move to next block (Step 3: increment by 1 and repeat)
			logger.Sugar.Infof("Successfully processed block %d", nextBlockToProcess)
			nextBlockToProcess++
			i.hasIndexedAtLeastOneBlock = true

			// Small delay to avoid overwhelming the API
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

// processBlock fetches and stores a single block with retry logic
func (i *ChainIndexer) processBlock(ctx context.Context, ethClient *rpc.EthereumClient, blockHandler *defra.BlockHandler, blockNum int64) error {
	var block *types.Block
	var err error

	// Retry logic for fetching block from Ethereum
	for attempt := 0; attempt < DefaultRetryAttempts; attempt++ {
		block, err = ethClient.GetBlockByNumber(ctx, big.NewInt(blockNum))
		if err == nil {
			break
		}

		if attempt < DefaultRetryAttempts-1 {
			retryDelay := time.Duration(attempt+1) * time.Second
			logger.Sugar.Warnf("Failed to fetch block %d (attempt %d/%d): %v, retrying in %v",
				blockNum, attempt+1, DefaultRetryAttempts, err, retryDelay)
			time.Sleep(retryDelay)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to fetch block %d after %d attempts: %w", blockNum, DefaultRetryAttempts, err)
	}

	// Retry logic for storing block in DefraDB
	var blockId string
	for attempt := 0; attempt < DefaultRetryAttempts; attempt++ {
		blockId, err = blockHandler.CreateBlock(ctx, block)
		if err == nil {
			break
		}

		// Handle duplicate block - skip if already exists
		if strings.Contains(err.Error(), "already exists") {
			logger.Sugar.Infof("Block %d already exists in DefraDB, skipping...", blockNum)
			return nil
		}

		if attempt < DefaultRetryAttempts-1 {
			retryDelay := time.Duration(attempt+1) * time.Second
			logger.Sugar.Warnf("Failed to create block %d in DefraDB (attempt %d/%d): %v, retrying in %v",
				blockNum, attempt+1, DefaultRetryAttempts, err, retryDelay)
			time.Sleep(retryDelay)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to create block %d in DefraDB after %d attempts: %w", blockNum, DefaultRetryAttempts, err)
	}

	// Store transactions with block relationships
	for _, tx := range block.Transactions {
		// Retry logic for creating transaction
		var txId string
		for attempt := 0; attempt < DefaultRetryAttempts; attempt++ {
			txId, err = blockHandler.CreateTransaction(ctx, &tx, blockId)
			if err == nil {
				break
			}

			if attempt < DefaultRetryAttempts-1 {
				retryDelay := time.Duration(attempt+1) * time.Second
				logger.Sugar.Warnf("Failed to create transaction %s (attempt %d/%d): %v, retrying in %v",
					tx.Hash, attempt+1, DefaultRetryAttempts, err, retryDelay)
				time.Sleep(retryDelay)
			}
		}
		if err != nil {
			logger.Sugar.Errorf("Failed to create transaction %s after %d attempts: %v", tx.Hash, DefaultRetryAttempts, err)
			continue
		}

		// Retry logic for fetching transaction receipt
		var receipt *types.TransactionReceipt
		for attempt := 0; attempt < DefaultRetryAttempts; attempt++ {
			receipt, err = ethClient.GetTransactionReceipt(ctx, tx.Hash)
			if err == nil {
				break
			}

			if attempt < DefaultRetryAttempts-1 {
				retryDelay := time.Duration(attempt+1) * time.Second
				logger.Sugar.Warnf("Failed to get receipt for transaction %s (attempt %d/%d): %v, retrying in %v",
					tx.Hash, attempt+1, DefaultRetryAttempts, err, retryDelay)
				time.Sleep(retryDelay)
			}
		}
		if err != nil {
			logger.Sugar.Errorf("Failed to get receipt for transaction %s after %d attempts: %v", tx.Hash, DefaultRetryAttempts, err)
			continue
		}

		// Store access list entries for EIP-2930/EIP-1559 transactions
		for _, accessListEntry := range tx.AccessList {
			_, err := blockHandler.CreateAccessListEntry(ctx, &accessListEntry, txId)
			if err != nil {
				logger.Sugar.Errorf("Failed to create access list entry for tx %s: %v", tx.Hash, err)
				continue
			}
		}

		// Store transaction logs from receipt
		for _, log := range receipt.Logs {
			_, err := blockHandler.CreateLog(ctx, &log, blockId, txId)
			if err != nil {
				logger.Sugar.Errorf("Failed to create log for tx %s: %v", tx.Hash, err)
				continue
			}
			// Note: Relationships are already established in CreateLog, no need to update separately
		}

		logger.Sugar.Debugf("Processed transaction %s with %d access list entries and %d logs", tx.Hash, len(tx.AccessList), len(receipt.Logs))
	}

	logger.Sugar.Debugf("Successfully processed block %d with %d transactions", blockNum, len(block.Transactions))
	i.updateBlockInfo(blockNum)
	return nil
}

// parseBlockNumber converts hex string to int64
func parseBlockNumber(hexStr string) (int64, error) {
	if strings.HasPrefix(hexStr, "0x") {
		blockNum := new(big.Int)
		blockNum.SetString(hexStr[2:], 16)
		return blockNum.Int64(), nil
	}

	blockNum := new(big.Int)
	blockNum.SetString(hexStr, 10)
	return blockNum.Int64(), nil
}

func applySchema(ctx context.Context, defraNode *node.Node) error {
	fmt.Println("Applying schema...")

	// Try different possible paths for the schema file
	possiblePaths := []string{
		"schema/schema.graphql",       // From project root
		"../schema/schema.graphql",    // From bin/ directory
		"../../schema/schema.graphql", // From pkg/host/ directory - test context
	}

	var schemaPath string
	var err error
	for _, path := range possiblePaths {
		if _, err = os.Stat(path); err == nil {
			schemaPath = path
			break
		}
	}

	if schemaPath == "" {
		return fmt.Errorf("Failed to find schema file in any of the expected locations: %v", possiblePaths)
	}

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("Failed to read schema file: %v", err)
	}

	_, err = defraNode.DB.AddSchema(ctx, string(schema))
	return err
}

func (i *ChainIndexer) StopIndexing() {
	i.shouldIndex = false
	i.isStarted = false

	// Stop health server
	if i.healthServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		i.healthServer.Stop(ctx)
	}

	// Close embedded DefraDB node if it exists
	if i.defraNode != nil {
		i.defraNode.Close(context.Background())
		i.defraNode = nil
	}
}

// Health checker interface implementation
func (i *ChainIndexer) IsHealthy() bool {
	i.mutex.RLock()
	defer i.mutex.RUnlock()

	// Consider healthy if started and processed at least one block recently
	if !i.isStarted {
		return false
	}

	// If we've never processed a block, we're still healthy (starting up)
	if i.lastProcessedTime.IsZero() {
		return true
	}

	// Consider unhealthy if no blocks processed in last 10 minutes
	return time.Since(i.lastProcessedTime) < 10*time.Minute
}

func (i *ChainIndexer) GetCurrentBlock() int64 {
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	return i.currentBlock
}

func (i *ChainIndexer) GetLastProcessedTime() time.Time {
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	return i.lastProcessedTime
}

// updateBlockInfo updates the current block and last processed time
func (i *ChainIndexer) updateBlockInfo(blockNum int64) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.currentBlock = blockNum
	i.lastProcessedTime = time.Now()
}

func findSchemaFile() (string, error) {
	schemaPaths := []string{
		"schema/schema.graphql",          // From project root
		"../schema/schema.graphql",       // From subdirectory (like integration/)
		"../../schema/schema.graphql",    // From deeper subdirectory (like integration/live/)
		"../../../schema/schema.graphql", // From even deeper directories
	}

	for _, path := range schemaPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("Failed to find schema file. Tried paths: %v", schemaPaths)
}

func applySchemaViaHTTP(defraUrl string) error {
	fmt.Println("Applying schema via HTTP...")

	schemaPath, err := findSchemaFile()
	if err != nil {
		return err
	}

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("Failed to read schema file: %v", err)
	}
	// Apply schema via REST API endpoint
	schemaURL := fmt.Sprintf("%s/api/v0/schema", defraUrl)
	resp, err := http.Post(schemaURL, "application/schema", bytes.NewBuffer(schema))
	if err != nil {
		return fmt.Errorf("Failed to send schema: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Schema application failed with status %d: %s", resp.StatusCode, string(body))
	}

	fmt.Println("Schema applied successfully!")
	return nil
}
