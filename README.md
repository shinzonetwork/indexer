# Shinzo Network Blockchain Indexer

A high-performance blockchain indexing solution built with Source Network, DefraDB, and LensVM.

## Architecture

- **GoLang**: High-performance indexing engine with concurrent processing
- **DefraDB**: Decentralized P2P datastore for blockchain data storage and querying
- **GCP Managed Blockchain Node**: Dual WebSocket/HTTP connections to Google Cloud managed Ethereum nodes
- **Uber Zap**: Structured logging with global logger integration
- **GraphQL**: Flexible query interface for indexed blockchain data
- **Viper Configuration**: YAML-based configuration with environment variable overrides

### Recent Improvements

- **Enhanced Error Handling**: Production-ready error system with categorization, retry logic, and structured context
- **Logger Stabilization**: Global logger initialization with proper test support and no file dependencies
- **Schema Updates**: Removed Event entity, added AccessListEntry support for EIP-2930 transactions
- **Test Suite Fixes**: Resolved all logger-related panics and GraphQL response parsing issues
- **Smart Retry Logic**: Intelligent retry behavior based on error types and severity

## Features

- Real-time blockchain data indexing
- GraphQL API for querying indexed data
- Support for blocks, transactions, logs, and access list entries
- Bi-directional relationships between blockchain entities
- Deterministic document IDs
- Graceful shutdown handling
- Concurrent indexing with duplicate block protection
- **Comprehensive Error Handling System**:
  - Categorized error types (Network, Data, Storage, System)
  - Severity levels (Info, Warning, Error, Critical)
  - Smart retry logic with exponential backoff
  - Structured error context and logging
  - Production-ready monitoring and alerting
- Global logger integration with Uber Zap
- Context-aware error reporting with block numbers and transaction hashes

## Prerequisites

