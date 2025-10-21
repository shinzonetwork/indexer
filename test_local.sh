#!/bin/bash

# Local Test Setup for Shinzo Network Indexer
# This script sets up environment variables and runs tests locally with your GCP endpoint

echo "🔧 Setting up local test environment..."

# Load existing .env file if it exists
if [ -f .env ]; then
    echo "📄 Loading .env file..."
    source .env
fi

# Ensure GCP_GETH_RPC_URL is set
if [ -z "$GCP_GETH_RPC_URL" ]; then
    echo "❌ GCP_GETH_RPC_URL not set. Please export it first:"
    echo "   export GCP_GETH_RPC_URL=http://your.gcp.ip:8545"
    exit 1
fi

echo "✅ Using Geth endpoint: $GCP_GETH_RPC_URL"

# Optional: Set WebSocket URL if available
if [ -n "$GCP_GETH_WS_URL" ]; then
    echo "✅ Using WebSocket endpoint: $GCP_GETH_WS_URL"
fi

# Optional: Set API key if available
if [ -n "$GCP_GETH_API_KEY" ]; then
    echo "✅ Using API key authentication"
fi

echo ""
echo "🧪 Running indexer tests locally..."
echo "=================================================="

# Run the specific test that was failing in GitHub
go test ./pkg/indexer -v -run TestIndexing

echo ""
echo "📊 Test completed!"
