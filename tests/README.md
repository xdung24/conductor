# Tests

This directory contains functional tests (Bruno) and performance tests (k6).

```
tests/
  functional/     Bruno collection — API correctness checks
  performance/    k6 scripts — load and performance tests
```

---

## Prerequisites

### Bruno CLI (`bru`)

```bash
# npm
npm install -g @usebruno/cli

# Homebrew (macOS/Linux)
brew install bruno
```

Verify: `bru --version`

### k6

```bash
# macOS/Linux — Homebrew
brew install k6

# Windows — winget
winget install k6 --source winget

# Windows — Chocolatey
choco install k6

# Docker (no install needed)
docker pull grafana/k6
```

Verify: `k6 version`

---

## Configuration

### Functional tests — Bruno environment

Environments live in `functional/environments/`. The default `local.bru` targets `http://localhost:3001`.

To override, edit `functional/environments/local.bru`:

```
vars {
  baseUrl: http://localhost:3001
}
```

Or add a new environment file (e.g. `staging.bru`) with the same structure and pass `--env staging` when running.

### Performance tests — k6 environment

Environment config files live in `performance/environments/`. The default is `local.json`:

```json
{
  "baseUrl": "http://localhost:3001"
}
```

To target a different environment, create a new JSON file (e.g. `staging.json`) with the same structure and pass its name via `ENV` when running.

---

## Running tests

Start the server before running any tests:

```bash
go run ./cmd/server
# or
docker compose up -d
```

### 1. Functional tests (Bruno)

```bash
# Run all requests in the collection against the local environment
bru run --env local

# Run a single request file
bru run get-healthz.bru --env local
```

A passing run exits with code `0`. Any failed assertion exits with a non-zero code.

### 2. Performance tests (k6)

```bash
# Run against the default local environment
k6 run get-healthz.k6

# Run against a different environment
k6 run -e ENV=staging get-healthz.k6
```

The script ramps up to 10 virtual users over 10 s, holds for 30 s, then ramps down.
It fails if the error rate exceeds 1% or the p95 latency exceeds 200 ms.
