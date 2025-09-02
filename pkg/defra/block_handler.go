package defra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"github.com/shinzonetwork/indexer/pkg/errors"
	"github.com/shinzonetwork/indexer/pkg/logger"
	"github.com/shinzonetwork/indexer/pkg/types"
	"github.com/shinzonetwork/indexer/pkg/utils"
	"strconv"
	"strings"
)

type BlockHandler struct {
	defraURL string
	client   *http.Client
}

func NewBlockHandler(host string, port int) (*BlockHandler, error) {
	if host == "" {
		return nil, errors.NewConfigurationError("defra", "NewBlockHandler",
			"host parameter is empty", host, nil)
	}
	if port == 0 {
		return nil, errors.NewConfigurationError("defra", "NewBlockHandler",
			"port parameter is zero", fmt.Sprintf("%d", port), nil)
	}
	return &BlockHandler{
		defraURL: fmt.Sprintf("http://%s:%d/api/v0/graphql", host, port),
		client:   &http.Client{},
	}, nil
}

func (h *BlockHandler) CreateBlock(ctx context.Context, block *types.Block) (string, error) {
	// Input validation
	if block == nil {
		return "", errors.NewInvalidBlockFormat("defra", "CreateBlock", fmt.Sprintf("%v", block), nil)
	}

	// Data conversion
	blockInt, err := utils.HexToInt(block.Number)
	if err != nil {
		return "", err // Already properly wrapped
	}

	// Create block data
	blockData := map[string]interface{}{
		"hash":             block.Hash,
		"number":           blockInt,
		"timestamp":        block.Timestamp,
		"parentHash":       block.ParentHash,
		"difficulty":       block.Difficulty,
		"totalDifficulty":  block.TotalDifficulty,
		"gasUsed":          block.GasUsed,
		"gasLimit":         block.GasLimit,
		"baseFeePerGas":    block.BaseFeePerGas,
		"nonce":            block.Nonce,
		"miner":            block.Miner,
		"size":             block.Size,
		"stateRoot":        block.StateRoot,
		"sha3Uncles":       block.Sha3Uncles,
		"transactionsRoot": block.TransactionsRoot,
		"receiptsRoot":     block.ReceiptsRoot,
		"logsBloom":        block.LogsBloom,
		"extraData":        block.ExtraData,
		"mixHash":          block.MixHash,
		"uncles":           block.Uncles,
	}
	// Post block data to collection endpoint
	logger.Sugar.Debug("Posting blockdata to collection endpoint: ", blockData)
	// Database operation
	docID, err := h.PostToCollection(ctx, "Block", blockData)
	if err != nil {
		return "", errors.NewQueryFailed("defra", "CreateBlock", fmt.Sprintf("%v", blockData), err)
	}

	return docID, nil
}

func (h *BlockHandler) CreateTransaction(ctx context.Context, tx *types.Transaction, block_id string) (string, error) {
	// Function input validation
	if tx == nil {
		return "", errors.NewInvalidInputFormat("defra", "CreateTransaction", "tx", nil)
	}

	blockInt, err := utils.HexToInt(tx.BlockNumber)
	if err != nil {
		return "", errors.NewParsingFailed("defra", "CreateTransaction", "block number", err)
	}

	txData := map[string]interface{}{
		"hash":                 tx.Hash,
		"blockNumber":          blockInt,
		"blockHash":            tx.BlockHash,
		"transactionIndex":     tx.TransactionIndex,
		"from":                 tx.From,
		"to":                   tx.To,
		"value":                tx.Value,
		"gas":                  tx.Gas,
		"gasPrice":             tx.GasPrice,
		"maxFeePerGas":         tx.MaxFeePerGas,
		"maxPriorityFeePerGas": tx.MaxPriorityFeePerGas,
		"input":                string(tx.Input),
		"nonce":                fmt.Sprintf("%v", tx.Nonce),
		"type":                 tx.Type,
		"chainId":              tx.ChainId,
		"v":                    tx.V,
		"r":                    tx.R,
		"s":                    tx.S,
		"gasUsed":              tx.GasUsed,
		"cumulativeGasUsed":    tx.CumulativeGasUsed,
		"effectiveGasPrice":    tx.EffectiveGasPrice,
		"status":               tx.Status,
		"block_id":             block_id, // Include block relationship directly

	}
	logger.Sugar.Debug("Creating transaction: ", txData)
	// Database operation
	docID, err := h.PostToCollection(ctx, "Transaction", txData)
	if err != nil {
		return "", errors.NewQueryFailed("defra", "CreateTransaction", fmt.Sprintf("%v", txData), err)
	}

	return docID, nil
}

