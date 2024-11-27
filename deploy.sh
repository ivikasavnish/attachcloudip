#!/bin/bash

# Load configuration from lightsail.yaml
HOST=$(grep 'host:' lightsail.yaml | awk '{print $2}')
USERNAME=$(grep 'username:' lightsail.yaml | awk '{print $2}')
SSH_KEY=$(grep 'ssh_key_path:' lightsail.yaml | awk '{print $2}')

echo "Building server..."
go build -o bin/server cmd/server/main.go

echo "Creating directories on remote server..."
ssh -i "$SSH_KEY" "$USERNAME@$HOST" "mkdir -p ~/attachcloudip/{bin,config,logs}"

echo "Copying files to remote server..."
scp -i "$SSH_KEY" bin/server "$USERNAME@$HOST:~/attachcloudip/bin/"
scp -i "$SSH_KEY" config/tunnel.yaml "$USERNAME@$HOST:~/attachcloudip/config/"
scp -i "$SSH_KEY" tunnel-server.service "$USERNAME@$HOST:~/attachcloudip/"

echo "Setting up systemd service..."
ssh -i "$SSH_KEY" "$USERNAME@$HOST" "sudo mv ~/attachcloudip/tunnel-server.service /etc/systemd/system/ && \
    sudo systemctl daemon-reload && \
    sudo systemctl enable tunnel-server.service && \
    sudo systemctl restart tunnel-server.service"

echo "Deployment completed!"
