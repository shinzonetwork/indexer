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
- ✅ **Ephemeral DefraDB instances** using `t.TempDir()` for isolation

### Usage:
```bash
# Run mock data integration tests
cd <path-to-repo>
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

### Usage Patterns: Ephemeral DefraDB with `t.TempDir()`

The integration tests now use **ephemeral DefraDB instances** that are automatically created and cleaned up for each test. This ensures complete test isolation and eliminates cleanup issues.

#### **Basic Pattern:**
```go
func TestYourFeature(t *testing.T) {
    // Create ephemeral DefraDB instance for this test
    ephemeralIndexer := createEphemeralDefraDB(t)
    defer ephemeralIndexer.StopIndexing() // Cleanup when test completes
    
    // Start the ephemeral DefraDB instance
    go func() {
        err := ephemeralIndexer.StartIndexing(false) // false = start embedded DefraDB
        if err != nil {
            logger.Testf("Indexer failed as expected (no Ethereum connection): %v", err)
        }
    }()
    
    // Wait for DefraDB to be ready
    waitForDefraDBReady(t)
    
    // Your test logic here...
}
```

#### **Multiple Isolated Instances:**
```go
func TestMultipleInstances(t *testing.T) {
    // Each t.TempDir() call creates a unique directory
    tempDir1 := t.TempDir() // e.g., /tmp/TestName123/001
    tempDir2 := t.TempDir() // e.g., /tmp/TestName123/002  
    tempDir3 := t.TempDir() // e.g., /tmp/TestName123/003
    
    // All directories are automatically cleaned up when test completes
    // Note: All instances share port 9181 but use different data directories
}
```

#### **Key Benefits:**
- **🔄 Automatic Cleanup**: No manual path management needed
- **🏝️ Test Isolation**: Each test gets a fresh, isolated DefraDB instance  
- **🎲 Unique Directories**: Every `t.TempDir()` call creates a different directory
- **🧹 Self-Cleaning**: Temporary directories are removed when tests complete
- **⚡ Deterministic**: No more cleanup race conditions or leftover data

#### **Port Management with Ephemeral Instances:**
```go
func TestWithEphemeralPort(t *testing.T) {
    // Option 1: Use default port 9181 (recommended for sequential tests)
    ephemeralIndexer := createEphemeralDefraDB(t)
    defer ephemeralIndexer.StopIndexing()
    
    // Option 2: For parallel tests, consider using ephemeral port 0
    // This would require updating createEphemeralDefraDB to support dynamic ports
    // tempDir := t.TempDir()
    // cfg.DefraDB.Url = "http://localhost:0" // Let system assign available port
}
```

**Port Strategy:**
- **Sequential Tests**: Use fixed port `9181` with ephemeral data directories
- **Parallel Tests**: Each test gets unique data directory, shared port access
- **Future Enhancement**: Could support ephemeral ports (port `0`) for true parallel execution

#### **Helper Functions Available:**
- `createEphemeralDefraDB(t)` - Creates isolated DefraDB instance using `t.TempDir()`
- `waitForDefraDBReady(t)` - Waits for DefraDB to accept connections
- `insertMockDataToEphemeralDB(t)` - Inserts test data into ephemeral instance

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
nano integration/live/.env
```

paste within .env:
```bash
# Edit .env with your GCP values:
GCP_GETH_RPC_URL=http://xx.xx.xx.xx:port
GCP_GETH_WS_URL=wss://ws.xx.xx.xx:port  
GCP_GETH_API_KEY=your-gcp-api-key-here
```

### Usage:

**Important**: Live tests use build tags and are excluded from default test runs.

```bash
# Set environment variables first (see Prerequisites)
source .env

# Run live integration tests with build tag
cd <path-to-repo>/integration/live/
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

### Mock Integration Tests Flow (Ephemeral):
```
1. Create ephemeral DefraDB instance using t.TempDir()
2. Start embedded DefraDB in unique temporary directory (port 9181)
3. Apply GraphQL schema automatically
4. Insert controlled mock data per test
5. Run GraphQL query tests
6. Automatic cleanup when test completes
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

- **Mock Tests (Ephemeral)**: DefraDB on port `9181` with unique `t.TempDir()` data directories
- **Live Tests**: DefraDB on port `9181` 
- **Production**: DefraDB on port `9181` (configurable)

**Port Strategy Changes:**
- **Before**: Manual cleanup of shared data directory on port `9181`
- **After**: Ephemeral data directories with shared port `9181` access
- **Isolation**: Achieved through unique temporary directories, not separate ports
- **Future**: Could implement ephemeral ports (`localhost:0`) for true parallel test execution

This approach maintains port consistency while achieving test isolation through ephemeral data directories.

## Troubleshooting

### Mock Tests Failing:
- Check if port 9181 is available
- Ensure no other DefraDB instances are running
- **Ephemeral directories**: No manual cleanup needed - `t.TempDir()` handles this automatically
- Check system temp directory permissions (usually `/tmp/` on Unix systems)

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

## Migration to Ephemeral DefraDB

**Previous Approach** (Manual Cleanup):
```go
// Old way - manual cleanup with hardcoded paths and shared port
cleanupPaths := []string{"./integration/.defra"}
for _, path := range cleanupPaths {
    os.RemoveAll(path) // Manual cleanup
}
// All tests shared port 9181 and same data directory
```

**New Approach** (Ephemeral with `t.TempDir()`):
```go
// New way - automatic ephemeral instances with isolated data
ephemeralIndexer := createEphemeralDefraDB(t) // Uses t.TempDir() internally
defer ephemeralIndexer.StopIndexing()        // Automatic cleanup
// Each test gets unique data directory, shared port 9181
```

**Port Management Changes:**
- **Before**: Single shared data directory `./integration/.defra` on port `9181`
- **After**: Multiple ephemeral directories (e.g., `/tmp/TestName123/001`) on shared port `9181`
- **Isolation Method**: Changed from port separation to data directory separation
- **Cleanup**: Automatic via Go's `t.TempDir()` instead of manual `os.RemoveAll()`

**Benefits of Migration:**
- ✅ **No more cleanup race conditions**
- ✅ **Perfect test isolation** - each test gets fresh DefraDB
- ✅ **Automatic directory management** - Go handles temp directory lifecycle
- ✅ **Parallel test execution** - no shared state conflicts
- ✅ **Simplified test code** - no manual path management needed


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