- Go 1.20+
- [DefraDB](https://github.com/sourcenetwork/defradb) (for local development)
- GCP Managed Blockchain Node (Ethereum) with API access
- Node.js v23.10.0 (for DefraDB playground)

## Prerequisite Setup

1. **Install DefraDB** (for local development):
   ```bash
   git clone https://github.com/sourcenetwork/defradb
   cd defradb
   echo "v23.10.0" > .nvmrc
   ```

2. **Set up GCP Managed Blockchain Node**:
   - Create an Ethereum node in Google Cloud Blockchain Node Engine
   - Note the JSON-RPC and WebSocket endpoints
   - Configure API key authentication if required


## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/shinzonetwork/indexer.git
   cd indexer
   ```

2. Install Go dependencies:
   ```bash
   go mod download
   ```

3. Create environment variables in `.env`:
   ```bash
   # GCP Managed Blockchain Node Configuration
   GCP_GETH_RPC_URL=https://json-rpc.*.blockchainnodeengine.com
   GCP_GETH_WS_URL=wss://ws.*.blockchainnodeengine.com
   GCP_GETH_API_KEY=your-x-goog-api-key-header-value
   
   # DefraDB Configuration
   DEFRADB_URL=http://localhost:9181
   DEFRADB_KEYRING_SECRET=your-keyring-secret
   DEFRADB_PLAYGROUND=true
   DEFRADB_P2P_ENABLED=true
   DEFRADB_P2P_BOOTSTRAP_PEERS=[]
   DEFRADB_P2P_LISTEN_ADDR="/ip4/127.0.0.1/tcp/9171"
   
   # Indexer Configuration
   INDEXER_START_HEIGHT=23000000
   
   # Logger Configuration
   LOGGER_DEBUG=true
   ```

## Configuration

1. Configure DefraDB schema:
   - GraphQL schema files are located in `schema/`
   - Main schema defines relationships between blocks, transactions, logs, and events
   - Each entity has its own schema file in `schema/types/blockchain/`

2. The configuration uses `config/config.yaml` with environment variable overrides:
   ```yaml
   # Default configuration - environment variables will override these
   defradb:
     url: ""  # Empty = embedded DefraDB
     keyring_secret: ""
     playground: true
     p2p:
       enabled: true
       bootstrap_peers: []
       listen_addr: "/ip4/127.0.0.1/tcp/9171"
     store:
       path: "./.defra"
   
   geth:
     node_url: "<your-geth-node-url>"  # GCP managed blockchain node
     ws_url: "<your-geth-ws-url>"
     api_key: "<your-geth-api-key>"    # Recommend using a .env file
   
   indexer:
     start_height: 23000000
   logger:
     development: true
   ```

## How to Run

### Using Makefile (Recommended)
```bash
# Build the indexer
make build

# Start the indexer
make start

# Or build and run in one step
go run cmd/block_poster/main.go
```

### Using DefraDB Playground 
TODO: update to be config var
```bash
# Start with playground (requires DefraDB repo path)
make playground DEFRA_PATH=/path/to/defradb

# Set DEFRA_PATH as environment variable to avoid passing it each time
export DEFRA_PATH=/path/to/defradb
make playground
```

### Manual Build
```bash
# Build binary
go build -o bin/block_poster cmd/block_poster/main.go

# Run binary
./bin/block_poster
```

### Available Makefile Targets
- `make build` - Build the indexer binary
- `make test` - Run all tests with summary
- `make geth-status` - Check GCP Geth node connectivity
- `make start` - Start the indexer
- `make stop` - Stop all services
- `make help` - Show all available targets

## Testing

### Unit Tests
To run unit tests, you can run `make test` or simply `go test ./...` per standard go.

### Integration Tests
To run the integration tests, you'll want to run
`make integration-test`
This runs `make bootstrap` under the hood, so you'll want to provide `DEFRA_PATH=/path/to/defradb` as an argument or set it as an environment variable (as above). After the `make integration-test` script bootstraps the infra in your local environment, it will run the integration test suite, and then finally teardown the infra.

### Live Integration Tests with GCP Endpoint
To run live integration tests with your GCP managed blockchain node:

1. **Set up environment variables**:
   ```bash
   export GCP_GETH_RPC_URL=https://json-rpc.*.blockchainnodeengine.com
   export GCP_GETH_WS_URL=wss://ws.*.blockchainnodeengine.com
   export GCP_GETH_API_KEY=your-x-goog-api-key-header-value
   ```

   b: **Use a .env file for environment variables**
   ```bash
   touch .env
   #GCP Managed Node
export GCP_GETH_API_KEY=AIzaSyChwEoj24VGkyItUPd9vQV5mC8w9Vi0mg8
export GCP_GETH_RPC_URL=https://json-rpc.che8qim8flet1lfjpapfmtl42.blockchainnodeengine.com
export GCP_GETH_WS_URL=ws://ws.che8qim8flet1lfjpapfmtl42.blockchainnodeengine.com


#GCP Geth node
GCP_GETH_API_KEY=0EPWxZDg6O743gGkHK7yqsNEOzKUkh1TtHBYeFaWUFY
GCP_GETH_RPC_URL=http://34.68.131.15:8545
GCP_GETH_WS_URL=ws://34.68.131.15:8546

LOGGER_DEBUG=false
DEFRADB_PLAYGROUND=true
DEFRADB_URL=http://localhost:9181 | empty for embedded defradb
DEFRADB_KEYRING_SECRET=<your-defradb-keyring-secret>
DEFRADB_P2P_ENABLED=true
DEFRADB_P2P_BOOTSRAP_PEERS=[]
DEFRADB_P2P_LISTEN_ADDR=""
DEFRADB_STORE_PATH=<your-defradb-store-path>
DEFRA_PATH=<your-defradb-path>

START_HEIGHT=<your-start-height-number>
   ```

2. **Test GCP node connectivity**:
   ```bash
   make geth-status
   ```

3. **Run local tests**:
   ```bash
   ./test_local.sh
   ```
   
   Or use the Makefile target:
   ```bash
   make test-local
   ```

This will run the indexer tests locally with your GCP managed blockchain node, providing comprehensive diagnostics and avoiding public node limitations.

## Data Model

### Entities and Relationships
- **Block**
  - Primary key: `hash` (unique index)
  - Secondary index: `number`
  - Has many transactions (`block_transactions`)
- **Transaction**
  - Primary key: `hash` (unique index)
  - Secondary indices: `blockHash`, `blockNumber`
  - Belongs to block (`block_transactions`)
  - Has many logs (`transaction_logs`)
- **Log**
  - Indices: `blockNumber`
  - Belongs to block and transaction
- **AccessListEntry**
  - Belongs to transaction
  - Contains address and storage keys

**Note**: The Event entity was removed from the schema in recent updates.

### Relationship Definitions

DefraDB relationships use the `@relation(name: "relationship_name")` syntax. Example:

```graphql
type Block {
  transactions: [Transaction] @relation(name: "block_transactions")
}

type Transaction {
  block: Block @relation(name: "block_transactions")
}
```

## Error Handling System

Shinzo implements a comprehensive error handling system designed for production-ready distributed blockchain indexing:

### Error Types
- **NetworkError**: RPC/HTTP communication issues (often retryable)
- **DataError**: Parsing/validation failures (usually non-retryable)
- **StorageError**: Database operation failures (sometimes retryable)
- **SystemError**: Critical system-level failures (requires attention)

### Severity Levels
- **Info**: Informational messages
- **Warning**: Issues that don't stop processing
- **Error**: Significant issues requiring attention
- **Critical**: Severe issues that may require immediate action

### Retry Behavior
- **NonRetryable**: Errors that should not be retried (e.g., data validation)
- **Retryable**: Simple retry without backoff
- **RetryableWithBackoff**: Exponential backoff retry for network issues

### Structured Context
All errors include rich context:
- Component and operation names
- Block numbers and transaction hashes when applicable
- Timestamps and error codes
- Underlying error chains
- Metadata for debugging

### Smart Retry Logic
The system implements intelligent retry logic:
```go
if errors.IsRetryable(err) {
    retryDelay := errors.GetRetryDelay(err, retryAttempts)
    time.Sleep(retryDelay)
    // Retry operation
}
```

### Logging Strategy

The indexer uses Uber Zap for structured logging:
- Global logger initialization via `logger.Init()` | `logger.InitConsoleOnly()` | `logger.InitWithFiles()`
- Context-aware error logging with `logger.LogError()`
- Structured fields for monitoring and debugging
- Different log levels based on error severity
- No file output during tests (stdout/stderr only)

### Querying Data

Access indexed data through DefraDB's GraphQL API at `http://localhost:9181/api/v0/graphql`

Example query:
```graphql
{
  Block(filter: { number: { _eq: 18100003 } }) {
    hash
    number
    transactions {
      hash
      value
      gasPrice
      accessList {
        address
        storageKeys
      }
      logs {
        logIndex
        data
        address
        topics
      }
    }
  }
}
```

## Documentation Links

- [DefraDB Documentation](https://github.com/sourcenetwork/defradb)
- [Source Network Documentation](https://docs.sourcenetwork.io)
- [Geth Documentation](https://geth.ethereum.org/docs/)

## Development Status

### ✅ Completed (Phase 1)
- **Error System**: Comprehensive IndexerError system with NetworkError, DataError, StorageError, and SystemError types
- **Logger Integration**: Global Zap logger with structured logging and test compatibility
- **Main Application**: Smart retry logic and structured error handling in block_poster
- **Test Stability**: All test suites pass without logger panics or GraphQL parsing issues

### 🔧 In Progress (Phase 2)
- **RPC Client**: Enhanced network error handling with timeout and retry logic
- **Type Conversions**: Improved data parsing error reporting

### 📋 Planned (Phase 3)
- **Advanced Logging**: Additional helper functions for error analysis
- **Monitoring Integration**: Metrics and alerting based on error codes
- **Performance Optimizations**: Error handling performance improvements

### Key Benefits Achieved
1. **Operational Excellence**: Clear retry guidance for different error types
2. **Observability**: Structured error logging with context and codes
3. **Debugging**: Rich context (block numbers, tx hashes, metadata)
4. **Reliability**: Smart retry logic based on error characteristics
5. **Maintainability**: Consistent error handling patterns across codebase

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines
- Use the new error system for all error handling
- Follow structured logging patterns with context
- Include retry logic based on error types
- Write tests that work with the global logger
- Update ERROR_HANDLING_AUDIT.md for significant changes

## License

This project is licensed under the MIT License - see the LICENSE file for details.
