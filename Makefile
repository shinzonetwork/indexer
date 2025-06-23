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

test:
	go test ./... -v

testrpc:
	go test ./pkg/rpc -v

coverage:
	go test -coverprofile=coverage.out ./... || true
	go tool cover -html=coverage.out -o coverage.html
	open coverage.html
	rm coverage.out
