#!/bin/bash
set -e

# Load configuration
CONFIG_FILE="$(dirname "$0")/lightsail.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: lightsail.yaml not found"
    exit 1
fi

# Read configuration
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

# Function to run commands on remote server
run_remote_command() {
    ssh -i "$SSH_KEY" -o StrictHostKeyChecking=no "$USERNAME@$HOST" "$1"
}

# Function to check if service exists on remote
check_service_exists() {
    run_remote_command "sudo systemctl list-unit-files | grep -q attachcloudip.service"
    return $?
}

# Build the server locally
echo "Building server..."
GOOS=linux GOARCH=amd64 go build -o bin/server cmd/server/main.go

# Create necessary directories on remote
echo "Creating directories on remote server..."
run_remote_command "mkdir -p ~/attachcloudip/bin ~/attachcloudip/config ~/attachcloudip/logs"

# Copy files to remote server
echo "Copying files to remote server..."
scp -i "$SSH_KEY" bin/server "$USERNAME@$HOST:~/attachcloudip/bin/"
scp -i "$SSH_KEY" config/tunnel.yaml "$USERNAME@$HOST:~/attachcloudip/config/"

# Setup service on remote
echo "Setting up service on remote server..."
cat > tunnel-server.service << EOF
[Unit]
Description=AttachCloudIP Server
After=network.target

[Service]
Type=simple
User=$USERNAME
WorkingDirectory=/home/$USERNAME/attachcloudip
ExecStart=/home/$USERNAME/attachcloudip/bin/server -config config/tunnel.yaml
Restart=always
RestartSec=5
StandardOutput=append:/home/$USERNAME/attachcloudip/logs/server.log
StandardError=append:/home/$USERNAME/attachcloudip/logs/server.error.log

[Install]
WantedBy=multi-user.target
EOF

# Copy and enable service
scp -i "$SSH_KEY" tunnel-server.service "$USERNAME@$HOST:/tmp/"
run_remote_command "sudo mv /tmp/tunnel-server.service /etc/systemd/system/attachcloudip.service"
run_remote_command "sudo systemctl daemon-reload"
run_remote_command "sudo systemctl enable attachcloudip"

# Configure firewall on remote
echo "Configuring firewall on remote server..."
run_remote_command "
    sudo ufw allow 9999/tcp && \
    sudo ufw allow 9998/tcp && \
    sudo ufw allow 9997/tcp && \
    sudo ufw allow 22/tcp && \
    sudo ufw --force enable
"

# Restart service
echo "Restarting service..."
run_remote_command "sudo systemctl restart attachcloudip"

# Check service status
echo "Checking service status..."
run_remote_command "sudo systemctl status attachcloudip"

# Show logs
echo "Recent logs:"
run_remote_command "tail -n 20 ~/attachcloudip/logs/server.log"

echo "Setup complete!"
echo "HTTP API running on port 9999"
echo "gRPC server running on port 9998"
echo "Registration server running on port 9997"

# Function to show status
show_status() {
    echo "Service status:"
    run_remote_command "sudo systemctl status attachcloudip"
    echo "Recent logs:"
    run_remote_command "tail -n 20 ~/attachcloudip/logs/server.log"
}

# Function to restart service
restart_service() {
    echo "Restarting service..."
    run_remote_command "sudo systemctl restart attachcloudip"
    show_status
}

# Handle command line arguments
case "${1:-}" in
    "status")
        show_status
        ;;
    "restart")
        restart_service
        ;;
    "logs")
        run_remote_command "tail -f ~/attachcloudip/logs/server.log"
        ;;
    *)
        if [ -n "${1:-}" ]; then
            echo "Unknown command: $1"
            echo "Available commands: status, restart, logs"
            exit 1
        fi
        ;;
esac
