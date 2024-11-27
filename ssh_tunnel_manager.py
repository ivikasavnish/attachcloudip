import tkinter as tk
from tkinter import ttk, messagebox
import yaml
import subprocess
import json
import os
from pathlib import Path

class SSHTunnelManager:
    def __init__(self, root):
        self.root = root
        self.root.title("SSH Tunnel Manager")
        self.root.geometry("800x600")
        
        # Load SSH config
        self.ssh_config = self.load_ssh_config()
        
        # Load saved mappings
        self.mappings_file = Path.home() / '.ssh_tunnel_mappings.json'
        self.port_mappings = self.load_mappings()
        
        self.create_widgets()
        self.tunnel_process = None
        self.nginx_setup_done = False

    def load_ssh_config(self):
        config_path = Path(__file__).parent / 'lightsail.yaml'
        try:
            with open(config_path) as f:
                return yaml.safe_load(f)
        except Exception as e:
            self.show_error("Error", f"Could not load SSH config: {e}")
            return None
            
    def load_mappings(self):
        try:
            if self.mappings_file.exists():
                with open(self.mappings_file) as f:
                    return json.load(f)
        except Exception:
            pass
        return []

    def save_mappings(self):
        try:
            with open(self.mappings_file, 'w') as f:
                json.dump(self.port_mappings, f)
        except Exception as e:
            self.show_error("Error", f"Could not save mappings: {e}")

    def create_widgets(self):
        # Main container
        main_container = ttk.Frame(self.root)
        main_container.pack(fill=tk.BOTH, expand=True)

        # Connection info frame
        info_frame = ttk.LabelFrame(main_container, text="Connection Info", padding="5")
        info_frame.pack(fill="x", padx=5, pady=5)
        
        ttk.Label(info_frame, text=f"Host: {self.ssh_config.get('host', 'N/A')}").pack()
        ttk.Label(info_frame, text=f"Username: {self.ssh_config.get('username', 'N/A')}").pack()
        ttk.Label(info_frame, text="Note: Traffic from remote port/subdomain will be forwarded to your local port").pack()
        
        # Port mapping frame
        mapping_frame = ttk.LabelFrame(main_container, text="Port/Subdomain Mapping", padding="5")
        mapping_frame.pack(fill="both", expand=True, padx=5, pady=5)
        
        # Add mapping inputs
        input_frame = ttk.Frame(mapping_frame)
        input_frame.pack(fill="x", padx=5, pady=5)
        
        # Local port input
        local_port_frame = ttk.Frame(input_frame)
        local_port_frame.pack(side="left", padx=5)
        ttk.Label(local_port_frame, text="Local Port:").pack()
        self.local_port = ttk.Entry(local_port_frame, width=10)
        self.local_port.pack()
        
        # Remote configuration
        remote_frame = ttk.Frame(input_frame)
        remote_frame.pack(side="left", padx=5)
        
        # Radio buttons for port or subdomain
        self.remote_type = tk.StringVar(value="port")
        ttk.Radiobutton(remote_frame, text="Use Port", variable=self.remote_type, 
                       value="port", command=self.toggle_remote_type).pack()
        ttk.Radiobutton(remote_frame, text="Use Subdomain", variable=self.remote_type, 
                       value="subdomain", command=self.toggle_remote_type).pack()
        
        # Remote port input
        self.remote_port_frame = ttk.Frame(remote_frame)
        self.remote_port_frame.pack()
        ttk.Label(self.remote_port_frame, text="Remote Port:").pack()
        self.remote_port = ttk.Entry(self.remote_port_frame, width=10)
        self.remote_port.pack()
        
        # Subdomain input (initially hidden)
        self.subdomain_frame = ttk.Frame(remote_frame)
        ttk.Label(self.subdomain_frame, text="Subdomain:").pack()
        self.subdomain = ttk.Entry(self.subdomain_frame, width=20)
        self.subdomain.pack()
        
        ttk.Button(input_frame, text="Add Mapping", command=self.add_mapping).pack(side="left", padx=5)
        
        # Mappings list
        self.mappings_tree = ttk.Treeview(mapping_frame, columns=("Local", "Remote", "URL"), show="headings")
        self.mappings_tree.heading("Local", text="Local Port")
        self.mappings_tree.heading("Remote", text="Remote Config")
        self.mappings_tree.heading("URL", text="Access URL")
        self.mappings_tree.column("URL", width=300)
        self.mappings_tree.pack(fill="both", expand=True, padx=5, pady=5)
        
        # Load existing mappings
        self.refresh_mappings_list()
        
        # Control buttons
        control_frame = ttk.Frame(main_container)
        control_frame.pack(fill="x", padx=5, pady=5)
        
        self.start_button = ttk.Button(control_frame, text="Start Tunnel", command=self.start_tunnel)
        self.start_button.pack(side="left", padx=5)
        
        self.stop_button = ttk.Button(control_frame, text="Stop Tunnel", command=self.stop_tunnel, state="disabled")
        self.stop_button.pack(side="left", padx=5)
        
        self.debug_button = ttk.Button(control_frame, text="Show Debug Log", command=self.toggle_debug_window)
        self.debug_button.pack(side="left", padx=5)
        self.debug_button.config(state="disabled")
        
        ttk.Button(control_frame, text="Remove Selected", command=self.remove_mapping).pack(side="left", padx=5)

        # Status bar
        self.status_var = tk.StringVar()
        self.status_bar = ttk.Label(self.root, textvariable=self.status_var, relief=tk.SUNKEN, padding=(5, 2))
        self.status_bar.pack(side=tk.BOTTOM, fill=tk.X)
        
        # Set initial status
        self.update_status("Ready")

    def toggle_remote_type(self):
        if self.remote_type.get() == "port":
            self.remote_port_frame.pack()
            self.subdomain_frame.pack_forget()
        else:
            self.remote_port_frame.pack_forget()
            self.subdomain_frame.pack()

    def setup_nginx_config(self):
        if self.nginx_setup_done:
            return

        # Get subdomains from mappings
        subdomain_configs = []
        for mapping in self.port_mappings:
            if mapping.get('subdomain'):
                remote_port = 10000 + hash(mapping['subdomain']) % 50000
                config = f'''
    server {{
        listen 80;
        listen [::]:80;
        server_name {mapping['subdomain']}.{self.ssh_config['host']};
        
        location / {{
            proxy_pass http://localhost:{remote_port};
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }}
    }}'''
                subdomain_configs.append(config)

        if not subdomain_configs:
            return

        # Create nginx config
        nginx_config = '''
events {
    worker_connections 1024;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;
    
    # Basic settings
    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    types_hash_max_size 2048;
    
    # SSL Settings
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;
    
    # Logging Settings
    access_log /var/log/nginx/access.log;
    error_log /var/log/nginx/error.log;
    
    # Gzip Settings
    gzip on;
    gzip_disable "msie6";
    
    # Virtual Host Configs
''' + "\n".join(subdomain_configs) + "\n}"
        
        # Save config to temporary file
        config_path = "/tmp/nginx_config"
        with open(config_path, "w") as f:
            f.write(nginx_config)

        # Upload and apply config
        try:
            # Upload config
            subprocess.run([
                "scp",
                "-i", self.ssh_config["ssh_key_path"],
                config_path,
                f"{self.ssh_config['username']}@{self.ssh_config['host']}:/tmp/nginx_config"
            ], check=True)

            # Apply config
            ssh_cmd = [
                "ssh",
                "-i", self.ssh_config["ssh_key_path"],
                f"{self.ssh_config['username']}@{self.ssh_config['host']}",
                "sudo mv /tmp/nginx_config /etc/nginx/nginx.conf && sudo nginx -t && sudo systemctl restart nginx"
            ]
            subprocess.run(ssh_cmd, check=True)
            self.nginx_setup_done = True
        except subprocess.CalledProcessError as e:
            self.show_error("Error", f"Failed to setup Nginx: {e}")

    def toggle_debug_window(self):
        if hasattr(self, 'debug_window'):
            if self.debug_window.winfo_viewable():
                self.debug_window.withdraw()
                self.debug_button.config(text="Show Debug Log")
            else:
                self.debug_window.deiconify()
                self.debug_button.config(text="Hide Debug Log")

    def add_mapping(self):
        try:
            local = int(self.local_port.get())
            
            if local < 1:
                raise ValueError("Ports must be positive numbers")
            
            mapping = {"local": local}
            
            if self.remote_type.get() == "port":
                remote = int(self.remote_port.get())
                if remote < 1:
                    raise ValueError("Ports must be positive numbers")
                mapping["remote"] = remote
                url = f"{self.ssh_config['host']}:{remote}"
            else:
                subdomain = self.subdomain.get().strip()
                if not subdomain:
                    raise ValueError("Subdomain cannot be empty")
                if not subdomain.isalnum():
                    raise ValueError("Subdomain must be alphanumeric")
                mapping["subdomain"] = subdomain
                url = f"{subdomain}.{self.ssh_config['host']}"
            
            if mapping not in self.port_mappings:
                self.port_mappings.append(mapping)
                self.save_mappings()
                self.refresh_mappings_list()
                self.local_port.delete(0, tk.END)
                if self.remote_type.get() == "port":
                    self.remote_port.delete(0, tk.END)
                else:
                    self.subdomain.delete(0, tk.END)
            else:
                self.show_error("Warning", "This mapping already exists!")
        except ValueError as e:
            self.show_error("Error", str(e))

    def remove_mapping(self):
        selected = self.mappings_tree.selection()
        if not selected:
            return
            
        for item in selected:
            values = self.mappings_tree.item(item)['values']
            local_port = int(values[0])
            remote_config = values[1]
            
            # Create the correct mapping dictionary based on the type
            if remote_config.startswith('Port'):
                remote_port = int(remote_config.split(' ')[1])
                mapping = {"local": local_port, "remote": remote_port}
            else:
                subdomain = remote_config.split(': ')[1]
                mapping = {"local": local_port, "subdomain": subdomain}
            
            try:
                self.port_mappings.remove(mapping)
            except ValueError:
                pass  # Ignore if mapping not found
            
        self.save_mappings()
        self.refresh_mappings_list()

    def refresh_mappings_list(self):
        for item in self.mappings_tree.get_children():
            self.mappings_tree.delete(item)
            
        for mapping in self.port_mappings:
            if 'remote' in mapping:
                remote_config = f"Port {mapping['remote']}"
                url = f"{self.ssh_config['host']}:{mapping['remote']}"
            else:
                remote_config = f"Subdomain: {mapping['subdomain']}"
                url = f"{mapping['subdomain']}.{self.ssh_config['host']}"
            self.mappings_tree.insert("", "end", values=(mapping["local"], remote_config, url))

    def build_ssh_command(self):
        cmd = [
            "ssh",
            "-i", self.ssh_config["ssh_key_path"],
            "-o", "StrictHostKeyChecking=no",
            "-o", "GatewayPorts=yes",
            "-N"
        ]
        
        for mapping in self.port_mappings:
            if 'remote' in mapping:
                # Port-based forwarding
                cmd.extend(["-L", f"*:{mapping['remote']}:localhost:{mapping['local']}"])
            else:
                # Subdomain-based forwarding (using a high random port)
                remote_port = 10000 + hash(mapping['subdomain']) % 50000
                cmd.extend(["-L", f"*:{remote_port}:localhost:{mapping['local']}"])
        
        # Add destination
        cmd.append(f"{self.ssh_config['username']}@{self.ssh_config['host']}")
        
        return cmd

    def start_tunnel(self):
        try:
            if not self.port_mappings:
                self.show_error("Error", "Please add at least one port mapping before starting the tunnel.")
                return
                
            # First setup nginx if we have any subdomain mappings
            if any('subdomain' in m for m in self.port_mappings):
                self.setup_nginx_config()
            
            cmd = self.build_ssh_command()
            self.tunnel_process = subprocess.Popen(
                cmd,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                universal_newlines=True,
                bufsize=1  # Line buffered
            )
            
            # Create a window to show SSH debug output
            self.debug_window = tk.Toplevel(self.root)
            self.debug_window.title("SSH Debug Output")
            self.debug_window.geometry("600x400")
            self.debug_window.transient(self.root)
            self.debug_window.protocol("WM_DELETE_WINDOW", lambda: self.toggle_debug_window())
            
            # Initially hide the debug window
            self.debug_window.withdraw()
            
            # Add text widget for output
            self.debug_text = tk.Text(self.debug_window, wrap=tk.WORD)
            self.debug_text.pack(fill=tk.BOTH, expand=True)
            
            # Add scrollbar
            scrollbar = ttk.Scrollbar(self.debug_window, command=self.debug_text.yview)
            scrollbar.pack(side=tk.RIGHT, fill=tk.Y)
            self.debug_text.config(yscrollcommand=scrollbar.set)
            
            # Start monitoring in a separate thread
            self.monitoring = True
            import threading
            self.monitor_thread = threading.Thread(target=self.monitor_tunnel, daemon=True)
            self.monitor_thread.start()
            
            self.start_button.config(state="disabled")
            self.stop_button.config(state="normal")
            self.debug_button.config(state="normal", text="Show Debug Log")
            self.update_status("SSH tunnel started successfully")
        except Exception as e:
            self.show_error("Error", f"Failed to start SSH tunnel: {e}")
            self.update_status("Failed to start tunnel", error=True)

    def monitor_tunnel(self):
        import queue
        import time
        
        def update_ui():
            try:
                while True:
                    line = self.output_queue.get_nowait()
                    self.debug_text.insert(tk.END, line)
                    self.debug_text.see(tk.END)
            except queue.Empty:
                pass
            
            if self.monitoring:
                self.root.after(100, update_ui)
        
        self.output_queue = queue.Queue()
        self.root.after(100, update_ui)
        
        while self.monitoring and self.tunnel_process:
            # Check if process is still running
            if self.tunnel_process.poll() is not None:
                error_output = self.tunnel_process.stderr.read()
                if error_output:
                    self.output_queue.put("\nTunnel stopped with error:\n" + error_output)
                    self.root.after(0, lambda: self.debug_window.deiconify())
                    self.root.after(0, lambda: self.debug_button.config(text="Hide Debug Log"))
                    self.root.after(0, lambda: self.show_error("Error", "SSH tunnel stopped unexpectedly. Check debug window for details."))
                    self.root.after(0, lambda: self.update_status("Tunnel stopped with error", error=True))
                self.root.after(0, self.stop_tunnel)
                break
            
            # Read any available output
            try:
                line = self.tunnel_process.stderr.readline()
                if line:
                    self.output_queue.put(line)
            except:
                pass
            
            time.sleep(0.1)  # Prevent busy waiting

    def stop_tunnel(self):
        self.monitoring = False
        if hasattr(self, 'monitor_thread'):
            self.monitor_thread.join(timeout=1.0)
        
        if self.tunnel_process:
            self.tunnel_process.terminate()
            try:
                self.tunnel_process.wait(timeout=5.0)
            except subprocess.TimeoutExpired:
                self.tunnel_process.kill()
            self.tunnel_process = None
            
        if hasattr(self, 'debug_window'):
            self.debug_window.destroy()
            delattr(self, 'debug_window')
            
        self.start_button.config(state="normal")
        self.stop_button.config(state="disabled")
        self.debug_button.config(state="disabled")
        self.update_status("SSH tunnel stopped successfully")

    def on_closing(self):
        self.stop_tunnel()
        self.root.destroy()

    def update_status(self, message, error=False):
        self.status_var.set(message)
        self.status_bar.configure(foreground='red' if error else 'black')
        # Clear status after 5 seconds for non-error messages
        if not error:
            self.root.after(5000, lambda: self.status_var.set("Ready"))

    def show_error(self, title, message):
        dialog = tk.Toplevel(self.root)
        dialog.title(title)
        dialog.transient(self.root)
        
        # Set dialog size and position
        dialog.geometry("400x200")
        dialog.geometry(f"+{self.root.winfo_x() + 50}+{self.root.winfo_y() + 50}")
        
        frame = ttk.Frame(dialog, padding="20")
        frame.pack(fill="both", expand=True)
        
        ttk.Label(frame, text=message, wraplength=350).pack(pady=(0, 20))
        
        ttk.Button(frame, text="OK", command=dialog.destroy).pack()
        
        # Don't block the main window
        dialog.focus_set()

if __name__ == "__main__":
    root = tk.Tk()
    app = SSHTunnelManager(root)
    root.protocol("WM_DELETE_WINDOW", app.on_closing)
    root.mainloop()
