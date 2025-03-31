package defra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type BlockHandler struct {
	defraURL string
	client   *http.Client
}

func NewBlockHandler(host string, port int) *BlockHandler {
	return &BlockHandler{
		defraURL: fmt.Sprintf("http://%s:%d/api/v0", host, port),
		client:   &http.Client{},
	}
}

type Block struct {
	Hash             string        `json:"hash"`
	Number           string        `json:"number"`
	Timestamp        string        `json:"timestamp"`
	ParentHash       string        `json:"parentHash"`
	Difficulty       string        `json:"difficulty"`
	GasUsed          string        `json:"gasUsed"`
	GasLimit         string        `json:"gasLimit"`
	Nonce            string        `json:"nonce"`
	Miner            string        `json:"miner"`
	Size             string        `json:"size"`
	StateRoot        string        `json:"stateRoot"`
	Sha3Uncles       string        `json:"sha3Uncles"`
	TransactionsRoot string        `json:"transactionsRoot"`
	ReceiptsRoot     string        `json:"receiptsRoot"`
	ExtraData        string        `json:"extraData"`
	Transactions     []Transaction `json:"transactions,omitempty"`
}

type Transaction struct {
	Hash             string `json:"hash"`
	BlockHash        string `json:"blockHash"`
	BlockNumber      string `json:"blockNumber"`
	From             string `json:"from"`
	To               string `json:"to"`
	Value            string `json:"value"`
	Gas              string `json:"gas"`
	GasPrice         string `json:"gasPrice"`
	Input            string `json:"input"`
	Nonce            string `json:"nonce"`
	TransactionIndex string `json:"transactionIndex"`
	Status           bool   `json:"status"`
	Logs             []Log  `json:"logs,omitempty"`
}

