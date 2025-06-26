.PHONY: deps env build start clean defradb gitpush test testrpc coverage bootstrap playground stop

DEFRA_PATH ?=

DEFRA_ROOT := $(abspath $(DEFRA_PATH))
ROOTDIR := $(abspath .defra)

deps:
	go mod download

env:
	export $(cat .env)

build:
	go build -o bin/block_poster cmd/block_poster/main.go

start:
	./bin/block_poster > logs/log.txt 1>&2   

defradb:
	sh scripts/apply_schema.sh

clean:
	rm -rf bin/ && rm -r logs/logfile && touch logs/logfile

gitpush: 
	git add . && git commit -m "${COMMIT_MESSAGE}" && git push origin ${BRANCH_NAME}

test:
	go test ./... -v

testrpc:
	go test ./pkg/rpc -v

coverage:
	go test -coverprofile=coverage.out ./... || true
	go tool cover -html=coverage.out -o coverage.html
	open coverage.html
	rm coverage.out

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
	@pkill -f "defradb start" || echo "defradb not running"
