#!/bin/bash

set -e

DEFRA_PATH="$1"
ROOTDIR="$(pwd)/.defra"
PLAYGROUND="$2"

if [[ -z "$DEFRA_PATH" ]]; then
  echo "ERROR: You must pass DEFRA_PATH. Usage:"
  echo "  ./scripts/bootstrap.sh /path/to/defradb [PLAYGROUND]"
  exit 1
fi

DEFRA_ROOT="$(cd "$DEFRA_PATH" && pwd)"
DEFRA_LOG_PATH="logs/defradb_logs.txt"
BLOCK_POSTER_LOG_PATH="logs/blockposter_logs.txt"

mkdir -p logs

# Build and run DefraDB
echo "===> Building DefraDB from $DEFRA_ROOT"
cd "$DEFRA_ROOT"
make deps:playground
GOFLAGS="-tags=playground" make build
./build/defradb start --rootdir "$ROOTDIR" > "$OLDPWD/$DEFRA_LOG_PATH" 2>&1 &
DEFRA_PID=$!
echo "$DEFRA_PID" > "$ROOTDIR/defradb.pid"
echo "Started defradb (PID $DEFRA_PID). Logs at $DEFRA_LOG_PATH"
cd "$OLDPWD"
sleep 3

# Apply schema
echo "===> Applying schema"
./scripts/apply_schema.sh || echo "⚠️  Warning: Schema application failed (likely already applied). Proceeding..."

# Build and run block_poster
echo "===> Building block_poster"
go build -o bin/block_poster cmd/block_poster/main.go
echo "===> Running block_poster"
./bin/block_poster > "$BLOCK_POSTER_LOG_PATH" 2>&1 &
POSTER_PID=$!
echo "$POSTER_PID" > "$ROOTDIR/block_poster.pid"
echo "Started block_poster (PID $POSTER_PID). Logs at $BLOCK_POSTER_LOG_PATH"

# If playground flag provided, open up the playground in browser
if [[ "$PLAYGROUND" == "1" ]]; then
  echo "===> Opening DefraDB Playground at http://localhost:9181"
  if command -v open >/dev/null 2>&1; then
    open http://localhost:9181
  elif command -v xdg-open >/dev/null 2>&1; then
    xdg-open http://localhost:9181
  else
    echo "⚠️ Could not auto-open browser. Please open http://localhost:9181 manually."
  fi
fi

# When the process is killed (via Control + C or other means), this trap statement will execute, shutting down the DefraDB and block_poster instances
trap 'echo "Stopping defradb (PID $DEFRA_PID)..."; kill $DEFRA_PID 2>/dev/null || true; rm -f "$ROOTDIR/defradb.pid"; \
      echo "Stopping block_poster (PID $POSTER_PID)..."; kill $POSTER_PID 2>/dev/null || true; rm -f "$ROOTDIR/block_poster.pid"; exit 0;' INT TERM

wait $DEFRA_PID $POSTER_PID