func (h *BlockHandler) CreateAccessListEntry(ctx context.Context, accessListEntry *types.AccessListEntry, tx_Id string) (string, error) {
	if accessListEntry == nil {
		logger.Sugar.Error("CreateAccessListEntry: AccessListEntry is nil")
		return "", errors.NewInvalidInputFormat("defra", "CreateAccessListEntry", "accessListEntry", nil)
	}
	if tx_Id == "" {
		logger.Sugar.Error("CreateAccessListEntry: tx_Id is empty")
		return "", errors.NewInvalidInputFormat("defra", "CreateAccessListEntry", "tx_Id", nil)
	}
	ALEData := map[string]interface{}{
		"address":        accessListEntry.Address,
		"storageKeys":    accessListEntry.StorageKeys,
		"transaction_id": tx_Id,
	}
	logger.Sugar.Debug("Creating access list entry: ", ALEData)
	// Database operation
	docID, err := h.PostToCollection(ctx, "AccessListEntry", ALEData)
	if err != nil {
		return "", errors.NewQueryFailed("defra", "CreateAccessListEntry", fmt.Sprintf("%v", ALEData), err)
	}

	return docID, nil
}

func (h *BlockHandler) CreateLog(ctx context.Context, log *types.Log, block_id, tx_Id string) (string, error) {
	blockInt, err := utils.HexToInt(log.BlockNumber)
	if err != nil {
		return "", errors.NewParsingFailed("defra", "CreateLog", fmt.Sprintf("block number: %s", log.BlockNumber), err)
	}
	if log == nil {
		return "", errors.NewInvalidInputFormat("defra", "CreateLog", "log", nil)
	}
	if block_id == "" {
		return "", errors.NewInvalidInputFormat("defra", "CreateLog", "block_id", nil)
	}
	if tx_Id == "" {
		return "", errors.NewInvalidInputFormat("defra", "CreateLog", "tx_Id", nil)
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
		"transaction_id":   tx_Id,
		"block_id":         block_id,
	}
	logger.Sugar.Debug("Creating log: ", logData)
	// Database operation
	docID, err := h.PostToCollection(ctx, "Log", logData)
	if err != nil {
		return "", errors.NewQueryFailed("defra", "CreateLog", fmt.Sprintf("%v", logData), err)
	}

	return docID, nil
}

func (h *BlockHandler) UpdateTransactionRelationships(ctx context.Context, blockId string, txHash string) (string, error) {

	if blockId == "" {
		return "", errors.NewInvalidInputFormat("defra", "UpdateTransactionRelationships", "blockId", nil)
	}
	if txHash == "" {
		return "", errors.NewInvalidInputFormat("defra", "UpdateTransactionRelationships", "txHash", nil)
	}

	// Update transaction with block relationship
	mutation := types.Request{Query: fmt.Sprintf(`mutation {
		update_Transaction(filter: {hash: {_eq: %q}}, input: {block: %q}) {
			_docID
		}
	}`, txHash, blockId)}

	resp, err := h.SendToGraphql(ctx, mutation)
	if err != nil {
		logger.Sugar.Errorf("failed to update transaction relationships: ", mutation)
		return "", errors.NewQueryFailed("defra", "UpdateTransactionRelationships", fmt.Sprintf("%v", mutation), err)
	}
	docId, err := h.parseGraphQLResponse(resp, "update_Transaction")
	if docId == "" {
		return "", errors.NewQueryFailed("defra", "UpdateTransactionRelationships", fmt.Sprintf("%v", mutation), nil)
	}
	return docId, nil

}

