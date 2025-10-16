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
	"time"

	"github.com/shinzonetwork/indexer/config"
	"github.com/shinzonetwork/indexer/pkg/defra"
	"github.com/shinzonetwork/indexer/pkg/errors"
	"github.com/shinzonetwork/indexer/pkg/logger"
	"github.com/shinzonetwork/indexer/pkg/rpc"
	"github.com/shinzonetwork/indexer/pkg/types"

	"github.com/sourcenetwork/defradb/node"
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

var shouldIndex = false
var IsStarted = false
var HasIndexedAtLeastOneBlock = false

// IndexingMode defines the type of indexing to perform
type IndexingMode string

const (
	ModeRealTime IndexingMode = "realtime"
	ModeCatchUp  IndexingMode = "catchup"
)

// StartIndexing starts the blockchain indexer in real-time mode.
// This is a convenience wrapper around StartIndexingWithMode.
//
// Parameters:
//   - defraStorePath: Path to DefraDB storage directory. Empty string assumes DefraDB is already running.
//   - defraUrl: URL of the DefraDB GraphQL endpoint (e.g., "http://localhost:9181")
//
// Returns error if indexer fails to start or connect to required services.
func StartIndexing(defraStorePath string, defraUrl string) error {
	return StartIndexingWithMode(defraStorePath, defraUrl, ModeRealTime)
}

// StartIndexingWithMode starts the indexer with the specified mode
func StartIndexingWithMode(defraStorePath, defraUrl string, mode IndexingMode) error {
	return StartIndexingWithModeAndConfig(defraStorePath, defraUrl, mode, nil)
}

// StartIndexingWithModeAndConfig starts the indexer with the specified mode and optional config
func StartIndexingWithModeAndConfig(defraStorePath, defraUrl string, mode IndexingMode, cfg *config.Config) error {
	ctx := context.Background()
	shouldIndex = true
	logger.Init(true)

	logger.Sugar.Info("Starting indexer with mode: ", mode)
	logger.Sugar.Info("Defra store path: ", defraStorePath)
	logger.Sugar.Info("Defra URL: ", defraUrl)

	if defraStorePath != "" {
		// Using embedded DefraDB
		options := []node.Option{
			node.WithDisableAPI(false),
			node.WithDisableP2P(false),
			node.WithStorePath(defraStorePath),
		}
		defraNode, err := node.New(ctx, options...)
		if err != nil {
			return fmt.Errorf("Failed to create defra node %v: ", err)
		}

		err = defraNode.Start(ctx)
		if err != nil {
			return fmt.Errorf("Failed to start defra node %v: ", err)
		}
		defer defraNode.Close(ctx)

		err = applySchema(ctx, defraNode)
		if err != nil && !strings.Contains(err.Error(), "collection already exists") {
			return fmt.Errorf("Failed to apply schema to defra node: %v", err)
		}

		err = defra.WaitForDefraDB(defraUrl)
		if err != nil {
			return err
		}
	} else {
		// Using external DefraDB - wait for it and apply schema via HTTP
		err := defra.WaitForDefraDB(defraUrl)
		if err != nil {
			return err
		}

		err = applySchemaViaHTTP(defraUrl)
		if err != nil && !strings.Contains(err.Error(), "collection already exists") {
			return fmt.Errorf("Failed to apply schema to external DefraDB: %v", err)
		}
	}

	// Load config if not provided
	if cfg == nil {
		var err error
		cfg, err = config.LoadConfig("config.yaml")
		if err != nil {
			logger.Sugar.Fatalf("Failed to load config: %v", err)
		}
	}
	logger.Init(cfg.Logger.Development)

	// Connect to Ethereum client with WebSocket and HTTP support
	client, err := rpc.NewEthereumClient(cfg.Geth.NodeURL, cfg.Geth.WsURL, cfg.Geth.APIKey)
	if err != nil {
		logCtx := errors.LogContext(err)
		logger.Sugar.With(logCtx).Fatalf("Failed to connect to Ethereum client: %v", err)
	}
	defer client.Close()

	// Create DefraDB block handler
	blockHandler, err := defra.NewBlockHandler(cfg.DefraDB.Url)
	if err != nil {
		// Log with structured context
		logCtx := errors.LogContext(err)
		logger.Sugar.With(logCtx).Fatalf("Failed to create block handler: ", err)
	}

	logger.Sugar.Info("Starting indexer - will process latest blocks from Geth ", cfg.Geth.NodeURL)

	// Start indexing based on mode
	switch mode {
	case ModeCatchUp:
		return startCatchUpIndexing(ctx, client, blockHandler)
	case ModeRealTime:
		return startRealTimeIndexing(ctx, client, blockHandler, cfg)
	default:
		return fmt.Errorf("unknown indexing mode: %s", mode)
	}
}

