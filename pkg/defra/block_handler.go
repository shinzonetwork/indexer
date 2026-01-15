package defra

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

	"github.com/shinzonetwork/shinzo-indexer-client/pkg/constants"
	"github.com/shinzonetwork/shinzo-indexer-client/pkg/errors"
	"github.com/shinzonetwork/shinzo-indexer-client/pkg/logger"
	"github.com/shinzonetwork/shinzo-indexer-client/pkg/types"
	"github.com/shinzonetwork/shinzo-indexer-client/pkg/utils"
	"github.com/sourcenetwork/defradb/node"
)

type BlockHandler struct {
	defraURL      string
	client        *http.Client
	defraNode     *node.Node // Direct access to embedded DefraDB (nil if using HTTP)
	maxDocsPerTxn int        // Threshold for single-txn vs batched block creation

	// Document throughput metrics
	metricsWindowStart  time.Time
	docsCreatedInWindow int
}

// logEntry holds a log and its associated transaction ID for batched processing
type logEntry struct {
	log  *types.Log
	txID string
}

// aleEntry holds an access list entry and its associated transaction ID for batched processing
type aleEntry struct {
	ale  *types.AccessListEntry
	txID string
}

func NewBlockHandler(url string) (*BlockHandler, error) {
	if url == "" {
		return nil, errors.NewConfigurationError("defra", "NewBlockHandler",
			"url parameter is empty", url, nil)
	}
	return &BlockHandler{
		defraURL: strings.Replace(fmt.Sprintf("%s/api/v0/graphql", url), "127.0.0.1", "localhost", 1),
		client: &http.Client{
			Timeout: 30 * time.Second, // Add 30-second timeout to prevent hanging
		},
		defraNode: nil,
	}, nil
}

// NewBlockHandlerWithNode creates a BlockHandler that uses direct DB calls for better performance.
// maxDocsPerTxn is the threshold for single-txn vs batched block creation (default 256 if <= 0).
func NewBlockHandlerWithNode(defraNode *node.Node, maxDocsPerTxn int) (*BlockHandler, error) {
	if defraNode == nil {
		return nil, errors.NewConfigurationError("defra", "NewBlockHandlerWithNode",
			"defraNode is nil", "", nil)
	}
	if maxDocsPerTxn <= 0 {
		maxDocsPerTxn = 256
	}
	return &BlockHandler{
		defraNode:     defraNode,
		client:        nil,
		defraURL:      "",
		maxDocsPerTxn: maxDocsPerTxn,
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
	docID, err := h.PostToCollection(ctx, constants.CollectionBlock, blockData)
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

	blockInt, err := strconv.ParseInt(tx.BlockNumber, 10, 64)
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
		"cumulativeGasUsed":    tx.CumulativeGasUsed,
		"effectiveGasPrice":    tx.EffectiveGasPrice,
		"status":               tx.Status,
		"block":                block_id,
	}
	docID, err := h.PostToCollection(ctx, constants.CollectionTransaction, txData)
	if err != nil {
		return "", errors.NewQueryFailed("defra", "CreateTransaction", fmt.Sprintf("%v", txData), err)
	}

	return docID, nil
}

func (h *BlockHandler) CreateAccessListEntry(ctx context.Context, accessListEntry *types.AccessListEntry, tx_Id string) (string, error) {
	if accessListEntry == nil {
		logger.Sugar.Error("CreateAccessListEntry: AccessListEntry is nil")
		return "", errors.NewInvalidInputFormat("defra", "CreateAccessListEntry", constants.CollectionAccessListEntry, nil)
	}
	if tx_Id == "" {
		logger.Sugar.Error("CreateAccessListEntry: tx_Id is empty")
		return "", errors.NewInvalidInputFormat("defra", "CreateAccessListEntry", "tx_Id", nil)
	}
	ALEData := map[string]interface{}{
		"address":     accessListEntry.Address,
		"storageKeys": accessListEntry.StorageKeys,
		"transaction": tx_Id,
	}
	docID, err := h.PostToCollection(ctx, constants.CollectionAccessListEntry, ALEData)
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
		return "", errors.NewInvalidInputFormat("defra", "CreateLog", constants.CollectionLog, nil)
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
		"removed":          fmt.Sprintf("%v", log.Removed),
		"transaction":      tx_Id,
		"block":            block_id,
	}
	docID, err := h.PostToCollection(ctx, constants.CollectionLog, logData)
	if err != nil {
		logger.Sugar.Errorf("Failed to create log (txHash=%s, logIndex=%v): %v", log.TransactionHash, log.LogIndex, err)
		return "", err
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
		return "", errors.NewQueryFailed("defra", "UpdateLogRelationships", fmt.Sprintf("%v", mutation), err)
	}
	docId, err := h.parseGraphQLResponse(resp, "update_Log")
	if docId == "" {
		return "", errors.NewQueryFailed("defra", "UpdateLogRelationships", fmt.Sprintf("%v", mutation), nil)
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
		return "", errors.NewQueryFailed("defra", "PostToCollection", fmt.Sprintf("%v", mutation), err)
	}

	// Parse response - handle both single object and array formats
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(resp, &rawResponse); err != nil {
		return "", errors.NewQueryFailed("defra", "PostToCollection", fmt.Sprintf("%v", mutation), err)
	}

	// Check for GraphQL errors first
	if graphqlErrors, hasErrors := rawResponse["errors"].([]interface{}); hasErrors && len(graphqlErrors) > 0 {
		if errorMap, ok := graphqlErrors[0].(map[string]interface{}); ok {
			if message, ok := errorMap["message"].(string); ok {
				// Handle duplicate document error gracefully
				if strings.Contains(message, "already exists") {
					if strings.Contains(message, "DocID: ") {
						parts := strings.Split(message, "DocID: ")
						if len(parts) > 1 {
							docID := strings.TrimSpace(parts[1])
							return docID, nil
						}
					}
					return "", errors.NewQueryFailed("defra", "PostToCollection", "document already exists", nil)
				}
				return "", errors.NewQueryFailed("defra", "PostToCollection", message, nil)
			}
		}
	}

	// Extract data field
	data, ok := rawResponse["data"].(map[string]interface{})
	if !ok {
		return "", errors.NewQueryFailed("defra", "PostToCollection", fmt.Sprintf("%v", mutation), nil)
	}

	// Get document ID
	createField := fmt.Sprintf("create_%s", collection)
	createData, ok := data[createField]
	if !ok {
		return "", errors.NewQueryFailed("defra", "PostToCollection", fmt.Sprintf("%v", mutation), nil)
	}

	// Handle both single object and array responses
	switch v := createData.(type) {
	case map[string]interface{}:
		// Single object response
		if docID, ok := v["_docID"].(string); ok {
			return docID, nil
		}
	case []interface{}:
		// Array response
		if len(v) > 0 {
			if item, ok := v[0].(map[string]interface{}); ok {
				if docID, ok := item["_docID"].(string); ok {
					return docID, nil
				}
			}
		}
	}

	return "", errors.NewQueryFailed("defra", "PostToCollection", fmt.Sprintf("%v", mutation), nil)
}

