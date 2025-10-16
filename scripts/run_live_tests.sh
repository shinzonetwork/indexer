#!/bin/bash

# Load environment variables from .env file
if [ -f "./integration/live/.env" ]; then
    export $(cat ./integration/live/.env | xargs)
    echo "✓ Loaded environment variables from .env file"
else
    echo "❌ .env file not found at ./integration/live/.env"
    exit 1
fi

# Run live integration tests
echo "🚀 Running live integration tests..."
go test -tags live ./integration/live -v
