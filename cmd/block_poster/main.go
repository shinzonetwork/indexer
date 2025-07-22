package main

import (
	"context"
	"time"

	"shinzo/version1/config"
	"shinzo/version1/pkg/defra"
	"shinzo/version1/pkg/logger"
	"shinzo/version1/pkg/rpc"
	"shinzo/version1/pkg/types"

	"math/big"
)

const (
	BlocksToIndexAtOnce = 10
)

func main() {
	// Load config
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		logger.Sugar.Fatalf("Failed to load config: ", err)
	}
	logger.Init(cfg.Logger.Development)

	// Connect to Geth RPC node (with JSON-RPC support and HTTP fallback)
	client, err := rpc.NewEthereumClient(cfg.Geth.NodeURL) // Empty JSON-RPC addr for now, will use HTTP fallback
	if err != nil {
		logger.Sugar.Fatalf("Failed to connect to Geth node: %v", err)
	}
	defer client.Close()

	// Create DefraDB block handler
	blockHandler, fatalErr := defra.NewBlockHandler(cfg.DefraDB.Host, cfg.DefraDB.Port)
	if fatalErr != nil {
		logger.Sugar.Fatalf("Failed to create block handler: ", fatalErr)
	}

	logger.Sugar.Info("Starting indexer - will process latest blocks from Geth ", cfg.Geth.NodeURL)

	// Main indexing loop - always get latest block from Geth
	for {
		// Always get the latest block from Geth as source of truth
		gethBlock, err := client.GetLatestBlock(context.Background())
		if err != nil {
			logger.Sugar.Error("Failed to get latest block from Geth: ", err)
			time.Sleep(time.Second * 3)
			continue
		}

		blockNum := gethBlock.Number
		logger.Sugar.Info("Processing latest block from Geth: ", blockNum)

		// Get network ID for transaction conversion (skip if it fails)
		networkID, err := client.GetNetworkID(context.Background())
		if err != nil {
			logger.Sugar.Warn("Failed to get Mainnet network ID... defaulting to 1: ", err)
			networkID = big.NewInt(1) // Default to mainnet
		}
		_ = networkID // Use networkID if needed for transaction processing
		logger.Sugar.Debug("Network ID: ", networkID)

		// get transactions from Geth variable
		transactions := gethBlock.Transactions

		// Build the complete block
		block := buildBlock(gethBlock, transactions)

		// Create block in DefraDB
		blockDocId, fatalErr := blockHandler.CreateBlock(context.Background(), block)
		if fatalErr != nil {
			logger.Sugar.Error("Failed to create block in DefraDB: ", fatalErr)
			time.Sleep(time.Second * 3)
			continue
		}
		logger.Sugar.Info("Created block with DocID: ", blockDocId)

		// Process transactions
		for _, tx := range transactions {
			// Create transaction in DefraDB (includes block relationship)
			txDocId, fatalErr := blockHandler.CreateTransaction(context.Background(), &tx, blockDocId)
			if fatalErr != nil {
				logger.Sugar.Error("Failed to create transaction in DefraDB: ", err)
				time.Sleep(time.Second * 3)
				continue
			}
			logger.Sugar.Info("Created transaction with DocID: ", txDocId)

			// Fetch transaction receipt to get logs and events
			receipt, receiptErr := client.GetTransactionReceipt(context.Background(), tx.Hash)
			if receiptErr != nil {
				logger.Sugar.Warn("Failed to get transaction receipt for ", tx.Hash, ": ", receiptErr)
				continue
			}

			//accessentrylist
			for _, accessListEntry := range tx.AccessList {
				ALEDocId, err := blockHandler.CreateAccessListEntry(context.Background(), &accessListEntry, txDocId)
				if err != nil {
					logger.Sugar.Error("Failed to create access list entry in DefraDB: ", err)
					time.Sleep(time.Second * 3)
					continue
				}
				logger.Sugar.Info("Created access list entry with DocID: ", ALEDocId)
			}

			// Process logs from the receipt
			for _, log := range receipt.Logs {
				// Create log in DefraDB (includes block and transaction relationships)
				logDocId, err := blockHandler.CreateLog(context.Background(), &log, blockDocId, txDocId)
				if err != nil {
					logger.Sugar.Error("Failed to create log in DefraDB: ", err)
					time.Sleep(time.Second * 3)
					continue
				}
				logger.Sugar.Info("Created log with DocID: ", logDocId)
			}

		}

		logger.Sugar.Info("Successfully processed block: ", blockNum)

		// Sleep for 12 seconds before checking for next latest block [block time is 13 seconds on avg]
		time.Sleep(time.Duration(cfg.Indexer.BlockPollingInterval) * time.Second)
	}
}

func buildBlock(block *types.Block, transactions []types.Transaction) *types.Block {
	return &types.Block{
		Hash:             block.Hash,
		Number:           block.Number,
		Timestamp:        block.Timestamp,
		ParentHash:       block.ParentHash,
		Difficulty:       block.Difficulty,
		GasUsed:          block.GasUsed,
		GasLimit:         block.GasLimit,
		Nonce:            block.Nonce,
		Miner:            block.Miner,
		Size:             block.Size,
		StateRoot:        block.StateRoot,
		Sha3Uncles:       block.Sha3Uncles,
		TransactionsRoot: block.TransactionsRoot,
		ReceiptsRoot:     block.ReceiptsRoot,
		ExtraData:        block.ExtraData,
		Transactions:     transactions,
	}
}
