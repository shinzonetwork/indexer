#!/bin/bash

# Local Test Setup for Shinzo Network Indexer
# This script sets up environment variables and runs tests locally with your endpoint

echo "🔧 Setting up local test environment..."

# Load existing .env file if it exists
if [ -f .env ]; then
    echo "📄 Loading .env file..."
    source .env
fi

# Ensure GETH_RPC_URL is set
if [ -z "$GETH_RPC_URL" ]; then
    echo "❌ GETH_RPC_URL not set. Please export it first:"
    echo "   export GETH_RPC_URL=<your-geth-url>"
    exit 1
fi

echo "✅ Using Geth endpoint: $GETH_RPC_URL"

# Optional: Set WebSocket URL if available
if [ -n "$GETH_WS_URL" ]; then
    echo "✅ Using WebSocket endpoint: $GETH_WS_URL"
fi

# Optional: Set API key if available
if [ -n "$GETH_API_KEY" ]; then
    echo "✅ Using API key authentication"
fi

echo ""
echo "🧪 Running indexer tests locally..."
echo "=================================================="

# Run the specific test that was failing in GitHub
go test ./pkg/indexer -v -run TestIndexing

echo ""
echo "📊 Test completed!"
