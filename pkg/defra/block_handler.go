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

// DocsPerTxn is the number of documents to create per transaction commit.
const DocsPerTxn = 4

type BlockHandler struct {
	defraURL      string
	client        *http.Client
	defraNode     *node.Node       // Direct access to embedded DefraDB (nil if using HTTP)
	rateLimiter   <-chan time.Time // Rate limiter for document pushes (nil = no limit) - used by HTTP mode
	docsPerSecond int              // Rate limit in docs/sec (0 = no limit) - used by batch mode

	// Rate limiter state (for periodic checking)
	rateLimitWindowStart time.Time // Start of current 1-second window
	docsInCurrentWindow  int       // Docs created in current window
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
func NewBlockHandlerWithNode(defraNode *node.Node) (*BlockHandler, error) {
	if defraNode == nil {
		return nil, errors.NewConfigurationError("defra", "NewBlockHandlerWithNode",
			"defraNode is nil", "", nil)
	}
	return &BlockHandler{
		defraNode: defraNode,
		client:    nil,
		defraURL:  "",
	}, nil
}

// SetRateLimit sets the maximum documents per second that can be pushed.
// Pass 0 to disable rate limiting.
func (h *BlockHandler) SetRateLimit(docsPerSecond int) {
	if docsPerSecond <= 0 {
		h.rateLimiter = nil
		h.docsPerSecond = 0
		return
	}
	h.docsPerSecond = docsPerSecond
	interval := time.Second / time.Duration(docsPerSecond)
	h.rateLimiter = time.Tick(interval)
}

// enforceRateLimit enforces rate limiting by sleeping based on how many docs were created.
// Uses a token bucket approach: each doc consumes time based on the rate limit.
func (h *BlockHandler) enforceRateLimit(docsJustCreated int) {
	if h.docsPerSecond <= 0 || docsJustCreated <= 0 {
		return
	}

	timePerDoc := time.Second / time.Duration(h.docsPerSecond)

	now := time.Now()

	if h.rateLimitWindowStart.IsZero() {
		h.rateLimitWindowStart = now
		h.docsInCurrentWindow = docsJustCreated
		return
	}

	elapsed := now.Sub(h.rateLimitWindowStart)
	h.docsInCurrentWindow += docsJustCreated
	totalRequiredTime := timePerDoc * time.Duration(h.docsInCurrentWindow)

	if elapsed < totalRequiredTime {
		sleepTime := totalRequiredTime - elapsed
		if sleepTime > 0 {
			time.Sleep(sleepTime)
		}
	}

	if h.docsInCurrentWindow >= h.docsPerSecond*10 {
		h.rateLimitWindowStart = time.Now()
		h.docsInCurrentWindow = 0
	}
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
		"block":                block_id, // Include block relationship directly

	}
	logger.Sugar.Debug("Creating transaction: ", txData)
	// Database operation
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
	logger.Sugar.Debug("Creating access list entry: ", ALEData)
	// Database operation
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
		"removed":          fmt.Sprintf("%v", log.Removed), // Convert bool to string
		"transaction":      tx_Id,
		"block":            block_id,
	}
	logger.Sugar.Debug("Creating log: ", logData)
	// Database operation
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

	if h.rateLimiter != nil {
		select {
		case <-h.rateLimiter:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

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

	// Check for GraphQL errors first
	if graphqlErrors, hasErrors := rawResponse["errors"].([]interface{}); hasErrors && len(graphqlErrors) > 0 {
		if errorMap, ok := graphqlErrors[0].(map[string]interface{}); ok {
			if message, ok := errorMap["message"].(string); ok {
				// Handle duplicate document error gracefully
				if strings.Contains(message, "already exists") {
					logger.Sugar.Infof("Document already exists in %s collection, skipping creation", collection)
					// Extract DocID from error message if possible
					if strings.Contains(message, "DocID: ") {
						parts := strings.Split(message, "DocID: ")
						if len(parts) > 1 {
							docID := strings.TrimSpace(parts[1])
							return docID, nil
						}
					}
					return "", errors.NewQueryFailed("defra", "PostToCollection", "document already exists", nil)
				}
				logger.Sugar.Errorf("GraphQL error for %s: %s", collection, message)
				return "", errors.NewQueryFailed("defra", "PostToCollection", message, nil)
			}
		}
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

	if h.defraNode != nil {
		return h.sendToGraphqlDirect(ctx, req)
	}

	return h.sendToGraphqlHTTP(ctx, req)
}

// sendToGraphqlDirect executes GraphQL directly on the embedded DefraDB node
func (h *BlockHandler) sendToGraphqlDirect(ctx context.Context, req types.Request) ([]byte, error) {
	logger.Sugar.Debug("Sending direct mutation: ", req.Query, "\n")

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

	logger.Sugar.Debug("DefraDB Direct Response: ", string(respBody), "\n")
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

	// Debug: Print the mutation
	logger.Sugar.Debug("Sending HTTP mutation: ", req.Query, "\n")

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, req.Type, h.defraURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		logger.Sugar.Errorf("failed to create request: ", err)
		return nil, errors.NewQueryFailed("defra", "sendToGraphqlHTTP", fmt.Sprintf("%v", req), err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := h.client.Do(httpReq)
	if err != nil {
		logger.Sugar.Errorf("Failed to send GraphQL request to DefraDB at %s: %v", h.defraURL, err)
		if strings.Contains(err.Error(), "connection refused") {
			logger.Sugar.Error("DefraDB appears to be down. Please ensure DefraDB is running on the configured port.")
		}
		return nil, errors.NewQueryFailed("defra", "sendToGraphqlHTTP", fmt.Sprintf("%v", req), err)
	}

	defer resp.Body.Close()
	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Sugar.Errorf("Read response error: ", err)
		return nil, errors.NewQueryFailed("defra", "sendToGraphqlHTTP", fmt.Sprintf("%v", req), err)
	}
	// Debug: Print the response
	logger.Sugar.Debug("DefraDB HTTP Response: ", string(respBody), "\n")
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

	txn, err := h.defraNode.DB.NewTxn(false)
	if err != nil {
		return "", errors.NewQueryFailed("defra", "CreateBlockBatch", "failed to create transaction", err)
	}

	blockMutation := h.buildBlockMutation(block, blockInt)
	result := txn.ExecRequest(ctx, blockMutation)
	if len(result.GQL.Errors) > 0 {
		txn.Discard()
		errMsg := result.GQL.Errors[0].Error()
		if strings.Contains(errMsg, "already exists") {
			logger.Sugar.Infof("Block %d already exists, skipping batch creation", blockInt)
			return "", fmt.Errorf("block already exists")
		}
		return "", errors.NewQueryFailed("defra", "CreateBlockBatch", errMsg, result.GQL.Errors[0])
	}

	blockID, err := h.extractDocID(result.GQL.Data, "create_"+constants.CollectionBlock)
	if err != nil || blockID == "" {
		txn.Discard()
		return "", errors.NewQueryFailed("defra", "CreateBlockBatch", "failed to get block ID", err)
	}

	if err := txn.Commit(); err != nil {
		return "", errors.NewQueryFailed("defra", "CreateBlockBatch", "failed to commit block", err)
	}

	txHashToID := make(map[string]string)
	txCount := 0
	docCount := 0
	txn, err = h.defraNode.DB.NewTxn(false)
	if err != nil {
		return "", errors.NewQueryFailed("defra", "CreateBlockBatch", "failed to create transaction", err)
	}

	for _, tx := range transactions {
		if tx == nil {
			continue
		}

		txMutation := h.buildTransactionMutation(tx, blockID)
		result := txn.ExecRequest(ctx, txMutation)
		if len(result.GQL.Errors) > 0 {
			logger.Sugar.Warnf("Failed to create tx %s: %v", tx.Hash, result.GQL.Errors[0])
			continue
		}

		txID, err := h.extractDocID(result.GQL.Data, "create_"+constants.CollectionTransaction)
		if err != nil || txID == "" {
			logger.Sugar.Warnf("Failed to get tx ID for %s", tx.Hash)
			continue
		}
		txHashToID[tx.Hash] = txID
		txCount++
		docCount++

		if docCount >= DocsPerTxn {
			if err := txn.Commit(); err != nil {
				logger.Sugar.Warnf("Failed to commit tx batch: %v", err)
			}
			h.enforceRateLimit(docCount)
			txn, _ = h.defraNode.DB.NewTxn(false)
			docCount = 0
		}
	}

	if docCount > 0 {
		if err := txn.Commit(); err != nil {
			logger.Sugar.Warnf("Failed to commit final tx batch: %v", err)
		}
		h.enforceRateLimit(docCount)
	} else {
		txn.Discard()
	}

	logCount := 0
	docCount = 0
	txn, err = h.defraNode.DB.NewTxn(false)
	if err != nil {
		return "", errors.NewQueryFailed("defra", "CreateBlockBatch", "failed to create transaction", err)
	}

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
			logMutation := h.buildLogMutation(&receipt.Logs[i], blockID, txID)
			result := txn.ExecRequest(ctx, logMutation)
			if len(result.GQL.Errors) > 0 {
				logger.Sugar.Warnf("Failed to create log: %v", result.GQL.Errors[0])
				continue
			}
			logCount++
			docCount++

			if docCount >= DocsPerTxn {
				if err := txn.Commit(); err != nil {
					logger.Sugar.Warnf("Failed to commit log batch: %v", err)
				}
				h.enforceRateLimit(docCount)
				txn, _ = h.defraNode.DB.NewTxn(false)
				docCount = 0
			}
		}
	}

	if docCount > 0 {
		if err := txn.Commit(); err != nil {
			logger.Sugar.Warnf("Failed to commit final log batch: %v", err)
		}
		h.enforceRateLimit(docCount)
	} else {
		txn.Discard()
	}

	aleCount := 0
	docCount = 0
	txn, err = h.defraNode.DB.NewTxn(false)
	if err != nil {
		return "", errors.NewQueryFailed("defra", "CreateBlockBatch", "failed to create transaction", err)
	}

	for _, tx := range transactions {
		if tx == nil {
			continue
		}
		txID, ok := txHashToID[tx.Hash]
		if !ok {
			continue
		}

		for i := range tx.AccessList {
			aleMutation := h.buildAccessListEntryMutation(&tx.AccessList[i], txID)
			result := txn.ExecRequest(ctx, aleMutation)
			if len(result.GQL.Errors) > 0 {
				logger.Sugar.Warnf("Failed to create ALE: %v", result.GQL.Errors[0])
				continue
			}
			aleCount++
			docCount++

			if docCount >= DocsPerTxn {
				if err := txn.Commit(); err != nil {
					logger.Sugar.Warnf("Failed to commit ALE batch: %v", err)
				}
				h.enforceRateLimit(docCount)
				txn, _ = h.defraNode.DB.NewTxn(false)
				docCount = 0
			}
		}
	}

	if docCount > 0 {
		if err := txn.Commit(); err != nil {
			logger.Sugar.Warnf("Failed to commit final batch: %v", err)
		}
		h.enforceRateLimit(docCount)
	} else {
		txn.Discard()
	}

	return blockID, nil
}

