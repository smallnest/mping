.PHONY: build docker test

build:
	@go build -o mping .

docker:
	@docker build -t mping .

test:
	@go test -v ./...
