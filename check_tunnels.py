#!/usr/bin/env python3
import subprocess
import yaml
from pathlib import Path

def load_config():
    config_path = Path.home() / 'Downloads/configs/lightsail.yaml'
    with open(config_path) as f:
        return yaml.safe_load(f)

def check_tunnels():
    config = load_config()
    ssh_cmd = [
        'ssh',
        '-i', config['ssh_key_path'],
        f"{config['username']}@{config['host']}",
        'ps aux | grep ssh.*:5000 || echo "No tunnel found for port 5000"'
    ]
    
    print("Checking for active SSH tunnels...")
    result = subprocess.run(ssh_cmd, capture_output=True, text=True)
    print(result.stdout)
    if result.stderr:
        print("Errors:", result.stderr)

if __name__ == "__main__":
    check_tunnels()