func (h *BlockHandler) UpdateLogRelationships(ctx context.Context, blockId string, txId string, txHash string, logIndex string) (string, error) {

	if blockId == "" {
		return "", errors.NewInvalidInputFormat("defra", "UpdateLogRelationships", "blockId", nil)
	}
	if txId == "" {
		return "", errors.NewInvalidInputFormat("defra", "UpdateLogRelationships", "txId", nil)
	}
	if txHash == "" {
		return "", errors.NewInvalidInputFormat("defra", "UpdateLogRelationships", "txHash", nil)
	}
	if logIndex == "" {
		return "", errors.NewInvalidInputFormat("defra", "UpdateLogRelationships", "logIndex", nil)
	}

	// Update log with block and transaction relationships
	mutation := types.Request{Query: fmt.Sprintf(`mutation {
		update_Log(filter: {logIndex: {_eq: %q}, transactionHash: {_eq: %q}}, input: {
			block: %q,
			transaction: %q
		}) {
			_docID
		}
	}`, logIndex, txHash, blockId, txId)}

	resp, err := h.SendToGraphql(ctx, mutation)
	if err != nil {
		logger.Sugar.Errorf("log relationship update failure: ", mutation)
		return "", errors.NewQueryFailed("defra", "UpdateLogRelationships", fmt.Sprintf("%v", mutation), err)
	}
	docId, err := h.parseGraphQLResponse(resp, "update_Log")
	if docId == "" {
		return "", errors.NewQueryFailed("defra", "UpdateLogRelationships", fmt.Sprintf("%v", mutation), nil)
	}
	return docId, nil
}

func (h *BlockHandler) UpdateEventRelationships(ctx context.Context, logDocId string, txHash string, logIndex string) (string, error) {

	if logDocId == "" {
		return "", errors.NewInvalidInputFormat("defra", "UpdateEventRelationships", "logDocId", nil)
	}
	if txHash == "" {
		return "", errors.NewInvalidInputFormat("defra", "UpdateEventRelationships", "txHash", nil)
	}
	if logIndex == "" {
		return "", errors.NewInvalidInputFormat("defra", "UpdateEventRelationships", "logIndex", nil)
	}

	// Update event with log relationship
	mutation := types.Request{Query: fmt.Sprintf(`mutation {
		update_Event(filter: {logIndex: {_eq: %q}, transactionHash:{_eq:%q}}, input: {
		log: %q
		}) {
			_docID
		}
	}`, logIndex, txHash, logDocId)}

	resp, err := h.SendToGraphql(ctx, mutation)
	if err != nil {
		logger.Sugar.Errorf("event relationship update failure: ", mutation)
		return "", errors.NewQueryFailed("defra", "UpdateEventRelationships", fmt.Sprintf("%v", mutation), err)
	}
	docId, err := h.parseGraphQLResponse(resp, "update_Event")
	if docId == "" {
		return "", errors.NewQueryFailed("defra", "UpdateEventRelationships", fmt.Sprintf("%v", mutation), nil)
	}
	return docId, nil
}

