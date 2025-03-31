package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"syscall"
	"time"

	"shinzo/version1/pkg/alchemy"
	"shinzo/version1/pkg/config"
	"shinzo/version1/pkg/defra"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	alchemyClient := alchemy.NewClient(cfg.AlchemyAPIKey)
	defraHandler := defra.NewBlockHandler(cfg.DefraHost, cfg.DefraPort)

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start from block 17500000 (or your desired starting block)
	currentBlock := big.NewInt(17500000)

	// Process blocks until interrupted
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Get the latest block number
				latestBlock, err := alchemyClient.GetLatestBlockNumber(ctx)
				if err != nil {
					log.Printf("Failed to get latest block number: %v", err)
					time.Sleep(5 * time.Second)
					continue
				}

				// If we've caught up to the latest block, wait before checking again
				if currentBlock.Cmp(latestBlock) >= 0 {
					log.Printf("Caught up to latest block %s, waiting for new blocks...", latestBlock.String())
					time.Sleep(12 * time.Second) // Average Ethereum block time
					continue
				}

				// Get block by number
				block, err := alchemyClient.GetBlockByNumber(ctx, hexutil.EncodeBig(currentBlock))
				if err != nil {
					log.Printf("Failed to get block %s: %v", currentBlock.String(), err)
					time.Sleep(5 * time.Second)
					continue
				}

				// Post block to DefraDB
				docID, err := defraHandler.PostBlock(ctx, &defra.Block{
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
					Transactions:     convertTransactions(block.Transactions),
				})
				if err != nil {
					log.Printf("Failed to post block %s: %v", currentBlock.String(), err)
					time.Sleep(5 * time.Second)
					continue
				}

				log.Printf("Successfully posted block %s with document ID: %s", currentBlock.String(), docID)

				// Increment block number
				currentBlock.Add(currentBlock, big.NewInt(1))
			}
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	fmt.Println("\nReceived interrupt signal. Shutting down gracefully...")
	cancel()
}

func convertTransactions(alchemyTxs []alchemy.Transaction) []defra.Transaction {
	var transactions []defra.Transaction
	for _, tx := range alchemyTxs {
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
		})
	}
	return transactions
}
