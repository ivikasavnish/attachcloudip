from http.server import HTTPServer, BaseHTTPRequestHandler
import json, sys

class TestHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        print(f"Received request from {self.client_address}")
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        response = {'status': 'ok', 'message': 'Tunnel is working!'}
        self.wfile.write(json.dumps(response).encode())

if __name__ == '__main__':
    port = int(sys.argv[1])
    server = HTTPServer(('0.0.0.0', port), TestHandler)
    print(f'Test server listening on all interfaces, port {port}')
    server.serve_forever()
