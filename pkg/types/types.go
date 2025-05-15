package types

// TransactionReceipt represents an Ethereum transaction receipt
type TransactionReceipt struct {
	TransactionHash   string `json:"transactionHash"`
	TransactionIndex  string `json:"transactionIndex"`
	BlockHash         string `json:"blockHash"`
	BlockNumber       string `json:"blockNumber"`
	From              string `json:"from"`
	To                string `json:"to"`
	CumulativeGasUsed string `json:"cumulativeGasUsed"`
	GasUsed           string `json:"gasUsed"`
	ContractAddress   string `json:"contractAddress"`
	Logs              []Log  `json:"logs"`
	Status            string `json:"status"`
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
	Logs             []LogA `json:"logs,omitempty"`
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

type LogA struct {
	Address          string  `json:"address"`
	Topics           string  `json:"topics"`
	Data             string  `json:"data"`
	BlockNumber      string  `json:"blockNumber"`
	TransactionHash  string  `json:"transactionHash"`
	TransactionIndex string  `json:"transactionIndex"`
	BlockHash        string  `json:"blockHash"`
	LogIndex         string  `json:"logIndex"`
	Removed          bool    `json:"removed"`
	Events           []Event `json:"events,omitempty"`
}

type Event struct {
	ContractAddress  string `json:"contractAddress"`
	EventName        string `json:"eventName"`
	Parameters       string `json:"parameters"`
	TransactionHash  string `json:"transactionHash"`
	BlockHash        string `json:"blockHash"`
	BlockNumber      string `json:"blockNumber"`
	TransactionIndex string `json:"transactionIndex"`
	LogIndex         string `json:"logIndex"`
}

type Response struct {
	Data map[string][]struct {
		DocID string `json:"_docID"` // the document ID of the item in the collection
	} `json:"data"` // the data returned from the query
}

type Request struct {
	Type  string `json:"type"`
	Query string `json:"query"`
}

type Error struct {
	Level   int    `json:"level"`
	Message string `json:"message"`
}

type DefraDoc struct {
	JSON interface{} `json:"json"`
}

type UpdateTransactionStruct struct {
	BlockId string `json:"blockId"`
	TxHash  string `json:"txHash"`
}

type UpdateLogStruct struct {
	BlockId  string `json:"blockId"`
	TxId     string `json:"txId"`
	TxHash   string `json:"txHash"`
	LogIndex string `json:"logIndex"`
}

type UpdateEventStruct struct {
	LogIndex string `json:"logIndex"`
	TxHash   string `json:"txHash"`
	LogDocId string `json:"logDocId"`
}
