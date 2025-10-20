.PHONY: deps env build start clean defradb gitpush test testrpc coverage bootstrap playground stop integration-test docker-build docker-up docker-down deploy

DEFRA_PATH ?=

deps:
	go mod download

env:
	export $(cat .env)

build:
	go build -o bin/block_poster cmd/block_poster/main.go

build-catch-up:
	go build -o bin/catch_up cmd/catch_up/main.go

start:
	./bin/block_poster

defradb:
	sh scripts/apply_schema.sh

setup-geth:
	./scripts/setup_geth.sh

setup-gcp-geth:
	./scripts/gcp_geth_setup.sh

clean:
	rm -rf bin/ && rm -r logs/logfile && touch logs/logfile

gitpush: 
	git add . && git commit -m "${COMMIT_MESSAGE}" && git push origin ${BRANCH_NAME}

geth-start:
	cd $GETH_DIR && geth --http --authrpc.jwtsecret=$HOME/.ethereum/jwt.hex --datadir=$HOME/.ethereum

prysm-start:
	cd $PRYSM_DIR && ./prysm.sh beacon-chain \
  --execution-endpoint=http://localhost:8551 \
  --jwt-secret=$HOME/.ethereum/jwt.hex \
  --checkpoint-sync-url=https://mainnet.checkpoint-sync.ethpandaops.io \
  --suggested-fee-recipient=0x8E4902d854e6A7eaF44A98D6f1E600413C99Ce07

geth-status:
	@echo "🔍 Checking Geth status..."
	@echo "📍 Target: $(GCP_GETH_RPC_URL)"
	@if [ -z "$(GCP_GETH_RPC_URL)" ]; then \
		echo "❌ GCP_GETH_RPC_URL not set. Please export it first:"; \
		echo "   export GCP_GETH_RPC_URL=http://34.68.131.15:8545"; \
		exit 1; \
	fi
	@echo "🌐 Testing basic connectivity..."
	@if curl -s --connect-timeout 5 --max-time 10 $(GCP_GETH_RPC_URL) >/dev/null 2>&1; then \
		echo "✅ HTTP connection successful"; \
	else \
		echo "❌ HTTP connection failed"; \
		exit 1; \
	fi
	@echo "🔗 Testing JSON-RPC endpoint..."
	@RESPONSE=$$(curl -s --connect-timeout 5 --max-time 10 -X POST -H "Content-Type: application/json" \
		--data '{"jsonrpc":"2.0","method":"web3_clientVersion","params":[],"id":1}' \
		$(GCP_GETH_RPC_URL) 2>/dev/null); \
	if echo "$$RESPONSE" | jq -e '.result' >/dev/null 2>&1; then \
		echo "✅ JSON-RPC responding"; \
		echo "📋 Client: $$(echo "$$RESPONSE" | jq -r '.result')"; \
	else \
		echo "❌ JSON-RPC not responding properly"; \
		echo "📄 Response: $$RESPONSE"; \
	fi
	@echo "🔄 Checking sync status..."
	@SYNC_RESPONSE=$$(curl -s --connect-timeout 5 --max-time 10 -X POST -H "Content-Type: application/json" \
		--data '{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}' \
		$(GCP_GETH_RPC_URL) 2>/dev/null); \
	if echo "$$SYNC_RESPONSE" | jq -e '.result' >/dev/null 2>&1; then \
		SYNC_STATUS=$$(echo "$$SYNC_RESPONSE" | jq -r '.result'); \
		if [ "$$SYNC_STATUS" = "false" ]; then \
			echo "✅ Node fully synced"; \
		else \
			echo "🔄 Node syncing..."; \
			echo "📊 Sync info: $$SYNC_STATUS"; \
		fi; \
	else \
		echo "❌ Could not get sync status"; \
	fi
	@echo "📊 Getting latest block..."
	@BLOCK_RESPONSE=$$(curl -s --connect-timeout 5 --max-time 10 -X POST -H "Content-Type: application/json" \
		--data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
		$(GCP_GETH_RPC_URL) 2>/dev/null); \
	if echo "$$BLOCK_RESPONSE" | jq -e '.result' >/dev/null 2>&1; then \
		BLOCK_HEX=$$(echo "$$BLOCK_RESPONSE" | jq -r '.result'); \
		BLOCK_NUM=$$(printf "%d" $$BLOCK_HEX 2>/dev/null || echo "unknown"); \
		echo "✅ Latest block: $$BLOCK_NUM"; \
	else \
		echo "❌ Could not get latest block"; \
	fi
	@echo "👥 Checking peer count..."
	@PEER_RESPONSE=$$(curl -s --connect-timeout 5 --max-time 10 -X POST -H "Content-Type: application/json" \
		--data '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":1}' \
		$(GCP_GETH_RPC_URL) 2>/dev/null); \
	if echo "$$PEER_RESPONSE" | jq -e '.result' >/dev/null 2>&1; then \
		PEER_HEX=$$(echo "$$PEER_RESPONSE" | jq -r '.result'); \
		PEER_COUNT=$$(printf "%d" $$PEER_HEX 2>/dev/null || echo "unknown"); \
		echo "✅ Connected peers: $$PEER_COUNT"; \
	else \
		echo "❌ Could not get peer count"; \
	fi
	@if [ -n "$(GCP_GETH_API_KEY)" ]; then \
		echo "🔑 Testing API key authentication..."; \
		AUTH_RESPONSE=$$(curl -s --connect-timeout 5 --max-time 10 -X POST \
			-H "Content-Type: application/json" \
			-H "X-API-Key: $(GCP_GETH_API_KEY)" \
			--data '{"jsonrpc":"2.0","method":"web3_clientVersion","params":[],"id":1}' \
			$(GCP_GETH_RPC_URL) 2>/dev/null); \
		if echo "$$AUTH_RESPONSE" | jq -e '.result' >/dev/null 2>&1; then \
			echo "✅ API key authentication working"; \
		else \
			echo "❌ API key authentication failed"; \
		fi; \
	else \
		echo "⚠️  No API key set (GCP_GETH_API_KEY)"; \
	fi
	@echo "✨ Geth status check complete!"

