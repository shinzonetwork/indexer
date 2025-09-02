package rpc

import (
	"context"
	"fmt"
	"math/big"

	"github.com/shinzonetwork/indexer/pkg/errors"
	"github.com/shinzonetwork/indexer/pkg/logger"
	"github.com/shinzonetwork/indexer/pkg/types"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// EthereumClient wraps both JSON-RPC and fallback HTTP client
type EthereumClient struct {
	httpClient *ethclient.Client
	nodeURL    string
}

// NewEthereumClient creates a new JSON-RPC Ethereum client with HTTP fallback
func NewEthereumClient(httpNodeURL string) (*EthereumClient, error) {
	client := &EthereumClient{
		nodeURL: httpNodeURL,
	}

	// Always establish HTTP client as fallback
	if httpNodeURL != "" {
		httpClient, err := ethclient.Dial(httpNodeURL)
		if err != nil {
			return nil, errors.NewRPCConnectionFailed("rpc", "NewEthereumClient", httpNodeURL, err)
		}
		client.httpClient = httpClient
	}

	return client, nil
}

// GetLatestBlock fetches the latest block
func (c *EthereumClient) GetLatestBlock(ctx context.Context) (*types.Block, error) {
	// For now, use HTTP client (you can implement JSON-RPC here when needed)
	if c.httpClient == nil {
		return nil, fmt.Errorf("no HTTP client available")
	}

	// Get the latest block number first
	latestHeader, err := c.httpClient.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest header: %w", err)
	}

	var gethBlock *ethtypes.Block

	for retries := 0; retries < 3; retries++ {
		gethBlock, err = c.httpClient.BlockByNumber(ctx, latestHeader.Number)
		if err != nil {
			if retries < 2 && (err.Error() == "transaction type not supported" ||
				err.Error() == "invalid transaction type") {
				logger.Sugar.Warnf("Retry %d: Transaction type error, trying again...", retries+1)
				// Try a block that's 1 block behind
				latestHeader.Number = big.NewInt(1).Sub(latestHeader.Number, big.NewInt(1))
				continue
			}
			return nil, fmt.Errorf("failed to get latest block: %w", err)
		}
		break
	}

	return c.convertGethBlock(gethBlock), nil
}

// GetBlockByNumber fetches a block by number
func (c *EthereumClient) GetBlockByNumber(ctx context.Context, blockNumber *big.Int) (*types.Block, error) {
	if c.httpClient == nil {
		return nil, fmt.Errorf("no HTTP client available")
	}

	gethBlock, err := c.httpClient.BlockByNumber(ctx, blockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get block %v: %w", blockNumber, err)
	}

	return c.convertGethBlock(gethBlock), nil
}

// GetNetworkID returns the network ID
func (c *EthereumClient) GetNetworkID(ctx context.Context) (*big.Int, error) {
	if c.httpClient == nil {
		return nil, fmt.Errorf("no HTTP client available")
	}

	return c.httpClient.NetworkID(ctx)
}

// GetTransactionReceipt fetches a transaction receipt by hash
func (c *EthereumClient) GetTransactionReceipt(ctx context.Context, txHash string) (*types.TransactionReceipt, error) {
	if c.httpClient == nil {
		return nil, fmt.Errorf("no HTTP client available")
	}

	hash := common.HexToHash(txHash)
	receipt, err := c.httpClient.TransactionReceipt(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction receipt: %w", err)
	}
	return c.convertGethReceipt(receipt), nil
}

// convertGethReceipt converts go-ethereum receipt to our custom receipt type
func (c *EthereumClient) convertGethReceipt(receipt *ethtypes.Receipt) *types.TransactionReceipt {
	if receipt == nil {
		return nil
	}

	// Convert logs
	logs := make([]types.Log, len(receipt.Logs))
	for i, log := range receipt.Logs {
		logs[i] = c.convertGethLog(log)
	}

	return &types.TransactionReceipt{
		TransactionHash:   receipt.TxHash.Hex(),
		TransactionIndex:  fmt.Sprintf("%d", receipt.TransactionIndex),
		BlockHash:         receipt.BlockHash.Hex(),
		BlockNumber:       receipt.BlockNumber.String(),
		CumulativeGasUsed: fmt.Sprintf("%d", receipt.CumulativeGasUsed),
		GasUsed:           fmt.Sprintf("%d", receipt.GasUsed),
		ContractAddress:   getContractAddress(receipt),
		Logs:              logs,
		Status:            getReceiptStatus(receipt),
	}
}

