# Configuration Guide

The Shinzo Network Indexer uses a flexible configuration system with YAML files and environment variable overrides.

## Quick Start

1. **Copy the template files:**
   ```bash
   cp config/config.yaml.template config/config.yaml
   cp .env.template .env
   ```

2. **Edit `.env` with your actual values:**
   - Set your GCP Managed Blockchain Node URLs and API key
   - Configure DefraDB settings
   - Adjust indexer start height as needed

3. **Run the indexer:**
   ```bash
   make run
   ```

## Configuration Priority

Settings are applied in this order (highest priority first):

1. **Environment Variables** (`.env` file or system environment)
2. **YAML Configuration** (`config/config.yaml`)
3. **Application Defaults**

## Key Configuration Sections

### GCP Managed Blockchain Node
```bash
# Required for Ethereum connectivity
GCP_GETH_RPC_URL=https://json-rpc.YOUR_PROJECT.blockchainnodeengine.com
GCP_GETH_WS_URL=wss://ws.YOUR_PROJECT.blockchainnodeengine.com  
GCP_GETH_API_KEY=your-gcp-api-key-here
```

### DefraDB Configuration
```bash
# Leave empty for embedded DefraDB (recommended for development)
DEFRADB_URL=

# Required encryption secret
DEFRADB_KEYRING_SECRET=your-secure-keyring-secret-here

# Development features
DEFRADB_PLAYGROUND=true
```

### Indexer Settings
```bash
# Starting block height (adjust based on your needs)
INDEXER_START_HEIGHT=23000000
```

## Production Considerations

- Set `DEFRADB_PLAYGROUND=false` in production
- Use a strong, random `DEFRADB_KEYRING_SECRET`
- Set `LOGGER_DEBUG=false` for production logging
- Consider using external DefraDB for scalability

## Environment Variables Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `GCP_GETH_RPC_URL` | GCP blockchain node HTTP endpoint | Required |
| `GCP_GETH_WS_URL` | GCP blockchain node WebSocket endpoint | Required |
| `GCP_GETH_API_KEY` | GCP API authentication key | Required |
| `DEFRADB_URL` | External DefraDB URL (empty = embedded) | "" |
| `DEFRADB_KEYRING_SECRET` | DefraDB encryption secret | Required |
| `DEFRADB_PLAYGROUND` | Enable GraphQL playground | true |
| `INDEXER_START_HEIGHT` | Starting Ethereum block height | 23000000 |
| `LOGGER_DEBUG` | Enable debug logging | true |
