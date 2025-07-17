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

	"shinzo/version1/pkg/logger"
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

func (h *BlockHandler) ConvertHexToInt(s string) int64 {

	// Handle empty string
	if s == "" {
		logger.Sugar.Error("Empty hex string provided")
		return 0
	}

	// Remove "0x" prefix if present
	hexStr := s
	if strings.HasPrefix(s, "0x") {
		hexStr = s[2:]
	}

	// Parse the hex string
	blockInt, err := strconv.ParseInt(hexStr, 16, 64)
	if err != nil {
		logger.Sugar.Errorf("Failed to parse hex string '%s': %v", s, err)
		return 0
	}

	return blockInt
}

func (h *BlockHandler) CreateBlock(ctx context.Context, block *types.Block) string {
	// Convert string number to int
	blockInt, err := strconv.ParseInt(block.Number, 0, 64)
	if err != nil {
		logger.Sugar.Errorf("failed to parse block number: %w", err)
		return ""
	}

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
	logger.Sugar.Debug("Posting blockdata to collection endpoint: ", blockData)
	return h.PostToCollection(ctx, "Block", blockData)
}

func (h *BlockHandler) CreateTransaction(ctx context.Context, tx *types.Transaction, block_id string) string {
	blockInt, err := strconv.ParseInt(tx.BlockNumber, 0, 64)
	if err != nil {
		logger.Sugar.Errorf("failed to parse block number: ", err)
		return ""
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
	// sugar.Debug("Transaction Input: ", txData["input"])
	return h.PostToCollection(ctx, "Transaction", txData)
}

func (h *BlockHandler) CreateAccessListEntry(ctx context.Context, accessListEntry *types.AccessListEntry, tx_Id string) string {
	if accessListEntry == nil {
		logger.Sugar.Error("CreateAccessListEntry: AccessListEntry is nil")
		return ""
	}
	if tx_Id == "" {
		logger.Sugar.Error("CreateAccessListEntry: tx_Id is empty")
		return ""
	}
	ALEData := map[string]interface{}{
		"address":        accessListEntry.Address,
		"storageKeys":    accessListEntry.StorageKeys,
		"transaction_id": tx_Id,
	}
	logger.Sugar.Debug("Creating access list entry: ", ALEData)
	return h.PostToCollection(ctx, "AccessListEntry", ALEData)
}

func (h *BlockHandler) CreateLog(ctx context.Context, log *types.Log, block_id, tx_Id string) string {
	blockInt, err := strconv.ParseInt(log.BlockNumber, 0, 64)
	if err != nil {
		logger.Sugar.Errorf("failed to parse block number: ", err)
		return ""
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
	return h.PostToCollection(ctx, "Log", logData)
}

func (h *BlockHandler) UpdateTransactionRelationships(ctx context.Context, blockId string, txHash string) string {

	// Update transaction with block relationship
	mutation := types.Request{Query: fmt.Sprintf(`mutation {
		update_Transaction(filter: {hash: {_eq: %q}}, input: {block: %q}) {
			_docID
		}
	}`, txHash, blockId)}

	resp := h.SendToGraphql(ctx, mutation)
	if resp == nil {
		logger.Sugar.Errorf("failed to update transaction relationships: ", mutation)
		return ""
	}

	return h.parseGraphQLResponse(resp, "update_Transaction")

}

// shinzo stuct
// alchemy client interface
// call start and measure what i am storing in defra
// mock alchemy block data { alter diff fields to create diff scenarios}

func (h *BlockHandler) UpdateLogRelationships(ctx context.Context, blockId string, txId string, txHash string, logIndex string) string {

	// Update log with block and transaction relationships
	mutation := types.Request{Query: fmt.Sprintf(`mutation {
		update_Log(filter: {logIndex: {_eq: %q}, transactionHash: {_eq: %q}}, input: {
			block: %q,
			transaction: %q
		}) {
			_docID
		}
	}`, logIndex, txHash, blockId, txId)}

	resp := h.SendToGraphql(ctx, mutation)
	if resp == nil {
		logger.Sugar.Errorf("log relationship update failure: ", mutation)
		return ""
	}

	return h.parseGraphQLResponse(resp, "update_Log")
}

func (h *BlockHandler) UpdateEventRelationships(ctx context.Context, logDocId string, txHash string, logIndex string) string {
	// Update event with log relationship
	mutation := types.Request{Query: fmt.Sprintf(`mutation {
		update_Event(filter: {logIndex: {_eq: %q}, transactionHash:{_eq:%q}}, input: {
		log: %q
		}) {
			_docID
		}
	}`, logIndex, txHash, logDocId)}

	resp := h.SendToGraphql(ctx, mutation)
	if resp == nil {
		logger.Sugar.Errorf("event relationship update failure: ", mutation)
		return ""
	}

	return h.parseGraphQLResponse(resp, "update_Event")
}

func (h *BlockHandler) PostToCollection(ctx context.Context, collection string, data map[string]interface{}) string {
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
				return ""
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
	resp := h.SendToGraphql(ctx, mutation)
	if resp == nil {
		logger.Sugar.Error("Received nil response from GraphQL")
		return ""
	}
	//TODO create access list and link to the transaction.
	logger.Sugar.Debug("DefraDB Response: ", string(resp))

	// Parse response - handle both single object and array formats
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(resp, &rawResponse); err != nil {
		logger.Sugar.Errorf("failed to decode response: %v", err)
		logger.Sugar.Debug("Raw response: ", string(resp))
		return ""
	}

	// Extract data field
	data, ok := rawResponse["data"].(map[string]interface{})
	if !ok {
		logger.Sugar.Error("data field not found in response")
		logger.Sugar.Debug("Response: ", rawResponse)
		return ""
	}

	// Get document ID
	createField := fmt.Sprintf("create_%s", collection)
	createData, ok := data[createField]
	if !ok {
		logger.Sugar.Errorf("create_%s field not found in response", collection)
		logger.Sugar.Debug("Response data: ", data)
		return ""
	}

	// Handle both single object and array responses
	switch v := createData.(type) {
	case map[string]interface{}:
		// Single object response
		if docID, ok := v["_docID"].(string); ok {
			// return good
			return docID
		}
	case []interface{}:
		// Array response
		if len(v) > 0 {
			if item, ok := v[0].(map[string]interface{}); ok {
				if docID, ok := item["_docID"].(string); ok {
					// return good
					return docID
				}
			}
		}
	}

	logger.Sugar.Errorf("unable to extract _docID from create_%s response", collection)
	logger.Sugar.Debug("Create data: ", createData)
	return ""
}

// Graph golang client check in defra

func (h *BlockHandler) SendToGraphql(ctx context.Context, req types.Request) []byte {

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
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// Send request
	resp, err := h.client.Do(httpReq)
	if err != nil {
		logger.Sugar.Errorf("failed to send request: ", err)
		return nil
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Sugar.Errorf("Read response error: ", err) // todo turn to error interface
	}
	// Debug: Print the response
	logger.Sugar.Debug("DefraDB Response: ", string(respBody), "\n")
	return respBody
}

// parseGraphQLResponse is a helper function to parse GraphQL responses and extract document IDs
func (h *BlockHandler) parseGraphQLResponse(resp []byte, fieldName string) string {
	// Parse response
	var response types.Response
	if err := json.Unmarshal(resp, &response); err != nil {
		logger.Sugar.Errorf("failed to decode response: %v", err)
		logger.Sugar.Debug("Raw response: ", string(resp))
		return ""
	}

	// Get document ID
	items, ok := response.Data[fieldName]
	if !ok {
		logger.Sugar.Errorf("%s field not found in response", fieldName)
		logger.Sugar.Debug("Response data: ", response.Data)
		return ""
	}
	if len(items) == 0 {
		logger.Sugar.Warnf("no document ID returned for %s", fieldName)
		return ""
	}
	return items[0].DocID
}

// GetHighestBlockNumber returns the highest block number stored in DefraDB
func (h *BlockHandler) GetHighestBlockNumber(ctx context.Context) int64 {
	query := types.Request{
		Type: "POST",
		Query: `query {
		Block(order: {number: DESC}, limit: 1) {
			number
		}	
	}`}

	resp := h.SendToGraphql(ctx, query)
	if resp == nil {
		logger.Sugar.Errorf("failed to query block numbers error: ", resp)
		return 0
	}

	// Parse response to handle both string and integer number formats
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(resp, &rawResponse); err != nil {
		logger.Sugar.Errorf("failed to decode response: %v", err)
		return 0
	}

	// Extract data field
	data, ok := rawResponse["data"].(map[string]interface{})
	if !ok {
		logger.Sugar.Error("data field not found in response")
		return 0
	}

	// Extract Block array
	blockArray, ok := data["Block"].([]interface{})
	if !ok {
		logger.Sugar.Error("Block field not found in response")
		return 0
	}

	if len(blockArray) == 0 {
		return 0 // Return 0 if no blocks exist
	}

	// Extract first block
	block, ok := blockArray[0].(map[string]interface{})
	if !ok {
		logger.Sugar.Error("Invalid block format in response")
		return 0
	}

	// Extract number field (handle both string and integer)
	numberValue := block["number"]
	switch v := numberValue.(type) {
	case string:
		// Try hex conversion first if string starts with 0x
		if strings.HasPrefix(v, "0x") {
			return h.ConvertHexToInt(v)
		}
		if num, err := strconv.ParseInt(v, 10, 64); err == nil {
			return num
		}
		logger.Sugar.Errorf("failed to parse number string: %v", v)
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	default:
		logger.Sugar.Errorf("unexpected number type: %T", numberValue)
	}
	return 0
}
