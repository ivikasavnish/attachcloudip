#!/bin/bash

# Default values
LOCAL_PORT=5001
REMOTE_PORT=8080
KEEPALIVE=60

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -l|--local-port)
            LOCAL_PORT="$2"
            shift 2
            ;;
        -r|--remote-port)
            REMOTE_PORT="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Load configuration
CONFIG_FILE="$(dirname "$0")/lightsail.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: lightsail.yaml not found"
    exit 1
fi

# Read configuration using grep and sed
HOST=$(grep "^host:" "$CONFIG_FILE" | sed 's/^host: *//g')
USERNAME=$(grep "^username:" "$CONFIG_FILE" | sed 's/^username: *//g')
SSH_KEY=$(grep "^ssh_key_path:" "$CONFIG_FILE" | sed 's/^ssh_key_path: *//g')

# Validate configuration
if [ -z "$HOST" ] || [ -z "$USERNAME" ] || [ -z "$SSH_KEY" ]; then
    echo "Error: Missing required configuration in lightsail.yaml"
    exit 1
fi

if [ ! -f "$SSH_KEY" ]; then
    echo "Error: SSH key not found at $SSH_KEY"
    exit 1
fi

echo "Configuration:"
echo "Host: $HOST"
echo "Username: $USERNAME"
echo "SSH Key: $SSH_KEY"
echo "Local Port: $LOCAL_PORT"
echo "Remote Port: $REMOTE_PORT"

# Check if local port is available
if nc -z localhost "$LOCAL_PORT" 2>/dev/null; then
    echo "Error: Local port $LOCAL_PORT is already in use"
    exit 1
fi

# Start test server
echo "Starting test server..."
cat > test_server.py << 'EOL'
from http.server import HTTPServer, BaseHTTPRequestHandler
import json

class TestHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-type', 'application/json')
        self.end_headers()
        response = {'status': 'ok', 'message': 'Test server is running'}
        self.wfile.write(json.dumps(response).encode())

    def log_message(self, format, *args):
        return  # Suppress logging

if __name__ == '__main__':
    import sys
    port = int(sys.argv[1])
    server = HTTPServer(('localhost', port), TestHandler)
    print(f'Test server listening on port {port}')
    server.serve_forever()
EOL

python3 test_server.py "$LOCAL_PORT" &
SERVER_PID=$!
sleep 2

if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo "Error: Failed to start test server"
    exit 1
fi

# Setup SSH tunnel
echo "Setting up SSH tunnel..."
ssh -i "$SSH_KEY" -N -R "$REMOTE_PORT:localhost:$LOCAL_PORT" \
    -o "ServerAliveInterval=$KEEPALIVE" \
    -o "ExitOnForwardFailure=yes" \
    -o "StrictHostKeyChecking=no" \
    "$USERNAME@$HOST" &

TUNNEL_PID=$!
sleep 2

if ! kill -0 $TUNNEL_PID 2>/dev/null; then
    echo "Error: Failed to establish SSH tunnel"
    kill $SERVER_PID 2>/dev/null
    exit 1
fi

echo "SSH tunnel established successfully"

# Monitor tunnel and test server
while kill -0 $TUNNEL_PID 2>/dev/null && kill -0 $SERVER_PID 2>/dev/null; do
    CONNECTIONS=$(netstat -ant | grep ":$LOCAL_PORT.*ESTABLISHED" | wc -l)
    echo "Active connections: $CONNECTIONS"
    sleep 5
done

# Cleanup
kill $TUNNEL_PID 2>/dev/null
kill $SERVER_PID 2>/dev/null
