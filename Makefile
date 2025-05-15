.PHONY: deps env build start clean defradb

deps:
	go mod download

env:
	export $(cat .env)

build:
	go build -o bin/block_poster cmd/block_poster/main.go

build_view:
	go build -o bin/view_creator cmd/view_creator/main.go

start:
	./bin/block_poster > logs/log.txt 1>&2   

start_apply_schema:
	./scripts/start_apply_schema.sh

start_view:
	./bin/view_creator > logs/view_logs.txt 1>&2

defradb:
	sh scripts/apply_schema.sh

clean:
	rm -rf bin/ && rm -r logs/logfile && touch logs/logfile

gitpush: 
	git add . && git commit -m "${COMMIT_MESSAGE}" && git push origin ${BRANCH_NAME}

view:
	./bin/view_creator > logs/log.txt 1>&2