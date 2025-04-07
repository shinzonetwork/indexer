package types

// Block represents an Ethereum block
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
	Events           []Event       `json:"events"`
}

// Transaction represents an Ethereum transaction
type Transaction struct {
	Hash             string  `json:"hash"`
	From             string  `json:"from"`
	To               string  `json:"to"`
	Value            string  `json:"value"`
	Gas              string  `json:"gas"`
	GasPrice         string  `json:"gasPrice"`
	Input            string  `json:"input"`
	Nonce            string  `json:"nonce"`
	TransactionIndex string  `json:"transactionIndex"`
	BlockHash        string  `json:"blockHash"`
	BlockNumber      string  `json:"blockNumber"`
	Status           string  `json:"status"`
	Logs             []Log   `json:"logs"`
	Events           []Event `json:"events"`
	Block            Block   `json:"block"`
}

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

// Log represents an Ethereum log entry
type Log struct {
	Address          string      `json:"address"`
	Topics           []string    `json:"topics"`
	Data             string      `json:"data"`
	BlockNumber      string      `json:"blockNumber"`
	TransactionHash  string      `json:"transactionHash"`
	TransactionIndex string      `json:"transactionIndex"`
	BlockHash        string      `json:"blockHash"`
	LogIndex         string      `json:"logIndex"`
	Removed          bool        `json:"removed"`
	Block            Block       `json:"block"`
	Transaction      Transaction `json:"transaction"`
}

// Event represents a decoded Ethereum event
type Event struct {
	Address          string      `json:"address"`
	Topics           []string    `json:"topics"`
	Data             string      `json:"data"`
	BlockNumber      string      `json:"blockNumber"`
	TransactionHash  string      `json:"transactionHash"`
	TransactionIndex string      `json:"transactionIndex"`
	BlockHash        string      `json:"blockHash"`
	LogIndex         string      `json:"logIndex"`
	Name             string      `json:"name"`
	Args             string      `json:"args"`
	Removed          bool        `json:"removed"`
	Block            Block       `json:"block"`
	Transaction      Transaction `json:"transaction"`
}
