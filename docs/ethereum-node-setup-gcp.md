# Ethereum Node Setup on GCP - Complete Instructions

## Overview
This guide sets up a complete Ethereum node (Execution + Consensus layers) on Google Cloud Platform using a single c2-standard-16 VM instance.

**Timeline**: 
- Setup: 1 hour
- Initial Sync: 6-12 hours
- Full Sync: 24-48 hours

**Monthly Cost**: ~$600-700

---

## Prerequisites

1. GCP account with billing enabled
2. `gcloud` CLI installed and authenticated
3. Your indexer server's external IP address

---

## Step 1: Create GCP VM Instance

### 1.1 Set Environment Variables
```bash
export PROJECT_ID="shinzo-468905"
export ZONE="us-central1-a"
export INDEXER_IP="66.207.195.70"  # Replace with actual IP
```

### 1.2 Create the VM Instance
```bash
gcloud compute instances create ethereum-node \
    --project=$PROJECT_ID \
    --zone=$ZONE \
    --machine-type=c2-standard-16 \
    --network-interface=network-tier=PREMIUM,stack-type=IPV4_ONLY,subnet=default \
    --maintenance-policy=MIGRATE \
    --provisioning-model=STANDARD \
    --boot-disk-size=100GB \
    --boot-disk-type=pd-ssd \
    --boot-disk-device-name=ethereum-node \
    --create-disk=auto-delete=yes,boot=no,device-name=geth-data,mode=rw,size=4000,type=pd-ssd \
    --no-shielded-secure-boot \
    --shielded-vtpm \
    --shielded-integrity-monitoring \
    --labels=environment=production,service=ethereum \
    --reservation-affinity=any \
    --image-family=ubuntu-2204-lts \
    --image-project=ubuntu-os-cloud \
    --tags=ethereum-node
```

### 1.3 Create Firewall Rules
```bash
# Ethereum P2P traffic (geth)
gcloud compute firewall-rules create ethereum-p2p \
    --allow tcp:30303,udp:30303 \
    --source-ranges 0.0.0.0/0 \
    --description "Ethereum P2P traffic"

# Lighthouse P2P traffic
gcloud compute firewall-rules create lighthouse-p2p \
    --allow tcp:9000,udp:9000 \
    --source-ranges 0.0.0.0/0 \
    --target-tags ethereum-node \
    --description "Lighthouse P2P traffic"

# RPC access via nginx proxy (restrict to your indexer IP)
gcloud compute firewall-rules create ethereum-rpc \
    --allow tcp:8080,tcp:8081 \
    --source-ranges $INDEXER_IP/32 \
    --target-tags ethereum-node \
    --description "Ethereum RPC access via nginx proxy"

# Optional: Lighthouse API for monitoring
gcloud compute firewall-rules create lighthouse-api \
    --allow tcp:5052 \
    --source-ranges $INDEXER_IP/32 \
    --target-tags ethereum-node \
    --description "Lighthouse Beacon API"
```

---

## Step 2: Initial Server Setup

### 2.1 Connect to the Instance
```bash
gcloud compute ssh ethereum-node --zone=$ZONE
```

### 2.2 Update System and Install Dependencies
```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install required packages
sudo apt install -y software-properties-common curl wget jq htop unattended-upgrades nginx

# Setup automatic security updates
sudo dpkg-reconfigure -plow unattended-upgrades
```

### 2.3 Mount Additional Disk
```bash
# Format and mount the 4TB disk
sudo mkfs.ext4 /dev/sdb
sudo mkdir -p /data
sudo mount /dev/sdb /data

# Make mount permanent
echo '/dev/sdb /data ext4 defaults 0 0' | sudo tee -a /etc/fstab

# Set ownership
sudo chown $USER:$USER /data
```

---

## Step 3: Install Ethereum Clients

### 3.1 Install Geth (Execution Client)
```bash
# Add Ethereum PPA and install geth
sudo add-apt-repository -y ppa:ethereum/ethereum
sudo apt update
sudo apt install -y ethereum

# Verify installation
geth version
```