func (h *BlockHandler) SendToGraphql(ctx context.Context, req types.Request) ([]byte, error) {
	if req.Query == "" {
		return nil, errors.NewInvalidInputFormat("defra", "SendToGraphql", "req.Query", nil)
	}

	if h.defraNode != nil {
		return h.sendToGraphqlDirect(ctx, req)
	}

	return h.sendToGraphqlHTTP(ctx, req)
}

// sendToGraphqlDirect executes GraphQL directly on the embedded DefraDB node
func (h *BlockHandler) sendToGraphqlDirect(ctx context.Context, req types.Request) ([]byte, error) {
	result := h.defraNode.DB.ExecRequest(ctx, req.Query)
	gqlResult := result.GQL

	response := map[string]interface{}{
		"data": gqlResult.Data,
	}

	if len(gqlResult.Errors) > 0 {
		errList := make([]map[string]interface{}, len(gqlResult.Errors))
		for i, err := range gqlResult.Errors {
			errList[i] = map[string]interface{}{
				"message": err.Error(),
			}
		}
		response["errors"] = errList
	}

	respBody, err := json.Marshal(response)
	if err != nil {
		return nil, errors.NewQueryFailed("defra", "sendToGraphqlDirect", fmt.Sprintf("%v", req), err)
	}

	return respBody, nil
}

// sendToGraphqlHTTP executes GraphQL via HTTP
func (h *BlockHandler) sendToGraphqlHTTP(ctx context.Context, req types.Request) ([]byte, error) {
	type RequestJSON struct {
		Query string `json:"query"`
	}

	// Create request body
	body := RequestJSON{req.Query}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		logger.Sugar.Errorf("failed to marshal request body: ", err)
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, req.Type, h.defraURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, errors.NewQueryFailed("defra", "sendToGraphqlHTTP", fmt.Sprintf("%v", req), err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := h.client.Do(httpReq)
	if err != nil {
		return nil, errors.NewQueryFailed("defra", "sendToGraphqlHTTP", fmt.Sprintf("%v", req), err)
	}

	defer resp.Body.Close()
	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewQueryFailed("defra", "sendToGraphqlHTTP", fmt.Sprintf("%v", req), err)
	}

	return respBody, nil
}

// parseGraphQLResponse is a helper function to parse GraphQL responses and extract document IDs
func (h *BlockHandler) parseGraphQLResponse(resp []byte, fieldName string) (string, error) {
	// Parse response
	var response types.Response
	if err := json.Unmarshal(resp, &response); err != nil {
		return "", errors.NewQueryFailed("defra", "parseGraphQLResponse", fmt.Sprintf("%v", resp), err)
	}

	// Get document ID
	items, ok := response.Data[fieldName]
	if !ok {
		return "", errors.NewQueryFailed("defra", "parseGraphQLResponse", fmt.Sprintf("%v", resp), nil)
	}
	if len(items) == 0 {
		return "", errors.NewQueryFailed("defra", "parseGraphQLResponse", fmt.Sprintf("%v", resp), nil)
	}
	return items[0].DocID, nil
}

// CreateBlockBatch creates a block with all its transactions, logs, and access list entries.
func (h *BlockHandler) CreateBlockBatch(ctx context.Context, block *types.Block, transactions []*types.Transaction, receipts []*types.TransactionReceipt) (string, error) {
	if h.defraNode == nil {
		return "", errors.NewConfigurationError("defra", "CreateBlockBatch",
			"batch creation requires embedded DefraDB node", "", nil)
	}

	if block == nil {
		return "", errors.NewInvalidBlockFormat("defra", "CreateBlockBatch", "nil", nil)
	}

	blockInt, err := utils.HexToInt(block.Number)
	if err != nil {
		return "", err
	}

	receiptMap := make(map[string]*types.TransactionReceipt)
	for _, receipt := range receipts {
		if receipt != nil {
			receiptMap[receipt.TransactionHash] = receipt
		}
	}

	totalLogs := 0
	totalALEs := 0
	for _, tx := range transactions {
		if tx == nil {
			continue
		}
		if receipt, ok := receiptMap[tx.Hash]; ok && receipt != nil {
			totalLogs += len(receipt.Logs)
		}
		totalALEs += len(tx.AccessList)
	}
	totalDocs := 1 + len(transactions) + totalLogs + totalALEs

	if totalDocs <= h.maxDocsPerTxn {
		return h.createBlockSingleTransaction(ctx, block, blockInt, transactions, receipts, receiptMap, totalDocs)
	}

	return h.createBlockBatched(ctx, block, blockInt, transactions, receipts, receiptMap)
}

