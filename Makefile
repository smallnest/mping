.PHONY: build docker test

build:
	GO_ENABLED=0 go build -ldflags '-extldflags "-static"' -tags timetzdata -o mping .

docker:
	docker build -t mping .

test:
	go test -v ./...