// buildBlockMutation creates a GraphQL mutation for a block
func (h *BlockHandler) buildBlockMutation(block *types.Block, blockInt int64) string {
	return fmt.Sprintf(`mutation {
		create_%s(input: {
			hash: %q,
			number: %d,
			timestamp: %q,
			parentHash: %q,
			difficulty: %q,
			totalDifficulty: %q,
			gasUsed: %q,
			gasLimit: %q,
			baseFeePerGas: %q,
			nonce: %q,
			miner: %q,
			size: %q,
			stateRoot: %q,
			sha3Uncles: %q,
			transactionsRoot: %q,
			receiptsRoot: %q,
			logsBloom: %q,
			extraData: %q,
			mixHash: %q,
			uncles: %s
		}) { _docID }
	}`, constants.CollectionBlock,
		block.Hash, blockInt, block.Timestamp, block.ParentHash,
		block.Difficulty, block.TotalDifficulty, block.GasUsed, block.GasLimit,
		block.BaseFeePerGas, block.Nonce, block.Miner, block.Size,
		block.StateRoot, block.Sha3Uncles, block.TransactionsRoot,
		block.ReceiptsRoot, block.LogsBloom, block.ExtraData, block.MixHash,
		h.formatStringArray(block.Uncles))
}

// buildTransactionMutation creates a GraphQL mutation for a transaction
func (h *BlockHandler) buildTransactionMutation(tx *types.Transaction, blockID string) string {
	txBlockNum, _ := strconv.ParseInt(tx.BlockNumber, 10, 64)
	return fmt.Sprintf(`mutation {
		create_%s(input: {
			hash: %q,
			blockNumber: %d,
			blockHash: %q,
			transactionIndex: %d,
			from: %q,
			to: %q,
			value: %q,
			gas: %q,
			gasPrice: %q,
			maxFeePerGas: %q,
			maxPriorityFeePerGas: %q,
			input: %q,
			nonce: %q,
			type: %q,
			chainId: %q,
			v: %q,
			r: %q,
			s: %q,
			cumulativeGasUsed: %q,
			effectiveGasPrice: %q,
			status: %t,
			block: %q
		}) { _docID }
	}`, constants.CollectionTransaction,
		tx.Hash, txBlockNum, tx.BlockHash, tx.TransactionIndex,
		tx.From, tx.To, tx.Value, tx.Gas, tx.GasPrice,
		tx.MaxFeePerGas, tx.MaxPriorityFeePerGas, string(tx.Input),
		tx.Nonce, tx.Type, tx.ChainId, tx.V, tx.R, tx.S,
		tx.CumulativeGasUsed, tx.EffectiveGasPrice, tx.Status, blockID)
}