// createBlockSingleTransaction creates the entire block in a single DB transaction.
// This is optimal for small-to-medium blocks as it minimizes commit overhead.
func (h *BlockHandler) createBlockSingleTransaction(ctx context.Context, block *types.Block, blockInt int64, transactions []*types.Transaction, receipts []*types.TransactionReceipt, receiptMap map[string]*types.TransactionReceipt, totalDocs int) (string, error) {
	// Start single transaction for everything
	txn, err := h.defraNode.DB.NewTxn(false)
	if err != nil {
		return "", errors.NewQueryFailed("defra", "createBlockSingleTransaction", "failed to create transaction", err)
	}

	// Build the complete mutation for the entire block
	mutation, _, _, _ := h.buildEntireBlockMutation(block, blockInt, transactions, receiptMap)

	result := txn.ExecRequest(ctx, mutation)
	if len(result.GQL.Errors) > 0 {
		txn.Discard()
		errMsg := result.GQL.Errors[0].Error()
		if strings.Contains(errMsg, "already exists") {
			return "", fmt.Errorf("block already exists")
		}
		return "", errors.NewQueryFailed("defra", "createBlockSingleTransaction", errMsg, result.GQL.Errors[0])
	}

	// Commit everything at once
	if err := txn.Commit(); err != nil {
		return "", errors.NewQueryFailed("defra", "createBlockSingleTransaction", "failed to commit", err)
	}

	// Extract block ID
	blockID := h.extractDocIDFromBatchedResponse(result.GQL.Data, "block0")
	if blockID == "" {
		return "", errors.NewQueryFailed("defra", "createBlockSingleTransaction", "failed to get block ID", nil)
	}

	return blockID, nil
}

