# Makefile
.PHONY: all build test test-coverage test-integration clean install lint demo

BINARY := strung
VERSION := 0.2.0
LDFLAGS := -ldflags "-X main.versionStr=$(VERSION)"

all: test build

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/strung

test:
	go test -v -race ./...

test-coverage:
	go test -coverprofile=coverage.out -race ./...
	@go tool cover -func=coverage.out | grep total
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | tr -d '%'); \
	if [ $$(echo "$$COVERAGE < 80" | bc -l) -eq 1 ]; then \
		echo "FAIL: Coverage $$COVERAGE% below 80% threshold"; \
		exit 1; \
	else \
		echo "PASS: Coverage $$COVERAGE% meets threshold"; \
	fi

test-integration:
	go test -v -tags=integration ./cmd/strung/...

clean:
	rm -f $(BINARY) $(BINARY).exe coverage.out

install:
	go install $(LDFLAGS) ./cmd/strung

lint:
	golangci-lint run ./...

demo:
	@echo '{"project":"/test","files_scanned":1,"findings":[{"file":"test.ts","line":42,"severity":"critical","category":"null-safety","message":"Unguarded access"}],"summary":{"critical":1,"warning":0,"info":0}}' | go run ./cmd/strung

# Phase 2 additions
test-db:
	go test -v ./pkg/db

clean-all: clean
	rm -f .strung.db .strung.db-shm .strung.db-wal .strung.db-journal
	rm -f .phase2-issue-ids.env

demo-sync:
	@echo "=== Sync Demo (dry-run) ===" && \
	cat testdata/ubs-sample.json | go run ./cmd/strung sync --dry-run --db-path=/tmp/demo.db || true && \
	rm -f /tmp/demo.db /tmp/demo.db-*
