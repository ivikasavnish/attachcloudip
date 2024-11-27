#!/bin/bash

LOCAL_PORT=5000
REMOTE_PORT=${1:-8080}

# Load config
CONFIG_FILE="lightsail.yaml"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: $CONFIG_FILE not found"
    exit 1
fi

# Parse YAML configuration using awk
parse_yaml() {
    local prefix=$2
    local s='[[:space:]]*' w='[a-zA-Z0-9_]*' fs=$(echo @|tr @ '\034')
    sed -ne "s|^\($s\):|\1|" \
         -e "s|^\($s\)\($w\)$s:$s[\"']\(.*\)[\"']$s\$|\1$fs\2$fs\3|p" \
         -e "s|^\($s\)\($w\)$s:$s\(.*\)$s\$|\1$fs\2$fs\3|p"  $1 |
    awk -F$fs '{
        indent = length($1)/2;
        vname[indent] = $2;
        for (i in vname) {if (i > indent) {delete vname[i]}}
        if (length($3) > 0) {
            vn=""; for (i=0; i<indent; i++) {vn=(vn)(vname[i])("_")}
            printf("%s%s=\"%s\"\n", "'$prefix'",vn$2,$3);
        }
    }'
}

# Load configuration
eval $(parse_yaml $CONFIG_FILE)

echo "Deploying tunnel server to $host..."

# Check if binaries are built
if [ ! -f "bin/tunnel-server" ]; then
    echo "Building tunnel server..."
    go build -o bin/tunnel-server cmd/server/main.go
fi

# Create a temporary directory for deployment files
DEPLOY_DIR=$(mktemp -d)
cp bin/tunnel-server $DEPLOY_DIR/
cp tunnel-server.service $DEPLOY_DIR/
cp $CONFIG_FILE $DEPLOY_DIR/

# Transfer files to remote server
echo "Transferring files to remote server..."
scp -i "$ssh_key_path" -r $DEPLOY_DIR/* $username@$host:/tmp/

# Install and start the service on remote server
echo "Installing and starting the service..."
ssh -i "$ssh_key_path" $username@$host << 'EOF'
    # Stop existing service if running
    sudo systemctl stop tunnel-server || true
    
    # Copy files to appropriate locations
    sudo cp /tmp/tunnel-server /usr/local/bin/
    sudo chmod +x /usr/local/bin/tunnel-server
    sudo cp /tmp/tunnel-server.service /etc/systemd/system/
    sudo mkdir -p /etc/tunnel-server
    sudo cp /tmp/lightsail.yaml /etc/tunnel-server/
    
    # Reload systemd and start service
    sudo systemctl daemon-reload
    sudo systemctl enable tunnel-server
    sudo systemctl start tunnel-server
    
    # Check service status
    echo "Service status:"
    sudo systemctl status tunnel-server
    
    # Clean up temporary files
    rm /tmp/tunnel-server
    rm /tmp/tunnel-server.service
    rm /tmp/lightsail.yaml
EOF

# Clean up local temporary directory
rm -rf $DEPLOY_DIR

echo "Deployment complete!"

# Test the connection
echo "Testing connection to tunnel server..."
sleep 5
curl -s "http://$host:8080/health" || echo "Note: Health check failed. The server might still be starting up."

echo "You can now start clients using:"
echo "./bin/tunnel-client -config $CONFIG_FILE -id <client-id> -port <port>"

echo "Setting up tunnel: localhost:$LOCAL_PORT -> $host:$REMOTE_PORT"

# Check if local server is already running
if ! curl -s http://localhost:$LOCAL_PORT/ > /dev/null; then
    echo "Error: Local server not running on port $LOCAL_PORT"
    echo "Please start the local server first"
    exit 1
fi

echo "Local server is running on port $LOCAL_PORT"

# Kill any existing tunnels
pkill -f "ssh.*:$REMOTE_PORT:localhost:$LOCAL_PORT" || true
sleep 1

# Setup tunnel with verbose output
echo "Setting up SSH tunnel..."
ssh -v -i "$ssh_key_path" -N -R "*:$REMOTE_PORT:localhost:$LOCAL_PORT" \
    -o "ServerAliveInterval=60" \
    -o "ExitOnForwardFailure=yes" \
    -o "StrictHostKeyChecking=no" \
    "$username@$host" &

TUNNEL_PID=$!
sleep 2

# Check if tunnel is running
if ! ps -p $TUNNEL_PID > /dev/null; then
    echo "Failed to establish tunnel"
    exit 1
fi

echo -e "\nTesting local server:"
curl -v http://localhost:$LOCAL_PORT/

echo -e "\nTo test remote tunnel, run:"
echo "curl http://$host:$REMOTE_PORT/"
echo "Press Ctrl+C to stop"

# Monitor connections
while kill -0 $TUNNEL_PID 2>/dev/null; do
    echo "Active connections:"
    netstat -ant | grep ":$LOCAL_PORT.*ESTABLISHED" || true
    sleep 5
done

# Cleanup
trap "kill $TUNNEL_PID 2>/dev/null" EXIT
wait