### 3.2 Install Lighthouse (Consensus Client)
```bash
# Download and install Lighthouse
cd /tmp
wget https://github.com/sigp/lighthouse/releases/download/v4.6.0/lighthouse-v4.6.0-x86_64-unknown-linux-gnu.tar.gz
tar -xzf lighthouse-v4.6.0-x86_64-unknown-linux-gnu.tar.gz
sudo mv lighthouse /usr/local/bin/
sudo chmod +x /usr/local/bin/lighthouse

# Verify installation
lighthouse --version
```

---

## Step 4: Setup Authentication

### 4.1 Create JWT Secret
```bash
# Create JWT secret for geth-lighthouse communication
sudo mkdir -p /data/jwt
openssl rand -hex 32 | sudo tee /data/jwt/jwt.hex
sudo chown $USER:$USER /data/jwt/jwt.hex
sudo chmod 600 /data/jwt/jwt.hex
```

### 4.2 Generate API Key for RPC Access
```bash
# Generate a secure API key for RPC access
API_KEY=$(openssl rand -base64 32)
echo "Generated API Key: $API_KEY"

# Save API key to file for reference
echo "$API_KEY" | sudo tee /data/jwt/api-key.txt
sudo chown $USER:$USER /data/jwt/api-key.txt
sudo chmod 600 /data/jwt/api-key.txt

# Store for nginx configuration
export ETHEREUM_API_KEY="$API_KEY"
```

---

## Step 5: Configure Geth

### 5.1 Create Geth Data Directory
```bash
mkdir -p /data/geth
```

### 5.2 Create Geth Configuration
```bash
# Create config directory
sudo mkdir -p /etc/geth

# Create geth configuration file
sudo tee /etc/geth/geth.toml << 'EOF'
[Eth]
NetworkId = 1
SyncMode = "snap"
DatabaseCache = 6144
TrieCleanCache = 1536
TrieDirtyCache = 768
TrieTimeout = "1h"
SnapshotCache = 768
TxLookupLimit = 0

[Node]
DataDir = "/data/geth"
HTTPHost = "127.0.0.1"
HTTPPort = 8545
HTTPModules = ["eth", "net", "web3", "txpool", "debug"]
WSHost = "127.0.0.1"
WSPort = 8546
WSModules = ["eth", "net", "web3", "txpool", "debug"]
AuthAddr = "localhost"
AuthPort = 8551
JWTSecret = "/data/jwt/jwt.hex"
MaxPeers = 50

[Node.P2P]
MaxPendingPeers = 50
NoDiscovery = false
ListenAddr = ":30303"

[Metrics]
HTTP = "0.0.0.0"
Port = 6060
EOF
```

### 5.3 Create Geth Systemd Service
```bash
sudo tee /etc/systemd/system/geth.service << 'EOF'
[Unit]
Description=Ethereum Go Client
Documentation=https://geth.ethereum.org/docs/
After=network.target
Wants=network.target

[Service]
Type=simple
User=ubuntu
Group=ubuntu
ExecStart=/usr/bin/geth --config /etc/geth/geth.toml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=geth
KillMode=mixed
TimeoutStopSec=60
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
```

---

## Step 6: Configure Nginx Reverse Proxy with API Key Authentication

