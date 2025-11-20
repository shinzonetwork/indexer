#!/bin/bash
set -e

# Set up environment variables (same as bootstrap.sh)
ROOTDIR="$(pwd)"
BLOCK_POSTER_LOG_PATH="logs/blockposter_logs.txt"

# Ensure logs directory exists
mkdir -p logs

# Build and run block_poster in real-time mode
echo "===> Building block_poster"
go build -o bin/block_poster cmd/block_poster/main.go
echo "===> Running block_poster in real-time mode"
# Set a recent block height to start from when database is empty
export INDEXER_START_HEIGHT=23500000
./bin/block_poster --mode=realtime --defra-store-path="$ROOTDIR/.defra" > "$BLOCK_POSTER_LOG_PATH" 2>&1 &
POSTER_PID=$!
echo "$POSTER_PID" > "$ROOTDIR/block_poster.pid"
echo "Started block_poster (PID $POSTER_PID). Logs at $BLOCK_POSTER_LOG_PATH"

# Ensure cleanup on exit or interruption
cleanup() {
  echo "===> Cleaning up block_poster (PID $POSTER_PID)..."
  kill $POSTER_PID 2>/dev/null || true
  wait $POSTER_PID 2>/dev/null || true
  rm -f "$ROOTDIR/block_poster.pid"
}
trap cleanup EXIT INT TERM

# Run integration tests
GO111MODULE=on go test -v -tags=integration ./integration/... > integration_test_output.txt 2>&1 || true
echo -e "\n\n===> Integration test output:"
cat integration_test_output.txt
rm integration_test_output.txt
echo -e "\n\n===> Starting cleanup..."