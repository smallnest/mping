.PHONY: build test

build:
	@go build -o mping .

test:
	@go test -v ./...