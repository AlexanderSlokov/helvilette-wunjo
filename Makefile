.PHONY: test test-integration vet build fmt

test: vet
	go test -race ./...
	python3 -B -m unittest discover -s tools/portgen -p 'test_*.py'

# Needs Docker and the OpenSSH client tools. The sshexec test starts sshd in a
# container; the suite in test/integration starts Docker hosts with Compose and
# deploys the sample apps, which takes tens of minutes.
test-integration: vet
	go test -tags integration -count=1 ./internal/kamal/sshexec/
	cd test/integration && go test -tags integration -count=1 -timeout 120m .

vet:
	go vet ./...

build:
	go build -o bin/wunjo ./cmd/wunjo

fmt:
	gofmt -w cmd internal