// buildEntireBlockMutation builds a single GraphQL mutation containing the block and all related documents.
func (h *BlockHandler) buildEntireBlockMutation(block *types.Block, blockInt int64, transactions []*types.Transaction, receiptMap map[string]*types.TransactionReceipt) (string, int, int, int) {
	// Estimate size for pre-allocation
	estimatedSize := 2048 + len(transactions)*1536
	for _, tx := range transactions {
		if tx == nil {
			continue
		}
		if receipt, ok := receiptMap[tx.Hash]; ok && receipt != nil {
			estimatedSize += len(receipt.Logs) * 1024
		}
		estimatedSize += len(tx.AccessList) * 512
	}

	var sb strings.Builder
	sb.Grow(estimatedSize)
	sb.WriteString("mutation {\n")

	// === Block ===
	sb.WriteString(`block0: create_`)
	sb.WriteString(constants.CollectionBlock)
	sb.WriteString(`(input: { hash: "`)
	sb.WriteString(block.Hash)
	sb.WriteString(`", number: `)
	sb.WriteString(strconv.FormatInt(blockInt, 10))
	sb.WriteString(`, timestamp: "`)
	sb.WriteString(block.Timestamp)
	sb.WriteString(`", parentHash: "`)
	sb.WriteString(block.ParentHash)
	sb.WriteString(`", difficulty: "`)
	sb.WriteString(block.Difficulty)
	sb.WriteString(`", totalDifficulty: "`)
	sb.WriteString(block.TotalDifficulty)
	sb.WriteString(`", gasUsed: "`)
	sb.WriteString(block.GasUsed)
	sb.WriteString(`", gasLimit: "`)
	sb.WriteString(block.GasLimit)
	sb.WriteString(`", baseFeePerGas: "`)
	sb.WriteString(block.BaseFeePerGas)
	sb.WriteString(`", nonce: "`)
	sb.WriteString(block.Nonce)
	sb.WriteString(`", miner: "`)
	sb.WriteString(block.Miner)
	sb.WriteString(`", size: "`)
	sb.WriteString(block.Size)
	sb.WriteString(`", stateRoot: "`)
	sb.WriteString(block.StateRoot)
	sb.WriteString(`", sha3Uncles: "`)
	sb.WriteString(block.Sha3Uncles)
	sb.WriteString(`", transactionsRoot: "`)
	sb.WriteString(block.TransactionsRoot)
	sb.WriteString(`", receiptsRoot: "`)
	sb.WriteString(block.ReceiptsRoot)
	sb.WriteString(`", logsBloom: "`)
	sb.WriteString(block.LogsBloom)
	sb.WriteString(`", extraData: "`)
	sb.WriteString(block.ExtraData)
	sb.WriteString(`", mixHash: "`)
	sb.WriteString(block.MixHash)
	sb.WriteString(`", uncles: `)
	sb.WriteString(h.formatStringArray(block.Uncles))
	sb.WriteString(` }) { _docID }`)
	sb.WriteString("\n")

	// === Transactions ===
	txCount := 0
	for i, tx := range transactions {
		if tx == nil {
			continue
		}
		alias := fmt.Sprintf("tx%d", i)
		txBlockNum, _ := strconv.ParseInt(tx.BlockNumber, 10, 64)

		sb.WriteString(alias)
		sb.WriteString(`: create_`)
		sb.WriteString(constants.CollectionTransaction)
		sb.WriteString(`(input: { hash: "`)
		sb.WriteString(tx.Hash)
		sb.WriteString(`", blockNumber: `)
		sb.WriteString(strconv.FormatInt(txBlockNum, 10))
		sb.WriteString(`, blockHash: "`)
		sb.WriteString(tx.BlockHash)
		sb.WriteString(`", transactionIndex: `)
		sb.WriteString(strconv.Itoa(tx.TransactionIndex))
		sb.WriteString(`, from: "`)
		sb.WriteString(tx.From)
		sb.WriteString(`", to: "`)
		sb.WriteString(tx.To)
		sb.WriteString(`", value: "`)
		sb.WriteString(tx.Value)
		sb.WriteString(`", gas: "`)
		sb.WriteString(tx.Gas)
		sb.WriteString(`", gasPrice: "`)
		sb.WriteString(tx.GasPrice)
		sb.WriteString(`", maxFeePerGas: "`)
		sb.WriteString(tx.MaxFeePerGas)
		sb.WriteString(`", maxPriorityFeePerGas: "`)
		sb.WriteString(tx.MaxPriorityFeePerGas)
		sb.WriteString(`", input: "`)
		sb.WriteString(string(tx.Input))
		sb.WriteString(`", nonce: "`)
		sb.WriteString(tx.Nonce)
		sb.WriteString(`", type: "`)
		sb.WriteString(tx.Type)
		sb.WriteString(`", chainId: "`)
		sb.WriteString(tx.ChainId)
		sb.WriteString(`", v: "`)
		sb.WriteString(tx.V)
		sb.WriteString(`", r: "`)
		sb.WriteString(tx.R)
		sb.WriteString(`", s: "`)
		sb.WriteString(tx.S)
		sb.WriteString(`", cumulativeGasUsed: "`)
		sb.WriteString(tx.CumulativeGasUsed)
		sb.WriteString(`", effectiveGasPrice: "`)
		sb.WriteString(tx.EffectiveGasPrice)
		sb.WriteString(`", status: `)
		sb.WriteString(strconv.FormatBool(tx.Status))
		sb.WriteString(` }) { _docID }`)
		sb.WriteString("\n")
		txCount++
	}

	// === Logs ===
	logIdx := 0
	for _, tx := range transactions {
		if tx == nil {
			continue
		}
		receipt, ok := receiptMap[tx.Hash]
		if !ok || receipt == nil {
			continue
		}

		for i := range receipt.Logs {
			log := &receipt.Logs[i]
			logBlockNum, _ := utils.HexToInt(log.BlockNumber)
			alias := fmt.Sprintf("log%d", logIdx)

			sb.WriteString(alias)
			sb.WriteString(`: create_`)
			sb.WriteString(constants.CollectionLog)
			sb.WriteString(`(input: { address: "`)
			sb.WriteString(log.Address)
			sb.WriteString(`", topics: `)
			sb.WriteString(h.formatStringArray(log.Topics))
			sb.WriteString(`, data: "`)
			sb.WriteString(log.Data)
			sb.WriteString(`", blockNumber: `)
			sb.WriteString(strconv.FormatInt(logBlockNum, 10))
			sb.WriteString(`, transactionHash: "`)
			sb.WriteString(log.TransactionHash)
			sb.WriteString(`", transactionIndex: `)
			sb.WriteString(strconv.Itoa(log.TransactionIndex))
			sb.WriteString(`, blockHash: "`)
			sb.WriteString(log.BlockHash)
			sb.WriteString(`", logIndex: `)
			sb.WriteString(strconv.Itoa(log.LogIndex))
			sb.WriteString(`, removed: "`)
			sb.WriteString(fmt.Sprintf("%v", log.Removed))
			sb.WriteString(`" }) { _docID }`)
			sb.WriteString("\n")
			logIdx++
		}
	}

	// === Access List Entries ===
	aleIdx := 0
	for _, tx := range transactions {
		if tx == nil {
			continue
		}

		for i := range tx.AccessList {
			ale := &tx.AccessList[i]
			alias := fmt.Sprintf("ale%d", aleIdx)

			sb.WriteString(alias)
			sb.WriteString(`: create_`)
			sb.WriteString(constants.CollectionAccessListEntry)
			sb.WriteString(`(input: { address: "`)
			sb.WriteString(ale.Address)
			sb.WriteString(`", storageKeys: `)
			sb.WriteString(h.formatStringArray(ale.StorageKeys))
			sb.WriteString(` }) { _docID }`)
			sb.WriteString("\n")
			aleIdx++
		}
	}

	sb.WriteString("}")

	return sb.String(), txCount, logIdx, aleIdx
}

