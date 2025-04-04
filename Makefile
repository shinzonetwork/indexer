.PHONY: deps playground build defra:playground

deps:
	go mod download

playground:
	GOFLAGS="-tags=playground" go mod download

defra:playground:
	cd $(GOPATH)/src/github.com/sourcenetwork/defradb && \
	GOFLAGS="-tags=playground" go install ./cmd/defradb

build:
	go build -o bin/block_poster cmd/block_poster/main.go

github:
	git add .
	git commit -m "Update dependencies"
	git push origin ${BRANCH}