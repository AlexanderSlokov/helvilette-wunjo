.PHONY: test test-integration vet build fmt

test: vet
	go test -race ./...

# Needs Docker and the OpenSSH client tools; starts sshd in a container.
test-integration: vet
	go test -tags integration -count=1 ./internal/kamal/sshexec/

vet:
	go vet ./...

build:
	go build -o bin/wunjo ./cmd/wunjo

fmt:
	gofmt -w cmd internal
