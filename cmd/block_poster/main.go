package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"shinzo/version1/config"
	"shinzo/version1/pkg/defra"
	"shinzo/version1/pkg/logger"
	"shinzo/version1/pkg/rpc"
	"shinzo/version1/pkg/types"
)

func main() {
	// Load config
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	logger.Init(cfg.Logger.Development)
	sugar := logger.Sugar
	// sugar := logger.Sugar()

	// Create Alchemy client
	alchemy := rpc.NewAlchemyClient(cfg.Alchemy.APIKey)

	// Create DefraDB block handler
	blockHandler := defra.NewBlockHandler(cfg.DefraDB.Host, cfg.DefraDB.Port)

	// Starting block number (in decimal)
	// Get the highest block number from DefraDB
	startBlock, err := blockHandler.GetHighestBlockNumber(context.Background(), sugar)
	if err != nil {
		log.Fatalf("Failed to get highest block number: %v", err)
	}
	if startBlock == 0 {
		startBlock = int64(cfg.Indexer.StartHeight)
	}

	endBlock := startBlock + 100

	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		// Convert to hex for Alchemy API
		blockHex := fmt.Sprintf("0x%x", blockNum)

		sugar.Info("Processing block: ", blockNum, ", hex: ", blockHex)

		// Get block with retry logic
		var block *types.Block
		for retries := 0; retries < 3; retries++ {
			block, err = alchemy.GetBlock(context.Background(), blockHex)
			if err == nil {
				sugar.Debug("Received block from Alechemy")
				break
			}
			sugar.Error("Failed to get block %d, retry %d: %v", blockNum, retries+1, err)
			time.Sleep(time.Second * 1)
		}
		if err != nil {
			sugar.Error("Skipping block ", blockNum, " after all retries failed: ", err)
			continue
		}

		sugar.Info("... grabbing transactions")
		// Get transaction receipts and build nested objects
		var transactions []types.Transaction
		for _, tx := range block.Transactions {
			// Get transaction receipt with retry logic
			var receipt *types.TransactionReceipt
			for retries := 0; retries < 3; retries++ {
				receipt, err = alchemy.GetTransactionReceipt(context.Background(), tx.Hash)
				if err == nil {
					sugar.Debug("Received transaction from Alcehmy...")
					break
				}
				sugar.Error("Failed to get receipt for tx %s, retry %d: %v", tx.Hash, retries+1, err)
				time.Sleep(time.Second * 2)
			}
			if err != nil {
				sugar.Error("Skipping transaction ", tx.Hash, " after all retries failed: ", err)
				continue
			}
			sugar.Info("... grabbing logs")
			// Build logs with events
			var logs []types.Log
			for _, rcptLog := range receipt.Logs {
				// Create events from log
				var events []types.Event
				if len(rcptLog.Topics) > 0 {
					// First topic is always the event signature
					eventSig := rcptLog.Topics[0]
					// blockInt := blockHandler.convertHexToInt(rcptLog.BlockNumber)
					// Create event
					event := types.Event{
						ContractAddress:  rcptLog.Address,
						EventName:        eventSig,     // We could decode this to human-readable name if we had ABI
						Parameters:       rcptLog.Data, // Raw data, could be decoded with ABI
						TransactionHash:  rcptLog.TransactionHash,
						BlockHash:        rcptLog.BlockHash,
						BlockNumber:      receipt.BlockNumber,
						TransactionIndex: rcptLog.TransactionIndex,
						LogIndex:         rcptLog.LogIndex,
					}
					events = append(events, event)
				}

				// Build log with events
				logs = append(logs, types.Log{
					Address:          rcptLog.Address,
					Topics:           rcptLog.Topics,
					Data:             rcptLog.Data,
					BlockNumber:      rcptLog.BlockNumber,
					TransactionHash:  rcptLog.TransactionHash,
					TransactionIndex: rcptLog.TransactionIndex,
					BlockHash:        rcptLog.BlockHash,
					LogIndex:         rcptLog.LogIndex,
					Removed:          rcptLog.Removed,
					Events:           events,
				})
			}
			// Build transaction
			transactions = append(transactions, types.Transaction{
				Hash:             tx.Hash,
				BlockHash:        tx.BlockHash,
				BlockNumber:      tx.BlockNumber,
				From:             tx.From,
				To:               tx.To,
				Value:            tx.Value,
				Gas:              tx.Gas,
				GasPrice:         tx.GasPrice,
				Input:            tx.Input,
				Nonce:            tx.Nonce,
				TransactionIndex: tx.TransactionIndex,
				Status:           receipt.Status == "0x1",
				Logs:             logs,
			})
		}

		// Post block with nested objects to DefraDB
		block = &types.Block{
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
		blockID, err := blockHandler.CreateBlock(context.Background(), block, sugar)
		if err != nil {
			sugar.Fatalf("failed to create block: %w", err)
		}
		var txID string

		for _, tx := range transactions {
			txID, err = blockHandler.CreateTransaction(context.Background(), &tx, sugar)
			if err != nil {
				sugar.Error("Failed to create transaction: %v", err)
				continue
			}
			// Update transaction relationships using the txID
			if err := blockHandler.UpdateTransactionRelationships(context.Background(), blockID, txID, sugar); err != nil {
				sugar.Error("Failed to update transaction relationships: %v", err)
				continue
			}
		}
		// Process logs and events for each transaction
		for _, tx := range transactions {
			for _, log := range tx.Logs {
				logId, err := blockHandler.CreateLog(context.Background(), &log, sugar)
				if err != nil {
					sugar.Error("Failed to create log: %v", err)
					continue
				}
				sugar.Debug("Log created: " + log.LogIndex)
				// func (h *BlockHandler) UpdateLogRelationships(ctx context.Context, blockId, txId, txHash string, logIndex string, sugar *zap.SugaredLogger) error {
				// Link log to transaction and block
				if err := blockHandler.UpdateLogRelationships(context.Background(), blockID, txID, tx.Hash, log.LogIndex, sugar); err != nil {
					sugar.Error("Failed to update log relationships: %v", err)
					continue
				}
				sugar.Debug("Log linked to transaction: " + log.LogIndex)

				// Process events
				for _, event := range log.Events {
					_, err := blockHandler.CreateEvent(context.Background(), &event, sugar)
					if err != nil {
						sugar.Error("Failed to create event: %v", err)
						continue
					}
					// func (h *BlockHandler) UpdateEventRelationships(ctx context.Context, logIndex, logId, txId, eventLogIndex string, sugar *zap.SugaredLogger) error {
					// Link event to log
					if err := blockHandler.UpdateEventRelationships(context.Background(), log.LogIndex, logId, txID, event.LogIndex, sugar); err != nil {
						sugar.Error("Failed to update event relationships: %v", err)
						continue
					}
				}
			}
		}
		if err != nil {
			sugar.Error("Failed to post block %d: %v", blockNum, err)
			continue
		}

		sugar.Info("Successfully processed block %d with DocID %s (%d transactions)", blockNum, blockID, len(transactions))

		// Add a small delay to avoid rate limiting
		// time.Sleep(time.Millisecond)
	}
}
