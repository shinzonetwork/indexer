package indexer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strconv"
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

// StartIndexingWithMode starts the blockchain indexer with the specified indexing mode.
// Supports both embedded DefraDB (when defraStorePath provided) and external DefraDB instances.
//
// Parameters:
//   - defraStorePath: Path to DefraDB storage directory. Empty string assumes DefraDB is already running.
//   - defraUrl: URL of the DefraDB GraphQL endpoint
//   - mode: IndexingMode (ModeRealTime for live indexing, ModeCatchUp for historical data)
//
// Returns error if any component fails to initialize or connect.
func StartIndexingWithMode(defraStorePath string, defraUrl string, mode IndexingMode) error {
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

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Sugar.Fatalf("Failed to load config: %v", err)
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
	blockHandler, err := defra.NewBlockHandler(cfg.DefraDB.Host, cfg.DefraDB.Port)
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

	// Apply block offset to avoid "transaction type not supported" errors from very recent blocks
	// Process blocks up to DefaultBlockOffset blocks behind the latest block
	targetBlockNum := latestBlockNum - DefaultBlockOffset
	if targetBlockNum < 0 {
		targetBlockNum = 0
	}

	logger.Sugar.Infof("Last indexed block: %d, Latest Ethereum block: %d, Target block (with %d block offset): %d",
		lastIndexedBlock, latestBlockNum, DefaultBlockOffset, targetBlockNum)

	if lastIndexedBlock >= targetBlockNum {
		logger.Sugar.Info("Already caught up to target block! Switching to real-time mode...")
		return startRealTimeIndexing(ctx, ethClient, blockHandler, nil)
	}

	// Calculate blocks to catch up
	blocksToCatchUp := targetBlockNum - lastIndexedBlock
	logger.Sugar.Infof("Need to catch up %d blocks (stopping %d blocks behind latest)", blocksToCatchUp, DefaultBlockOffset)

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

// startRealTimeIndexing performs real-time indexing using GetLatestBlock
func startRealTimeIndexing(ctx context.Context, ethClient *rpc.EthereumClient, blockHandler *defra.BlockHandler, cfg *config.Config) error {
	logger.Sugar.Info("Starting real-time indexing mode...")

	// Load config if not provided
	if cfg == nil {
		var err error
		cfg, err = config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	ticker := time.NewTicker(12 * time.Second) // Ethereum block time
	defer ticker.Stop()

	for shouldIndex {
		IsStarted = true

		select {
		case <-ticker.C:
			// Get latest block from Ethereum
			latestBlock, err := ethClient.GetLatestBlock(ctx)
			if err != nil {
				logCtx := errors.LogContext(err)
				logger.Sugar.With(logCtx).Error("Failed to get latest block from Ethereum: ", err)

				// Handle specific error types
				if strings.Contains(err.Error(), "403 Forbidden") ||
					strings.Contains(err.Error(), "PERMISSION_DENIED") ||
					strings.Contains(err.Error(), "unregistered callers") {
					logger.Sugar.Warn("API key authentication failed, sleeping for 5 seconds before retry...")
					time.Sleep(5 * time.Second)
				} else if strings.Contains(err.Error(), "transaction type not supported") {
					logger.Sugar.Warn("Transaction type not supported, sleeping for 2 seconds before retry...")
					time.Sleep(2 * time.Second)
				} else {
					time.Sleep(1 * time.Second)
				}
				continue
			}

			latestBlockNum, err := parseBlockNumber(latestBlock.Number)
			if err != nil {
				logger.Sugar.Errorf("Failed to parse block number: %v", err)
				continue
			}

			// In real-time mode, process the latest block immediately for critical real-time indexing
			logger.Sugar.Infof("Processing latest block for real-time indexing: %d", latestBlockNum)

			if err := processBlock(ctx, ethClient, blockHandler, latestBlockNum); err != nil {
				// Handle transaction type errors gracefully - log but continue
				if strings.Contains(err.Error(), "transaction type not supported") {
					logger.Sugar.Warnf("Block %d contains unsupported transaction types, but continuing real-time indexing: %v", latestBlockNum, err)
					// Still mark as processed since we attempted the block
					HasIndexedAtLeastOneBlock = true
				} else {
					logger.Sugar.Errorf("Failed to process block %d in real-time mode: %v", latestBlockNum, err)
				}
			} else {
				HasIndexedAtLeastOneBlock = true
			}

		case <-ctx.Done():
			logger.Sugar.Info("Real-time indexing stopped")
			return nil
		}
	}

	return nil
}

// getLastIndexedBlock gets the highest block number from DefraDB
func getLastIndexedBlock(ctx context.Context, blockHandler *defra.BlockHandler) (int64, error) {
	lastBlock, err := blockHandler.GetHighestBlockNumber(ctx)
	if err != nil {
		// If no blocks exist, start from configured start height
		if strings.Contains(err.Error(), "blockArray is empty") || strings.Contains(err.Error(), "not found") {
			logger.Sugar.Info("No blocks found in DefraDB, starting from beginning")
			return 0, nil
		}
		return 0, err
	}
	return lastBlock, nil
}

// processBlock fetches and stores a single block
func processBlock(ctx context.Context, ethClient *rpc.EthereumClient, blockHandler *defra.BlockHandler, blockNum int64) error {
	// Fetch block from Ethereum
	block, err := ethClient.GetBlockByNumber(ctx, big.NewInt(blockNum))
	if err != nil {
		return err
	}

	// Store block in DefraDB
	blockId, err := blockHandler.CreateBlock(ctx, block)
	if err != nil {
		// Handle duplicate block - skip if already exists
		if strings.Contains(err.Error(), "already exists") {
			logger.Sugar.Infof("Block %d already exists in DefraDB, skipping...", blockNum)
			return nil
		}
		return err
	}

	// Store transactions with block relationships
	for _, tx := range block.Transactions {
		txId, err := blockHandler.CreateTransaction(ctx, &tx, blockId)
		if err != nil {
			logger.Sugar.Errorf("Failed to create transaction %s: %v", tx.Hash, err)
			continue
		}

		// Store transaction logs
		for _, log := range tx.Logs {
			_, err := blockHandler.CreateLog(ctx, &log, blockId, txId)
			if err != nil {
				logger.Sugar.Errorf("Failed to create log for tx %s: %v", tx.Hash, err)
				continue
			}

			// Update log relationships
			_, err = blockHandler.UpdateLogRelationships(ctx, blockId, txId, tx.Hash, strconv.Itoa(log.LogIndex))
			if err != nil {
				logger.Sugar.Errorf("Failed to update log relationships: %v", err)
			}
		}
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

// createBlockWithRetry attempts to create a block in DefraDB with retry logic
func createBlockWithRetry(blockHandler *defra.BlockHandler, block *types.Block, blockNum string) (string, error) {
	var blockDocId string
	blockRetryAttempts := 0

	for {
		var err error
		blockDocId, err = blockHandler.CreateBlock(context.Background(), block)
		if err == nil {
			return blockDocId, nil // Success
		}

		logCtx := errors.LogContext(err)
		logger.Sugar.With(logCtx).Errorf("Failed to create block: ", blockNum, " in DefraDB (attempt ", blockRetryAttempts+1)

		// Check if error is retryable
		if errors.IsRetryable(err) && blockRetryAttempts < DefaultRetryAttempts {
			retryDelay := errors.GetRetryDelay(err, blockRetryAttempts)
			logger.Sugar.Warnf("Retrying block: ", blockNum, " creation after ", retryDelay)
			time.Sleep(retryDelay)
			blockRetryAttempts++
			continue // Retry the same block
		}

		// Non-retryable error or max retries exceeded - skip this block
		if errors.IsDataError(err) || blockRetryAttempts >= DefaultRetryAttempts {
			logger.Sugar.Errorf("Skipping block: ", blockNum, " due to error: ", err)
			return "", err // Return error to skip block
		}

		// Critical error - may need to exit
		if errors.IsCritical(err) {
			logger.Sugar.Fatalf("Critical error processing block: ", blockNum, " : ", err)
		}

		// Unknown error type - skip block
		logger.Sugar.Errorf("Unknown error processing block: ", blockNum, " : ", err)
		return "", err
	}
}

// processTransactions handles the processing of all transactions for a block
func processTransactions(blockHandler *defra.BlockHandler, client *rpc.EthereumClient, transactions []types.Transaction, blockDocId string) {
	for _, tx := range transactions {
		processSingleTransaction(blockHandler, client, tx, blockDocId)
	}
}

// processSingleTransaction handles the processing of a single transaction and its related data
func processSingleTransaction(blockHandler *defra.BlockHandler, client *rpc.EthereumClient, tx types.Transaction, blockDocId string) {
	// Create transaction in DefraDB (includes block relationship)
	txDocId, err := blockHandler.CreateTransaction(context.Background(), &tx, blockDocId)
	if err != nil {
		// Log with structured context
		logCtx := errors.LogContext(err)
		logger.Sugar.With(logCtx).Error("Failed to create transaction in DefraDB: ", err)
		return
	}
	logger.Sugar.Info("Created transaction with DocID: ", txDocId)

	// Fetch transaction receipt to get logs and events
	receipt, err := client.GetTransactionReceipt(context.Background(), tx.Hash)
	if err != nil {
		// Log with structured context
		logCtx := errors.LogContext(err)
		logger.Sugar.With(logCtx).Warn("Failed to get transaction receipt for ", tx.Hash, ": ", err)
		return
	}

	// Process access list entries
	processAccessListEntries(blockHandler, tx.AccessList, txDocId)

	// Process logs from the receipt
	processTransactionLogs(blockHandler, receipt.Logs, blockDocId, txDocId)
}

// processAccessListEntries handles the processing of access list entries for a transaction
func processAccessListEntries(blockHandler *defra.BlockHandler, accessList []types.AccessListEntry, txDocId string) {
	for _, accessListEntry := range accessList {
		ALEDocId, err := blockHandler.CreateAccessListEntry(context.Background(), &accessListEntry, txDocId)
		if err != nil {
			// Log with structured context
			logCtx := errors.LogContext(err)
			logger.Sugar.With(logCtx).Error("Failed to create access list entry in DefraDB: ", err)
			continue
		}
		logger.Sugar.Info("Created access list entry with DocID: ", ALEDocId)
	}
}

// processTransactionLogs handles the processing of logs for a transaction
func processTransactionLogs(blockHandler *defra.BlockHandler, logs []types.Log, blockDocId, txDocId string) {
	for _, log := range logs {
		// Create log in DefraDB (includes block and transaction relationships)
		logDocId, err := blockHandler.CreateLog(context.Background(), &log, blockDocId, txDocId)
		if err != nil {
			// Log with structured context
			logCtx := errors.LogContext(err)
			logger.Sugar.With(logCtx).Error("Failed to create log in DefraDB: ", err)
			continue
		}
		logger.Sugar.Info("Created log with DocID: ", logDocId)
	}
}

// buildBlock creates a new block with the same data from gethBlock
func buildBlock(gethBlock *types.Block, transactions []types.Transaction) *types.Block {
	return &types.Block{
		Number:           gethBlock.Number,
		Hash:             gethBlock.Hash,
		ParentHash:       gethBlock.ParentHash,
		Nonce:            gethBlock.Nonce,
		Sha3Uncles:       gethBlock.Sha3Uncles,
		LogsBloom:        gethBlock.LogsBloom,
		TransactionsRoot: gethBlock.TransactionsRoot,
		StateRoot:        gethBlock.StateRoot,
		ReceiptsRoot:     gethBlock.ReceiptsRoot,
		Miner:            gethBlock.Miner,
		Difficulty:       gethBlock.Difficulty,
		TotalDifficulty:  gethBlock.TotalDifficulty,
		ExtraData:        gethBlock.ExtraData,
		Size:             gethBlock.Size,
		GasLimit:         gethBlock.GasLimit,
		GasUsed:          gethBlock.GasUsed,
		Timestamp:        gethBlock.Timestamp,
		Transactions:     transactions,
	}
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

func applySchema(ctx context.Context, defraNode *node.Node) error {
	fmt.Println("Applying schema...")

	schemaPath, err := findSchemaFile()
	if err != nil {
		return err
	}

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("Failed to read schema file: %v", err)
	}

	_, err = defraNode.DB.AddSchema(ctx, string(schema))
	return err
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
