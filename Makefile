.PHONY: test lint test-all clean

# Unit tests
test:
	go test -race ./...

# Linting
lint:
	golangci-lint run

# CI calls this target
test-all: lint test
	@echo "All tests passed"

# Clean test cache
clean:
	go clean -testcache
