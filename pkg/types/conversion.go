package types

import (
	"encoding/json"
	"fmt"
	"strings"

	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"

	"shinzo/version1/pkg/utils"
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
		blockNumber = big.NewInt(int64(receipt.BlockNumber.Uint64())).String()
	} else {
		blockNumber = "0"
	}

	return &TransactionReceipt{
		TransactionHash:   receipt.TxHash.Hex(),
		TransactionIndex:  big.NewInt(int64(receipt.TransactionIndex)).String(),
		BlockHash:         receipt.BlockHash.Hex(),
		BlockNumber:       blockNumber,
		From:              "", // Not available directly
		To:                "", // Not available directly
		CumulativeGasUsed: big.NewInt(int64(receipt.CumulativeGasUsed)).String(),
		GasUsed:           big.NewInt(int64(receipt.GasUsed)).String(),
		ContractAddress:   receipt.ContractAddress.Hex(),
		Logs:              logs,
		Status:            big.NewInt(int64(receipt.Status)).String(),
	}
}

// ExtractEventsFromLog extracts events from a log using ABI decoding
func ExtractEventsFromLog(l *gethtypes.Log, contractABIs map[string]*abi.ABI) []Event {
	var events []Event

	// Get the contract address
	contractAddr := l.Address.Hex()

	// Look up ABI for this contract
	contractABI, exists := contractABIs[strings.ToLower(contractAddr)]
	if !exists {
		// If no ABI found, try to extract basic event info from topics
		if len(l.Topics) > 0 {
			event := Event{
				ContractAddress:  contractAddr,
				EventName:        "UnknownEvent",
				Parameters:       fmt.Sprintf("topic0: %s, data: %s", l.Topics[0].Hex(), common.Bytes2Hex(l.Data)),
				TransactionHash:  l.TxHash.Hex(),
				BlockHash:        l.BlockHash.Hex(),
				BlockNumber:      big.NewInt(int64(l.BlockNumber)).String(),
				TransactionIndex: big.NewInt(int64(l.TxIndex)).String(),
				LogIndex:         big.NewInt(int64(l.Index)).String(),
			}
			events = append(events, event)
		}
		return events
	}

	// If we have topics, try to decode the event
	if len(l.Topics) > 0 {
		eventSig := l.Topics[0]

		// Find the event by signature
		for eventName, event := range contractABI.Events {
			if event.ID == eventSig {
				// Found matching event, decode it
				decodedEvent, err := decodeEvent(event, l)
				if err == nil {
					decodedEvent.ContractAddress = contractAddr
					decodedEvent.EventName = eventName
					decodedEvent.TransactionHash = l.TxHash.Hex()
					decodedEvent.BlockHash = l.BlockHash.Hex()
					decodedEvent.BlockNumber = big.NewInt(int64(l.BlockNumber)).String()
					decodedEvent.TransactionIndex = big.NewInt(int64(l.TxIndex)).String()
					decodedEvent.LogIndex = big.NewInt(int64(l.Index)).String()
					events = append(events, decodedEvent)
				}
				break
			}
		}
	}

	return events
}

// decodeEvent decodes an individual event from a log
func decodeEvent(eventABI abi.Event, l *gethtypes.Log) (Event, error) {
	var event Event

	// Prepare topics for decoding (excluding the first topic which is the event signature)
	topics := make([]common.Hash, len(l.Topics)-1)
	copy(topics, l.Topics[1:])

	// Decode the event
	values := make(map[string]interface{})
	err := eventABI.Inputs.UnpackIntoMap(values, l.Data)
	if err != nil {
		return event, fmt.Errorf("failed to unpack event data: %w", err)
	}

	// Handle indexed parameters from topics
	topicIndex := 0
	for _, input := range eventABI.Inputs {
		if input.Indexed && topicIndex < len(topics) {
			// For indexed parameters, we store the raw topic value
			// More complex decoding would require type-specific handling
			values[input.Name] = topics[topicIndex].Hex()
			topicIndex++
		}
	}

	// Convert values to JSON string for storage
	paramsJSON, err := json.Marshal(values)
	if err != nil {
		return event, fmt.Errorf("failed to marshal event parameters: %w", err)
	}

	event.Parameters = string(paramsJSON)
	return event, nil
}

// ConvertLogWithAutoABI converts a geth Log to local Log type with automatic ABI fetching
func ConvertLogWithAutoABI(l *gethtypes.Log) Log {
	topics := make([]string, len(l.Topics))
	for i, t := range l.Topics {
		topics[i] = t.Hex()
	}

	// Try to fetch ABI for this contract and decode events
	var events []Event
	contractAddr := strings.ToLower(l.Address.Hex())

	// Try to get ABI from Etherscan and decode events
	if parsedABI, err := utils.GetOrFetchABI(contractAddr); err == nil {
		// Create ABI map for this single contract
		contractABIs := map[string]*abi.ABI{
			contractAddr: parsedABI,
		}
		events = ExtractEventsFromLog(l, contractABIs)
	} else {
		// If ABI fetch fails, create basic event info from topics
		if len(l.Topics) > 0 {
			event := Event{
				ContractAddress:  l.Address.Hex(),
				EventName:        "UnknownEvent",
				Parameters:       fmt.Sprintf("topic0: %s, data: %s", l.Topics[0].Hex(), common.Bytes2Hex(l.Data)),
				TransactionHash:  l.TxHash.Hex(),
				BlockHash:        l.BlockHash.Hex(),
				BlockNumber:      big.NewInt(int64(l.BlockNumber)).String(),
				TransactionIndex: big.NewInt(int64(l.TxIndex)).String(),
				LogIndex:         big.NewInt(int64(l.Index)).String(),
			}
			events = append(events, event)
		}
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

// ConvertLog converts a geth Log to local Log type with automatic ABI fetching
func ConvertLog(l *gethtypes.Log) Log {
	// Try to get ABI for this contract address and decode events
	return ConvertLogWithAutoABI(l)
}

// ConvertLogWithABI converts a geth Log to local Log type with event extraction
func ConvertLogWithABI(l *gethtypes.Log, contractABIs map[string]*abi.ABI) Log {
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
