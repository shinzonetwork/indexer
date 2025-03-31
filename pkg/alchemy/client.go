package alchemy

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
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
	Transactions     []Transaction `json:"transactions"`
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
}

type TransactionReceipt struct {
	Status  string `json:"status"`
	Logs    []Log  `json:"logs"`
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
}

type jsonRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int          `json:"id"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID int `json:"id"`
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) GetLatestBlockNumber(ctx context.Context) (*big.Int, error) {
	var response jsonRPCResponse
	err := c.doRPC(ctx, "eth_blockNumber", nil, &response)
	if err != nil {
		return nil, err
	}

	var blockNumber string
	if err := json.Unmarshal(response.Result, &blockNumber); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block number: %w", err)
	}

	number, err := hexutil.DecodeBig(blockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to decode block number: %w", err)
	}

	return number, nil
}

func (c *Client) GetBlockByNumber(ctx context.Context, blockNumber string) (*Block, error) {
	var response jsonRPCResponse
	err := c.doRPC(ctx, "eth_getBlockByNumber", []interface{}{blockNumber, true}, &response)
	if err != nil {
		return nil, err
	}

	var block Block
	if err := json.Unmarshal(response.Result, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return &block, nil
}

func (c *Client) GetTransactionReceipt(ctx context.Context, txHash string) (*TransactionReceipt, error) {
	var response jsonRPCResponse
	err := c.doRPC(ctx, "eth_getTransactionReceipt", []interface{}{txHash}, &response)
	if err != nil {
		return nil, err
	}

	var receipt TransactionReceipt
	if err := json.Unmarshal(response.Result, &receipt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal receipt: %w", err)
	}

	return &receipt, nil
}

func (c *Client) doRPC(ctx context.Context, method string, params []interface{}, response *jsonRPCResponse) error {
	request := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://eth-mainnet.g.alchemy.com/v2/"+c.apiKey, strings.NewReader(string(requestBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do request: %w", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != nil {
		return fmt.Errorf("RPC error: %s", response.Error.Message)
	}

	return nil
}
