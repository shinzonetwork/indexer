package defra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"shinzo/version1/pkg/types"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

type BlockHandler struct {
	defraURL string
	client   *http.Client
}

type FatalError interface {
	err() string
}

func NewBlockHandler(host string, port int) *BlockHandler {
	return &BlockHandler{
		defraURL: fmt.Sprintf("http://%s:%d/api/v0/graphql", host, port),
		client:   &http.Client{},
	}
}

type Response struct {
	Data map[string][]struct {
		DocID string `json:"_docID"`
	} `json:"data"`
}

// PostBlock posts a block and its nested objects to DefraDB
func (h *BlockHandler) PostBlock(ctx context.Context, block *types.Block, sugar *zap.SugaredLogger) (string, error) {
	// Post block first
	blockID, err := h.CreateBlock(ctx, block, sugar)
	if err != nil {
		sugar.Fatal("failed to create block: %w", err)
		return "", fmt.Errorf("failed to create block: %w", err)
	}

	sugar.Debug("Block created: " + blockID)
	// Process transactions
	for _, tx := range block.Transactions {
		txId, err := h.CreateTransaction(ctx, &tx, sugar)
		if err != nil {
			sugar.Error("failed to create transaction: %w", err)
			return "", fmt.Errorf("failed to create transaction: %w", err)
		}
		sugar.Debug("Transaction created: " + tx.Hash)

		// Link transaction to block
		if err := h.UpdateTransactionRelationships(ctx, block.Hash, tx.Hash, sugar); err != nil {
			sugar.Fatal("failed to update transaction relationships: %w", err)
			return "", fmt.Errorf("failed to update transaction relationships: %w", err)
		}

		sugar.Debug("Transaction linked to block: " + tx.Hash)
		// Process logs
		for _, log := range tx.Logs {
			logDocId, err := h.CreateLog(ctx, &log, sugar)
			if err != nil {
				sugar.Fatal("failed to create log: %w", err)
				return "", fmt.Errorf("failed to create log: %w", err)
			}
			sugar.Debug("Log created: " + log.LogIndex)

			// Link log to transaction and block
			if err := h.UpdateLogRelationships(ctx, blockID, txId, tx.Hash, log.LogIndex, sugar); err != nil {
				sugar.Fatal("failed to update log relationships: %w", err)
				return "", fmt.Errorf("failed to update log relationships: %w", err)
			}
			sugar.Debug("Log linked to transaction: " + log.LogIndex)

			// Process events
			for _, event := range log.Events {
				_, err := h.CreateEvent(ctx, &event, sugar)
				if err != nil {
					sugar.Fatal("failed to create event: %w", err)
					return "", fmt.Errorf("failed to create event: %w", err)
				}

				// Link event to log
				if err := h.UpdateEventRelationships(ctx, log.LogIndex, logDocId, txId, event.LogIndex, sugar); err != nil {
					sugar.Fatal("failed to update event relationships: %w", err)
					return "", fmt.Errorf("failed to update event relationships: %w", err)
				}
			}
		}
	}

	return blockID, nil
}

func (h *BlockHandler) ConvertHexToInt(s string, sugar *zap.SugaredLogger) int64 {
	block16 := s[2:]
	blockInt, err := strconv.ParseInt(block16, 16, 64)
	if err != nil {
		sugar.Fatalf("Failed to ParseInt(%v): ", err)
	}
	return blockInt
}

func (h *BlockHandler) CreateBlock(ctx context.Context, block *types.Block, sugar *zap.SugaredLogger) (string, error) {
	// Convert string number to int
	blockInt, err := strconv.ParseInt(block.Number, 0, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse block number: %w", err)
	}

	blockData := map[string]interface{}{
		"hash":             block.Hash,
		"number":           blockInt,
		"timestamp":        block.Timestamp,
		"parentHash":       block.ParentHash,
		"difficulty":       block.Difficulty,
		"gasUsed":          block.GasUsed,
		"gasLimit":         block.GasLimit,
		"nonce":            block.Nonce,
		"miner":            block.Miner,
		"size":             block.Size,
		"stateRoot":        block.StateRoot,
		"sha3Uncles":       block.Sha3Uncles,
		"transactionsRoot": block.TransactionsRoot,
		"receiptsRoot":     block.ReceiptsRoot,
		"extraData":        block.ExtraData,
	}
	sugar.Debug("Posting blockdata to collection endpoint: ", blockData, ctx)
	return h.postToCollection(ctx, "Block", blockData, sugar)
}

func (h *BlockHandler) CreateTransaction(ctx context.Context, tx *types.Transaction, sugar *zap.SugaredLogger) (string, error) {
	blockInt, err := strconv.ParseInt(tx.BlockNumber, 0, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse block number: %w", err)
	}

	txData := map[string]interface{}{
		"hash":             tx.Hash,
		"blockHash":        tx.BlockHash,
		"blockNumber":      blockInt, // This is correct - blockInt is already converted to int64
		"from":             tx.From,
		"to":               tx.To,
		"value":            tx.Value,
		"gasUsed":          tx.Gas, // Map Gas to gasUsed
		"gasPrice":         tx.GasPrice,
		"inputData":        tx.Input, // Map Input to inputData
		"nonce":            tx.Nonce,
		"transactionIndex": tx.TransactionIndex,
	}
	sugar.Debug("Posting blockdata to collection endpoint: ", txData, ctx)
	return h.postToCollection(ctx, "Transaction", txData, sugar)
}

