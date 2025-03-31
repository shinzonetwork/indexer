package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"shinzo/version1/config"
	"shinzo/version1/pkg/rpc"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Indexer struct {
	defraURL  string
	alchemy   *rpc.AlchemyClient
	logger    *zap.Logger
	config    *config.Config
	lastBlock int
	client    *http.Client
}

type Response struct {
	Data map[string][]struct {
		DocID string `json:"_docID"`
	} `json:"data"`
}

func NewIndexer(cfg *config.Config) (*Indexer, error) {
	logConfig := zap.NewDevelopmentConfig()
	logConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	logger, err := logConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	client := &http.Client{
		Timeout: time.Second * 30,
	}

	i := &Indexer{
		defraURL:  fmt.Sprintf("http://%s:%d", cfg.DefraDB.Host, cfg.DefraDB.Port),
		alchemy:   rpc.NewAlchemyClient(cfg.Alchemy.APIKey),
		logger:    logger,
		config:    cfg,
		lastBlock: cfg.Indexer.StartHeight - 1,
		client:    client,
	}

	return i, nil
}

func (i *Indexer) Start(ctx context.Context) error {
	i.logger.Info("starting indexer",
		zap.Int("start_height", i.config.Indexer.StartHeight),
		zap.Int("batch_size", i.config.Indexer.BatchSize),
		zap.Float64("polling_interval", i.config.Indexer.BlockPollingInterval),
	)

	// Get the highest block number from the database
	highestBlock, err := i.getHighestBlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get highest block: %w", err)
	}

	i.lastBlock = highestBlock
	i.logger.Info("starting from block", zap.Int("block", i.lastBlock+1))

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		ticker := time.NewTicker(time.Duration(i.config.Indexer.BlockPollingInterval * float64(time.Second)))
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				if err := i.processNextBlock(ctx); err != nil {
					i.logger.Error("failed to process block", zap.Error(err))
				}
			}
		}
	})

	return g.Wait()
}

func (i *Indexer) getHighestBlockNumber(ctx context.Context) (int, error) {
	query := `{
		Block(orderBy: {number: DESC}, limit: 1) {
			number
		}
	}`

	resp, err := i.client.Post(
		fmt.Sprintf("%s/api/v0/graphql", i.defraURL),
		"application/json",
		bytes.NewReader([]byte(fmt.Sprintf(`{"query": %q}`, query))),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to get highest block: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to get highest block: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			Block []struct {
				Number string `json:"number"`
			} `json:"Block"`
		}
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data.Block) == 0 {
		return i.config.Indexer.StartHeight - 1, nil
	}

	// Convert hex string to int
	highestBlock := result.Data.Block[0].Number
	if !strings.HasPrefix(highestBlock, "0x") {
		return 0, fmt.Errorf("invalid block number format: %s", highestBlock)
	}

	blockNum, err := strconv.ParseInt(highestBlock[2:], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse block number: %w", err)
	}

	return int(blockNum), nil
}

