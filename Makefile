.PHONY: test vet build fmt

test: vet
	go test ./...

vet:
	go vet ./...

build:
	go build -o bin/wunjo ./cmd/wunjo

fmt:
	gofmt -w cmd internal