gcp-geth-status:
	@echo "🔍 Checking GCP Geth status..."
	@if [ -z "$(GCP_GETH_RPC_URL)" ]; then \
		echo "❌ Please provide GCP_GETH_RPC_URL. Usage: make gcp-geth-status GCP_GETH_RPC_URL=http://your.instance.ip:8545"; \
		exit 1; \
	fi
	@echo "📍 Target: $(GCP_GETH_RPC_URL)"
	@echo "🌐 Testing basic connectivity..."
	@if curl -s --connect-timeout 5 --max-time 10 $(GCP_GETH_RPC_URL) >/dev/null 2>&1; then \
		echo "✅ HTTP connection successful"; \
	else \
		echo "❌ HTTP connection failed"; \
		exit 1; \
	fi
	@echo "🔗 Testing JSON-RPC endpoint..."
	@RESPONSE=$$(curl -s --connect-timeout 5 --max-time 10 -X POST -H "Content-Type: application/json" \
		--data '{"jsonrpc":"2.0","method":"web3_clientVersion","params":[],"id":1}' \
		$(GCP_GETH_RPC_URL) 2>/dev/null); \
	if echo "$$RESPONSE" | jq -e '.result' >/dev/null 2>&1; then \
		echo "✅ JSON-RPC responding"; \
		echo "📋 Client: $$(echo "$$RESPONSE" | jq -r '.result')"; \
	else \
		echo "❌ JSON-RPC not responding properly"; \
		echo "📄 Response: $$RESPONSE"; \
	fi
	@echo "📊 Getting latest block..."
	@BLOCK_RESPONSE=$$(curl -s --connect-timeout 5 --max-time 10 -X POST -H "Content-Type: application/json" \
		--data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
		$(GCP_GETH_RPC_URL) 2>/dev/null); \
	if echo "$$BLOCK_RESPONSE" | jq -e '.result' >/dev/null 2>&1; then \
		BLOCK_HEX=$$(echo "$$BLOCK_RESPONSE" | jq -r '.result'); \
		BLOCK_NUM=$$(printf "%d" $$BLOCK_HEX 2>/dev/null || echo "unknown"); \
		echo "✅ Latest block: $$BLOCK_NUM"; \
	else \
		echo "❌ Could not get latest block"; \
	fi
	@if [ -n "$(GCP_GETH_WS_URL)" ]; then \
		echo "🔌 Testing WebSocket connection to $(GCP_GETH_WS_URL)..."; \
		timeout 5 wscat -c $(GCP_GETH_WS_URL) -x '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' 2>/dev/null && \
		echo "✅ WebSocket responding" || echo "❌ WebSocket not responding"; \
	else \
		echo "⚠️  No WebSocket URL provided (GCP_GETH_WS_URL)"; \
	fi
	@echo "✨ GCP Geth status check complete!"

