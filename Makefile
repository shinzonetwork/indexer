.PHONY: deps env build start

deps:
	go mod download

env:
	export `cat .env`

build:
	go build -o bin/block_poster cmd/block_poster/main.go

start:
	./bin/block_poster > logs/log.txt 1>&2   