### 6.1 Create Nginx Configuration
```bash
# First, add rate limiting zones to main nginx config
sudo tee -a /etc/nginx/nginx.conf << 'EOF'

# Rate limiting zones for Ethereum RPC
limit_req_zone $binary_remote_addr zone=rpc_limit:10m rate=100r/s;
limit_req_zone $binary_remote_addr zone=ws_limit:10m rate=50r/s;
EOF

# Create nginx configuration for API key authentication
sudo tee /etc/nginx/sites-available/ethereum-rpc << EOF
# Map for API key validation
map \$http_x_api_key \$api_key_valid {
    default 0;
    "$ETHEREUM_API_KEY" 1;
}

# HTTP RPC Proxy (Port 8080)
server {
    listen 8080;
    server_name _;
    
    location / {
        # Rate limiting
        limit_req zone=rpc_limit burst=20 nodelay;
        
        # Check API key
        if (\$api_key_valid = 0) {
            return 401 '{"error":"Invalid or missing API key"}';
        }
        
        # Proxy to local geth
        proxy_pass http://127.0.0.1:8545;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}

# WebSocket RPC Proxy (Port 8081)  
server {
    listen 8081;
    server_name _;
    
    location / {
        # Rate limiting
        limit_req zone=ws_limit burst=10 nodelay;
        
        # Check API key
        if (\$api_key_valid = 0) {
            return 401 '{"error":"Invalid or missing API key"}';
        }
        
        # Proxy to local geth WebSocket
        proxy_pass http://127.0.0.1:8546;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        
        # WebSocket headers
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}
EOF

# Enable the site
sudo ln -sf /etc/nginx/sites-available/ethereum-rpc /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default

# Test nginx configuration
sudo nginx -t

# Start and enable nginx
sudo systemctl enable nginx
sudo systemctl start nginx
```

### 6.2 Create API Key Management Script
```bash
# Create script to manage API keys
sudo tee /usr/local/bin/ethereum-api-key << 'EOF'
#!/bin/bash

API_KEY_FILE="/data/jwt/api-key.txt"
NGINX_CONFIG="/etc/nginx/sites-available/ethereum-rpc"

case "$1" in
    "show")
        if [ -f "$API_KEY_FILE" ]; then
            echo "Current API Key:"
            cat "$API_KEY_FILE"
        else
            echo "No API key found"
        fi
        ;;
    "rotate")
        # Generate new API key
        NEW_KEY=$(openssl rand -base64 32)
        echo "$NEW_KEY" | sudo tee "$API_KEY_FILE" > /dev/null
        
        # Update nginx configuration
        sudo sed -i "s/\".*\" 1;/\"$NEW_KEY\" 1;/" "$NGINX_CONFIG"
        
        # Reload nginx
        sudo nginx -t && sudo systemctl reload nginx
        
        echo "API key rotated successfully:"
        echo "$NEW_KEY"
        ;;
    *)
        echo "Usage: $0 {show|rotate}"
        echo "  show   - Display current API key"
        echo "  rotate - Generate and apply new API key"
        exit 1
        ;;
esac
EOF

# Make script executable
sudo chmod +x /usr/local/bin/ethereum-api-key
```

---

## Step 7: Configure Lighthouse

### 7.1 Create Lighthouse Data Directory
```bash
mkdir -p /data/lighthouse
```

### 7.2 Create Lighthouse Systemd Service
```bash
sudo tee /etc/systemd/system/lighthouse.service << 'EOF'
[Unit]
Description=Lighthouse Ethereum Consensus Client
Documentation=https://lighthouse-book.sigmaprime.io/
After=network.target geth.service
Wants=network.target
Requires=geth.service

[Service]
Type=simple
User=ubuntu
Group=ubuntu
ExecStart=/usr/local/bin/lighthouse bn \
    --network mainnet \
    --datadir /data/lighthouse \
    --http \
    --http-address 0.0.0.0 \
    --http-port 5052 \
    --execution-endpoint http://localhost:8551 \
    --execution-jwt /data/jwt/jwt.hex \
    --checkpoint-sync-url https://mainnet.checkpoint.sigp.io \
    --disable-deposit-contract-sync
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=lighthouse
KillMode=mixed
TimeoutStopSec=60
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
```

---

## Step 7: System Optimizations

### 7.1 Increase File Descriptor Limits
```bash
# Add to limits.conf
echo 'ubuntu soft nofile 65536' | sudo tee -a /etc/security/limits.conf
echo 'ubuntu hard nofile 65536' | sudo tee -a /etc/security/limits.conf
```

