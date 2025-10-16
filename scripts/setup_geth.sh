#!/bin/bash

# Geth Node Setup Script for Shinzo Indexer
# This script sets up a Geth node optimized for blockchain indexing

set -e

echo "🚀 Setting up Geth node for blockchain indexing..."

# Configuration
GETH_DIR="$HOME/.ethereum"
DATA_DIR="$GETH_DIR/mainnet"
JWT_SECRET="$GETH_DIR/jwt.hex"

# Create directories
mkdir -p "$GETH_DIR"
mkdir -p "$DATA_DIR"

# Generate JWT secret if it doesn't exist
if [ ! -f "$JWT_SECRET" ]; then
    echo "🔐 Generating JWT secret..."
    openssl rand -hex 32 > "$JWT_SECRET"
    echo "JWT secret created at: $JWT_SECRET"
fi

# Download Geth if not installed
if ! command -v geth &> /dev/null; then
    echo "📥 Installing Geth..."
    
    # macOS installation
    if [[ "$OSTYPE" == "darwin"* ]]; then
        if command -v brew &> /dev/null; then
            brew tap ethereum/ethereum
            brew install ethereum
        else
            echo "❌ Please install Homebrew first: https://brew.sh"
            exit 1
        fi
    else
        echo "❌ Please install Geth manually for your OS: https://geth.ethereum.org/downloads/"
        exit 1
    fi
fi

echo "✅ Geth setup complete!"
echo ""
echo "📋 Next steps:"
echo "1. Start Geth with: make geth-start"
echo "2. Wait for sync (this can take several hours for full sync)"
echo "3. Update config.yaml to use localhost:8545"
echo ""
echo "🔧 Geth will be configured with:"
echo "   - HTTP RPC: http://localhost:8545"
echo "   - WebSocket: ws://localhost:8546"
echo "   - Data directory: $DATA_DIR"
echo "   - JWT secret: $JWT_SECRET"
