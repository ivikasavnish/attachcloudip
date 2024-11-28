#!/bin/bash
set -e

# Create bin directory if it doesn't exist
mkdir -p bin

# Build function
build_binary() {
    local dir=$1
    local binary_name=$(basename "$dir")
    
    echo "Building $binary_name..."
    
    # Find all .go files in the directory
    GO_FILES=$(find "$dir" -maxdepth 1 -name "*.go")
    
    # Build with cross-compilation support
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "bin/$binary_name" $GO_FILES
    
    if [ $? -eq 0 ]; then
        echo "Successfully built $binary_name"
        chmod +x "bin/$binary_name"
    else
        echo "Failed to build $binary_name"
        exit 1
    fi
}

# Update dependencies
go mod tidy

# Build server
build_binary cmd/server

# Build client
build_binary cmd/client

# List built binaries
echo "Build completed. Binaries:"
ls -l bin/