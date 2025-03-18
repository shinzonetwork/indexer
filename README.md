# Blockchain Indexer

A blockchain indexing solution built with Source Network, DefraDB, and CocoIndex.

## Architecture

- **Source Network**: Handles consensus and transaction management
- **DefraDB**: P2P datastore for blockchain data
- **CocoIndex**: ETL pipeline for data processing

## Prerequisites

- Go 1.20+
- Source Network CLI
- DefraDB
- CocoIndex

## Setup

1. Install dependencies
2. Configure DefraDB connection
3. Set up Source Network node
4. Initialize CocoIndex pipeline

## Configuration

Configuration files are located in the `config` directory.

## Development

```bash
# Start DefraDB
defra start

# Run Source Network node
source-cli start

# Start indexer
go run cmd/indexer/main.go
```

### Notes:
- Defra Run command `export $(cat .env) && ~/go/bin/defrad
b start`