// createBlockBatched creates the block using multiple transactions for large blocks.
// This is the fallback for blocks exceeding MaxDocsPerTransaction.
func (h *BlockHandler) createBlockBatched(ctx context.Context, block *types.Block, blockInt int64, transactions []*types.Transaction, receipts []*types.TransactionReceipt, receiptMap map[string]*types.TransactionReceipt) (string, error) {
	txn, err := h.defraNode.DB.NewTxn(false)
	if err != nil {
		return "", errors.NewQueryFailed("defra", "createBlockBatched", "failed to create transaction", err)
	}

	blockMutation := h.buildBlockMutation(block, blockInt)
	result := txn.ExecRequest(ctx, blockMutation)
	if len(result.GQL.Errors) > 0 {
		txn.Discard()
		errMsg := result.GQL.Errors[0].Error()
		return "", errors.NewQueryFailed("defra", "createBlockBatched", errMsg, result.GQL.Errors[0])
	}

	blockID, err := h.extractDocID(result.GQL.Data, "create_"+constants.CollectionBlock)
	if err != nil || blockID == "" {
		txn.Discard()
		return "", errors.NewQueryFailed("defra", "createBlockBatched", "failed to get block ID", err)
	}

	if err := txn.Commit(); err != nil {
		return "", errors.NewQueryFailed("defra", "createBlockBatched", "failed to commit block", err)
	}

	batchSize := 64 // Batch size for large blocks that exceed single-txn threshold
	txHashToID := make(map[string]string)
	txCount := 0

	for i := 0; i < len(transactions); i += batchSize {
		end := i + batchSize
		if end > len(transactions) {
			end = len(transactions)
		}

		batch := transactions[i:end]
		if len(batch) == 0 {
			continue
		}

		batchedMutation, txInfos := h.buildBatchedTransactionMutation(batch, blockID, i)
		if batchedMutation == "" {
			continue
		}

		txn, err = h.defraNode.DB.NewTxn(false)
		if err != nil {
			logger.Sugar.Warnf("Failed to create txn for tx batch: %v", err)
			continue
		}

		result := txn.ExecRequest(ctx, batchedMutation)
		if len(result.GQL.Errors) > 0 {
			txn.Discard()
			logger.Sugar.Warnf("Batch tx mutation error: %v", result.GQL.Errors[0])
			continue
		}

		if err := txn.Commit(); err != nil {
			logger.Sugar.Warnf("Failed to commit tx batch: %v", err)
			continue
		}

		for _, txInfo := range txInfos {
			docID := h.extractDocIDFromBatchedResponse(result.GQL.Data, txInfo.alias)
			if docID != "" {
				txHashToID[txInfo.hash] = docID
				txCount++
			}
		}
	}

	// Phase 3: Create Logs in batches
	var allLogs []logEntry
	for _, tx := range transactions {
		if tx == nil {
			continue
		}
		receipt, ok := receiptMap[tx.Hash]
		if !ok || receipt == nil {
			continue
		}
		txID, ok := txHashToID[tx.Hash]
		if !ok {
			continue
		}
		for i := range receipt.Logs {
			allLogs = append(allLogs, logEntry{log: &receipt.Logs[i], txID: txID})
		}
	}

	logCount := 0
	for i := 0; i < len(allLogs); i += batchSize {
		end := i + batchSize
		if end > len(allLogs) {
			end = len(allLogs)
		}

		batch := allLogs[i:end]
		if len(batch) == 0 {
			continue
		}

		batchedMutation := h.buildBatchedLogMutation(batch, blockID, i)
		if batchedMutation == "" {
			continue
		}

		txn, err = h.defraNode.DB.NewTxn(false)
		if err != nil {
			logger.Sugar.Warnf("Failed to create txn for log batch: %v", err)
			continue
		}

		result := txn.ExecRequest(ctx, batchedMutation)
		if len(result.GQL.Errors) > 0 {
			txn.Discard()
			logger.Sugar.Warnf("Batch log mutation error: %v", result.GQL.Errors[0])
			continue
		}

		if err := txn.Commit(); err != nil {
			logger.Sugar.Warnf("Failed to commit log batch: %v", err)
			continue
		}

		logCount += len(batch)
	}

	// Phase 4: Create Access List Entries in batches
	var allALEs []aleEntry
	for _, tx := range transactions {
		if tx == nil {
			continue
		}
		txID, ok := txHashToID[tx.Hash]
		if !ok {
			continue
		}
		for i := range tx.AccessList {
			allALEs = append(allALEs, aleEntry{ale: &tx.AccessList[i], txID: txID})
		}
	}

	aleCount := 0
	for i := 0; i < len(allALEs); i += batchSize {
		end := i + batchSize
		if end > len(allALEs) {
			end = len(allALEs)
		}

		batch := allALEs[i:end]
		if len(batch) == 0 {
			continue
		}

		batchedMutation := h.buildBatchedALEMutation(batch, i)
		if batchedMutation == "" {
			continue
		}

		txn, err = h.defraNode.DB.NewTxn(false)
		if err != nil {
			logger.Sugar.Warnf("Failed to create txn for ALE batch: %v", err)
			continue
		}

		result := txn.ExecRequest(ctx, batchedMutation)
		if len(result.GQL.Errors) > 0 {
			txn.Discard()
			logger.Sugar.Warnf("Batch ALE mutation error: %v", result.GQL.Errors[0])
			continue
		}

		if err := txn.Commit(); err != nil {
			logger.Sugar.Warnf("Failed to commit ALE batch: %v", err)
			continue
		}

		aleCount += len(batch)
	}

	return blockID, nil
}

// txAliasInfo holds the alias and hash for a transaction in a batched mutation
type txAliasInfo struct {
	alias string
	hash  string
}

