.PHONY: deps env build start clean defradb

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

test-types:
	go test -v ./pkg/types/

test-rpc:
	go test -v ./pkg/rpc/

test-defra:
	go test -v ./pkg/defra/

race-types:
	go test -race -v ./pkg/types/

race-rpc:
	go test -race -v ./pkg/rpc/

race-defra:
	go test -race -v ./pkg/defra/

cover-types:
	go test -coverprofile=coverage.out ./pkg/types/
	go tool cover -html=coverage.out

cover-rpc:
	go test -coverprofile=coverage.out ./pkg/rpc/
	go tool cover -html=coverage.out

cover-defra:
	go test -coverprofile=coverage.out ./pkg/defra/
	go tool cover -html=coverage.out	

test:
	go test -v ./pkg/types/ ./pkg/rpc/ ./pkg/defra/

race:
	go test -race -v ./pkg/types/ ./pkg/rpc/ ./pkg/defra/

cover:
	go test -coverprofile=coverage.out ./pkg/types/ ./pkg/rpc/ ./pkg/defra/
	go tool cover -html=coverage.out

test-all:
	go test ./...