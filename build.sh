#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

# Get the root directory of the project
root=$(pwd)

echo "🚀 Starting Controly build process..."

echo "📦 Building SDK..."
cd "${root}"/sdk
pnpm install
pnpm run build
echo "✅ SDK build complete."

echo "🕹️ Building Controller..."
cd "${root}"/server/controller
pnpm install
pnpm run build
echo "✅ Controller build complete."

echo "🌐 Building Server..."
cd "${root}"/server
go build -o "${root}"/controly
echo "✅ Server build complete."

echo "🎉 Controly build finished successfully!"
