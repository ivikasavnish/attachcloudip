#!/bin/bash

# Default values
LOCAL_PORT=5001
TEST_DURATION=30
NUM_REQUESTS=1000
CONCURRENCY=10

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -l|--local-port)
            LOCAL_PORT="$2"
            shift 2
            ;;
        -t|--time)
            TEST_DURATION="$2"
            shift 2
            ;;
        -n|--requests)
            NUM_REQUESTS="$2"
            shift 2
            ;;
        -c|--concurrency)
            CONCURRENCY="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [-l LOCAL_PORT] [-t TEST_DURATION] [-n NUM_REQUESTS] [-c CONCURRENCY]"
            echo "Tests an established SSH tunnel"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Check if tunnel exists
TUNNEL_INFO="/tmp/tunnel_$LOCAL_PORT"
if [ ! -f "${TUNNEL_INFO}.pid" ] || [ ! -f "${TUNNEL_INFO}.ports" ]; then
    echo "Error: No tunnel found on port $LOCAL_PORT"
    echo "Please run setup_tunnel.sh first"
    exit 1
fi

TUNNEL_PID=$(cat "${TUNNEL_INFO}.pid")
PORTS=$(cat "${TUNNEL_INFO}.ports")
LOCAL_PORT=$(echo "$PORTS" | cut -d: -f1)
REMOTE_PORT=$(echo "$PORTS" | cut -d: -f2)

if ! kill -0 "$TUNNEL_PID" 2>/dev/null; then
    echo "Error: Tunnel process not running"
    rm -f "${TUNNEL_INFO}.pid" "${TUNNEL_INFO}.ports"
    exit 1
fi

echo "Found tunnel (PID: $TUNNEL_PID)"
echo "Local Port: $LOCAL_PORT"
echo "Remote Port: $REMOTE_PORT"

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

echo "Test server started successfully"

# Function to run bandwidth test
test_bandwidth() {
    if ! command -v iperf3 &> /dev/null; then
        echo "Warning: iperf3 not found, skipping bandwidth test"
        return
    fi
    
    echo "Testing bandwidth..."
    echo "Starting iperf3 server..."
    iperf3 -s -p $LOCAL_PORT -1 > /dev/null 2>&1 &
    IPERF_PID=$!
    sleep 2
    
    echo "Running upload test..."
    iperf3 -c localhost -p $LOCAL_PORT -t 10 -J > upload.json
    
    echo "Running download test..."
    iperf3 -c localhost -p $LOCAL_PORT -t 10 -R -J > download.json
    
    if [ -f upload.json ] && [ -f download.json ]; then
        UP=$(python3 -c "import json; data=json.load(open('upload.json')); print(f\"{data['end']['sum_sent']['bits_per_second']/1000000:.2f}\")")
        DOWN=$(python3 -c "import json; data=json.load(open('download.json')); print(f\"{data['end']['sum_sent']['bits_per_second']/1000000:.2f}\")")
        echo "Upload: $UP Mbps"
        echo "Download: $DOWN Mbps"
        rm upload.json download.json
    fi
    
    kill $IPERF_PID 2>/dev/null
}

# Function to test HTTP throughput
test_http() {
    if ! command -v ab &> /dev/null; then
        echo "Warning: ab (Apache Bench) not found, skipping HTTP test"
        return
    fi
    
    echo "Testing HTTP throughput..."
    ab -n $NUM_REQUESTS -c $CONCURRENCY "http://localhost:$LOCAL_PORT/" > ab_results.txt
    
    if [ -f ab_results.txt ]; then
        RPS=$(grep "Requests per second" ab_results.txt | awk '{print $4}')
        TIME=$(grep "Time per request" ab_results.txt | head -n 1 | awk '{print $4}')
        FAILED=$(grep "Failed requests" ab_results.txt | awk '{print $3}')
        
        echo "HTTP Test Results:"
        echo "Requests per second: $RPS"
        echo "Time per request: $TIME ms"
        echo "Failed requests: $FAILED"
        
        rm ab_results.txt
    fi
}

# Run tests
echo "Starting tests..."
test_bandwidth
test_http

# Monitor for specified duration
echo "Monitoring tunnel for $TEST_DURATION seconds..."
END=$((SECONDS + TEST_DURATION))
while [ $SECONDS -lt $END ]; do
    if ! kill -0 $TUNNEL_PID 2>/dev/null; then
        echo "Error: Tunnel connection lost"
        break
    fi
    
    CONNS=$(netstat -ant | grep ":$LOCAL_PORT.*ESTABLISHED" | wc -l)
    echo "Active connections: $CONNS"
    sleep 5
done

# Cleanup
echo "Cleaning up..."
kill $SERVER_PID 2>/dev/null