// buildBatchedTransactionMutation creates a single GraphQL mutation for multiple transactions
func (h *BlockHandler) buildBatchedTransactionMutation(txs []*types.Transaction, blockID string, startIdx int) (string, []txAliasInfo) {
	var sb strings.Builder
	sb.Grow(len(txs) * 1536)
	sb.WriteString("mutation {\n")

	var txInfos []txAliasInfo
	for i, tx := range txs {
		if tx == nil {
			continue
		}
		alias := fmt.Sprintf("tx%d", startIdx+i)
		txInfos = append(txInfos, txAliasInfo{alias: alias, hash: tx.Hash})
		txBlockNum, _ := strconv.ParseInt(tx.BlockNumber, 10, 64)

		sb.WriteString(alias)
		sb.WriteString(`: create_`)
		sb.WriteString(constants.CollectionTransaction)
		sb.WriteString(`(input: { hash: "`)
		sb.WriteString(tx.Hash)
		sb.WriteString(`", blockNumber: `)
		sb.WriteString(strconv.FormatInt(txBlockNum, 10))
		sb.WriteString(`, blockHash: "`)
		sb.WriteString(tx.BlockHash)
		sb.WriteString(`", transactionIndex: `)
		sb.WriteString(strconv.Itoa(tx.TransactionIndex))
		sb.WriteString(`, from: "`)
		sb.WriteString(tx.From)
		sb.WriteString(`", to: "`)
		sb.WriteString(tx.To)
		sb.WriteString(`", value: "`)
		sb.WriteString(tx.Value)
		sb.WriteString(`", gas: "`)
		sb.WriteString(tx.Gas)
		sb.WriteString(`", gasPrice: "`)
		sb.WriteString(tx.GasPrice)
		sb.WriteString(`", maxFeePerGas: "`)
		sb.WriteString(tx.MaxFeePerGas)
		sb.WriteString(`", maxPriorityFeePerGas: "`)
		sb.WriteString(tx.MaxPriorityFeePerGas)
		sb.WriteString(`", input: "`)
		sb.WriteString(string(tx.Input))
		sb.WriteString(`", nonce: "`)
		sb.WriteString(tx.Nonce)
		sb.WriteString(`", type: "`)
		sb.WriteString(tx.Type)
		sb.WriteString(`", chainId: "`)
		sb.WriteString(tx.ChainId)
		sb.WriteString(`", v: "`)
		sb.WriteString(tx.V)
		sb.WriteString(`", r: "`)
		sb.WriteString(tx.R)
		sb.WriteString(`", s: "`)
		sb.WriteString(tx.S)
		sb.WriteString(`", cumulativeGasUsed: "`)
		sb.WriteString(tx.CumulativeGasUsed)
		sb.WriteString(`", effectiveGasPrice: "`)
		sb.WriteString(tx.EffectiveGasPrice)
		sb.WriteString(`", status: `)
		sb.WriteString(strconv.FormatBool(tx.Status))
		sb.WriteString(`, block: "`)
		sb.WriteString(blockID)
		sb.WriteString(`" }) { _docID }`)
		sb.WriteString("\n")
	}

	sb.WriteString("}")

	if len(txInfos) == 0 {
		return "", nil
	}
	return sb.String(), txInfos
}

// buildBatchedLogMutation creates a single GraphQL mutation for multiple logs
func (h *BlockHandler) buildBatchedLogMutation(logs []logEntry, blockID string, startIdx int) string {
	var sb strings.Builder
	sb.Grow(len(logs) * 1024)
	sb.WriteString("mutation {\n")

	count := 0
	for i, entry := range logs {
		if entry.log == nil {
			continue
		}
		logBlockNum, _ := utils.HexToInt(entry.log.BlockNumber)
		alias := fmt.Sprintf("log%d", startIdx+i)

		sb.WriteString(alias)
		sb.WriteString(`: create_`)
		sb.WriteString(constants.CollectionLog)
		sb.WriteString(`(input: { address: "`)
		sb.WriteString(entry.log.Address)
		sb.WriteString(`", topics: `)
		sb.WriteString(h.formatStringArray(entry.log.Topics))
		sb.WriteString(`, data: "`)
		sb.WriteString(entry.log.Data)
		sb.WriteString(`", blockNumber: `)
		sb.WriteString(strconv.FormatInt(logBlockNum, 10))
		sb.WriteString(`, transactionHash: "`)
		sb.WriteString(entry.log.TransactionHash)
		sb.WriteString(`", transactionIndex: `)
		sb.WriteString(strconv.Itoa(entry.log.TransactionIndex))
		sb.WriteString(`, blockHash: "`)
		sb.WriteString(entry.log.BlockHash)
		sb.WriteString(`", logIndex: `)
		sb.WriteString(strconv.Itoa(entry.log.LogIndex))
		sb.WriteString(`, removed: "`)
		sb.WriteString(fmt.Sprintf("%v", entry.log.Removed))
		sb.WriteString(`", transaction: "`)
		sb.WriteString(entry.txID)
		sb.WriteString(`", block: "`)
		sb.WriteString(blockID)
		sb.WriteString(`" }) { _docID }`)
		sb.WriteString("\n")
		count++
	}

	sb.WriteString("}")

	if count == 0 {
		return ""
	}
	return sb.String()
}

// buildBatchedALEMutation creates a single GraphQL mutation for multiple access list entries
func (h *BlockHandler) buildBatchedALEMutation(ales []aleEntry, startIdx int) string {
	var sb strings.Builder
	sb.Grow(len(ales) * 512)
	sb.WriteString("mutation {\n")

	count := 0
	for i, entry := range ales {
		if entry.ale == nil {
			continue
		}
		alias := fmt.Sprintf("ale%d", startIdx+i)

		sb.WriteString(alias)
		sb.WriteString(`: create_`)
		sb.WriteString(constants.CollectionAccessListEntry)
		sb.WriteString(`(input: { address: "`)
		sb.WriteString(entry.ale.Address)
		sb.WriteString(`", storageKeys: `)
		sb.WriteString(h.formatStringArray(entry.ale.StorageKeys))
		sb.WriteString(`, transaction: "`)
		sb.WriteString(entry.txID)
		sb.WriteString(`" }) { _docID }`)
		sb.WriteString("\n")
		count++
	}

	sb.WriteString("}")

	if count == 0 {
		return ""
	}
	return sb.String()
}

