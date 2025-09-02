package indexer

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"github.com/shinzonetwork/indexer/config"
	"github.com/shinzonetwork/indexer/pkg/defra"
	"github.com/shinzonetwork/indexer/pkg/errors"
	"github.com/shinzonetwork/indexer/pkg/logger"
	"github.com/shinzonetwork/indexer/pkg/rpc"
	"github.com/shinzonetwork/indexer/pkg/types"
	"strings"
	"time"

	"github.com/sourcenetwork/defradb/node"
)

const (
	BlocksToIndexAtOnce = 10
	TotalRetryAttempts  = 3
)

var shouldIndex = false
var IsStarted = false
var HasIndexedAtLeastOneBlock = false

func StartIndexing(defraStorePath string, defraUrl string) error {
	ctx := context.Background()
	shouldIndex = true

	if defraStorePath != "" {
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
		if err != nil && !strings.HasPrefix(err.Error(), "collection already exists") { // Todo we are swallowing this error for now, but we should investigate how we update the schemas - do we need to not swallow this error?
			return fmt.Errorf("Failed to apply schema to defra node: %v", err)
		}

		err = defra.WaitForDefraDB(defraUrl)
		if err != nil {
			return err
		}
	}

	// Load config
	_, caller, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(caller)
	filePath := basepath + "/../../" + "config.yaml"
	cfg, err := config.LoadConfig(filePath)
	if err != nil {
		logger.Sugar.Fatalf("Failed to load config: ", err)
	}
	logger.Init(cfg.Logger.Development)

	// Connect to Geth RPC node (with JSON-RPC support and HTTP fallback)
	client, err := rpc.NewEthereumClient(cfg.Geth.NodeURL) // Empty JSON-RPC addr for now, will use HTTP fallback
	if err != nil {
		logCtx := errors.LogContext(err)
		logger.Sugar.With(logCtx).Fatalf("Failed to connect to Geth node: ", err)
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

	// Main indexing loop - always get latest block from Geth
	for shouldIndex {
		IsStarted = true

		// Always get the latest block from Geth as source of truth
		gethBlock, err := client.GetLatestBlock(context.Background())
		if err != nil {
			logCtx := errors.LogContext(err)
			logger.Sugar.With(logCtx).Error("Failed to get latest block from Geth: ", err)
			continue
		}

		blockNum := gethBlock.Number
		logger.Sugar.Info("Processing latest block from Geth: ", blockNum)

		// Get network ID for transaction conversion (skip if it fails)
		networkID, err := client.GetNetworkID(context.Background())
		if err != nil {
			logCtx := errors.LogContext(err)
			logger.Sugar.With(logCtx).Warn("Failed to get network ID... defaulting to 1: ", err)
			networkID = big.NewInt(1) // Default to mainnet
		}
		_ = networkID // Use networkID if needed for transaction processing
		logger.Sugar.Debug("Network ID: ", networkID)

		// get transactions from Geth variable
		transactions := gethBlock.Transactions

		// Build the complete block
		block := buildBlock(gethBlock, transactions)

		// Create block in DefraDB with retry logic
		blockDocId, err := createBlockWithRetry(blockHandler, block, blockNum)
		if err != nil {
			continue // Skip to next block if creation failed
		}
		logger.Sugar.Info("Created block with DocID: ", blockDocId)

		// Process all transactions for this block
		processTransactions(blockHandler, client, transactions, blockDocId)

		logger.Sugar.Info("Successfully processed block: ", blockNum)

		HasIndexedAtLeastOneBlock = true

		// Sleep for 12 seconds before checking for next latest block [block time is 13 seconds on avg]
		time.Sleep(time.Duration(cfg.Indexer.BlockPollingInterval) * time.Second)
	}

	return nil
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
		if errors.IsRetryable(err) && blockRetryAttempts < TotalRetryAttempts {
			retryDelay := errors.GetRetryDelay(err, blockRetryAttempts)
			logger.Sugar.Warnf("Retrying block: ", blockNum, " creation after ", retryDelay)
			time.Sleep(retryDelay)
			blockRetryAttempts++
			continue // Retry the same block
		}

		// Non-retryable error or max retries exceeded - skip this block
		if errors.IsDataError(err) || blockRetryAttempts >= TotalRetryAttempts {
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

func applySchema(ctx context.Context, defraNode *node.Node) error {
	fmt.Println("Applying schema...")

	// If we're in the bin directory, go up one level to find schema
	schemaPath := "schema/schema.graphql"
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		// Try going up one directory (from bin/ to project root)
		schemaPath = "../schema/schema.graphql"
	}

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("Failed to read schema file: %v", err)
	}

	_, err = defraNode.DB.AddSchema(ctx, string(schema))
	return err
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
