#!/bin/bash
set -e

# Function to print colored output
print_status() {
    echo -e "\033[1;34m>>> $1\033[0m"
}

print_error() {
    echo -e "\033[1;31m>>> Error: $1\033[0m"
}

print_success() {
    echo -e "\033[1;32m>>> Success: $1\033[0m"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed"
    exit 1
fi

# Clean previous builds
print_status "Cleaning previous builds..."
rm -rf bin/
mkdir -p bin/

# Generate proto files
print_status "Generating proto files..."
# Commented out proto generation
# protoc --go_out=. \
#     --go_opt=paths=source_relative \
#     --go-grpc_out=. \
#     --go-grpc_opt=paths=source_relative \
#     proto/tunnel.proto

# Update dependencies
print_status "Updating dependencies..."
go mod tidy

# Build server
print_status "Building server..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/server cmd/server/main.go
if [ $? -eq 0 ]; then
    print_success "Server built successfully: bin/server"
else
    print_error "Failed to build server"
    exit 1
fi

# Build client
print_status "Building client..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/client cmd/client/main.go
if [ $? -eq 0 ]; then
    print_success "Client built successfully: bin/client"
else
    print_error "Failed to build client"
    exit 1
fi

# Make binaries executable
chmod +x bin/server bin/client

print_success "Build completed successfully!"
echo "Server binary: bin/server"
echo "Client binary: bin/client"