func (h *BlockHandler) CreateLog(ctx context.Context, log *types.Log, sugar *zap.SugaredLogger) (string, error) {
	blockInt, err := strconv.ParseInt(log.BlockNumber, 0, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse block number: %w", err)
	}

	logData := map[string]interface{}{
		"address":          log.Address,
		"topics":           log.Topics,
		"data":             log.Data,
		"blockNumber":      blockInt,
		"transactionHash":  log.TransactionHash,
		"transactionIndex": log.TransactionIndex,
		"blockHash":        log.BlockHash,
		"logIndex":         log.LogIndex,
		"removed":          fmt.Sprintf("%v", log.Removed), // Convert bool to string
	}
	sugar.Debug("Posting to collection, ", logData, ctx)
	return h.postToCollection(ctx, "Log", logData, sugar)
}

func (h *BlockHandler) CreateEvent(ctx context.Context, event *types.Event, sugar *zap.SugaredLogger) (string, error) {
	blockInt, err := strconv.ParseInt(event.BlockNumber, 0, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse block number: %w", err)
	}

	eventData := map[string]interface{}{
		"contractAddress":  event.ContractAddress,
		"eventName":        event.EventName,
		"parameters":       event.Parameters,
		"transactionHash":  event.TransactionHash,
		"blockHash":        event.BlockHash,
		"blockNumber":      blockInt,
		"transactionIndex": event.TransactionIndex,
		"logIndex":         event.LogIndex,
	}

	return h.postToCollection(ctx, "Event", eventData, sugar)
}

func (h *BlockHandler) UpdateTransactionRelationships(ctx context.Context, blockId string, txHash string, sugar *zap.SugaredLogger) error {
	// Get block ID
	// query := fmt.Sprintf(`query {
	// 	Block(filter: {hash: {_eq: %q}}) {
	// 		_docID
	// 	}
	// }`, blockHash)

	// resp, err := h.postGraphQL(ctx, query, sugar)
	// if err != nil {
	// 	sugar.Error("failed to get block ID: %w", err)
	// 	return fmt.Errorf("failed to get block ID: %w", err)
	// }

	// var blockResp struct {
	// 	Data struct {
	// 		Block []struct {
	// 			DocID string `json:"_docID"`
	// 		}
	// 	}
	// }
	// if err := json.Unmarshal(resp, &blockResp); err != nil {
	// 	sugar.Error("failed to decode block response: %w", err)
	// 	return fmt.Errorf("failed to decode block response: %w", err)
	// }

	// if len(blockResp.Data.Block) == 0 {
	// 	sugar.Error("block not found")
	// 	return fmt.Errorf("block not found")
	// }

	// Update transaction with block relationship
	mutation := fmt.Sprintf(`mutation {
		update_Transaction(filter: {hash: {_eq: %q}}, input: {block: %q}) {
			_docID
		}
	}`, txHash, blockId)

	docId := h.PostGraphQL(ctx, mutation, sugar)
	if docId == nil {
		sugar.Errorf("failed to update transaction relationships: ", mutation)
	}

	return nil
}

func (h *BlockHandler) UpdateLogRelationships(ctx context.Context, blockId, txId, txHash string, logIndex string, sugar *zap.SugaredLogger) error {
	// Get block and transaction IDs
	// query := fmt.Sprintf(`query {
	// 	Block(filter: {hash: {_eq: %q}}) {
	// 		_docID
	// 	}
	// 	Transaction(filter: {hash: {_eq: %q}}) {
	// 		_docID
	// 	}
	// }`, blockHash, txHash)

	// resp, err := h.postGraphQL(ctx, query, sugar)
	// if err != nil {
	// 	return fmt.Errorf("failed to get IDs: %w", err)
	// }

	// var idResp struct {
	// 	Data struct {
	// 		Block []struct {
	// 			DocID string `json:"_docID"`
	// 		}
	// 		Transaction []struct {
	// 			DocID string `json:"_docID"`
	// 		}
	// 	}
	// }
	// if err := json.Unmarshal(resp, &idResp); err != nil {
	// 	sugar.Error("failed to decode ID response: %w", err)
	// 	return fmt.Errorf("failed to decode ID response: %w", err)
	// }

	// if len(idResp.Data.Block) == 0 || len(idResp.Data.Transaction) == 0 {
	// 	sugar.Error("block or transaction not found")
	// 	return fmt.Errorf("block or transaction not found")
	// }

	// Update log with block and transaction relationships
	mutation := fmt.Sprintf(`mutation {
		update_Log(filter: {logIndex: {_eq: %q}, transactionHash: {_eq: %q}}, input: {
			block: %q,
			transaction: %q
		}) {
			_docID
		}
	}`, logIndex, txHash, blockId, txId)

	docId := h.PostGraphQL(ctx, mutation, sugar)
	if docId == nil {
		sugar.Fatalf("log relationship update failure")
	}
	return nil
}

