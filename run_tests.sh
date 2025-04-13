#!/bin/bash

# Кількість повторів (за замовчуванням 10)
REPEATS=${1:-10}
# Шлях до пакету (за замовчуванням все)
PKG=${2:-"./..."}

echo "🔁 Running tests $REPEATS times in package: $PKG"

for i in $(seq 1 $REPEATS); do
  echo ""
  echo "=============================="
  echo "🔂 Run #$i"
  echo "=============================="
  if ! go test -v "$PKG"; then
    echo "❌ Tests failed on run #$i"
    exit 1
  fi
done

echo ""
echo "✅ All $REPEATS runs passed successfully!"
echo "=============================="