func (i *Indexer) blockExists(ctx context.Context, blockHash string) (bool, error) {
	query := fmt.Sprintf(`{
		Block(filter: { hash: %q }) {
			_docID
			hash
		}
	}`, blockHash)

	resp, err := i.client.Post(
		fmt.Sprintf("%s/api/v0/graphql", i.defraURL),
		"application/json",
		bytes.NewReader([]byte(fmt.Sprintf(`{"query": %q}`, query))),
	)
	if err != nil {
		return false, fmt.Errorf("failed to check if block exists: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Block []struct {
				DocID string `json:"_docID"`
				Hash  string `json:"hash"`
			} `json:"Block"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	return len(result.Data.Block) > 0, nil
}

func (i *Indexer) postToCollection(ctx context.Context, collection string, data map[string]interface{}) (string, error) {
	// Convert data to GraphQL input format
	var inputFields []string
	for key, value := range data {
		switch v := value.(type) {
		case string:
			inputFields = append(inputFields, fmt.Sprintf("%s: %q", key, v))
		case bool:
			inputFields = append(inputFields, fmt.Sprintf("%s: %v", key, v))
		case []string:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return "", fmt.Errorf("failed to marshal field %s: %w", key, err)
			}
			inputFields = append(inputFields, fmt.Sprintf("%s: %s", key, string(jsonBytes)))
		default:
			inputFields = append(inputFields, fmt.Sprintf("%s: %q", key, fmt.Sprint(v)))
		}
	}

	// Create mutation
	mutation := fmt.Sprintf(`mutation {
		create_%s(input: { %s }) {
			_docID
		}
	}`, collection, strings.Join(inputFields, ", "))

	// Send mutation
	resp, err := i.client.Post(
		fmt.Sprintf("%s/api/v0/collections/%s", i.defraURL, collection),
		"application/json",
		bytes.NewReader([]byte(fmt.Sprintf(`{"query": %q}`, mutation))),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create %s: %w", collection, err)
	}
	defer resp.Body.Close()

	// Parse response
	var response Response
	if err := json.Unmarshal(resp, &response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Look for the create_{collection} field in the response
	createField := fmt.Sprintf("create_%s", collection)
	items, ok := response.Data[createField]
	if !ok || len(items) == 0 {
		return "", fmt.Errorf("no document ID returned")
	}

	return items[0].DocID, nil
}

func (i *Indexer) processNextBlock(ctx context.Context) error {
	blockNumber := i.lastBlock + 1
	i.logger.Info("processing block", zap.Int("number", blockNumber))
	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)

	// Get block from Alchemy
	block, err := i.alchemy.GetBlock(ctx, blockNumberHex)
	if err != nil {
		if ctx.Err() != nil {
			// Context was cancelled, return gracefully
			return nil
		}
		return fmt.Errorf("failed to get block: %w", err)
	}

	// Create block in DefraDB
	blockData := map[string]interface{}{
		"hash":                block.Hash,
		"number":              block.Number,
		"time":                block.Timestamp,
		"parentHash":          block.ParentHash,
		"difficulty":          block.Difficulty,
		"gasUsed":             block.GasUsed,
		"gasLimit":            block.GasLimit,
		"nonce":               block.Nonce,
		"miner":               block.Miner,
		"size":                block.Size,
		"stateRootHash":       block.StateRoot,
		"uncleHash":           block.Sha3Uncles,
		"transactionRootHash": block.TransactionsRoot,
		"receiptRootHash":     block.ReceiptsRoot,
		"extraData":           block.ExtraData,
	}

	_, err = i.postToCollection(ctx, "Block", blockData)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("failed to create block: %w", err)
	}

	// Process each transaction
	for _, tx := range block.Transactions {
		select {
		case <-ctx.Done():
			// Context was cancelled, stop processing transactions
			i.logger.Info("stopping transaction processing due to shutdown",
				zap.Int("block", blockNumber),
				zap.String("hash", block.Hash))
			return nil
		default:
			// Get transaction receipt
			receipt, err := i.alchemy.GetTransactionReceipt(ctx, tx.Hash)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				i.logger.Error("failed to get transaction receipt", zap.String("tx", tx.Hash), zap.Error(err))
				continue
			}

			// Create transaction in DefraDB
			txData := map[string]interface{}{
				"hash":             tx.Hash,
				"blockHash":        tx.BlockHash,
				"blockNumber":      tx.BlockNumber,
				"from":             tx.From,
				"to":               tx.To,
				"value":            tx.Value,
				"gas":              tx.Gas,
				"gasPrice":         tx.GasPrice,
				"input":            tx.Input,
				"nonce":            tx.Nonce,
				"transactionIndex": tx.TransactionIndex,
				"status":           receipt.Status == "0x1",
			}

			_, err = i.postToCollection(ctx, "Transaction", txData)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				i.logger.Error("failed to create transaction", zap.String("tx", tx.Hash), zap.Error(err))
				continue
			}

			// Process logs for this transaction
			for _, log := range receipt.Logs {
				logData := map[string]interface{}{
					"address":          log.Address,
					"topics":           log.Topics,
					"data":             log.Data,
					"blockNumber":      log.BlockNumber,
					"transactionHash":  log.TransactionHash,
					"transactionIndex": log.TransactionIndex,
					"blockHash":        log.BlockHash,
					"logIndex":         log.LogIndex,
					"removed":          log.Removed,
				}

				_, err := i.postToCollection(ctx, "Log", logData)
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					i.logger.Error("failed to create log", zap.String("tx", tx.Hash), zap.String("logIndex", log.LogIndex), zap.Error(err))
					continue
				}

				// Update relationships for this log
				if err := i.updateLogRelationships(ctx, block.Hash, tx.Hash, log.LogIndex); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					i.logger.Error("failed to update log relationships", zap.String("tx", tx.Hash), zap.String("logIndex", log.LogIndex), zap.Error(err))
					continue
				}
			}

			// Update relationships for this transaction
			if err := i.updateTransactionRelationships(ctx, block.Hash, tx.Hash); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				i.logger.Error("failed to update transaction relationships", zap.String("tx", tx.Hash), zap.Error(err))
				continue
			}
		}
	}

	i.lastBlock = blockNumber
	return nil
}

func (i *Indexer) updateTransactionRelationships(ctx context.Context, blockHash, txHash string) error {
	// First query for the block's ID using its hash
	query := fmt.Sprintf(`query {
		Block(filter: {hash: {_eq: %q}}) {
			_docID
		}
	}`, blockHash)

	resp, err := i.postGraphQL(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to get block ID: %w", err)
	}

	var blockResp struct {
		Data struct {
			Block []struct {
				DocID string `json:"_docID"`
			}
		}
	}
	if err := json.Unmarshal(resp, &blockResp); err != nil {
		return fmt.Errorf("failed to decode block response: %w", err)
	}

	if len(blockResp.Data.Block) == 0 {
		return fmt.Errorf("block not found")
	}

	// Update transaction with block relationship using block's ID
	mutation := fmt.Sprintf(`mutation {
		update_Transaction(filter: {hash: {_eq: %q}}, input: {block: %q}) {
			_docID
		}
	}`, txHash, blockResp.Data.Block[0].DocID)

	_, err = i.postGraphQL(ctx, mutation)
	if err != nil {
		return fmt.Errorf("failed to update transaction relationships: %w", err)
	}

	return nil
}

func (i *Indexer) updateLogRelationships(ctx context.Context, blockHash, txHash, logIndex string) error {
	// First query for block and transaction IDs using their hashes
	query := fmt.Sprintf(`query {
		Block(filter: {hash: {_eq: %q}}) {
			_docID
		}
		Transaction(filter: {hash: {_eq: %q}}) {
			_docID
		}
	}`, blockHash, txHash)

	resp, err := i.postGraphQL(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to get IDs: %w", err)
	}

	var idResp struct {
		Data struct {
			Block []struct {
				DocID string `json:"_docID"`
			}
			Transaction []struct {
				DocID string `json:"_docID"`
			}
		}
	}
	if err := json.Unmarshal(resp, &idResp); err != nil {
		return fmt.Errorf("failed to decode ID response: %w", err)
	}

	if len(idResp.Data.Block) == 0 || len(idResp.Data.Transaction) == 0 {
		return fmt.Errorf("block or transaction not found")
	}

	// Update log with block and transaction relationships using their IDs
	mutation := fmt.Sprintf(`mutation {
		update_Log(filter: {logIndex: {_eq: %q}}, input: {
			block: %q,
			transaction: %q
		}) {
			_docID
		}
	}`, logIndex, idResp.Data.Block[0].DocID, idResp.Data.Transaction[0].DocID)

	_, err = i.postGraphQL(ctx, mutation)
	if err != nil {
		return fmt.Errorf("failed to update log relationships: %w", err)
	}

	return nil
}

func (i *Indexer) postGraphQL(ctx context.Context, mutation string) ([]byte, error) {
	i.logger.Debug("sending mutation", zap.String("mutation", mutation))

	resp, err := i.client.Post(
		fmt.Sprintf("%s/api/v0/graphql", i.defraURL),
		"application/json",
		bytes.NewReader([]byte(fmt.Sprintf(`{"query": %q}`, mutation))),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	i.logger.Debug("store response", zap.ByteString("body", body))
	return body, nil
}