// startCatchUpIndexing performs catch-up indexing from last indexed block
func startCatchUpIndexing(ctx context.Context, ethClient *rpc.EthereumClient, blockHandler *defra.BlockHandler) error {
	logger.Sugar.Info("Starting catch-up indexing mode...")

	// Get the last indexed block from DefraDB
	lastIndexedBlock, err := getLastIndexedBlock(ctx, blockHandler)
	if err != nil {
		return fmt.Errorf("failed to get last indexed block: %w", err)
	}

	// Get the latest block from Ethereum
	latestEthBlock, err := ethClient.GetLatestBlock(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest Ethereum block: %w", err)
	}

	latestBlockNum, err := parseBlockNumber(latestEthBlock.Number)
	if err != nil {
		return fmt.Errorf("failed to parse latest block number: %w", err)
	}

	// Process blocks up to the latest block (no offset for newest blocks)
	targetBlockNum := latestBlockNum
	if targetBlockNum < 0 {
		targetBlockNum = 0
	}

	logger.Sugar.Infof("Last indexed block: %d, Latest Ethereum block: %d, Target block: %d",
		lastIndexedBlock, latestBlockNum, targetBlockNum)

	if lastIndexedBlock >= targetBlockNum {
		logger.Sugar.Info("Already caught up to target block! Switching to real-time mode...")
		return startRealTimeIndexing(ctx, ethClient, blockHandler, nil)
	}

	// Calculate blocks to catch up
	blocksToCatchUp := targetBlockNum - lastIndexedBlock
	logger.Sugar.Infof("Need to catch up %d blocks (processing up to latest)", blocksToCatchUp)

	// Start catch-up process
	currentBlock := lastIndexedBlock + 1
	for currentBlock <= targetBlockNum && shouldIndex {
		// Process blocks in batches
		endBlock := currentBlock + DefaultBlocksToIndexAtOnce - 1
		if endBlock > targetBlockNum {
			endBlock = targetBlockNum
		}

		logger.Sugar.Infof("Processing blocks %d to %d", currentBlock, endBlock)

		// Process batch of blocks
		for blockNum := currentBlock; blockNum <= endBlock; blockNum++ {
			if err := processBlock(ctx, ethClient, blockHandler, blockNum); err != nil {
				logger.Sugar.Errorf("Failed to process block %d: %v", blockNum, err)

				// Skip duplicate blocks, retry others
				if strings.Contains(err.Error(), "already exists") {
					logger.Sugar.Infof("Block %d already exists, continuing...", blockNum)
					continue
				}

				// Retry logic for other errors
				retrySuccess := false
				for attempt := 0; attempt < DefaultRetryAttempts; attempt++ {
					logger.Sugar.Warnf("Block %d processing failed (attempt %d/%d): %v", blockNum, attempt+1, DefaultRetryAttempts, err)
					time.Sleep(5 * time.Second)

					if err := processBlock(ctx, ethClient, blockHandler, blockNum); err == nil {
						retrySuccess = true
						break
					}
				}

				if !retrySuccess {
					return fmt.Errorf("failed to process block %d after %d attempts", blockNum, DefaultRetryAttempts)
				}
			}
		}

		currentBlock = endBlock + 1

		// Brief pause between batches to avoid overwhelming the system
		time.Sleep(100 * time.Millisecond)
	}

	logger.Sugar.Info("Catch-up complete! Switching to real-time mode...")
	return startRealTimeIndexing(ctx, ethClient, blockHandler, nil)
}

// startRealTimeIndexing performs simple, efficient real-time indexing
func startRealTimeIndexing(ctx context.Context, ethClient *rpc.EthereumClient, blockHandler *defra.BlockHandler, cfg *config.Config) error {
	logger.Sugar.Info("Starting real-time indexing mode...")

	// Load config if not provided
	if cfg == nil {
		var err error
		cfg, err = config.LoadConfig("config.yaml")
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	// Get starting point - get the latest block from the blockchain
	latestEthBlock, err := ethClient.GetLatestBlock(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest Ethereum block: %w", err)
	}

	latestBlockNum, err := parseBlockNumber(latestEthBlock.Number)
	if err != nil {
		return fmt.Errorf("failed to parse latest block number: %w", err)
	}

	// Start from the latest block
	nextBlockToProcess := latestBlockNum
	if nextBlockToProcess < 0 {
		nextBlockToProcess = 0
	}

	logger.Sugar.Infof("Latest Ethereum block: %d, starting real-time indexing from block %d",
		latestBlockNum, nextBlockToProcess)

	for shouldIndex {
		IsStarted = true

		select {
		case <-ctx.Done():
			logger.Sugar.Info("Real-time indexing stopped")
			return nil
		default:
			// Step 2: Process the specific block we want (nextBlockToProcess)
			logger.Sugar.Infof("=== Processing block %d ===", nextBlockToProcess)

			err := processBlock(ctx, ethClient, blockHandler, nextBlockToProcess)
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
					HasIndexedAtLeastOneBlock = true
					continue
				} else if strings.Contains(err.Error(), "transaction type not supported") {
					// Skip problematic block
					logger.Sugar.Warnf("Block %d has unsupported transaction types, skipping", nextBlockToProcess)
					nextBlockToProcess++
					HasIndexedAtLeastOneBlock = true
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
			HasIndexedAtLeastOneBlock = true

			// Small delay to avoid overwhelming the API
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

// getLastIndexedBlock gets the highest block number from DefraDB
func getLastIndexedBlock(ctx context.Context, blockHandler *defra.BlockHandler) (int64, error) {
	latestBlockNum, err := blockHandler.GetHighestBlockNumber(ctx)
	if err != nil {
		// If no blocks exist, start from configured start height
		if strings.Contains(err.Error(), "blockArray is empty") || strings.Contains(err.Error(), "not found") {
			logger.Sugar.Info("No blocks found in DefraDB, starting from beginning")
			return 23577000, nil
		}
		return 0, err
	}
	return latestBlockNum, nil
}

// processBlock fetches and stores a single block with retry logic
func processBlock(ctx context.Context, ethClient *rpc.EthereumClient, blockHandler *defra.BlockHandler, blockNum int64) error {
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

func StopIndexing() {
	shouldIndex = false
	IsStarted = false
}

// findSchemaFile tries multiple paths to locate the schema file from different working directories
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
