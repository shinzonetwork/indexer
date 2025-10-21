# Integration Tests

This directory contains two types of integration tests for the Shinzo Network blockchain indexer:

## 1. Mock Data Integration Tests (`integration/`)

**Purpose**: Fast, deterministic tests using controlled mock data
**Location**: `integration/integration_test.go`

### Features:
- ✅ Self-contained with embedded DefraDB
- ✅ Uses predictable mock blockchain data
- ✅ No external dependencies (no real Ethereum connections)
- ✅ Fast execution (~2 seconds)
- ✅ Deterministic results

### Usage:
```bash
# Run mock data integration tests
cd ~/Developer/shinzo/new/new3
go test -v ./integration/

# Expected output:
# TestGetHighestBlockNumber - PASS
# TestGetLatestBlocks - PASS  
# TestGetBlockWithTransactions - PASS
# TestGraphQLConnection - PASS
```

### What it tests:
- DefraDB schema application
- GraphQL query functionality
- Block and transaction data structure
- Database relationships

---

## 2. Live Integration Tests (`integration/live/`)

**Purpose**: End-to-end tests with real Ethereum blockchain data
**Location**: `integration/live/live_integration_test.go`

### Features:
- 🌐 Connects to real GCP managed Ethereum nodes
- 📊 Tests with live blockchain data
- 🔄 Validates complete indexing pipeline
- ⏱️ Longer execution time (~5+ minutes)
- 🎯 Tests production-like scenarios

### Prerequisites:
1. **GCP Managed Blockchain Node**: You need access to a GCP managed Ethereum node
2. **Environment Variables**: Set the following environment variables:

```bash
# Copy the template and fill in your values
cp integration/live/test_config_template.env .env

# Edit .env with your GCP values:
GCP_RPC_URL=https://json-rpc.YOUR_PROJECT.blockchainnodeengine.com
GCP_WS_URL=wss://ws.YOUR_PROJECT.blockchainnodeengine.com  
GCP_API_KEY=your-gcp-api-key-here
```

### Usage:

**Important**: Live tests use build tags and are excluded from default test runs.

```bash
# Set environment variables first (see Prerequisites)
source .env

# Run live integration tests with build tag
cd ~/Developer/shinzo/new/new3
go test -tags live -v ./integration/live/ -timeout=10m

# Note: Without -tags live, these tests are skipped
go test -v ./integration/live/  # Will skip live tests

# Expected output:
# TestLiveEthereumConnection - PASS
# TestLiveGetLatestBlocks - PASS
# TestLiveGetTransactions - PASS  
# TestLiveBlockTransactionRelationship - PASS
# TestLiveIndexerPerformance - PASS
```

**Build Tags**: Live tests require the `live` build tag to compile and run. This prevents them from running during normal `go test ./...` commands and allows selective execution.

### What it tests:
- Real Ethereum node connectivity (WebSocket + HTTP)
- Live block and transaction indexing
- DefraDB storage with real data
- GraphQL queries against live data
- Indexer performance metrics
- Block-transaction relationships

---

## Test Architecture

### Mock Integration Tests Flow:
```
1. Clean DefraDB data directory (./integration/.defra)
2. Start embedded DefraDB (port 9181)
3. Apply GraphQL schema
4. Insert controlled mock data
5. Run GraphQL query tests
6. Cleanup
```

### Live Integration Tests Flow:
```
1. Check environment variables (GCP credentials)
2. Clean DefraDB data directory (./integration/.defra)  
3. Start indexer with real Ethereum connections (port 9181)
4. Wait for DefraDB readiness
5. Wait for live blocks to be indexed
6. Run tests against live data
7. Cleanup
```

## Port Usage

- **Mock Tests**: DefraDB on port `9181`
- **Live Tests**: DefraDB on port `9181` 
- **Production**: DefraDB on port `9181` (configurable)

This separation prevents port conflicts when running both test suites.

## Troubleshooting

### Mock Tests Failing:
- Check if port 9181 is available
- Ensure no other DefraDB instances are running
- Check file permissions for `./integration/.defra` directory

### Live Tests Failing:
- Verify GCP environment variables are set correctly
- Check GCP API key permissions
- Ensure network connectivity to GCP blockchain nodes
- Verify WebSocket connections are allowed through firewall
- Check if port 9181 is available

### Common Issues:
- **Port conflicts**: Use `lsof -i :9181` or `lsof -i :9181` to check port usage
- **Permission errors**: Ensure write permissions for DefraDB data directories
- **Timeout errors**: Live tests may take longer depending on network conditions

## Development Workflow

1. **During development**: Use mock integration tests for fast feedback
   ```bash
   go test -v ./integration/  # Fast mock tests
   ```

2. **Before deployment**: Run live integration tests to validate end-to-end functionality
   ```bash
   go test -tags live -v ./integration/live/ -timeout=10m
   ```

3. **CI/CD**: 
   - Mock tests in PR validation: `go test ./...` (excludes live tests)
   - Live tests in deployment pipeline: `go test -tags live ./integration/live`

4. **Debugging**: Use live tests to reproduce production issues with real data


# Live Integration Test Configuration Template

# Copy this to .env in the root directory and fill in your GCP values

# GCP Managed Blockchain Node Configuration
GCP_RPC_URL=
GCP_WS_URL=
GCP_API_KEY=

# DefraDB Configuration for Live Tests
DEFRADB_HOST=localhost
DEFRADB_PORT=9181

# Logger Configuration
LOGGER_DEBUG=true