test:
	@echo "🧪 Running all tests with summary output..."
	@go test ./... -v -count=1 | tee /tmp/test_output.log; \
	exit_code=$$?; \
	echo ""; \
	echo "📊 TEST SUMMARY:"; \
	echo "================"; \
	if [ $$exit_code -eq 0 ]; then \
		echo "✅ ALL TESTS PASSED"; \
		echo "📈 Passed packages:"; \
		grep "^ok" /tmp/test_output.log | sed 's/^/  ✓ /'; \
	else \
		echo "❌ SOME TESTS FAILED (Exit Code: $$exit_code)"; \
		echo ""; \
		echo "📈 Passed packages:"; \
		grep "^ok" /tmp/test_output.log | sed 's/^/  ✓ /' || echo "  (none)"; \
		echo ""; \
		echo "❌ Failed packages:"; \
		grep "^FAIL" /tmp/test_output.log | sed 's/^/  ✗ /' || echo "  (check output above for details)"; \
		echo ""; \
		echo "🔍 Failed test details:"; \
		grep -A 5 -B 1 "FAIL:" /tmp/test_output.log | sed 's/^/  /' || echo "  (check full output above)"; \
	fi; \
	echo ""; \
	rm -f /tmp/test_output.log; \
	exit $$exit_code

integration-test:
	@if [ -z "$(DEFRA_PATH)" ]; then \
		echo "ERROR: You must pass DEFRA_PATH. Usage:"; \
		echo "  make integration-test DEFRA_PATH=../path/to/defradb"; \
		exit 1; \
	fi
	@scripts/test_integration.sh "$(DEFRA_PATH)"

testrpc:
	go test ./pkg/rpc -v

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

docker-build:
	docker build -t shinzo-indexer:latest .

docker-up-catch-up:
	docker-compose --profile catch-up up -d

docker-up-indexer:
	docker-compose --profile indexer up -d

docker-down:
	docker-compose down -v

docker-logs:
	docker-compose logs -f

deploy:
	./deploy/deploy.sh

bootstrap:
	@if [ -z "$(DEFRA_PATH)" ]; then \
		echo "ERROR: You must pass DEFRA_PATH. Usage:"; \
		echo "  make bootstrap DEFRA_PATH=../path/to/defradb"; \
		exit 1; \
	fi
	@scripts/bootstrap.sh "$(DEFRA_PATH)" "$(PLAYGROUND)"

playground:
	@if [ -z "$(DEFRA_PATH)" ]; then \
		echo "ERROR: You must pass DEFRA_PATH. Usage:"; \
		echo "  make playground DEFRA_PATH=../path/to/defradb"; \
		exit 1; \
	fi
	@$(MAKE) bootstrap PLAYGROUND=1 DEFRA_PATH="$(DEFRA_PATH)"

stop:
	@echo "===> Stopping defradb if running..."
	@DEFRA_ROOTDIR="$(shell pwd)/.defra"; \
	DEFRA_PIDS=$$(ps aux | grep '[d]efradb start --rootdir ' | grep "$$DEFRA_ROOTDIR" | awk '{print $$2}'); \
	if [ -n "$$DEFRA_PIDS" ]; then \
	  echo "Killing defradb PIDs: $$DEFRA_PIDS"; \
	  echo "$$DEFRA_PIDS" | xargs -r kill -9 2>/dev/null; \
	  echo "Stopped all defradb processes using $$DEFRA_ROOTDIR"; \
	else \
	  echo "No defradb processes found for $$DEFRA_ROOTDIR"; \
	fi; \
	rm -f .defra/defradb.pid;
	@echo "===> Stopping block_poster if running..."
	@BLOCK_PIDS=$$(ps aux | grep '[b]lock_poster' | awk '{print $$2}'); \
	if [ -n "$$BLOCK_PIDS" ]; then \
	  echo "Killing block_poster PIDs: $$BLOCK_PIDS"; \
	  echo "$$BLOCK_PIDS" | xargs -r kill -9 2>/dev/null; \
	  echo "Stopped all block_poster processes"; \
	else \
	  echo "No block_poster processes found"; \
	fi; \
	rm -f .defra/block_poster.pid;