### 7.2 Network Optimizations
```bash
# Add network optimizations
sudo tee -a /etc/sysctl.conf << 'EOF'

# Network optimizations for Ethereum node
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.ipv4.tcp_rmem = 4096 87380 134217728
net.ipv4.tcp_wmem = 4096 65536 134217728
net.core.netdev_max_backlog = 5000
EOF

# Apply changes
sudo sysctl -p
```

### 7.3 Setup UFW Firewall
```bash
# Enable UFW
sudo ufw --force enable

# Allow SSH
sudo ufw allow ssh

# Allow Ethereum P2P
sudo ufw allow 30303
sudo ufw allow 9000

# Allow RPC only from indexer IP
sudo ufw allow from $INDEXER_IP to any port 8545
sudo ufw allow from $INDEXER_IP to any port 8546
sudo ufw allow from $INDEXER_IP to any port 5052

# Check status
sudo ufw status
```

---

## Step 8: Start Services

### 8.1 Enable and Start Services
```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable services
sudo systemctl enable geth
sudo systemctl enable lighthouse

# Start geth first
sudo systemctl start geth

# Wait 30 seconds, then start lighthouse
sleep 30
sudo systemctl start lighthouse
```

### 8.2 Check Service Status
```bash
# Check both services
sudo systemctl status geth
sudo systemctl status lighthouse
```

---

## Step 9: Create Monitoring Tools

### 9.1 Create Monitoring Script
```bash
tee ~/check-ethereum-node.sh << 'EOF'
#!/bin/bash

echo "=== Ethereum Node Status Check ==="
echo "Timestamp: $(date)"

echo -e "\n=== Service Status ==="
echo "Geth: $(sudo systemctl is-active geth)"
echo "Lighthouse: $(sudo systemctl is-active lighthouse)"

echo -e "\n=== Latest Block ==="
LATEST=$(curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545 | jq -r .result)

if [ "$LATEST" != "null" ] && [ "$LATEST" != "" ]; then
    BLOCK_NUM=$((16#${LATEST:2}))
    echo "Block Number: $BLOCK_NUM"
else
    echo "Unable to get block number"
fi

echo -e "\n=== Sync Status ==="
SYNC=$(curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}' \
  http://localhost:8545 | jq -r .result)

if [ "$SYNC" = "false" ]; then
    echo "✅ Geth: Fully Synced"
else
    echo "🔄 Geth: Syncing"
fi

BEACON_SYNC=$(curl -s http://localhost:5052/eth/v1/node/syncing 2>/dev/null | jq -r .data.is_syncing 2>/dev/null)
if [ "$BEACON_SYNC" = "false" ]; then
    echo "✅ Lighthouse: Fully Synced"
elif [ "$BEACON_SYNC" = "true" ]; then
    echo "🔄 Lighthouse: Syncing"
else
    echo "⚠️  Lighthouse: Status Unknown"
fi

echo -e "\n=== Peer Counts ==="
GETH_PEERS=$(curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}' \
  http://localhost:8545 | jq -r .result)

BEACON_PEERS=$(curl -s http://localhost:5052/eth/v1/node/peer_count 2>/dev/null | jq -r .data.connected 2>/dev/null)

if [ "$GETH_PEERS" != "null" ] && [ "$GETH_PEERS" != "" ]; then
    echo "Geth Peers: $((16#${GETH_PEERS:2}))"
else
    echo "Geth Peers: Unknown"
fi

if [ "$BEACON_PEERS" != "null" ] && [ "$BEACON_PEERS" != "" ]; then
    echo "Lighthouse Peers: $BEACON_PEERS"
else
    echo "Lighthouse Peers: Unknown"
fi

echo -e "\n=== Disk Usage ==="
df -h /data | tail -1

echo -e "\n=== Memory Usage ==="
free -h
EOF

chmod +x ~/check-ethereum-node.sh
```