// extractDocIDFromBatchedResponse extracts a doc ID from a batched mutation response by alias
func (h *BlockHandler) extractDocIDFromBatchedResponse(data any, alias string) string {
	dataMap, ok := data.(map[string]any)
	if !ok {
		return ""
	}

	aliasData, ok := dataMap[alias]
	if !ok {
		keys := make([]string, 0, 5)
		for k := range dataMap {
			keys = append(keys, k)
			if len(keys) >= 5 {
				break
			}
		}
		return ""
	}

	switch v := aliasData.(type) {
	case map[string]any:
		if docID, ok := v["_docID"].(string); ok {
			return docID
		}
	case []map[string]interface{}:
		// DefraDB returns this type for batched mutations
		if len(v) > 0 {
			if docID, ok := v[0]["_docID"].(string); ok {
				return docID
			}
		}
	case []any:
		if len(v) > 0 {
			if item, ok := v[0].(map[string]any); ok {
				if docID, ok := item["_docID"].(string); ok {
					return docID
				}
			}
		}
	}
	return ""
}

// buildBlockMutation creates a GraphQL mutation for a block using strings.Builder for efficiency
func (h *BlockHandler) buildBlockMutation(block *types.Block, blockInt int64) string {
	var sb strings.Builder
	sb.Grow(2048) // Pre-allocate for typical block mutation size

	sb.WriteString(`mutation { create_`)
	sb.WriteString(constants.CollectionBlock)
	sb.WriteString(`(input: { hash: "`)
	sb.WriteString(block.Hash)
	sb.WriteString(`", number: `)
	sb.WriteString(strconv.FormatInt(blockInt, 10))
	sb.WriteString(`, timestamp: "`)
	sb.WriteString(block.Timestamp)
	sb.WriteString(`", parentHash: "`)
	sb.WriteString(block.ParentHash)
	sb.WriteString(`", difficulty: "`)
	sb.WriteString(block.Difficulty)
	sb.WriteString(`", totalDifficulty: "`)
	sb.WriteString(block.TotalDifficulty)
	sb.WriteString(`", gasUsed: "`)
	sb.WriteString(block.GasUsed)
	sb.WriteString(`", gasLimit: "`)
	sb.WriteString(block.GasLimit)
	sb.WriteString(`", baseFeePerGas: "`)
	sb.WriteString(block.BaseFeePerGas)
	sb.WriteString(`", nonce: "`)
	sb.WriteString(block.Nonce)
	sb.WriteString(`", miner: "`)
	sb.WriteString(block.Miner)
	sb.WriteString(`", size: "`)
	sb.WriteString(block.Size)
	sb.WriteString(`", stateRoot: "`)
	sb.WriteString(block.StateRoot)
	sb.WriteString(`", sha3Uncles: "`)
	sb.WriteString(block.Sha3Uncles)
	sb.WriteString(`", transactionsRoot: "`)
	sb.WriteString(block.TransactionsRoot)
	sb.WriteString(`", receiptsRoot: "`)
	sb.WriteString(block.ReceiptsRoot)
	sb.WriteString(`", logsBloom: "`)
	sb.WriteString(block.LogsBloom)
	sb.WriteString(`", extraData: "`)
	sb.WriteString(block.ExtraData)
	sb.WriteString(`", mixHash: "`)
	sb.WriteString(block.MixHash)
	sb.WriteString(`", uncles: `)
	sb.WriteString(h.formatStringArray(block.Uncles))
	sb.WriteString(` }) { _docID } }`)

	return sb.String()
}

// buildTransactionMutation creates a GraphQL mutation for a transaction using strings.Builder
func (h *BlockHandler) buildTransactionMutation(tx *types.Transaction, blockID string) string {
	txBlockNum, _ := strconv.ParseInt(tx.BlockNumber, 10, 64)

	var sb strings.Builder
	sb.Grow(1536) // Pre-allocate for typical transaction mutation size

	sb.WriteString(`mutation { create_`)
	sb.WriteString(constants.CollectionTransaction)
	sb.WriteString(`(input: { hash: "`)
	sb.WriteString(tx.Hash)
	sb.WriteString(`", blockNumber: `)
	sb.WriteString(strconv.FormatInt(txBlockNum, 10))
	sb.WriteString(`, blockHash: "`)
	sb.WriteString(tx.BlockHash)
	sb.WriteString(`", transactionIndex: `)
	sb.WriteString(strconv.Itoa(tx.TransactionIndex))
	sb.WriteString(`, from: "`)
	sb.WriteString(tx.From)
	sb.WriteString(`", to: "`)
	sb.WriteString(tx.To)
	sb.WriteString(`", value: "`)
	sb.WriteString(tx.Value)
	sb.WriteString(`", gas: "`)
	sb.WriteString(tx.Gas)
	sb.WriteString(`", gasPrice: "`)
	sb.WriteString(tx.GasPrice)
	sb.WriteString(`", maxFeePerGas: "`)
	sb.WriteString(tx.MaxFeePerGas)
	sb.WriteString(`", maxPriorityFeePerGas: "`)
	sb.WriteString(tx.MaxPriorityFeePerGas)
	sb.WriteString(`", input: "`)
	sb.WriteString(string(tx.Input))
	sb.WriteString(`", nonce: "`)
	sb.WriteString(tx.Nonce)
	sb.WriteString(`", type: "`)
	sb.WriteString(tx.Type)
	sb.WriteString(`", chainId: "`)
	sb.WriteString(tx.ChainId)
	sb.WriteString(`", v: "`)
	sb.WriteString(tx.V)
	sb.WriteString(`", r: "`)
	sb.WriteString(tx.R)
	sb.WriteString(`", s: "`)
	sb.WriteString(tx.S)
	sb.WriteString(`", cumulativeGasUsed: "`)
	sb.WriteString(tx.CumulativeGasUsed)
	sb.WriteString(`", effectiveGasPrice: "`)
	sb.WriteString(tx.EffectiveGasPrice)
	sb.WriteString(`", status: `)
	if tx.Status {
		sb.WriteString("true")
	} else {
		sb.WriteString("false")
	}
	sb.WriteString(`, block: "`)
	sb.WriteString(blockID)
	sb.WriteString(`" }) { _docID } }`)

	return sb.String()
}

