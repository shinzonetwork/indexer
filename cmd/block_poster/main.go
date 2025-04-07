package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.uber.org/zap"

	"shinzo/version1/config"
	"shinzo/version1/pkg/defra"
	"shinzo/version1/pkg/rpc"
	"shinzo/version1/pkg/types"
)

func main() {
	// Load config
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger, err := zap.NewProduction()
	sugar := logger.Sugar()
	defer logger.Sync()

	// Create Alchemy client
	alchemy := rpc.NewAlchemyClient(cfg.Alchemy.APIKey)

	// Create DefraDB block handler
	blockHandler := defra.NewBlockHandler(cfg.DefraDB.Host, cfg.DefraDB.Port)

	// Starting block number (in decimal)
	// Get the highest block number from DefraDB
	startBlock, err := blockHandler.GetHighestBlockNumber(context.Background())
	if err != nil {
		log.Fatalf("Failed to get highest block number: %v", err)
	}
	if startBlock == 0 {
		startBlock = 21000000
	}
	// startBlock, err := strconv.ParseInt(highestBlock, 10, 64)
	// if err != nil {
	// 	log.Fatalf("failed to decode the block number: %v", err)
	// }
	// endBlock := startBlock + 9000

	// startBlock := 21000000
	endBlock := startBlock + 9000

	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		// Convert to hex for Alchemy API
		blockHex := fmt.Sprintf("0x%x", blockNum)

		sugar.Info("Processing block %d (0x%x)", blockNum, blockNum)

		// Get block with retry logic
		var block *types.Block
		for retries := 0; retries < 3; retries++ {
			block, err = alchemy.GetBlock(context.Background(), blockHex)
			if err == nil {
				break
			}
			sugar.Error("Failed to get block %d, retry %d: %v", blockNum, retries+1, err)
			time.Sleep(time.Second * 1)
		}
		if err != nil {
			sugar.Error("Skipping block %d after all retries failed: %v", blockNum, err)
			continue
		}

		// Get transaction receipts and build nested objects
		var transactions []types.Transaction
		for _, tx := range block.Transactions {
			// Get transaction receipt with retry logic
			var receipt *types.TransactionReceipt
			for retries := 0; retries < 3; retries++ {
				receipt, err = alchemy.GetTransactionReceipt(context.Background(), tx.Hash)
				if err == nil {
					break
				}
				sugar.Error("Failed to get receipt for tx %s, retry %d: %v", tx.Hash, retries+1, err)
				time.Sleep(time.Second * 2)
			}
			if err != nil {
				sugar.Error("Skipping transaction %s after all retries failed: %v", tx.Hash, err)
				continue
			}

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
						BlockNumber:      fmt.Sprintf("0x%x", blockNum),
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
					BlockNumber:      fmt.Sprintf("0x%x", blockNum),
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
				BlockNumber:      fmt.Sprintf("0x%x", blockNum),
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
		docID, err := blockHandler.PostBlock(context.Background(), &types.Block{
			Hash:             block.Hash,
			Number:           fmt.Sprintf("0x%x", blockNum),
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
		}, sugar)

		if err != nil {
			sugar.Error("Failed to post block %d: %v", blockNum, err)
			continue
		}

		sugar.Info("Successfully processed block %d with DocID %s (%d transactions)", blockNum, docID, len(transactions))

		// Add a small delay to avoid rate limiting
		// time.Sleep(time.Millisecond)
	}
}