// convertGethLog converts go-ethereum log to our custom log type
func (c *EthereumClient) convertGethLog(log *ethtypes.Log) types.Log {
	// Convert topics
	topics := make([]string, len(log.Topics))
	for i, topic := range log.Topics {
		topics[i] = topic.Hex()
	}

	return types.Log{
		Address:          log.Address.Hex(),
		Topics:           topics,
		Data:             common.Bytes2Hex(log.Data),
		BlockNumber:      fmt.Sprintf("%d", log.BlockNumber),
		TransactionHash:  log.TxHash.Hex(),
		TransactionIndex: int(log.TxIndex),
		BlockHash:        log.BlockHash.Hex(),
		LogIndex:         int(log.Index),
		Removed:          log.Removed,
	}
}

// Helper functions for receipt conversion
func getContractAddress(receipt *ethtypes.Receipt) string {
	if receipt.ContractAddress == (common.Address{}) {
		return ""
	}
	return receipt.ContractAddress.Hex()
}

func getReceiptStatus(receipt *ethtypes.Receipt) string {
	if receipt.Status == ethtypes.ReceiptStatusSuccessful {
		return "1"
	}
	return "0"
}

// convertGethBlock converts go-ethereum Block to our custom Block type
func (c *EthereumClient) convertGethBlock(gethBlock *ethtypes.Block) *types.Block {
	if gethBlock == nil {
		return nil
	}

	// Convert transactions
	transactions := make([]types.Transaction, 0, len(gethBlock.Transactions()))

	for i, tx := range gethBlock.Transactions() {
		// Skip transaction conversion if it fails (continue with others)
		logger.Sugar.Info("Transaction", tx)
		localTx, err := c.convertTransaction(tx, gethBlock, i)
		if err != nil {
			logger.Sugar.Warnf("Warning: Failed to convert transaction %s: %v", tx.Hash().Hex(), err)
			continue
		}

		transactions = append(transactions, *localTx)
	}

	// Convert uncles
	uncles := make([]string, len(gethBlock.Uncles()))
	for i, uncle := range gethBlock.Uncles() {
		uncles[i] = uncle.Hash().Hex()
	}

	// Convert the block
	return &types.Block{
		Hash:             gethBlock.Hash().Hex(),
		Number:           fmt.Sprintf("%d", gethBlock.NumberU64()),
		Timestamp:        fmt.Sprintf("%d", gethBlock.Time()),
		ParentHash:       gethBlock.ParentHash().Hex(),
		Difficulty:       gethBlock.Difficulty().String(),
		TotalDifficulty:  "", // Will be populated separately if needed
		GasUsed:          fmt.Sprintf("%d", gethBlock.GasUsed()),
		GasLimit:         fmt.Sprintf("%d", gethBlock.GasLimit()),
		BaseFeePerGas:    getBaseFeePerGas(gethBlock),
		Nonce:            int(gethBlock.Nonce()),
		Miner:            gethBlock.Coinbase().Hex(),
		Size:             fmt.Sprintf("%d", gethBlock.Size()),
		StateRoot:        gethBlock.Root().Hex(),
		Sha3Uncles:       gethBlock.UncleHash().Hex(),
		TransactionsRoot: gethBlock.TxHash().Hex(),
		ReceiptsRoot:     gethBlock.ReceiptHash().Hex(),
		LogsBloom:        common.Bytes2Hex(gethBlock.Bloom().Bytes()),
		ExtraData:        common.Bytes2Hex(gethBlock.Extra()),
		MixHash:          gethBlock.MixDigest().Hex(),
		Uncles:           uncles,
		Transactions:     transactions,
	}
}

