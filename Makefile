# Makefile для turbo-restler
# Використання: make [target]

.PHONY: help build clean test test-race test-coverage lint format examples install-deps

# Змінні
BIN_DIR = bin
GO = go
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Кольори для виводу
GREEN = \033[32m
YELLOW = \033[33m
RED = \033[31m
RESET = \033[0m

# Допомога
help:
	@echo "$(GREEN)🚀 Turbo-Restler Makefile$(RESET)"
	@echo ""
	@echo "$(YELLOW)Доступні команди:$(RESET)"
	@echo "  build          - Збірка всіх пакетів та прикладів"
	@echo "  clean          - Очищення папки bin"
	@echo "  test           - Запуск всіх тестів"
	@echo "  test-race      - Запуск тестів з race detector"
	@echo "  test-coverage  - Запуск тестів з покриттям коду"
	@echo "  lint           - Перевірка коду з golangci-lint"
	@echo "  format         - Форматування коду"
	@echo "  examples       - Збірка прикладів"
	@echo "  install-deps   - Встановлення залежностей"
	@echo "  help           - Показати цю довідку"
	@echo ""
	@echo "$(YELLOW)Змінні:$(RESET)"
	@echo "  GOOS=$(GOOS)"
	@echo "  GOARCH=$(GOARCH)"
	@echo "  VERSION=$(VERSION)"

# Створення папки bin якщо не існує
$(BIN_DIR):
	@mkdir -p $(BIN_DIR)
	@echo "$(GREEN)📁 Created $(BIN_DIR) directory$(RESET)"

# Збірка основних пакетів
build: $(BIN_DIR)
	@echo "$(GREEN)🔨 Building turbo-restler...$(RESET)"
	@echo "$(YELLOW)📦 Building packages...$(RESET)"
	
	# WebSocket пакет
	@echo "  - Building web_socket package..."
	@$(GO) build -o $(BIN_DIR)/websocket_test ./web_socket/
	
	# REST API пакет
	@echo "  - Building rest_api package..."
	@$(GO) build -o $(BIN_DIR)/restapi_test ./rest_api/
	
	@echo "$(GREEN)✅ Packages built successfully!$(RESET)"

# Збірка прикладів
examples: $(BIN_DIR)
	@echo "$(YELLOW)🚀 Building examples...$(RESET)"
	
	# WebSocket приклад
	@echo "  - Building websocket_example..."
	@$(GO) build -o $(BIN_DIR)/websocket_example ./examples/
	
	@echo "$(GREEN)✅ Examples built successfully!$(RESET)"

# Збірка тестових бінарників
test-binaries: $(BIN_DIR)
	@echo "$(YELLOW)🧪 Building test binaries...$(RESET)"
	
	# WebSocket тести
	@echo "  - Building web_socket tests..."
	@$(GO) test -c -o $(BIN_DIR)/websocket_tests ./web_socket/
	
	# REST API тести
	@echo "  - Building rest_api tests..."
	@$(GO) test -c -o $(BIN_DIR)/restapi_tests ./rest_api/
	
	@echo "$(GREEN)✅ Test binaries built successfully!$(RESET)"

# Повна збірка
all: build examples test-binaries
	@echo "$(GREEN)📊 Binary sizes:$(RESET)"
	@ls -lh $(BIN_DIR)/
	@echo "$(GREEN)✅ Full build completed successfully!$(RESET)"
	@echo "$(YELLOW)📁 Binaries are in: $(shell pwd)/$(BIN_DIR)/$(RESET)"

# Очищення
clean:
	@echo "$(YELLOW)🧹 Cleaning build artifacts...$(RESET)"
	@rm -rf $(BIN_DIR)/*
	@echo "$(GREEN)✅ Clean completed!$(RESET)"

# Запуск тестів
test:
	@echo "$(YELLOW)🧪 Running tests...$(RESET)"
	@$(GO) test -v ./...
	@echo "$(GREEN)✅ Tests completed!$(RESET)"

# Запуск "тихих" тестів (без зайвих повідомлень)
test-quiet:
	@echo "$(YELLOW)🔇 Running quiet tests...$(RESET)"
	@$(GO) test -v ./... 2>&1 | grep -v "broken pipe\|Server write error\|Server read error" || true
	@echo "$(GREEN)✅ Quiet tests completed!$(RESET)"

# Запуск тестів з race detector
test-race:
	@echo "$(YELLOW)🏃 Running tests with race detector...$(RESET)"
	@$(GO) test -race -v ./...
	@echo "$(GREEN)✅ Race tests completed!$(RESET)"

# Запуск тестів з покриттям
test-coverage:
	@echo "$(YELLOW)📊 Running tests with coverage...$(RESET)"
	@$(GO) test -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✅ Coverage report generated: coverage.html$(RESET)"

# Перевірка коду
lint:
	@echo "$(YELLOW)🔍 Running linter...$(RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "$(RED)❌ golangci-lint not installed. Install with:$(RESET)"; \
		echo "   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Форматування коду
format:
	@echo "$(YELLOW)✨ Formatting code...$(RESET)"
	@$(GO) fmt ./...
	@echo "$(GREEN)✅ Code formatted!$(RESET)"

# Встановлення залежностей
install-deps:
	@echo "$(YELLOW)📦 Installing dependencies...$(RESET)"
	@$(GO) mod download
	@$(GO) mod tidy
	@echo "$(GREEN)✅ Dependencies installed!$(RESET)"

# Перевірка залежностей
deps-check:
	@echo "$(YELLOW)🔍 Checking dependencies...$(RESET)"
	@$(GO) mod verify
	@$(GO) list -m all
	@echo "$(GREEN)✅ Dependencies checked!$(RESET)"

# Запуск прикладів
run-example: examples
	@echo "$(YELLOW)🚀 Running websocket example...$(RESET)"
	@./$(BIN_DIR)/websocket_example

# Перевірка якості коду
quality: lint test test-race
	@echo "$(GREEN)✅ Quality checks completed!$(RESET)"

# Допомога з короткими командами
b: build
e: examples
t: test
tq: test-quiet
c: clean
h: help

# Default target
.DEFAULT_GOAL := help
