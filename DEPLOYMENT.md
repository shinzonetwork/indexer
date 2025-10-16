# Shinzo Network Indexer - Deployment Guide

This guide covers deploying the Shinzo Network blockchain indexer on Ethereum validator infrastructure.

## 🏗️ Architecture

- **Shinzo Indexer**: Processes Ethereum blocks and stores in DefraDB
- **DefraDB**: Decentralized database for blockchain data storage
- **GCP Blockchain Node**: Managed Ethereum node with Erigon backend
- **Monitoring**: Prometheus/Grafana for observability

## 🚀 Deployment Options

### 1. Docker Compose (Development/Testing)

```bash
# Copy environment template
cp .env.docker .env
# Edit .env with your GCP credentials

# Build the image
make docker-build

# Run catch-up mode (initial sync)
make docker-up-catch-up

# Or run real-time mode
make docker-up-indexer

# View logs
make docker-logs

# Stop services
make docker-down
```

### 2. Systemd Services (Production)

For production deployment on validator infrastructure:

```bash
# Configure environment
cp .env.docker .env
# Edit .env with production values

# Deploy to production
make deploy

# Check service status
sudo systemctl status shinzo-indexer
sudo systemctl status shinzo-defradb

# View logs
sudo journalctl -u shinzo-indexer -f
sudo journalctl -u shinzo-defradb -f
```

## ⚙️ Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GCP_API_KEY` | GCP Blockchain Node API key | Required |
| `GCP_RPC_URL` | GCP RPC endpoint | Required |
| `GCP_WS_URL` | GCP WebSocket endpoint | Required |
| `DEFRADB_HOST` | DefraDB host | localhost |
| `DEFRADB_PORT` | DefraDB port | 9181 |
| `LOGGER_DEBUG` | Enable debug logging | false |
| `INDEXER_START_HEIGHT` | Starting block height | 1800000 |

### Security Configuration

The deployment follows validator infrastructure security best practices:

- **Non-root execution**: Services run as dedicated `shinzo` user
- **Filesystem isolation**: Read-only system, writable data directories only
- **Resource limits**: CPU, memory, and file descriptor limits
- **Network security**: Minimal port exposure
- **Secret management**: Environment-based configuration

## 📊 Monitoring

### Health Checks

- **DefraDB**: `curl -f http://localhost:9181/api/v0/graphql`
- **Indexer**: Monitor systemd service status and logs

### Prometheus Metrics

Configure Prometheus to scrape:
- DefraDB metrics: `localhost:9181/api/v0/metrics`
- System metrics: Node Exporter on `localhost:9100`

### Alerting Rules

Key alerts configured:
- Service downtime detection
- High memory/disk usage warnings
- Block processing lag alerts

## 🔧 Operations

### Starting Services

```bash
# Start DefraDB first
sudo systemctl start shinzo-defradb

# Wait for DefraDB to be ready, then start indexer
sudo systemctl start shinzo-indexer
```

### Catch-up Indexing

For initial sync or after downtime:

```bash
# Stop real-time indexer
sudo systemctl stop shinzo-indexer

# Run catch-up mode
sudo -u shinzo /opt/shinzo/bin/catch_up

# Resume real-time indexing
sudo systemctl start shinzo-indexer
```

### Backup Strategy

```bash
# Backup DefraDB data
sudo tar -czf shinzo-backup-$(date +%Y%m%d).tar.gz /opt/shinzo/data

# Backup configuration
sudo cp /opt/shinzo/.env /backup/location/
```

### Log Management

```bash
# Rotate logs (configure logrotate)
sudo logrotate -f /etc/logrotate.d/shinzo

# Archive old logs
sudo find /opt/shinzo/logs -name "*.log" -mtime +30 -delete
```

## 🛠️ Troubleshooting

### Common Issues

1. **Transaction Type Errors**
   - Increase Erigon compatibility buffer
   - Check GCP node status

2. **DefraDB Connection Issues**
   - Verify DefraDB service is running
   - Check port 9181 accessibility

3. **Memory Issues**
   - Monitor DefraDB memory usage
   - Configure swap if needed

4. **Disk Space**
   - Monitor `/opt/shinzo/data` usage
   - Implement log rotation

### Performance Tuning

1. **DefraDB Optimization**
   - Adjust BadgerDB settings
   - Configure appropriate cache sizes

2. **Indexer Performance**
   - Tune batch processing sizes
   - Adjust retry delays

3. **System Resources**
   - Allocate sufficient RAM (8GB+ recommended)
   - Use SSD storage for DefraDB data

## 📋 Validator Infrastructure Integration

### Resource Requirements

- **CPU**: 4+ cores recommended
- **RAM**: 8GB+ (DefraDB can be memory-intensive)
- **Storage**: 100GB+ SSD for blockchain data
- **Network**: Stable connection to GCP endpoints

### Service Dependencies

- Network connectivity to GCP blockchain nodes
- DefraDB must start before indexer
- Consider firewall rules for port 9181

### Maintenance Windows

- Schedule updates during low validator activity
- Test deployments on staging environment first
- Monitor block processing lag after updates

## 🔐 Security Considerations

- Store GCP API keys securely (environment variables only)
- Regular security updates for base system
- Monitor for unusual network activity
- Backup encryption for sensitive data
- Access logging for administrative actions
