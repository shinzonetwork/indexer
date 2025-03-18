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

type createDocumentResult struct {
	DocID string `json:"_docID"`
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

func (i *Indexer) postToCollection(ctx context.Context, collection string, data map[string]interface{}) (*createDocumentResult, error) {
	// Convert data to GraphQL input format
	var inputFields []string
	for key, value := range data {
		switch v := value.(type) {
		case string:
			inputFields = append(inputFields, fmt.Sprintf("%s: %q", key, v))
		case bool:
			inputFields = append(inputFields, fmt.Sprintf("%s: %v", key, v))
		case map[string]interface{}:
			// Handle relationship fields based on our schema
			if docID, ok := v["docID"].(string); ok && docID != "" {
				switch key {
				case "block":
					inputFields = append(inputFields, fmt.Sprintf("block_id: %q", docID))
				case "transaction":
					inputFields = append(inputFields, fmt.Sprintf("transaction_id: %q", docID))
				case "log":
					inputFields = append(inputFields, fmt.Sprintf("log_id: %q", docID))
				case "event":
					inputFields = append(inputFields, fmt.Sprintf("event_id: %q", docID))
				}
			}
		case []string:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal field %s: %w", key, err)
			}
			inputFields = append(inputFields, fmt.Sprintf("%s: %s", key, string(jsonBytes)))
		default:
			inputFields = append(inputFields, fmt.Sprintf("%s: %q", key, fmt.Sprint(v)))
		}
	}

	// Use create mutation for all collections and handle conflicts gracefully
	mutation := fmt.Sprintf(`mutation {
		create_%s(input: { %s }) {
			_docID
		}
	}`, collection, strings.Join(inputFields, ", "))

	i.logger.Debug("sending mutation", zap.String("collection", collection), zap.String("payload", mutation), zap.Any("data", data))

	resp, err := i.client.Post(
		fmt.Sprintf("%s/api/v0/graphql", i.defraURL),
		"application/json",
		bytes.NewReader([]byte(fmt.Sprintf(`{"query": %q}`, mutation))),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	i.logger.Debug("store response", zap.String("body", string(body)))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create document: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			Create struct {
				DocID string `json:"_docID"`
			} `json:"create_Block,omitempty"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If we get a document ID conflict for a block, just skip it
	if collection == "Block" && len(result.Errors) > 0 && strings.Contains(result.Errors[0].Message, "document with the given ID already exists") {
		i.logger.Debug("skipping existing block", zap.String("hash", data["hash"].(string)))
		query := fmt.Sprintf(`{
			Block(filter: { hash: %q }) {
				_docID
			}
		}`, data["hash"].(string))

		resp, err := i.client.Post(
			fmt.Sprintf("%s/api/v0/graphql", i.defraURL),
			"application/json",
			bytes.NewReader([]byte(fmt.Sprintf(`{"query": %q}`, query))),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing block: %w", err)
		}
		defer resp.Body.Close()

		var existingResult struct {
			Data struct {
				Block []struct {
					DocID string `json:"_docID"`
				} `json:"Block"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&existingResult); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		if len(existingResult.Data.Block) > 0 {
			return &createDocumentResult{DocID: existingResult.Data.Block[0].DocID}, nil
		}
	} else if len(result.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", result.Errors[0].Message)
	}

	return &createDocumentResult{DocID: result.Data.Create.DocID}, nil
}

