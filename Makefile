BIN := "./bin/imgresizr"
DOCKER_IMG="imgresizr:develop"

GIT_HASH := $(shell git log --format="%h" -n 1)
LDFLAGS := -X main.release="develop" -X main.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%S) -X main.gitHash=$(GIT_HASH)

build:
	go build -v -o $(BIN) -ldflags "$(LDFLAGS)" ./cmd/imgresizr

run-local: build
	$(BIN)

build-img:
	docker build \
		--build-arg=LDFLAGS="$(LDFLAGS)" \
		-t $(DOCKER_IMG) \
		-f ./build/imgresizr/Dockerfile .

run: build-img
	docker-compose -f ./build/imgresizr/docker-compose.yml up --remove-orphans

version: build
	$(BIN) version

test: build
	go test -race -count 100 ./internal/... ./pkg/... ./test/...
	$(MAKE) integration-test
#	go test -race ./internal/... ./pkg/...

install-lint-deps:
	(which golangci-lint > /dev/null) || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.54.2

lint: install-lint-deps
	golangci-lint run ./...

integration-test:
	docker-compose -f ./test/integration/testimgsrv/docker-compose.yml up -d --remove-orphans || true
	go test -tags integration ./test/...
#   run test
	docker-compose -f ./test/integration/testimgsrv/docker-compose.yml down || true

.PHONY: build run build-img run-local version test lint integration-test
