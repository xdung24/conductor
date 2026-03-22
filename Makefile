ifeq ($(OS),Windows_NT)
    BINARY := conductor.exe
else
    BINARY := conductor
endif
MAIN     := ./cmd/server
BUILD_FLAGS := -ldflags="-s -w"

## build: compile the binary
.PHONY: build
build:
	rm -f $(BINARY)
	go build -v $(BUILD_FLAGS) -o $(BINARY) $(MAIN)

## test: run all tests
.PHONY: test
test:
	go vet ./... &&	go test ./... -v

## coverage: run tests and generate HTML coverage report
.PHONY: coverage
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report written to coverage.html"

## lint: run linters
.PHONY: lint
lint:
	go vet ./...
	@grep -rn '\.Exec\b\|\.Query\b\|\.QueryRow\b' --include='*.go' internal/ cmd/ | grep -v 'c\.Query(' | grep -v 'c\.PostForm(' | grep -v '\.URL\.Query(' | grep -v '\.URL\.Query()' && echo "ERROR: use ExecContext/QueryContext/QueryRowContext instead" && exit 1 || echo "noctx: OK"
	@command -v staticcheck >/dev/null && staticcheck ./... || echo "staticcheck not installed, skipping"

## clean: remove build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY)
	rm -rf data/users.db

## docker-build: build Docker image
.PHONY: docker-build
docker-build:
	docker build -t conductor:latest .

## docker-run: run via Docker Compose
.PHONY: docker-run
docker-run:
	docker compose up --build

.PHONY: help
help:
	@grep -E '^##' Makefile | sed 's/## /  /'