func (h *BlockHandler) PostToCollection(ctx context.Context, collection string, data map[string]interface{}) (string, error) {
	if collection == "" {
		return "", errors.NewInvalidInputFormat("defra", "PostToCollection", "collection", nil)
	}
	if data == nil {
		return "", errors.NewInvalidInputFormat("defra", "PostToCollection", "data", nil)
	}

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
				logger.Sugar.Errorf("failed to marshal field ", key, "err: ", err)
				return "", errors.NewParsingFailed("defra", "PostToCollection", fmt.Sprintf("%v", key), err)
			}
			inputFields = append(inputFields, fmt.Sprintf("%s: %s", key, string(jsonBytes)))
		default:
			inputFields = append(inputFields, fmt.Sprintf("%s: %q", key, fmt.Sprint(v)))
		}
	}

	// Create mutation
	mutation := types.Request{
		Type: "POST",
		Query: fmt.Sprintf(`mutation {
		create_%s(input: { %s }) {
			_docID
		}
	}`, collection, strings.Join(inputFields, ", "))}

	// Send mutation
	resp, err := h.SendToGraphql(ctx, mutation)
	if err != nil {
		logger.Sugar.Error("Received nil response from GraphQL")
		return "", errors.NewQueryFailed("defra", "PostToCollection", fmt.Sprintf("%v", mutation), err)
	}

	logger.Sugar.Debug("DefraDB Response: ", string(resp))

	// Parse response - handle both single object and array formats
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(resp, &rawResponse); err != nil {
		logger.Sugar.Errorf("failed to decode response: %v", err)
		logger.Sugar.Debug("Raw response: ", string(resp))
		return "", errors.NewQueryFailed("defra", "PostToCollection", fmt.Sprintf("%v", mutation), err)
	}

	// Extract data field
	data, ok := rawResponse["data"].(map[string]interface{})
	if !ok {
		logger.Sugar.Error("data field not found in response\n", "Response: ", rawResponse)
		return "", errors.NewQueryFailed("defra", "PostToCollection", fmt.Sprintf("%v", mutation), nil)
	}

	// Get document ID
	createField := fmt.Sprintf("create_%s", collection)
	createData, ok := data[createField]
	if !ok {
		logger.Sugar.Errorf("create_", collection, " field not found in response\n", "Response data: ", data)
		return "", errors.NewQueryFailed("defra", "PostToCollection", fmt.Sprintf("%v", mutation), nil)
	}

	// Handle both single object and array responses
	switch v := createData.(type) {
	case map[string]interface{}:
		// Single object response
		if docID, ok := v["_docID"].(string); ok {
			// return good
			return docID, nil
		}
	case []interface{}:
		// Array response
		if len(v) > 0 {
			if item, ok := v[0].(map[string]interface{}); ok {
				if docID, ok := item["_docID"].(string); ok {
					// return good
					return docID, nil
				}
			}
		}
	}

	logger.Sugar.Errorf("unable to extract _docID from create_"+collection+" response\n", "Create data: ", createData)
	return "", errors.NewQueryFailed("defra", "PostToCollection", fmt.Sprintf("%v", mutation), nil)
}

