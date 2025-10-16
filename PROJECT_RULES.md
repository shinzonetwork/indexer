# Shinzo Network - Project Rules & Documentation

## Project Overview

**Shinzo Network** is a blockchain indexer system that fetches Ethereum blockchain data and stores it in DefraDB (a decentralized database). The system provides real-time blockchain data indexing with robust error handling and scalable architecture.

## Architecture Components

### 1. Blockchain Data Fetching (`pkg/rpc/`)
- **EthereumClient**: Handles connections to Ethereum nodes
- **Dual Connection Support**: WebSocket (primary) + HTTP (fallback)
- **GCP Integration**: Uses Google Cloud Platform managed blockchain nodes
- **Authentication**: Supports `X-goog-api-key` header authentication

### 2. Database Layer (`pkg/defra/`)
- **DefraDB Integration**: Decentralized database for blockchain data storage
- **GraphQL Schema**: Defines Block, Transaction, Log relationships
- **Relationship Management**: Proper DocID handling and relationship establishment

### 3. Data Processing Pipeline
- **Multi-stage Processing**: Blocks → Transactions → Logs
- **Configurable Workers**: Parallel processing with buffer management
- **Error Handling**: Graceful degradation and retry mechanisms

## Current Configuration

### GCP Managed Blockchain Node
```yaml
geth:
  node_url: "https://json.********.********.com"
  ws_url: "wss://ws.********.********.com"
  api_key: "****"
```

### DefraDB Configuration
```yaml
defradb:
  host: "localhost"
  port: 9181
  keyring_secret: ""
  store:
    path: "./data"
```

## Critical Development Rules

### 1. DefraDB Relationships
- **Schema Definition**: Use `@relation(name: "relationship_name")` syntax
- **Mutation Usage**: Use field names (not relation names) in mutations
- **DocID Handling**: Never pre-generate DocIDs - let DefraDB handle them
- **Example**:
  ```graphql
  type Block {
    transactions: [Transaction] @relation(name: "block_transactions")
  }
  type Transaction {
    block: Block @relation(name: "block_transactions")
  }
  ```

### 2. Logging Standards
- **Structured Logging**: Use zap.SugaredLogger with proper field separation
- **Log Levels**: INFO for state changes, ERROR for actual errors
- **Context**: Include block numbers and relevant identifiers
- **Format**: `Logger.With("key", "value").Info("message")` not `Logger.Info("message", "key", "value")`

### 3. Error Handling
- **Transaction Types**: Handle "transaction type not supported" gracefully
- **Retry Logic**: Use exponential backoff for RPC failures
- **Fallback Strategy**: HTTP client when WebSocket fails
- **Block Selection**: Use blocks 2-3 behind current head to avoid newest transaction types

### 4. Connection Management
- **Preferred Client**: WebSocket over HTTP when available
- **API Key Authentication**: Proper header handling for GCP nodes
- **Connection Cleanup**: Always close both HTTP and WebSocket clients
- **Graceful Degradation**: Continue operation if one connection type fails

## Data Flow Architecture

### Block Processing Pipeline
1. **Fetch Latest Block**: Get current blockchain head
2. **Block Validation**: Ensure block hasn't been processed
3. **Transaction Processing**: Extract and convert all transactions
4. **Log Processing**: Process transaction receipts and logs
5. **DefraDB Storage**: Store with proper relationships
6. **Progress Tracking**: Log successful processing

### Entity Relationships
```
Block (1) → (N) Transactions
Transaction (1) → (N) Logs
Log (1) → (N) Events (removed in current version)
```

## Testing Guidelines

### Test Structure
- **Global Logger**: Initialize `logger.Init(true)` in TestMain
- **Mock Servers**: Use httptest.Server for RPC testing
- **No Buffer Checks**: Don't test log buffer contents (use stdout)
### Test Coverage Areas
- Connection establishment (HTTP/WebSocket)
- API key authentication
- Block/transaction conversion
- Error handling and retry mechanisms
- API keys in configuration (not hardcoded)
- Secure connection handling
- Error message sanitization
- Rate limiting compliance

### Connection Strategy
1. **Primary**: WebSocket with API key for real-time data
2. **Fallback**: HTTP with API key for reliability
3. **Retry Logic**: Smart backoff for failed connections
4. **Block Selection**: Avoid newest blocks to prevent type errors

### Pipeline Tuning
- **Worker Pools**: Configurable parallel processing
- **Buffer Sizes**: Memory vs throughput balance
- **Batch Processing**: Efficient DefraDB operations
- **Polling Intervals**: Balance between real-time and resource usage

## Known Issues & Solutions

### Transaction Type Errors
- **Problem**: "transaction type not supported" for newest blocks
- **Solution**: Process blocks 2-3 behind current head
- **Fallback**: Retry with even older blocks if needed

### WebSocket Authentication
- **Problem**: go-ethereum doesn't support custom headers for WebSocket
- **Solution**: Use URL parameter approach for API key
- **Fallback**: HTTP connection with proper header authentication

### DefraDB Relationships
- **Problem**: Relationship syntax confusion
- **Solution**: Always use `@relation(name: "...")` in schema, field names in mutations
- **Validation**: Test relationship creation in development

## Development Workflow

### Code Changes
1. **Schema Updates**: Update GraphQL schema first
2. **Type Conversion**: Ensure proper Geth → DefraDB mapping
3. **Testing**: Run all test suites before deployment
4. **Logging**: Add appropriate log statements for debugging
5. **Error Handling**: Consider all failure modes

### Deployment Process
1. **Configuration Review**: Verify all connection parameters
2. **Database Migration**: Handle schema changes properly
3. **Connection Testing**: Verify both WebSocket and HTTP work
4. **Monitoring**: Watch logs for connection and processing errors
5. **Performance**: Monitor indexing speed and resource usage

## Future Enhancements

### Planned Features
- Real-time event streaming via WebSocket
- Enhanced error recovery mechanisms
- Performance metrics and monitoring
- Horizontal scaling capabilities
- Advanced filtering and querying

### Technical Debt
- Simplify connection management code
- Improve test coverage for edge cases
- Optimize DefraDB query performance
- Enhance configuration validation