func (h *BlockHandler) UpdateEventRelationships(ctx context.Context, logIndex, logId, txId, eventLogIndex string, sugar *zap.SugaredLogger) error {
	// Get log ID
	// query := fmt.Sprintf(`query {
	// 	Log(filter: {logIndex: {_eq: %q}, transactionHash:{_eq:%q} }) {
	// 		_docID
	// 	}
	// }`, logIndex, txHash)

	// resp, err := h.postGraphQL(ctx, query, sugar)
	// if err != nil {
	// 	sugar.Error("failed to get log ID: %w", err)
	// 	return fmt.Errorf("failed to get log ID: %w", err)
	// }

	// var logResp struct {
	// 	Data struct {
	// 		Log []struct {
	// 			DocID string `json:"_docID"`
	// 		}
	// 	}
	// }
	// if err := json.Unmarshal(resp, &logResp); err != nil {
	// 	sugar.Error("failed to decode log response: %w", err)
	// 	return fmt.Errorf("failed to decode log response: %w", err)
	// }

	// if len(logResp.Data.Log) == 0 {
	// 	sugar.Error("log not found")
	// 	return fmt.Errorf("log not found")
	// }

	// Update event with log relationship
	mutation := fmt.Sprintf(`mutation {
		update_Event(filter: {logIndex: {_eq: %q}}, input: {log: %q}) {
			_docID
		}
	}`, eventLogIndex, logId)

	docId := h.PostGraphQL(ctx, mutation, sugar)
	if docId == nil {
		sugar.Fatalf("log relationship update failure")
	}
	return nil
}

func (h *BlockHandler) postToCollection(ctx context.Context, collection string, data map[string]interface{}, sugar *zap.SugaredLogger) (string, error) {
	// Convert data to GraphQL input format
	var inputFields []string
	for key, value := range data {
		switch v := value.(type) {
		case string:
			inputFields = append(inputFields, fmt.Sprintf("%s: %q", key, v))
		case bool:
			inputFields = append(inputFields, fmt.Sprintf("%s: %v", key, v))
		case int, int64:
			inputFields = append(inputFields, fmt.Sprintf("%s: %d", key, v))
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
	sugar.Debug("Input fields: ", strings.Join(inputFields, ", "), "\n")
	sugar.Debug("Collection: ", collection, "\n")
	sugar.Debug("Http: ", h.defraURL, "\n")
	// Create mutation
	mutation := fmt.Sprintf(`mutation {
		create_%s(input: { %s }) {
			_docID
		}
	}`, collection, strings.Join(inputFields, ", "))

	// Debug: Print the mutation
	sugar.Info("Sending mutation: ", mutation)

	// Send mutation
	resp := h.PostGraphQL(ctx, mutation, sugar)

	// Parse response
	var response Response
	if err := json.Unmarshal(resp, &response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Get document ID
	createField := fmt.Sprintf("create_%s", collection)
	items, ok := response.Data[createField]
	if !ok || len(items) == 0 {
		return "", fmt.Errorf("no document ID returned")
	}

	return items[0].DocID, nil
}

func (h *BlockHandler) PostGraphQL(ctx context.Context, mutation string, sugar *zap.SugaredLogger) []byte {
	// Create request body
	body := map[string]string{
		"query": mutation,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		sugar.Errorf("failed to marshal request body: ", err)
	}

	// Debug: Print the mutation
	sugar.Info("Sending mutation: ", mutation, "\n")

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", h.defraURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		sugar.Errorf("failed to create request: ", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := h.client.Do(req)
	if err != nil {
		sugar.Errorf("failed to send request: ", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		sugar.Errorf("Read response error: ", err) // todo turn to error interface
	}

	// Debug: Print the response
	sugar.Info("DefraDB Response: ", string(respBody), "\n")

	return respBody
}

// GetHighestBlockNumber returns the highest block number stored in DefraDB
func (h *BlockHandler) GetHighestBlockNumber(ctx context.Context, sugar *zap.SugaredLogger) (int64, error) {
	query := `query {
		Block(order: {number: DESC}, limit: 1) {
			number
		}	
	}`

	resp := h.PostGraphQL(ctx, query, sugar)
	if resp == nil {
		sugar.Errorf("failed to query block numbers error: ", resp)
	}

	var result struct {
		Data struct {
			Block []struct {
				Number int64 `json:"number"`
			} `json:"Block"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data.Block) == 0 {
		return 0, nil // Return 0 if no blocks exist
	}

	// Find the highest block number
	var highest int64
	for _, block := range result.Data.Block {
		if block.Number > highest {
			highest = block.Number
		}
	}

	return highest + 1, nil
}
