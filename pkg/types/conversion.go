package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

// ConvertBlock converts a geth Block to local Block type
func ConvertBlock(block *gethtypes.Block, txs []Transaction) *Block {
	if block == nil {
		return nil
	}
	return &Block{
		Hash:             block.Hash().Hex(),
		Number:           block.Number().String(),
		Timestamp:        big.NewInt(int64(block.Time())).String(),
		ParentHash:       block.ParentHash().Hex(),
		Difficulty:       block.Difficulty().String(),
		GasUsed:          big.NewInt(int64(block.GasUsed())).String(),
		GasLimit:         big.NewInt(int64(block.GasLimit())).String(),
		Nonce:            int(block.Nonce()),
		Miner:            block.Coinbase().Hex(),
		Size:             "", // Not available directly
		StateRoot:        block.Root().Hex(),
		Sha3Uncles:       block.UncleHash().Hex(),
		TransactionsRoot: block.TxHash().Hex(),
		ReceiptsRoot:     block.ReceiptHash().Hex(),
		ExtraData:        string(block.Extra()),
		Transactions:     txs,
	}
}

// ConvertTransaction converts a geth Transaction to local Transaction type
func ConvertTransaction(tx *gethtypes.Transaction, msgSender common.Address, block *gethtypes.Block, receipt *TransactionReceipt, status bool) Transaction {
	var to string
	if tx.To() != nil {
		to = tx.To().Hex()
	}

	var logs []Log
	if receipt != nil {
		logs = receipt.Logs
	}

	return Transaction{
		Hash:             tx.Hash().Hex(),
		BlockHash:        block.Hash().Hex(),
		BlockNumber:      block.Number().String(),
		From:             msgSender.Hex(),
		To:               to,
		Value:            tx.Value().String(),
		Gas:              big.NewInt(int64(tx.Gas())).String(),
		GasPrice:         tx.GasPrice().String(),
		Input:            string(common.Bytes2Hex(tx.Data())), // common.Bytes2Hex(tx.Data()) || "0x0",
		Nonce:            int(tx.Nonce()),
		TransactionIndex: int(tx.Nonce()), // Not available directly
		Status:           status,
		Logs:             logs,
	}
}

// ConvertReceipt converts a geth Receipt to local TransactionReceipt type
func ConvertReceipt(receipt *gethtypes.Receipt) *TransactionReceipt {
	if receipt == nil {
		return nil
	}
	// Convert logs if they exist
	logs := make([]Log, len(receipt.Logs))
	for i, l := range receipt.Logs {
		logs[i] = ConvertLog(l)
	}

	var blockNumber string
	if receipt.BlockNumber != nil {
		blockNumber = receipt.BlockNumber.String()
	} else {
		blockNumber = "0"
	}

	return &TransactionReceipt{
		TransactionHash:   receipt.TxHash.Hex(),
		TransactionIndex:  big.NewInt(int64(receipt.TransactionIndex)).String(),
		BlockHash:         receipt.BlockHash.Hex(),
		BlockNumber:       blockNumber,
		From:              "",                            // Not available directly
		To:                receipt.ContractAddress.Hex(), // contract address
		CumulativeGasUsed: big.NewInt(int64(receipt.CumulativeGasUsed)).String(),
		GasUsed:           big.NewInt(int64(receipt.GasUsed)).String(),
		ContractAddress:   receipt.ContractAddress.Hex(),
		Logs:              logs,
		Status:            big.NewInt(int64(receipt.Status)).String(),
	}
}

// ConvertLog converts a geth Log to local Log type
func ConvertLog(l *gethtypes.Log) Log {
	topics := make([]string, len(l.Topics))
	for i, t := range l.Topics {
		topics[i] = t.Hex()
	}

	return Log{
		Address:          l.Address.Hex(),
		Topics:           topics,
		Data:             common.Bytes2Hex(l.Data),
		BlockNumber:      big.NewInt(int64(l.BlockNumber)).String(),
		TransactionHash:  l.TxHash.Hex(),
		TransactionIndex: int(l.TxIndex),
		BlockHash:        l.BlockHash.Hex(),
		LogIndex:         int(l.Index),
		Removed:          l.Removed,
	}
}
