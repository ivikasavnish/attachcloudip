# AttachCloudIP

A client-server system for monitoring and managing IP attachments with heartbeat functionality.

## Features

- Client-server architecture with TCP and HTTP communication
- Heartbeat mechanism for connection monitoring
- Dynamic client registration with path specification
- Real-time connection status monitoring
- Automatic client reconnection
- Path-based message routing

## Prerequisites

- Go 1.16 or higher
- Git (for version control)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/attachcloudip.git
cd attachcloudip
```

2. Build the server:
```bash
cd cmd/server
go build -o server
```

3. Build the client:
```bash
cd cmd/client
go build -o client
```

## Usage

### Starting the Server

The server handles both HTTP and TCP connections. Start it with:

```bash
./server
```

By default, the server:
- HTTP server runs on port 9999
- TCP server runs on port 9998
- Provides health check endpoint at `/health`
- Lists connected clients at `/clients`

### Running the Client

The client requires a path specification and can optionally specify a server address:

```bash
./client -path /your/watch/path [-server localhost:9999]
```

Options:
- `-path`: Required. Specifies the path to watch (e.g., `/stocks`, `/uiapp`)
- `-server`: Optional. Server address (default: `localhost:9999`)

### Features

1. **Client Registration**
   - Clients register with a unique ID and path
   - Server assigns TCP port for ongoing communication
   - Registration format: `clientID|path`

2. **Heartbeat Mechanism**
   - Clients send heartbeats every 2 seconds
   - Server acknowledges heartbeats
   - Automatic client cleanup on disconnection

3. **Client List**
   - View connected clients via HTTP endpoint `/clients`
   - Shows client IDs, paths, and last active times

## API Endpoints

### Server Endpoints

1. `/health`
   - Method: GET
   - Response: Server health status

2. `/register`
   - Method: POST
   - Body: `{"client_id": "string", "paths": ["string"]}`
   - Response: `{"port": [number]}`

3. `/clients`
   - Method: GET
   - Response: List of connected clients with their status

## API Examples

### Sample CURL Commands

1. Check Server Health
```bash
curl -X GET http://localhost:9999/health
```
Response:
```json
{"status": "ok"}
```

2. Register a Client
```bash
curl -X POST http://localhost:9999/register \
  -H "Content-Type: application/json" \
  -d '{"client_id": "test-client", "paths": ["/test/path"]}'
```
Response:
```json
{"port": 9998}
```

3. List Connected Clients
```bash
curl -X GET http://localhost:9999/clients
```
Response:
```json
{
  "clients": [
    {
      "id": "test-client",
      "path": "/test/path",
      "last_active": "2024-01-20T15:30:45Z",
      "status": "connected"
    }
  ]
}
```

## Development

### Project Structure

```
attachcloudip/
├── cmd/
│   ├── client/         # Client implementation
│   └── server/         # Server implementation
├── pkg/                # Shared packages
└── README.md
```

### Building from Source

```bash
# Build server
cd cmd/server
go build

# Build client
cd cmd/client
go build
```

## Troubleshooting

1. **Connection Issues**
   - Ensure server is running before starting clients
   - Check if ports are available
   - Verify network connectivity

2. **Path Registration**
   - Ensure path is specified with -path flag
   - Check server logs for registration status
   - Verify client ID in /clients endpoint

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Version History

- v1.0.2: Fixed heartbeat handling and path registration
- v1.0.1: Fixed client path handling and registration
- v1.0.0: Initial release with basic functionality
