package defra

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/shinzonetwork/shinzo-indexer-client/pkg/constants"
	"github.com/shinzonetwork/shinzo-indexer-client/pkg/errors"
	"github.com/shinzonetwork/shinzo-indexer-client/pkg/logger"
	"github.com/shinzonetwork/shinzo-indexer-client/pkg/types"
	"github.com/shinzonetwork/shinzo-indexer-client/pkg/utils"
)

// BatchMutation represents a single mutation to be batched with others
type BatchMutation struct {
	Alias      string                 // Unique alias like "tx0", "log5" - required for GraphQL
	Collection string                 // Collection name (e.g., constants.CollectionTransaction)
	Data       map[string]interface{} // Document data to insert
}

// BatchResult holds the result of a single mutation in a batch
type BatchResult struct {
	Alias string
	DocID string
	Error error
}

// BatchExecute executes multiple mutations in a single GraphQL request.
// This dramatically reduces overhead compared to individual mutations.
// Returns a map of alias -> docID for successful creations.
func (h *BlockHandler) BatchExecute(ctx context.Context, mutations []BatchMutation) (map[string]string, error) {
	if len(mutations) == 0 {
		return make(map[string]string), nil
	}

	// Build combined mutation query with aliases
	var mutationParts []string
	for _, m := range mutations {
		inputStr := h.formatMutationInput(m.Data)
		part := fmt.Sprintf(`%s: create_%s(input: { %s }) { _docID }`,
			m.Alias, m.Collection, inputStr)
		mutationParts = append(mutationParts, part)
	}

	query := fmt.Sprintf("mutation {\n%s\n}", strings.Join(mutationParts, "\n"))

	req := types.Request{
		Type:  "POST",
		Query: query,
	}

	logger.Sugar.Debugf("Executing batch mutation with %d operations", len(mutations))

	resp, err := h.SendToGraphql(ctx, req)
	if err != nil {
		return nil, errors.NewQueryFailed("defra", "BatchExecute", "failed to send batch mutation", err)
	}

	return h.parseBatchResponse(resp, mutations)
}

