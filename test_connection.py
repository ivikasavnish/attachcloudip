#!/usr/bin/env python3
import socket
import sys
import time

def test_port(host, port):
    print(f"Testing connection to {host}:{port}")
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(5)
        result = sock.connect_ex((host, port))
        if result == 0:
            print(f"✓ Port {port} is open")
            return True
        else:
            print(f"✗ Port {port} is closed (error: {result})")
            return False
    except socket.gaierror:
        print(f"✗ Could not resolve hostname {host}")
        return False
    except socket.timeout:
        print(f"✗ Connection to {host}:{port} timed out")
        return False
    except Exception as e:
        print(f"✗ Error testing {host}:{port}: {e}")
        return False
    finally:
        sock.close()

def main():
    if len(sys.argv) != 3:
        print("Usage: python3 test_connection.py <host> <port>")
        sys.exit(1)
        
    host = sys.argv[1]
    port = int(sys.argv[2])
    
    # Test the connection multiple times
    for i in range(3):
        if i > 0:
            print(f"\nRetrying in 2 seconds... (attempt {i+1}/3)")
            time.sleep(2)
        if test_port(host, port):
            sys.exit(0)
    
    sys.exit(1)

if __name__ == "__main__":
    main()
