#!/bin/bash
set -e

# Shinzo Network Indexer Deployment Script
# For Ethereum Validator Infrastructure

DEPLOY_USER="shinzo"
DEPLOY_DIR="/opt/shinzo"
SERVICE_NAME="shinzo-indexer"
DEFRADB_SERVICE="shinzo-defradb"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}"
    exit 1
}

# Check if running as root
if [[ $EUID -eq 0 ]]; then
    error "This script should not be run as root. Run as a user with sudo privileges."
fi

# Check if .env file exists
if [[ ! -f ".env" ]]; then
    error ".env file not found. Please create it with your configuration."
fi

log "Starting Shinzo Network Indexer deployment..."

# Create user if it doesn't exist
if ! id "$DEPLOY_USER" &>/dev/null; then
    log "Creating user $DEPLOY_USER..."
    sudo useradd -r -s /bin/false -d "$DEPLOY_DIR" "$DEPLOY_USER"
fi

# Create directories
log "Creating directories..."
sudo mkdir -p "$DEPLOY_DIR"/{bin,data,logs,config}
sudo chown -R "$DEPLOY_USER:$DEPLOY_USER" "$DEPLOY_DIR"

# Build binaries
log "Building Shinzo indexer binaries..."
make build
make build-catch-up

# Copy binaries
log "Installing binaries..."
sudo cp bin/block_poster "$DEPLOY_DIR/bin/"
sudo cp bin/catch_up "$DEPLOY_DIR/bin/"
sudo chmod +x "$DEPLOY_DIR/bin/"*

# Copy configuration
log "Installing configuration..."
sudo cp .env "$DEPLOY_DIR/"
sudo cp -r scripts "$DEPLOY_DIR/" 2>/dev/null || true
sudo chown -R "$DEPLOY_USER:$DEPLOY_USER" "$DEPLOY_DIR"

# Install DefraDB if not present
if [[ ! -f "$DEPLOY_DIR/bin/defradb" ]]; then
    log "Installing DefraDB..."
    DEFRADB_VERSION="v0.9.0"
    DEFRADB_URL="https://github.com/sourcenetwork/defradb/releases/download/$DEFRADB_VERSION/defradb-linux-amd64"
    
    sudo wget -O "$DEPLOY_DIR/bin/defradb" "$DEFRADB_URL"
    sudo chmod +x "$DEPLOY_DIR/bin/defradb"
    sudo chown "$DEPLOY_USER:$DEPLOY_USER" "$DEPLOY_DIR/bin/defradb"
fi

# Install systemd services
log "Installing systemd services..."
sudo cp deploy/systemd/shinzo-defradb.service /etc/systemd/system/
sudo cp deploy/systemd/shinzo-indexer.service /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload

# Enable services
log "Enabling services..."
sudo systemctl enable "$DEFRADB_SERVICE"
sudo systemctl enable "$SERVICE_NAME"

# Start DefraDB first
log "Starting DefraDB..."
sudo systemctl start "$DEFRADB_SERVICE"

# Wait for DefraDB to be ready
log "Waiting for DefraDB to be ready..."
for i in {1..30}; do
    if curl -f http://localhost:9181/api/v0/graphql >/dev/null 2>&1; then
        log "DefraDB is ready!"
        break
    fi
    if [[ $i -eq 30 ]]; then
        error "DefraDB failed to start within 30 seconds"
    fi
    sleep 1
done

# Apply DefraDB schema
log "Applying DefraDB schema..."
if [[ -f "scripts/apply_schema.sh" ]]; then
    bash scripts/apply_schema.sh
fi

# Start indexer service
log "Starting Shinzo indexer..."
sudo systemctl start "$SERVICE_NAME"

# Check service status
log "Checking service status..."
sudo systemctl status "$DEFRADB_SERVICE" --no-pager
sudo systemctl status "$SERVICE_NAME" --no-pager

log "Deployment completed successfully!"
log "Service logs: sudo journalctl -u $SERVICE_NAME -f"
log "DefraDB logs: sudo journalctl -u $DEFRADB_SERVICE -f"
log "Data directory: $DEPLOY_DIR/data"
log "Configuration: $DEPLOY_DIR/.env"
