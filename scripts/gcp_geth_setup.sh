#!/bin/bash

# GCP Geth Node Setup Script
# This script helps configure a Geth node on Google Cloud Platform

set -e

echo "🌐 GCP Geth Node Setup Guide"
echo "================================"
echo ""

echo "📋 GCP Instance Recommendations:"
echo "  • Machine Type: n2-standard-8 (8 vCPUs, 32GB RAM) minimum"
echo "  • Boot Disk: 100GB SSD for OS"
echo "  • Additional Disk: 2TB SSD for blockchain data"
echo "  • Network: Allow HTTP/HTTPS traffic"
echo "  • Firewall: Open ports 8545 (HTTP RPC) and 8546 (WebSocket)"
echo ""

echo "🔧 Geth Configuration for GCP:"
echo "  • Sync Mode: snap (fastest initial sync)"
echo "  • Cache: 4-8GB (adjust based on instance RAM)"
echo "  • Max Peers: 50-100"
echo "  • Archive Mode: Optional (requires more storage)"
echo ""

echo "📝 Sample startup script for GCP instance:"
cat << 'EOF'

#!/bin/bash
# GCP Instance Startup Script

# Update system
apt-get update
apt-get install -y software-properties-common curl jq

# Install Geth
add-apt-repository -y ppa:ethereum/ethereum
apt-get update
apt-get install -y ethereum

# Create geth user
useradd -m -s /bin/bash geth

# Create data directory on additional disk
mkdir -p /mnt/ethereum-data
chown geth:geth /mnt/ethereum-data

# Generate JWT secret
mkdir -p /etc/ethereum
openssl rand -hex 32 > /etc/ethereum/jwt.hex
chown geth:geth /etc/ethereum/jwt.hex
chmod 600 /etc/ethereum/jwt.hex

# Create systemd service
cat > /etc/systemd/system/geth.service << 'SERVICE'
[Unit]
Description=Ethereum Geth Node
After=network.target

[Service]
Type=simple
User=geth
Group=geth
ExecStart=/usr/bin/geth \
    --datadir=/mnt/ethereum-data \
    --http --http.api=eth,net,web3,txpool,debug \
    --http.addr=0.0.0.0 --http.port=8545 \
    --http.corsdomain="*" \
    --ws --ws.api=eth,net,web3,txpool \
    --ws.addr=0.0.0.0 --ws.port=8546 \
    --ws.origins="*" \
    --authrpc.jwtsecret=/etc/ethereum/jwt.hex \
    --syncmode=snap \
    --cache=6144 \
    --maxpeers=100 \
    --metrics --metrics.addr=0.0.0.0 --metrics.port=6060
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=geth

[Install]
WantedBy=multi-user.target
SERVICE

# Enable and start service
systemctl daemon-reload
systemctl enable geth
systemctl start geth

# Install monitoring tools
apt-get install -y prometheus-node-exporter

EOF

echo ""
echo "🔐 Security Recommendations:"
echo "  • Use VPC with private subnets"
echo "  • Restrict RPC access to your indexer's IP"
echo "  • Enable Cloud Armor for DDoS protection"
echo "  • Use Cloud IAM for access control"
echo "  • Enable audit logging"
echo ""

echo "📊 Monitoring Setup:"
echo "  • Use Cloud Monitoring for metrics"
echo "  • Set up alerts for sync status"
echo "  • Monitor disk usage (blockchain grows ~1GB/day)"
echo "  • Track peer count and sync progress"
echo ""

echo "💰 Cost Optimization:"
echo "  • Use preemptible instances for non-critical environments"
echo "  • Consider regional persistent disks"
echo "  • Set up automatic snapshots for data backup"
echo "  • Use committed use discounts for long-term deployment"
echo ""

echo "✅ Next Steps:"
echo "1. Create GCP instance with the specifications above"
echo "2. Run the startup script on the instance"
echo "3. Wait for initial sync (6-24 hours depending on sync mode)"
echo "4. Update your config.yaml with the GCP instance IP"
echo "5. Test connection with: make geth-status"