// formatMutationInput converts a data map to GraphQL input string format
func (h *BlockHandler) formatMutationInput(data map[string]interface{}) string {
	var fields []string
	for key, value := range data {
		switch v := value.(type) {
		case string:
			// Escape special characters in strings
			escaped := strings.ReplaceAll(v, `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `"`, `\"`)
			fields = append(fields, fmt.Sprintf(`%s: "%s"`, key, escaped))
		case bool:
			fields = append(fields, fmt.Sprintf("%s: %v", key, v))
		case int:
			fields = append(fields, fmt.Sprintf("%s: %d", key, v))
		case int64:
			fields = append(fields, fmt.Sprintf("%s: %d", key, v))
		case []string:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				logger.Sugar.Warnf("Failed to marshal string array for field %s: %v", key, err)
				fields = append(fields, fmt.Sprintf("%s: []", key))
			} else {
				fields = append(fields, fmt.Sprintf("%s: %s", key, string(jsonBytes)))
			}
		default:
			// Default: quote as string
			escaped := strings.ReplaceAll(fmt.Sprint(v), `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `"`, `\"`)
			fields = append(fields, fmt.Sprintf(`%s: "%s"`, key, escaped))
		}
	}
	return strings.Join(fields, ", ")
}

// parseBatchResponse parses the GraphQL response and extracts docIDs by alias
func (h *BlockHandler) parseBatchResponse(resp []byte, mutations []BatchMutation) (map[string]string, error) {
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(resp, &rawResponse); err != nil {
		return nil, errors.NewParsingFailed("defra", "parseBatchResponse", string(resp), err)
	}

	// Check for GraphQL errors - log but don't fail entirely
	if graphqlErrors, ok := rawResponse["errors"].([]interface{}); ok && len(graphqlErrors) > 0 {
		for _, e := range graphqlErrors {
			if errMap, ok := e.(map[string]interface{}); ok {
				msg := errMap["message"]
				// Check if it's a "already exists" error - this is expected for duplicates
				if msgStr, ok := msg.(string); ok {
					if strings.Contains(msgStr, "already exists") {
						logger.Sugar.Debugf("Document already exists (expected for retries): %s", msgStr)
					} else {
						logger.Sugar.Warnf("Batch mutation error: %s", msgStr)
					}
				}
			}
		}
	}

	results := make(map[string]string)
	data, ok := rawResponse["data"].(map[string]interface{})
	if !ok {
		// No data returned - all mutations may have failed
		return results, nil
	}

	// Extract docID for each alias
	for _, m := range mutations {
		if fieldData, ok := data[m.Alias]; ok && fieldData != nil {
			docID := h.extractDocIDFromBatchField(fieldData)
			if docID != "" {
				results[m.Alias] = docID
			}
		}
	}

	return results, nil
}

// extractDocIDFromBatchField extracts docID from various response formats
func (h *BlockHandler) extractDocIDFromBatchField(field interface{}) string {
	switch v := field.(type) {
	case map[string]interface{}:
		if docID, ok := v["_docID"].(string); ok {
			return docID
		}
	case []interface{}:
		if len(v) > 0 {
			if item, ok := v[0].(map[string]interface{}); ok {
				if docID, ok := item["_docID"].(string); ok {
					return docID
				}
			}
		}
	case []map[string]interface{}:
		if len(v) > 0 {
			if docID, ok := v[0]["_docID"].(string); ok {
				return docID
			}
		}
	}
	return ""
}

// CreateBlockBatchOptimized creates a block with all related documents using batched mutations.
// This is significantly faster than CreateBlockBatch as it combines many mutations into single requests.
func (h *BlockHandler) CreateBlockBatchOptimized(ctx context.Context, block *types.Block, transactions []*types.Transaction, receipts []*types.TransactionReceipt) (string, error) {
	if block == nil {
		return "", errors.NewInvalidBlockFormat("defra", "CreateBlockBatchOptimized", "nil", nil)
	}

	blockInt, err := utils.HexToInt(block.Number)
	if err != nil {
		return "", err
	}

	// Build receipt lookup map for fast access
	receiptMap := make(map[string]*types.TransactionReceipt)
	for _, receipt := range receipts {
		if receipt != nil {
			receiptMap[receipt.TransactionHash] = receipt
		}
	}

	// === PHASE 1: Create Block ===
	blockData := h.buildBlockDataMap(block, blockInt)
	blockMutations := []BatchMutation{{
		Alias:      "block0",
		Collection: constants.CollectionBlock,
		Data:       blockData,
	}}

	blockResults, err := h.BatchExecute(ctx, blockMutations)
	if err != nil {
		// Check if block already exists
		if strings.Contains(err.Error(), "already exists") {
			return "", fmt.Errorf("block already exists")
		}
		return "", errors.NewQueryFailed("defra", "CreateBlockBatchOptimized", "failed to create block", err)
	}

	blockID, ok := blockResults["block0"]
	if !ok || blockID == "" {
		return "", errors.NewQueryFailed("defra", "CreateBlockBatchOptimized", "failed to get block ID from response", nil)
	}

	logger.Sugar.Debugf("Created block %d with ID %s", blockInt, blockID)

	// === PHASE 2: Create All Transactions in Batches ===
	const txBatchSize = 50 // Optimal batch size - adjust based on testing
	txHashToID := make(map[string]string)
	txCount := 0

	for i := 0; i < len(transactions); i += txBatchSize {
		end := i + txBatchSize
		if end > len(transactions) {
			end = len(transactions)
		}

		var txMutations []BatchMutation
		txAliasToHash := make(map[string]string) // Track which alias maps to which tx hash

		for j, tx := range transactions[i:end] {
			if tx == nil {
				continue
			}
			alias := fmt.Sprintf("tx%d", i+j)
			txData := h.buildTransactionDataMap(tx, blockID)
			txMutations = append(txMutations, BatchMutation{
				Alias:      alias,
				Collection: constants.CollectionTransaction,
				Data:       txData,
			})
			txAliasToHash[alias] = tx.Hash
		}

		if len(txMutations) > 0 {
			results, err := h.BatchExecute(ctx, txMutations)
			if err != nil {
				logger.Sugar.Warnf("Batch tx creation error (batch %d-%d): %v", i, end, err)
				// Continue with next batch instead of failing entirely
				continue
			}

			// Map tx hashes to doc IDs
			for alias, docID := range results {
				if txHash, ok := txAliasToHash[alias]; ok {
					txHashToID[txHash] = docID
					txCount++
				}
			}

			h.enforceRateLimit(len(txMutations))
		}
	}

	logger.Sugar.Debugf("Created %d transactions for block %d", txCount, blockInt)

	// === PHASE 3: Create All Logs in Batches ===
	const logBatchSize = 100

	// Collect all logs with their associated tx IDs
	type logEntry struct {
		Log  *types.Log
		TxID string
	}
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
			// Transaction wasn't created successfully, skip its logs
			continue
		}
		for i := range receipt.Logs {
			allLogs = append(allLogs, logEntry{
				Log:  &receipt.Logs[i],
				TxID: txID,
			})
		}
	}

	logCount := 0
	for i := 0; i < len(allLogs); i += logBatchSize {
		end := i + logBatchSize
		if end > len(allLogs) {
			end = len(allLogs)
		}

		var logMutations []BatchMutation
		for j, entry := range allLogs[i:end] {
			alias := fmt.Sprintf("log%d", i+j)
			logData := h.buildLogDataMap(entry.Log, blockID, entry.TxID)
			logMutations = append(logMutations, BatchMutation{
				Alias:      alias,
				Collection: constants.CollectionLog,
				Data:       logData,
			})
		}

		if len(logMutations) > 0 {
			results, err := h.BatchExecute(ctx, logMutations)
			if err != nil {
				logger.Sugar.Warnf("Batch log creation error (batch %d-%d): %v", i, end, err)
				continue
			}
			logCount += len(results)
			h.enforceRateLimit(len(logMutations))
		}
	}

	logger.Sugar.Debugf("Created %d logs for block %d", logCount, blockInt)

	// === PHASE 4: Create All Access List Entries in Batches ===
	const aleBatchSize = 100

	type aleEntry struct {
		ALE  *types.AccessListEntry
		TxID string
	}
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
			allALEs = append(allALEs, aleEntry{
				ALE:  &tx.AccessList[i],
				TxID: txID,
			})
		}
	}

	aleCount := 0
	for i := 0; i < len(allALEs); i += aleBatchSize {
		end := i + aleBatchSize
		if end > len(allALEs) {
			end = len(allALEs)
		}

		var aleMutations []BatchMutation
		for j, entry := range allALEs[i:end] {
			alias := fmt.Sprintf("ale%d", i+j)
			aleData := h.buildAccessListEntryDataMap(entry.ALE, entry.TxID)
			aleMutations = append(aleMutations, BatchMutation{
				Alias:      alias,
				Collection: constants.CollectionAccessListEntry,
				Data:       aleData,
			})
		}

		if len(aleMutations) > 0 {
			results, err := h.BatchExecute(ctx, aleMutations)
			if err != nil {
				logger.Sugar.Warnf("Batch ALE creation error (batch %d-%d): %v", i, end, err)
				continue
			}
			aleCount += len(results)
			h.enforceRateLimit(len(aleMutations))
		}
	}

	logger.Sugar.Infof("Batch created block %d: %d txs, %d logs, %d ALEs",
		blockInt, txCount, logCount, aleCount)

	return blockID, nil
}

