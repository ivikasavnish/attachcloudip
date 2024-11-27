#!/usr/bin/env python3
import subprocess
import sys
import yaml
import json
from pathlib import Path
import time

class RemoteDebugger:
    def __init__(self, config_path):
        self.config = self.load_config(config_path)
        self.ssh_cmd_base = [
            'ssh',
            '-i', self.config['ssh_key_path'],
            f"{self.config['username']}@{self.config['host']}"
        ]

    def load_config(self, config_path):
        with open(config_path) as f:
            return yaml.safe_load(f)

    def run_remote_command(self, command):
        print(f"\n🔍 Running: {command}")
        try:
            full_cmd = self.ssh_cmd_base + [command]
            result = subprocess.run(full_cmd, capture_output=True, text=True, timeout=10)
            print("Output:")
            if result.stdout:
                print(result.stdout)
            if result.stderr:
                print("Errors:", result.stderr)
            return result
        except subprocess.TimeoutExpired:
            print("❌ Command timed out after 10 seconds")
            return None
        except Exception as e:
            print(f"❌ Error: {e}")
            return None

    def check_server_status(self):
        print("\n=== 🖥️  Checking Server Status ===")
        commands = [
            "uptime",
            "free -h",
            "df -h /",
        ]
        for cmd in commands:
            self.run_remote_command(cmd)

    def check_network_status(self):
        print("\n=== 🌐 Checking Network Status ===")
        commands = [
            "netstat -tulpn 2>/dev/null | grep LISTEN",
            "sudo iptables -L -n 2>/dev/null || echo 'No iptables access'",
            "curl -I localhost:5000 2>/dev/null || echo 'Cannot connect to port 5000'",
        ]
        for cmd in commands:
            self.run_remote_command(cmd)

    def check_process_status(self):
        print("\n=== 👥 Checking Process Status ===")
        commands = [
            "ps aux | grep ssh",
            "lsof -i :5000 2>/dev/null || echo 'No process on port 5000'",
        ]
        for cmd in commands:
            self.run_remote_command(cmd)

    def check_ssh_status(self):
        print("\n=== 🔑 Checking SSH Status ===")
        commands = [
            "sudo systemctl status sshd 2>/dev/null || echo 'Cannot check SSH status'",
            "grep -i listen /etc/ssh/sshd_config 2>/dev/null || echo 'Cannot read sshd_config'",
        ]
        for cmd in commands:
            self.run_remote_command(cmd)

    def check_dns_resolution(self):
        print("\n=== 🔍 Checking DNS Resolution ===")
        commands = [
            f"dig {self.config['host']} +short",
            f"host {self.config['host']}",
        ]
        for cmd in commands:
            self.run_remote_command(cmd)

    def run_all_checks(self):
        print(f"\n🚀 Starting remote server diagnostics for {self.config['host']}")
        
        checks = [
            self.check_server_status,
            self.check_network_status,
            self.check_process_status,
            self.check_ssh_status,
            self.check_dns_resolution,
        ]
        
        for check in checks:
            try:
                check()
                time.sleep(1)  # Prevent overwhelming the server
            except Exception as e:
                print(f"❌ Error during {check.__name__}: {e}")

        print("\n✅ Diagnostic checks completed")

def main():
    config_path = Path.home() / 'Downloads/configs/lightsail.yaml'
    if not config_path.exists():
        print(f"Error: Config file not found at {config_path}")
        sys.exit(1)

    debugger = RemoteDebugger(str(config_path))
    debugger.run_all_checks()

if __name__ == "__main__":
    main()