func (i *Indexer) processNextBlock(ctx context.Context) error {
	blockNumber := i.lastBlock + 1
	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)

	// Get block from Alchemy
	customBlock, err := i.alchemy.GetBlock(ctx, blockNumberHex)
	if err != nil {
		if ctx.Err() != nil {
			// Context was cancelled, return gracefully
			return nil
		}
		return fmt.Errorf("failed to get block %d: %w", blockNumber, err)
	}

	// Create block mutation
	blockData := map[string]interface{}{
		"hash":                customBlock.Hash,
		"number":              blockNumberHex,
		"time":                customBlock.Timestamp,
		"parentHash":          customBlock.ParentHash,
		"difficulty":          customBlock.Difficulty,
		"gasUsed":             customBlock.GasUsed,
		"gasLimit":            customBlock.GasLimit,
		"nonce":               customBlock.Nonce,
		"miner":               customBlock.Miner,
		"size":                customBlock.Size,
		"stateRootHash":       customBlock.StateRoot,
		"uncleHash":           customBlock.Sha3Uncles,
		"transactionRootHash": customBlock.TransactionsRoot,
		"receiptRootHash":     customBlock.ReceiptsRoot,
		"extraData":           customBlock.ExtraData,
	}

	// Create block first
	blockResult, err := i.postToCollection(ctx, "Block", blockData)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("failed to create block: %w", err)
	}

	// Process transactions and create relationships
	for _, tx := range customBlock.Transactions {
		select {
		case <-ctx.Done():
			// Context was cancelled, stop processing transactions
			i.logger.Info("stopping transaction processing due to shutdown",
				zap.Int("block", blockNumber),
				zap.String("hash", customBlock.Hash))
			return nil
		default:
			// Get transaction receipt
			receipt, err := i.alchemy.GetTransactionReceipt(ctx, tx.Hash)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				i.logger.Error("failed to get transaction receipt",
					zap.String("tx", tx.Hash),
					zap.Error(err),
				)
				continue
			}

			// Convert hex status to boolean
			status := false
			if receipt.Status == "0x1" {
				status = true
			}

			// Create transaction with block relationship
			txData := map[string]interface{}{
				"hash":        tx.Hash,
				"blockHash":   customBlock.Hash,
				"blockNumber": blockNumberHex,
				"from":        receipt.From,
				"to":          receipt.To,
				"gas":         receipt.GasUsed,
				"status":      status,
				"block": map[string]interface{}{
					"docID": blockResult.DocID,
				},
			}

			txResult, err := i.postToCollection(ctx, "Transaction", txData)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				i.logger.Error("failed to create transaction",
					zap.String("tx", tx.Hash),
					zap.Error(err),
				)
				continue
			}

			// Process logs for this transaction
			for _, log := range receipt.Logs {
				logData := map[string]interface{}{
					"address":         log.Address,
					"topics":          log.Topics,
					"data":            log.Data,
					"blockNumber":     blockNumberHex,
					"blockHash":       customBlock.Hash,
					"transactionHash": tx.Hash,
					"logIndex":        log.LogIndex,
					"removed":         log.Removed,
					"block": map[string]interface{}{
						"docID": blockResult.DocID,
					},
					"transaction": map[string]interface{}{
						"docID": txResult.DocID,
					},
				}

				_, err = i.postToCollection(ctx, "Log", logData)
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					i.logger.Error("failed to create log",
						zap.String("tx", tx.Hash),
						zap.String("log_index", log.LogIndex),
						zap.Error(err),
					)
					continue
				}
			}
		}
	}

	i.lastBlock = blockNumber
	return nil
}

func (i *Indexer) postGraphQL(ctx context.Context, query string) ([]byte, error) {
	// Create the final JSON payload
	payload := map[string]interface{}{
		"query": query,
	}

	// Marshal the entire payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal mutation: %w", err)
	}

	i.logger.Debug("sending mutation",
		zap.String("query", query),
		zap.String("payload", string(jsonPayload)),
	)

	resp, err := i.client.Post(
		fmt.Sprintf("%s/api/v0/graphql", i.defraURL),
		"application/json",
		bytes.NewReader(jsonPayload),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to store data: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		i.logger.Error("failed to store data",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
			zap.String("payload", string(jsonPayload)),
		)
		return nil, fmt.Errorf("failed to store data: status=%d body=%s", resp.StatusCode, string(body))
	}

	// Parse response to check for GraphQL errors
	var response struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &response); err == nil && len(response.Errors) > 0 {
		i.logger.Error("graphql error",
			zap.String("error", response.Errors[0].Message),
			zap.String("payload", string(jsonPayload)),
		)
		return nil, fmt.Errorf("graphql error: %s", response.Errors[0].Message)
	}

	// Log response on success
	i.logger.Debug("store response",
		zap.String("body", string(body)),
	)

	return body, nil
}
