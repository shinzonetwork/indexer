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
	@curl -s -X POST -H "Content-Type: application/json" \
		--data '{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}' \
		http://localhost:8545 | jq '.' || echo "❌ Geth not responding"

gcp-geth-status:
	@echo "🔍 Checking GCP Geth status..."
	@if [ -z "$(GCP_IP)" ]; then \
		echo "❌ Please provide GCP_IP. Usage: make gcp-geth-status GCP_IP=your.instance.ip"; \
		exit 1; \
	fi
	@echo "Testing connection to $(GCP_IP):8545..."
	@curl -s -X POST -H "Content-Type: application/json" \
		--data '{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}' \
		http://$(GCP_IP):8545 | jq '.' || echo "❌ GCP Geth not responding"
	@echo "Testing WebSocket connection to $(GCP_IP):8546..."
	@timeout 5 wscat -c ws://$(GCP_IP):8546 -x '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' || echo "❌ WebSocket not responding"

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

clean-deploy:
	sudo systemctl stop shinzo-indexer shinzo-defradb || true
	sudo systemctl disable shinzo-indexer shinzo-defradb || true
	sudo rm -f /etc/systemd/system/shinzo-*.service
	sudo systemctl daemon-reload

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