type Log struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	BlockNumber      string   `json:"blockNumber"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	BlockHash        string   `json:"blockHash"`
	LogIndex         string   `json:"logIndex"`
	Removed          bool     `json:"removed"`
	Events           []Event  `json:"events,omitempty"`
}

type Event struct {
	ContractAddress string `json:"contractAddress"`
	EventName       string `json:"eventName"`
	Parameters      string `json:"parameters"`
	TransactionHash string `json:"transactionHash"`
	BlockHash       string `json:"blockHash"`
	LogIndex        string `json:"logIndex"`
}

type Response struct {
	Data map[string][]struct {
		DocID string `json:"_docID"`
	} `json:"data"`
}

// PostBlock posts a block and its nested objects to DefraDB
func (h *BlockHandler) PostBlock(ctx context.Context, block *Block) (string, error) {
	// Create block first
	blockID, err := h.createBlock(ctx, block)
	if err != nil {
		return "", fmt.Errorf("failed to create block: %w", err)
	}

	// Process transactions
	for _, tx := range block.Transactions {
		// Create transaction
		txID, err := h.createTransaction(ctx, &tx)
		if err != nil {
			return "", fmt.Errorf("failed to create transaction: %w", err)
		}

		// Link transaction to block
		mutation := fmt.Sprintf(`mutation {
			update_Transaction(
				filter: {hash: {_eq: %q}},
				input: {block: %q}
			) {
				_docID
			}
		}`, tx.Hash, blockID)

		_, err = h.postGraphQL(ctx, mutation)
		if err != nil {
			return "", fmt.Errorf("failed to link transaction to block: %w", err)
		}

		// Process logs
		for _, log := range tx.Logs {
			_, err := h.createLog(ctx, &log)
			if err != nil {
				return "", fmt.Errorf("failed to create log: %w", err)
			}

			// Link log to transaction
			mutation := fmt.Sprintf(`mutation {
				update_Log(
					filter: {logIndex: {_eq: %q}},
					input: {transaction: %q}
				) {
					_docID
				}
			}`, log.LogIndex, txID)

			_, err = h.postGraphQL(ctx, mutation)
			if err != nil {
				return "", fmt.Errorf("failed to link log to transaction: %w", err)
			}
		}
	}

	return blockID, nil
}

func (h *BlockHandler) createBlock(ctx context.Context, block *Block) (string, error) {
	mutation := fmt.Sprintf(`mutation {
		create_Block(
			input: {
				difficulty: %q,
				extraData: %q,
				gasLimit: %q,
				gasUsed: %q,
				hash: %q,
				miner: %q,
				nonce: %q,
				number: %q,
				parentHash: %q,
				receiptsRoot: %q,
				size: %q,
				stateRoot: %q,
				timestamp: %q,
				transactionsRoot: %q,
				sha3Uncles: %q
			}
		) {
			_docID
		}
	}`, block.Difficulty, block.ExtraData, block.GasLimit, block.GasUsed,
		block.Hash, block.Miner, block.Nonce, block.Number,
		block.ParentHash, block.ReceiptsRoot, block.Size,
		block.StateRoot, block.Timestamp, block.TransactionsRoot,
		block.Sha3Uncles)

	resp, err := h.postGraphQL(ctx, mutation)
	if err != nil {
		return "", fmt.Errorf("failed to create block: %w", err)
	}

	var response Response
	if err := json.Unmarshal(resp, &response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	items, ok := response.Data["create_Block"]
	if !ok || len(items) == 0 {
		return "", fmt.Errorf("no document ID returned")
	}

	return items[0].DocID, nil
}

func (h *BlockHandler) createTransaction(ctx context.Context, tx *Transaction) (string, error) {
	mutation := fmt.Sprintf(`mutation {
		create_Transaction(
			input: {
				hash: %q,
				blockHash: %q,
				blockNumber: %q,
				from: %q,
				to: %q,
				value: %q,
				gasUsed: %q,
				gasPrice: %q,
				inputData: %q,
				nonce: %q,
				transactionIndex: %q
			}
		) {
			_docID
		}
	}`, tx.Hash, tx.BlockHash, tx.BlockNumber, tx.From, tx.To,
		tx.Value, tx.Gas, tx.GasPrice, tx.Input, tx.Nonce,
		tx.TransactionIndex)

	resp, err := h.postGraphQL(ctx, mutation)
	if err != nil {
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	var response Response
	if err := json.Unmarshal(resp, &response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	items, ok := response.Data["create_Transaction"]
	if !ok || len(items) == 0 {
		return "", fmt.Errorf("no document ID returned")
	}

	return items[0].DocID, nil
}

func (h *BlockHandler) createLog(ctx context.Context, log *Log) (string, error) {
	// Convert removed boolean to string
	removed := "false"
	if log.Removed {
		removed = "true"
	}

	mutation := fmt.Sprintf(`mutation {
		create_Log(
			input: {
				address: %q,
				topics: %q,
				data: %q,
				transactionHash: %q,
				blockHash: %q,
				logIndex: %q,
				removed: %q
			}
		) {
			_docID
		}
	}`, log.Address, log.Topics, log.Data,
		log.TransactionHash, log.BlockHash,
		log.LogIndex, removed)

	resp, err := h.postGraphQL(ctx, mutation)
	if err != nil {
		return "", fmt.Errorf("failed to create log: %w", err)
	}

	var response Response
	if err := json.Unmarshal(resp, &response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	items, ok := response.Data["create_Log"]
	if !ok || len(items) == 0 {
		return "", fmt.Errorf("no document ID returned")
	}

	return items[0].DocID, nil
}

func (h *BlockHandler) createEvent(ctx context.Context, event *Event) (string, error) {
	mutation := fmt.Sprintf(`mutation {
		create_Event(
			input: {
				contractAddress: %q,
				eventName: %q,
				parameters: %q,
				transactionHash: %q,
				blockHash: %q,
				logIndex: %q
			}
		) {
			_docID
		}
	}`, event.ContractAddress, event.EventName, event.Parameters,
		event.TransactionHash, event.BlockHash, event.LogIndex)

	resp, err := h.postGraphQL(ctx, mutation)
	if err != nil {
		return "", fmt.Errorf("failed to create event: %w", err)
	}

	var response Response
	if err := json.Unmarshal(resp, &response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	items, ok := response.Data["create_Event"]
	if !ok || len(items) == 0 {
		return "", fmt.Errorf("no document ID returned")
	}

	return items[0].DocID, nil
}

func (h *BlockHandler) updateTransactionRelationships(ctx context.Context, blockHash, txHash string) error {
	query := fmt.Sprintf(`query {
		Block(filter: {hash: {_eq: %q}}) {
			_docID
		}
	}`, blockHash)

	resp, err := h.postGraphQL(ctx, query)
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

	mutation := fmt.Sprintf(`mutation {
		update_Transaction(
			filter: {hash: {_eq: %q}},
			input: {block: %q}
		) {
			_docID
		}
	}`, txHash, blockResp.Data.Block[0].DocID)

	_, err = h.postGraphQL(ctx, mutation)
	if err != nil {
		return fmt.Errorf("failed to update transaction relationships: %w", err)
	}

	return nil
}

func (h *BlockHandler) updateLogRelationships(ctx context.Context, blockHash, txHash, logIndex string) error {
	query := fmt.Sprintf(`query {
		Block(filter: {hash: {_eq: %q}}) {
			_docID
		}
		Transaction(filter: {hash: {_eq: %q}}) {
			_docID
		}
	}`, blockHash, txHash)

	resp, err := h.postGraphQL(ctx, query)
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

	mutation := fmt.Sprintf(`mutation {
		update_Log(
			filter: {logIndex: {_eq: %q}},
			input: {
				block: %q,
				transaction: %q
			}
		) {
			_docID
		}
	}`, logIndex, idResp.Data.Block[0].DocID, idResp.Data.Transaction[0].DocID)

	_, err = h.postGraphQL(ctx, mutation)
	if err != nil {
		return fmt.Errorf("failed to update log relationships: %w", err)
	}

	return nil
}

func (h *BlockHandler) updateEventRelationships(ctx context.Context, logIndex, eventLogIndex string) error {
	query := fmt.Sprintf(`query {
		Log(filter: {logIndex: {_eq: %q}}) {
			_docID
		}
	}`, logIndex)

	resp, err := h.postGraphQL(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to get log ID: %w", err)
	}

	var logResp struct {
		Data struct {
			Log []struct {
				DocID string `json:"_docID"`
			}
		}
	}
	if err := json.Unmarshal(resp, &logResp); err != nil {
		return fmt.Errorf("failed to decode log response: %w", err)
	}

	if len(logResp.Data.Log) == 0 {
		return fmt.Errorf("log not found")
	}

	mutation := fmt.Sprintf(`mutation {
		update_Event(
			filter: {logIndex: {_eq: %q}},
			input: {log: %q}
		) {
			_docID
		}
	}`, eventLogIndex, logResp.Data.Log[0].DocID)

	_, err = h.postGraphQL(ctx, mutation)
	if err != nil {
		return fmt.Errorf("failed to update event relationships: %w", err)
	}

	return nil
}

func (h *BlockHandler) postToCollection(ctx context.Context, collection string, data map[string]interface{}) (string, error) {
	inputFields := []string{}
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
	log.Printf("Input fields: %s\n", strings.Join(inputFields, ", "))
	log.Printf("Collection: %s\n", collection)
	log.Printf("Mutation: %s\n", fmt.Sprintf(`mutation {
		create_%s(input: { %s }) {
			%s
		}
	}`, collection, strings.Join(inputFields, ", "), collection))
	log.Printf("Http: %s\n", h.defraURL)

	mutation := fmt.Sprintf(`mutation {
		create_%s(input: { %s }) {
			%s
		}
	}`, collection, strings.Join(inputFields, ", "), collection)

	resp, err := h.postGraphQL(ctx, mutation)
	if err != nil {
		return "", fmt.Errorf("failed to create %s: %w", collection, err)
	}

	var response Response
	if err := json.Unmarshal(resp, &response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	createField := fmt.Sprintf("create_%s", collection)
	items, ok := response.Data[createField]
	if !ok || len(items) == 0 {
		return "", fmt.Errorf("no document ID returned")
	}

	return items[0].DocID, nil
}

func (h *BlockHandler) postGraphQL(ctx context.Context, mutation string) ([]byte, error) {
	body := map[string]string{
		"query": mutation,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", h.defraURL+"/graphql", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("DefraDB Response: %s\n", string(respBody))

	return respBody, nil
}
