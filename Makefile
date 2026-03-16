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
	go build -v $(BUILD_FLAGS) -o $(BINARY) $(MAIN)

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

## check-templates: parse all HTML templates for errors
.PHONY: check-templates
check-templates:
	go test ./internal/web/... -run TestTemplatesParse -v

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
