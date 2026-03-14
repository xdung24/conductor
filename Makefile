ifeq ($(OS),Windows_NT)
    BINARY := service-monitor.exe
else
    BINARY := service-monitor
endif
MAIN     := ./cmd/server
BUILD_FLAGS := -ldflags="-s -w"

## build: compile the binary
.PHONY: build
build:
	rm -f $(BINARY)
	go build $(BUILD_FLAGS) -o $(BINARY) $(MAIN)

## run: build and run locally
.PHONY: run
run: build
	./$(BINARY)

## dev: run with live env (no build cache optimisation)
.PHONY: dev
dev:
	go run $(MAIN)

## test: run all tests
.PHONY: test
test:
	go test ./... -v

## lint: run linters
.PHONY: lint
lint:
	go vet ./...
	@command -v staticcheck >/dev/null && staticcheck ./... || echo "staticcheck not installed, skipping"

## clean: remove build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY)
	rm -rf data/service-monitor.db

## docker-build: build Docker image
.PHONY: docker-build
docker-build:
	docker build -t service-monitor:latest .

## docker-run: run via Docker Compose
.PHONY: docker-run
docker-run:
	docker compose up --build

## tidy: clean up go.mod / go.sum
.PHONY: tidy
tidy:
	go mod tidy

.PHONY: help
help:
	@grep -E '^##' Makefile | sed 's/## /  /'
