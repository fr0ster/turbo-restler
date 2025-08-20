#!/bin/bash

# Скрипт збірки turbo-restler
# Компілює всі пакети та приклади в папку bin

set -e

echo "🔨 Building turbo-restler..."

# Створення папки bin якщо не існує
mkdir -p bin

# Очищення попередніх збірок
echo "🧹 Cleaning previous builds..."
rm -rf bin/*

# Компіляція основних пакетів
echo "📦 Building packages..."

# WebSocket пакет
echo "  - Building web_socket package..."
go build -o bin/websocket_test ./web_socket/

# REST API пакет  
echo "  - Building rest_api package..."
go build -o bin/restapi_test ./rest_api/

# Компіляція прикладів
echo "🚀 Building examples..."

# WebSocket приклад
echo "  - Building websocket_example..."
go build -o bin/websocket_example ./examples/

# Компіляція тестових бінарників
echo "🧪 Building test binaries..."

# WebSocket тести
echo "  - Building web_socket tests..."
go test -c -o bin/websocket_tests ./web_socket/

# REST API тести
echo "  - Building rest_api tests..."
go test -c -o bin/restapi_tests ./rest_api/

# Перевірка розміру бінарників
echo "📊 Binary sizes:"
ls -lh bin/

echo "✅ Build completed successfully!"
echo "📁 Binaries are in: $(pwd)/bin/"
