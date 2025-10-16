# Shinzo Network - Windsurf Rules

## Project: Blockchain Indexer for Ethereum → DefraDB

### Core Architecture
- **EthereumClient**: Dual WebSocket/HTTP connections to GCP managed blockchain nodes
- **DefraDB**: Decentralized database storage with GraphQL schema
- **Pipeline**: Blocks → Transactions → Logs processing with configurable workers

### Critical Rules

#### 1. DefraDB Relationships
```graphql
// Schema: Use @relation(name: "relationship_name")
type Block {
  transactions: [Transaction] @relation(name: "block_transactions")
}
// Mutations: Use field names (not relation names)
update_Transaction(input: {block: "docID"})  // ✓ Use 'block'
```

#### 2. EthereumClient Usage
```go
// Always use full constructor signature
client, err := rpc.NewEthereumClient(httpURL, wsURL, apiKey)
// Client automatically prefers WebSocket over HTTP
```

#### 3. Logging Standards
```go
// ✓ Structured logging
logger.Sugar.With("block", blockNum).Info("Processing block")
// ✗ Avoid concatenated logging
logger.Sugar.Info("Processing block", "block", blockNum)
```

#### 4. Error Handling
- **Transaction Types**: Use blocks 2-3 behind current head to avoid "transaction type not supported"
- **Connection Failures**: WebSocket fails → HTTP fallback
- **DocID**: Never pre-generate, let DefraDB handle automatically

#### 5. GCP Configuration
```yaml
geth:
  node_url: "https://json-rpc.*.blockchainnodeengine.com"
  ws_url: "wss://ws.*.blockchainnodeengine.com" 
  api_key: "X-goog-api-key header value"
```

#### 6. Testing Requirements
- Initialize logger in TestMain: `logger.Init(true)`
- Use full constructor: `NewEthereumClient(url, "", "")`
- No log buffer assertions (use stdout)

### Current Status
- ✅ GCP managed blockchain node integration
- ✅ Dual WebSocket/HTTP connections with API key auth
- ✅ DefraDB relationships working
- ✅ Transaction type error handling
- ✅ Structured logging implementation
- ❌ Event processing removed (ABI calls eliminated)

### Key Files
- `pkg/rpc/ethereum_client.go` - Blockchain connections
- `pkg/defra/` - DefraDB integration  
- `config/config.go` - Configuration structure
- `cmd/block_poster/main.go` - Main indexer application