// buildBlockDataMap creates a data map for block creation
func (h *BlockHandler) buildBlockDataMap(block *types.Block, blockInt int64) map[string]interface{} {
	return map[string]interface{}{
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
}

// buildTransactionDataMap creates a data map for transaction creation
func (h *BlockHandler) buildTransactionDataMap(tx *types.Transaction, blockID string) map[string]interface{} {
	blockInt, _ := strconv.ParseInt(tx.BlockNumber, 10, 64)
	return map[string]interface{}{
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
		"block":                blockID,
	}
}

// buildLogDataMap creates a data map for log creation
func (h *BlockHandler) buildLogDataMap(log *types.Log, blockID, txID string) map[string]interface{} {
	blockInt, _ := utils.HexToInt(log.BlockNumber)
	return map[string]interface{}{
		"address":          log.Address,
		"topics":           log.Topics,
		"data":             log.Data,
		"blockNumber":      blockInt,
		"transactionHash":  log.TransactionHash,
		"transactionIndex": log.TransactionIndex,
		"blockHash":        log.BlockHash,
		"logIndex":         log.LogIndex,
		"removed":          fmt.Sprintf("%v", log.Removed),
		"transaction":      txID,
		"block":            blockID,
	}
}

// buildAccessListEntryDataMap creates a data map for access list entry creation
func (h *BlockHandler) buildAccessListEntryDataMap(ale *types.AccessListEntry, txID string) map[string]interface{} {
	return map[string]interface{}{
		"address":     ale.Address,
		"storageKeys": ale.StorageKeys,
		"transaction": txID,
	}
}