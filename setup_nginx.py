#!/usr/bin/env python3
import subprocess
import yaml
from pathlib import Path

def load_config():
    config_path = Path.home() / 'Downloads/configs/lightsail.yaml'
    with open(config_path) as f:
        return yaml.safe_load(f)

def setup_nginx():
    config = load_config()
    
    # Commands to install and configure nginx
    commands = [
        "sudo apt-get update",
        "sudo apt-get install -y nginx",
        "sudo systemctl enable nginx",
        "sudo systemctl start nginx",
        # Allow nginx in firewall
        "sudo ufw allow 'Nginx Full'",
        # Create nginx config directory
        "sudo mkdir -p /etc/nginx/sites-available",
        # Backup default config
        "sudo cp /etc/nginx/nginx.conf /etc/nginx/nginx.conf.backup",
    ]
    
    ssh_base = [
        "ssh",
        "-i", config["ssh_key_path"],
        f"{config['username']}@{config['host']}"
    ]
    
    print("Setting up Nginx on remote server...")
    for cmd in commands:
        print(f"Running: {cmd}")
        try:
            result = subprocess.run(ssh_base + [cmd], capture_output=True, text=True)
            if result.stdout:
                print("Output:", result.stdout)
            if result.stderr:
                print("Errors:", result.stderr)
        except subprocess.CalledProcessError as e:
            print(f"Error running command: {e}")
            if "already installed" not in str(e):
                raise

    print("\nNginx setup completed!")
    print("You can now use the SSH Tunnel Manager to create subdomain mappings.")

if __name__ == "__main__":
    setup_nginx()
