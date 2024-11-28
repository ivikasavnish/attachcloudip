#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Test script
run_integration_test() {
    echo "Starting Integration Test..."

    # Start server in background
    ./bin/server -port 9999 &
    SERVER_PID=$!
    sleep 2  # Give server time to start

    # Test server health endpoint
    echo "Testing server health endpoint..."
    HEALTH_RESPONSE=$(curl -s http://localhost:9999/health)
    if [[ "$HEALTH_RESPONSE" == "Server is healthy" ]]; then
        echo "${GREEN}✓ Health endpoint working${NC}"
    else
        echo "${RED}✗ Health endpoint failed${NC}"
        kill $SERVER_PID
        exit 1
    fi

    # Register client
    echo "Registering client..."
    CLIENT_RESPONSE=$(./bin/client -server localhost:9999 -paths /test,/demo -protocol tcp)
    
    # Check client registration output
    if [[ "$CLIENT_RESPONSE" == *"Client Registration Successful"* ]]; then
        echo "${GREEN}✓ Client Registration Successful${NC}"
    else
        echo "${RED}✗ Client Registration Failed${NC}"
        kill $SERVER_PID
        exit 1
    fi

    # Test status endpoint
    echo "Testing status endpoint..."
    STATUS_RESPONSE=$(curl -s http://localhost:9999/status)
    if [[ "$STATUS_RESPONSE" == *"total_clients"* ]]; then
        echo "${GREEN}✓ Status endpoint working${NC}"
    else
        echo "${RED}✗ Status endpoint failed${NC}"
        kill $SERVER_PID
        exit 1
    fi

    # Test client list endpoint
    echo "Testing client list endpoint..."
    CLIENTS_RESPONSE=$(curl -s http://localhost:9999/clients)
    if [[ "$CLIENTS_RESPONSE" == *"ClientID"* ]]; then
        echo "${GREEN}✓ Clients list endpoint working${NC}"
    else
        echo "${RED}✗ Clients list endpoint failed${NC}"
        kill $SERVER_PID
        exit 1
    fi

    # Kill server
    kill $SERVER_PID
    wait $SERVER_PID 2>/dev/null

    echo "${GREEN}✓ All tests passed successfully!${NC}"
}

# Ensure binaries are built
./build.sh

# Run the test
run_integration_test