# Shinzo Network Blockchain Indexer

A high-performance blockchain indexing solution built with Source Network, DefraDB, and LensVM.

## Architecture

- **GoLang**: Indexing engine for storing block data at source
- **DefraDB**: P2P datastore for blockchain data storage and querying
- **Ethereum Go Client**: RPC connectivity to Ethereum nodes
- **Uber Zap**: High-performance structured logging
- **GraphQL**: Flexible query interface for blockchain data

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
- [DefraDB](https://github.com/sourcenetwork/defradb)
- [Source Network CLI](https://docs.sourcenetwork.io/cli)
- [Geth](https://geth.ethereum.org/docs/) 
  - either run locally or hosted.

## Prerequisit setup

- Install [DefraDB](https://github.com/sourcenetwork/defradb)
- Navigate to defradb
- Add a .nvmrc file with the node `v23.10.0`


## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/shinzonetwork/version1.git
   cd version1
   ```

2. Install Go dependencies:
   ```bash
   go mod download
   ```

3. Create environment variables in `.env`:
   ```bash
    DEFRA_KEYRING_SECRET=<DefraDB_SECRET> # DEFRA KEY RING PASSWORD
    VERSION=<VERSION> # verisoning 
    DEFRA_LOGGING_DEVELOPMENT=<DEFRA_LOGGING_DEVELOPMENT> # logging switch
    DEFRA_DEVELOPMENT=<DEFRA_DEVELOPMENT>
    DEFRA_P2P_ENABLED=<DEFRA_P2P_ENABLED> # p2p enable true required for prod
    RPC_URL=<RPC_URL> # RPC HTTP URL
    DEFRADB_URL=<DEFRADB_URL> # DefraDB HTTP URL
   ```

## Configuration

1. Configure DefraDB schema:
   - GraphQL schema files are located in `schema/`
   - Main schema defines relationships between blocks, transactions, logs, and events
   - Each entity has its own schema file in `schema/types/blockchain/`

2. Update `config.yaml` with your settings:
   ```yaml
   geth:
       node_url: "localhost:<PORT>" || "https://ethereum-rpc.publicnode.com"

   defra:
     url: ${DEFRA_URL}
   ```

## How to Run

`go build -o bin/indexer cmd/block_poster/main.go`
then
`./bin/indexer` // can optionally pass in `-defra-started=true` flag if you've already started a defra instance elsewhere

or
`go run cmd/block_poster/main.go` // again, can optionally pass in `-defra-started=true` flag if you've already started a defra instance elsewhere

or, to open the playground as well, use
`make playground DEFRA_PATH=/path/to/defradb` // this defra path should be the path to the defra repo cloned on your machine

To avoid passing the `DEFRA_PATH=/path/to/defradb` portion of the command, set `DEFRA_PATH` as an environment variable.

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
   export GCP_GETH_RPC_URL=http://your.gcp.ip:8545
   export GCP_GETH_WS_URL=ws://your.gcp.ip:8546  # optional
   export GCP_GETH_API_KEY=your-api-key-here     # optional
   ```

2. **Run using the local test script**:
   ```bash
   ./test_local.sh
   ```
   
   Or use the Makefile target:
   ```bash
   make test-local
   ```

This will run the indexer tests locally with your GCP endpoint instead of the public node, avoiding the port conflicts and endpoint issues that occur in GitHub Actions.

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
- Global logger initialization via `logger.Init()`
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