### 9.2 Create Log Monitoring Aliases
```bash
# Add useful aliases to .bashrc
tee -a ~/.bashrc << 'EOF'

# Ethereum node aliases
alias geth-logs='sudo journalctl -u geth -f'
alias lighthouse-logs='sudo journalctl -u lighthouse -f'
alias node-status='~/check-ethereum-node.sh'
alias geth-restart='sudo systemctl restart geth'
alias lighthouse-restart='sudo systemctl restart lighthouse'
EOF

source ~/.bashrc
```

---

## Step 10: Get Node Information

### 10.1 Get External IP
```bash
# Get the external IP of your node
EXTERNAL_IP=$(gcloud compute instances describe ethereum-node \
    --zone=$ZONE \
    --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

echo "Your Ethereum node external IP: $EXTERNAL_IP"
```

### 10.2 Get API Key
```bash
# Display the generated API key
ethereum-api-key show
```

### 10.3 Test RPC Connection with API Key
```bash
# Get your API key first
API_KEY=$(ethereum-api-key show | tail -1)

# Test HTTP RPC from external IP (run this from your indexer server)
curl -X POST -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://$EXTERNAL_IP:8080

# Test WebSocket connection (if supported by your client)
# Note: WebSocket endpoint is on port 8081
```

---

## Step 11: Update Indexer Configuration

Update your indexer's configuration file with the new node details:

```yaml
geth:
  node_url: "http://EXTERNAL_IP:8080"    # HTTP RPC via nginx proxy
  ws_url: "ws://EXTERNAL_IP:8081"        # WebSocket via nginx proxy  
  api_key: "YOUR_API_KEY_HERE"           # Use API key from Step 10.2
```

Replace:
- `EXTERNAL_IP` with the actual external IP from Step 10.1
- `YOUR_API_KEY_HERE` with the API key from Step 10.2

**Important**: The indexer must include the API key in the `X-API-Key` header for all requests.

---

## Monitoring and Maintenance

### Daily Monitoring Commands
```bash
# Check overall status
node-status

# Monitor logs in real-time
geth-logs          # Geth logs
lighthouse-logs    # Lighthouse logs

# Check disk space
df -h /data

# Check system resources
htop
```

### Expected Sync Timeline
1. **Lighthouse Checkpoint Sync**: 10-30 minutes
2. **Geth Snap Sync**: 6-12 hours
3. **Full Historical Sync**: 24-48 hours

### API Key Management
```bash
# View current API key
ethereum-api-key show

# Rotate API key (generates new key and updates nginx)
ethereum-api-key rotate

# Test API key after rotation
API_KEY=$(ethereum-api-key show | tail -1)
curl -H "X-API-Key: $API_KEY" http://localhost:8080 \
  -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

### Troubleshooting
- If geth fails to start: Check `/data/geth` permissions
- If lighthouse fails: Ensure geth is running and JWT file exists
- If sync is slow: Check peer count and network connectivity
- If API key fails: Check nginx logs with `sudo journalctl -u nginx -f`
- For detailed logs: Use `geth-logs` and `lighthouse-logs` commands

---

## Security Notes

1. **Dual Security**: IP restrictions + API key authentication
2. **Rate Limiting**: nginx limits requests to prevent abuse
3. **Local Binding**: Geth only binds to localhost, nginx handles external access
4. **Firewall**: Only your indexer IP can access RPC ports (8080/8081)
5. **Updates**: Automatic security updates are enabled
6. **API Key Rotation**: Use `ethereum-api-key rotate` regularly
7. **Monitoring**: Check node status daily
8. **Backups**: Consider backing up `/data/geth/keystore` if using for signing

---

## Cost Optimization

- **Preemptible Instance**: Can reduce costs by ~70% but may restart
- **Committed Use Discounts**: Available for 1-3 year commitments
- **Regional Persistent Disks**: Slightly cheaper than zonal

Your node will be ready for production use once both clients are fully synced!