// buildLogMutation creates a GraphQL mutation for a log using strings.Builder
func (h *BlockHandler) buildLogMutation(log *types.Log, blockID, txID string) string {
	logBlockNum, _ := utils.HexToInt(log.BlockNumber)

	var sb strings.Builder
	sb.Grow(1024) // Pre-allocate for typical log mutation size

	sb.WriteString(`mutation { create_`)
	sb.WriteString(constants.CollectionLog)
	sb.WriteString(`(input: { address: "`)
	sb.WriteString(log.Address)
	sb.WriteString(`", topics: `)
	sb.WriteString(h.formatStringArray(log.Topics))
	sb.WriteString(`, data: "`)
	sb.WriteString(log.Data)
	sb.WriteString(`", blockNumber: `)
	sb.WriteString(strconv.FormatInt(logBlockNum, 10))
	sb.WriteString(`, transactionHash: "`)
	sb.WriteString(log.TransactionHash)
	sb.WriteString(`", transactionIndex: `)
	sb.WriteString(strconv.Itoa(log.TransactionIndex))
	sb.WriteString(`, blockHash: "`)
	sb.WriteString(log.BlockHash)
	sb.WriteString(`", logIndex: `)
	sb.WriteString(strconv.Itoa(log.LogIndex))
	sb.WriteString(`, removed: "`)
	if log.Removed {
		sb.WriteString("true")
	} else {
		sb.WriteString("false")
	}
	sb.WriteString(`", transaction: "`)
	sb.WriteString(txID)
	sb.WriteString(`", block: "`)
	sb.WriteString(blockID)
	sb.WriteString(`" }) { _docID } }`)

	return sb.String()
}

// buildAccessListEntryMutation creates a GraphQL mutation for an access list entry using strings.Builder
func (h *BlockHandler) buildAccessListEntryMutation(ale *types.AccessListEntry, txID string) string {
	var sb strings.Builder
	sb.Grow(512) // Pre-allocate for typical ALE mutation size

	sb.WriteString(`mutation { create_`)
	sb.WriteString(constants.CollectionAccessListEntry)
	sb.WriteString(`(input: { address: "`)
	sb.WriteString(ale.Address)
	sb.WriteString(`", storageKeys: `)
	sb.WriteString(h.formatStringArray(ale.StorageKeys))
	sb.WriteString(`, transaction: "`)
	sb.WriteString(txID)
	sb.WriteString(`" }) { _docID } }`)

	return sb.String()
}

// formatStringArray formats a string slice as a GraphQL array
func (h *BlockHandler) formatStringArray(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}
	jsonBytes, _ := json.Marshal(arr)
	return string(jsonBytes)
}

// extractDocID extracts the document ID from a GraphQL response
func (h *BlockHandler) extractDocID(data any, fieldName string) (string, error) {
	if data == nil {
		return "", fmt.Errorf("nil data")
	}

	dataMap, ok := data.(map[string]any)
	if !ok {
		return "", fmt.Errorf("data is not a map")
	}

	field, ok := dataMap[fieldName]
	if !ok {
		return "", fmt.Errorf("field %s not found", fieldName)
	}

	switch v := field.(type) {
	case []any:
		if len(v) > 0 {
			if item, ok := v[0].(map[string]any); ok {
				if docID, ok := item["_docID"].(string); ok {
					return docID, nil
				}
			}
		}
	case []map[string]any:
		if len(v) > 0 {
			if docID, ok := v[0]["_docID"].(string); ok {
				return docID, nil
			}
		}
	case map[string]any:
		if docID, ok := v["_docID"].(string); ok {
			return docID, nil
		}
	}

	return "", fmt.Errorf("could not extract docID from %v", field)
}

// GetHighestBlockNumber returns the highest block number stored in DefraDB
func (h *BlockHandler) GetHighestBlockNumber(ctx context.Context) (int64, error) {
	query := types.Request{
		Type: "POST",
		Query: `query {` +
			constants.CollectionBlock +
			` (order: {number: DESC}, limit: 1) {
			number
		}	
	}`}

	resp, err := h.SendToGraphql(ctx, query)
	if err != nil {
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
		return 0, errors.NewDocumentNotFound("defra", "GetHighestBlockNumber", constants.CollectionBlock, fmt.Sprintf("%v", data))
	}

	// Extract Block array
	blockArray, ok := data[constants.CollectionBlock].([]interface{})
	if !ok {
		logger.Sugar.Error("Block field not found in response")
		return 0, errors.NewDocumentNotFound("defra", "GetHighestBlockNumber", constants.CollectionBlock, fmt.Sprintf("%v", data[constants.CollectionBlock]))
	}

	if len(blockArray) == 0 {
		return 0, errors.NewDocumentNotFound("defra", "GetHighestBlockNumber", constants.CollectionBlock, "blockArray is empty")
	}

	// Extract first block
	block, ok := blockArray[0].(map[string]interface{})
	if !ok {
		logger.Sugar.Error("Invalid block format in response")
		return 0, errors.NewDocumentNotFound("defra", "GetHighestBlockNumber", constants.CollectionBlock, fmt.Sprintf("%v", blockArray))
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
	return 0, errors.NewDocumentNotFound("defra", "GetHighestBlockNumber", constants.CollectionBlock, fmt.Sprintf("%v", numberValue))
}
