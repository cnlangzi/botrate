.PHONY: all test test-short test-race test-coverage bench bench-all clean help

# Go commands
GOCMD = go
GOTEST = $(GOCMD) test
GOBUILD = $(GOCMD) build
GOBENCH = $(GOCMD) bench

# Test flags
TEST_FLAGS = -short
RACE_FLAGS = -race
COVERAGE_FLAGS = -coverprofile=coverage.txt -covermode=atomic

# Default target
all: build

# Build the project
build:
	$(GOBUILD) ./...

# Run short tests (fast, for CI)
test-short:
	$(GOTEST) -short ./...

# Run tests with race detector
test-race:
	$(GOTEST) $(RACE_FLAGS) ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) $(COVERAGE_FLAGS) ./...

# Run all tests (short + race)
test: test-short test-race

# Run benchmarks
bench:
	$(GOBENCH) -run=^$ -bench=. -benchmem -cpu=1,4

# Run all benchmarks with more detail
bench-all:
	$(GOBENCH) -run=^$ -bench=. -benchmem -cpu=1,4,8 -timeout 120s

# Clean build artifacts
clean:
	$(GOCMD) clean
	rm -f coverage.txt
	rm -f benchmark_output.txt
	rm -f *.test

# Show this help message
help:
	@echo "BotRate Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  all          - Build the project (default)"
	@echo "  build        - Build all packages"
	@echo "  test         - Run short tests and race tests"
	@echo "  test-short   - Run short tests (fast, for CI)"
	@echo "  test-race    - Run tests with race detector"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  bench        - Run benchmarks (1 and 4 CPUs)"
	@echo "  bench-all    - Run all benchmarks (1, 4, 8 CPUs)"
	@echo "  clean        - Clean build artifacts"
	@echo "  help         - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make test         # Run all tests"
	@echo "  make bench        # Run benchmarks"
	@echo "  make test-coverage# Generate coverage report"