func (h *BlockHandler) SendToGraphql(ctx context.Context, req types.Request) ([]byte, error) {

	if req.Query == "" {
		return nil, errors.NewInvalidInputFormat("defra", "SendToGraphql", "req.Query", nil)
	}

	type RequestJSON struct {
		Query string `json:"query"`
	}

	// Create request body
	body := RequestJSON{req.Query}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		logger.Sugar.Errorf("failed to marshal request body: ", err)
	}

	// Debug: Print the mutation
	logger.Sugar.Debug("Sending mutation: ", req.Query, "\n")

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, req.Type, h.defraURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		logger.Sugar.Errorf("failed to create request: ", err)
		return nil, errors.NewQueryFailed("defra", "SendToGraphql", fmt.Sprintf("%v", req), err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// Send request
	resp, err := h.client.Do(httpReq)
	if err != nil {
		logger.Sugar.Errorf("failed to send request: ", err)
		return nil, errors.NewQueryFailed("defra", "SendToGraphql", fmt.Sprintf("%v", req), err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Sugar.Errorf("Read response error: ", err) // todo turn to error interface
		return nil, errors.NewQueryFailed("defra", "SendToGraphql", fmt.Sprintf("%v", req), err)
	}
	// Debug: Print the response
	logger.Sugar.Debug("DefraDB Response: ", string(respBody), "\n")
	return respBody, nil
}

// parseGraphQLResponse is a helper function to parse GraphQL responses and extract document IDs
func (h *BlockHandler) parseGraphQLResponse(resp []byte, fieldName string) (string, error) {
	// Parse response
	var response types.Response
	if err := json.Unmarshal(resp, &response); err != nil {
		logger.Sugar.Errorf("failed to decode response: ", err, "\n", "Raw response: ", string(resp))
		return "", errors.NewQueryFailed("defra", "parseGraphQLResponse", fmt.Sprintf("%v", resp), err)
	}

	// Get document ID
	items, ok := response.Data[fieldName]
	if !ok {
		logger.Sugar.Errorf("%s field not found in response\n", fieldName, "Response data: ", response.Data)
		return "", errors.NewQueryFailed("defra", "parseGraphQLResponse", fmt.Sprintf("%v", resp), nil)
	}
	if len(items) == 0 {
		logger.Sugar.Warnf("no document ID returned for %s", fieldName)
		return "", errors.NewQueryFailed("defra", "parseGraphQLResponse", fmt.Sprintf("%v", resp), nil)
	}
	return items[0].DocID, nil
}

// GetHighestBlockNumber returns the highest block number stored in DefraDB
func (h *BlockHandler) GetHighestBlockNumber(ctx context.Context) (int64, error) {
	query := types.Request{
		Type: "POST",
		Query: `query {
		Block(order: {number: DESC}, limit: 1) {
			number
		}	
	}`}

	resp, err := h.SendToGraphql(ctx, query)
	if err != nil {
		logger.Sugar.Errorf("failed to query block numbers error: ", err)
		return 0, errors.NewQueryFailed("defra", "GetHighestBlockNumber", query.Query, err)
	}
	// Parse response to handle both string and integer number formats
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(resp, &rawResponse); err != nil {
		logger.Sugar.Errorf("failed to decode response: %v", err)
		return 0, errors.NewParsingFailed("defra", "GetHighestBlockNumber", string(resp), err)
	}

	// Extract data field
	data, ok := rawResponse["data"].(map[string]interface{})
	if !ok {
		logger.Sugar.Error("data field not found in response")
		return 0, errors.NewDocumentNotFound("defra", "GetHighestBlockNumber", "Block", fmt.Sprintf("%v", data))
	}

	// Extract Block array
	blockArray, ok := data["Block"].([]interface{})
	if !ok {
		logger.Sugar.Error("Block field not found in response")
		return 0, errors.NewDocumentNotFound("defra", "GetHighestBlockNumber", "Block", fmt.Sprintf("%v", data["Block"]))
	}

	if len(blockArray) == 0 {
		return 0, errors.NewDocumentNotFound("defra", "GetHighestBlockNumber", "Block", "blockArray is empty")
	}

	// Extract first block
	block, ok := blockArray[0].(map[string]interface{})
	if !ok {
		logger.Sugar.Error("Invalid block format in response")
		return 0, errors.NewDocumentNotFound("defra", "GetHighestBlockNumber", "Block", fmt.Sprintf("%v", blockArray))
	}

	// Extract number field (handle both string and integer)
	numberValue := block["number"]
	switch v := numberValue.(type) {
	case string:
		// Try hex conversion first if string starts with 0x
		if strings.HasPrefix(v, "0x") {
			val, err := utils.HexToInt(v)
			if err != nil {
				return 0, errors.NewParsingFailed("defra", "GetHighestBlockNumber", fmt.Sprintf("block number: %s", v), err)
			}
			return val, nil
		}
		if num, err := strconv.ParseInt(v, 10, 64); err == nil {
			return num, nil
		}
		logger.Sugar.Errorf("failed to parse number string: %v", v)
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	default:
		logger.Sugar.Errorf("unexpected number type: %T", numberValue)
	}
	return 0, errors.NewDocumentNotFound("defra", "GetHighestBlockNumber", "Block", fmt.Sprintf("%v", numberValue))
}