// buildLogMutation creates a GraphQL mutation for a log
func (h *BlockHandler) buildLogMutation(log *types.Log, blockID, txID string) string {
	logBlockNum, _ := utils.HexToInt(log.BlockNumber)
	removed := "false"
	if log.Removed {
		removed = "true"
	}
	return fmt.Sprintf(`mutation {
		create_%s(input: {
			address: %q,
			topics: %s,
			data: %q,
			blockNumber: %d,
			transactionHash: %q,
			transactionIndex: %d,
			blockHash: %q,
			logIndex: %d,
			removed: %q,
			transaction: %q,
			block: %q
		}) { _docID }
	}`, constants.CollectionLog,
		log.Address, h.formatStringArray(log.Topics), log.Data, logBlockNum,
		log.TransactionHash, log.TransactionIndex, log.BlockHash,
		log.LogIndex, removed, txID, blockID)
}

// buildAccessListEntryMutation creates a GraphQL mutation for an access list entry
func (h *BlockHandler) buildAccessListEntryMutation(ale *types.AccessListEntry, txID string) string {
	return fmt.Sprintf(`mutation {
		create_%s(input: {
			address: %q,
			storageKeys: %s,
			transaction: %q
		}) { _docID }
	}`, constants.CollectionAccessListEntry,
		ale.Address, h.formatStringArray(ale.StorageKeys), txID)
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
