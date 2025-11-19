# Integration Tests

## Mock Tests
```bash
go test -v ./integration/
```
Fast tests with mock data. No external dependencies.

## Live Tests  
```bash
# Set environment variables first
source .env

# Run with build tag
make integration-test
```
End-to-end tests with real Ethereum data. Requires `GETH_RPC_URL`, `GETH_WS_URL`, `GETH_API_KEY`.