// convertTransaction safely converts a single transaction
func (c *EthereumClient) convertTransaction(tx *ethtypes.Transaction, gethBlock *ethtypes.Block, index int) (*types.Transaction, error) {
	// Get transaction details with error handling
	fromAddr, err := getFromAddress(tx)
	if err != nil {
		return nil, fmt.Errorf("Failed to get from address from transaction: %v", err)
	}
	toAddr := getToAddress(tx)

	// Handle different transaction types
	var gasPrice *big.Int
	switch tx.Type() {
	case ethtypes.LegacyTxType, ethtypes.AccessListTxType:
		gasPrice = tx.GasPrice()
	case ethtypes.DynamicFeeTxType:
		// For EIP-1559 transactions, use effective gas price if available
		// Fall back to gas fee cap if not
		gasPrice = tx.GasFeeCap()
	default:
		// For unknown transaction types, try to get gas price
		// If it fails, we'll catch it in the calling function
		gasPrice = tx.GasPrice()
	}

	// Extract signature components
	v, r, s := tx.RawSignatureValues()

	// Get access list for EIP-2930/EIP-1559 transactions
	accessList := make([]types.AccessListEntry, 0)
	if tx.AccessList() != nil {
		for _, entry := range tx.AccessList() {
			storageKeys := make([]string, len(entry.StorageKeys))
			for i, key := range entry.StorageKeys {
				storageKeys[i] = key.Hex()
			}
			accessList = append(accessList, types.AccessListEntry{
				Address:     entry.Address.Hex(),
				StorageKeys: storageKeys,
			})
		}
	}

	localTx := types.Transaction{
		Hash:                 tx.Hash().Hex(),              // string
		BlockHash:            gethBlock.Hash().Hex(),       // string
		BlockNumber:          gethBlock.Number().String(),  // string
		From:                 fromAddr.Hex(),               // string
		To:                   toAddr,                       // string
		Value:                tx.Value().String(),          // string
		Gas:                  fmt.Sprintf("%d", tx.Gas()),  // string
		GasPrice:             gasPrice.String(),            // string
		MaxFeePerGas:         getMaxFeePerGas(tx),          // string
		MaxPriorityFeePerGas: getMaxPriorityFeePerGas(tx),  // string
		Input:                common.Bytes2Hex(tx.Data()),  // string
		Nonce:                int(tx.Nonce()),              // int
		TransactionIndex:     index,                        // int
		Type:                 fmt.Sprintf("%d", tx.Type()), // string
		ChainId:              getChainId(tx),               // string
		AccessList:           accessList,                   // []accessListEntry
		V:                    v.String(),                   // string
		R:                    r.String(),                   // string
		S:                    s.String(),                   // string
		Status:               true,                         // Default to true, will be updated from receipt
	}

	return &localTx, nil
}

// Helper functions for transaction conversion
func getFromAddress(tx *ethtypes.Transaction) (*common.Address, error) {
	chainId := tx.ChainId()
	if chainId == nil && chainId.Sign() <= 0 {
		return nil, fmt.Errorf("Received invalid chain id") // Otherwise, when we go to create a `modernSigner`, we will panic if these conditions are met
	}

	// Try different signers to handle various transaction types
	signers := []ethtypes.Signer{
		ethtypes.LatestSignerForChainID(tx.ChainId()),
		ethtypes.NewEIP155Signer(tx.ChainId()),
		ethtypes.NewLondonSigner(tx.ChainId()),
	}

	for _, signer := range signers {
		if from, err := ethtypes.Sender(signer, tx); err == nil {
			return &from, nil
		}
	}

	// If all signers fail, return zero address
	return &common.Address{}, nil
}

func getToAddress(tx *ethtypes.Transaction) string {
	if tx.To() == nil {
		return "" // Contract creation
	}
	return tx.To().Hex()
}

// getBaseFeePerGas extracts base fee from EIP-1559 blocks
func getBaseFeePerGas(block *ethtypes.Block) string {
	if block.BaseFee() == nil {
		return "" // Not an EIP-1559 block
	}
	return block.BaseFee().String()
}

// getMaxFeePerGas extracts max fee per gas from EIP-1559 transactions
func getMaxFeePerGas(tx *ethtypes.Transaction) string {
	if tx.Type() == ethtypes.DynamicFeeTxType {
		return tx.GasFeeCap().String()
	}
	return ""
}

// getMaxPriorityFeePerGas extracts max priority fee per gas from EIP-1559 transactions
func getMaxPriorityFeePerGas(tx *ethtypes.Transaction) string {
	if tx.Type() == ethtypes.DynamicFeeTxType {
		return tx.GasTipCap().String()
	}
	return ""
}

// getChainId extracts chain ID from transaction
func getChainId(tx *ethtypes.Transaction) string {
	if tx.ChainId() == nil {
		return ""
	}
	return tx.ChainId().String()
}

// Close closes the connections
func (c *EthereumClient) Close() error {
	var err error
	if c.httpClient != nil {
		c.httpClient.Close()
	}
	return err
}
