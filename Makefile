.PHONY: test test-integration vet build fmt release install

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

# Release builds are static, without local paths or debug symbols. Each build
# overwrites its file in dist/.
RELEASE_FLAGS = -trimpath -ldflags "-s -w"
PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
PREFIX ?= $(HOME)/.local

release:
	mkdir -p dist
	for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build $(RELEASE_FLAGS) -o dist/wunjo_$${os}_$${arch} ./cmd/wunjo || exit 1; \
	done
	cd dist && sha256sum wunjo_* > SHA256SUMS

install:
	mkdir -p $(PREFIX)/bin
	CGO_ENABLED=0 go build $(RELEASE_FLAGS) -o $(PREFIX)/bin/wunjo ./cmd/wunjo

fmt:
	gofmt -w cmd internal
