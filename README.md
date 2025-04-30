# Shinzo Network Blockchain Indexer

A high-performance blockchain indexing solution built with Source Network, DefraDB, and LensVM.

## Architecture

- **GoLang**: Indexing engine for storing block data at source
- **DefraDB**: P2P datastore for blockchain data storage and querying

## Features

- Real-time blockchain data indexing
- GraphQL API for querying indexed data
- Support for blocks, transactions, logs, and events
- Bi-directional relationships between blockchain entities
- Deterministic document IDs
- Graceful shutdown handling
- Concurrent indexing with duplicate block protection
- Structured logging with clear error reporting

## Prerequisites

- Go 1.20+
- [DefraDB](https://github.com/sourcenetwork/defradb)
- [Source Network CLI](https://docs.sourcenetwork.io/cli)
- [Alchemy API Key](https://www.alchemy.com/docs)

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

3. Set up environment variables in `.env`:
   ```bash
   ALCHEMY_API_KEY=your_api_key
   DEFRA_URL=http://localhost:9181  # Default DefraDB URL
   ```

4. Install DefraDB:
   ```bash
   go install github.com/sourcenetwork/defradb/cmd/defradb@latest
   ```

## Configuration

1. Configure DefraDB schema:
   - GraphQL schema files are located in `schema/`
   - Main schema defines relationships between blocks, transactions, logs, and events
   - Each entity has its own schema file in `schema/types/blockchain/`

2. Update `config/config.yaml` with your settings:
   ```yaml
   alchemy:
     api_key: ${ALCHEMY_API_KEY}
   defra:
     url: ${DEFRA_URL}
   ```

## Running the Indexer

1. Start DefraDB:
   ```bash
   export $(cat .env) && ~/go/bin/defradb start --root-dir /path/to/version1/.defra/
   ```

2. Build and run the indexer:
   ```bash
   go build -o bin/block_poster cmd/block_poster/main.go
   ./bin/block_poster > logs/log.txt 1>&2   
   ```

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
  - Has many events (`log_events`)
- **Event**
  - Indices: `transactionHash`, `blockNumber`
  - Belongs to log (`log_events`)

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

### Logging Strategy

The indexer implements a structured logging strategy:
- INFO level: Important state changes and block processing
- ERROR level: Critical issues requiring attention
- Block number context in error messages
- Clear distinction between normal shutdown and errors
- Success logging for block processing progress

### Querying Data

Access indexed data through DefraDB's GraphQL API at `http://localhost:9181/api/v0/graphql`

Example query:
```graphql
{
  Block(filter: { number: { _eq: 18100003 } }) {
    hash
    transactions {
      hash
      logs {
        logIndex
        data
        events {
          eventName
          parameters
        }
      }
    }
  }
}
```

## Documentation Links

- [DefraDB Documentation](https://github.com/sourcenetwork/defradb)
- [Source Network Documentation](https://docs.sourcenetwork.io)
- [Alchemy API Documentation](https://docs.alchemy.com/reference/api-overview)

## Development

- Use `go run cmd/indexer/main.go` for development
- The indexer supports graceful shutdown via SIGINT (Ctrl+C)
- Logs are structured using zap logger

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
