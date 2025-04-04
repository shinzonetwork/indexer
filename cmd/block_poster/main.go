package main

import (
	"context"
	"fmt"
	"log"

	"shinzo/version1/config"
	"shinzo/version1/pkg/defra"
	"shinzo/version1/pkg/rpc"
)

func main() {
	// Load config
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create Alchemy client
	alchemy := rpc.NewAlchemyClient(cfg.Alchemy.APIKey)

	// Create DefraDB block handler
	blockHandler := defra.NewBlockHandler(cfg.DefraDB.Host, cfg.DefraDB.Port)

	// Get block from Alchemy
	blockNumber := "0x1145141" // Example block number
	block, err := alchemy.GetBlock(context.Background(), blockNumber)
	if err != nil {
		log.Fatalf("Failed to get block: %v", err)
	}

	// Get transaction receipts and build nested objects
	var transactions []defra.Transaction
	for _, tx := range block.Transactions {
		// Get transaction receipt
		receipt, err := alchemy.GetTransactionReceipt(context.Background(), tx.Hash)
		if err != nil {
			log.Printf("Failed to get receipt for transaction %s: %v", tx.Hash, err)
			continue
		}

		// Build logs
		var logs []defra.Log
		for _, rcptLog := range receipt.Logs {
			logs = append(logs, defra.Log{
				Address:          rcptLog.Address,
				Topics:           rcptLog.Topics,
				Data:             rcptLog.Data,
				BlockNumber:      rcptLog.BlockNumber,
				TransactionHash:  rcptLog.TransactionHash,
				TransactionIndex: rcptLog.TransactionIndex,
				BlockHash:        rcptLog.BlockHash,
				LogIndex:         rcptLog.LogIndex,
				Removed:          rcptLog.Removed,
			})
		}

		// Build transaction
		transactions = append(transactions, defra.Transaction{
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
	docID, err := blockHandler.PostBlock(context.Background(), &defra.Block{
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
	})
	if err != nil {
		log.Fatalf("Failed to post block: %v", err)
	}

	fmt.Printf("Successfully posted block with document ID: %s\n", docID)
}
