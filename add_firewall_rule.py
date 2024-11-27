#!/usr/bin/env python3
import subprocess
import yaml
from pathlib import Path

def load_config():
    config_path = Path.home() / 'Downloads/configs/lightsail.yaml'
    with open(config_path) as f:
        return yaml.safe_load(f)

def add_firewall_rule():
    config = load_config()
    ssh_cmd = [
        'ssh',
        '-i', config['ssh_key_path'],
        f"{config['username']}@{config['host']}",
        'sudo ufw allow 5000/tcp && sudo ufw status'
    ]
    
    print("Adding firewall rule for port 5000...")
    result = subprocess.run(ssh_cmd, capture_output=True, text=True)
    print(result.stdout)
    if result.stderr:
        print("Errors:", result.stderr)

if __name__ == "__main__":
    add_firewall_rule()
