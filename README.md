# Kolibri Mesh

Mesh VPN with custom AWG (KolibriAWG) protocol for Kolibri infrastructure.

## Features

- **KolibriAWG Protocol**: Custom WireGuard-based protocol with obfuscation, TLS masking, jitter, and padding
- **Mesh Networking**: Automatic peer discovery and routing
- **MikroTik Integration**: Native WireGuard support for MikroTik routers
- **CLI Management**: Easy-to-use command line interface
- **Coordinator-Agent Architecture**: Centralized management with distributed execution

## Architecture

```
Kolibri Mesh Network (10.99.0.0/24)
├── Home (Coordinator) — 10.99.0.1
├── Main — 10.99.0.2
├── UIAP — 10.99.0.3
├── ...
├── hostvds-* (14 servers) — 10.99.0.10-23
├── worker-backup — 10.99.0.24
├── MikroTik — 10.99.99.1
└── Mac — 10.99.0.100
```

## Quick Start

### 1. Install

```bash
# Clone repository
git clone https://github.com/kolibri/kolibri-mesh.git
cd kolibri-mesh

# Build
go build -o mesh ./cmd/cli
go build -o coordinator ./cmd/coordinator
go build -o agent ./cmd/agent

# Or use make
make build
```

### 2. Start Coordinator (Home)

```bash
# Start coordinator
./coordinator --port 8080

# Or with environment variables
PORT=8080 ./coordinator
```

### 3. Start Agent (Servers)

```bash
# Set environment variables
export NODE_ID=server-01
export NODE_NAME=main
export COORDINATOR_URL=http://10.99.0.1:8080
export NODE_IP=10.99.0.2

# Start agent
./agent
```

### 4. Manage with CLI

```bash
# Check status
./mesh status

# List nodes
./mesh nodes

# Add node
./mesh add server-01 main 10.99.0.2

# Show config
./mesh config
```

## KolibriAWG Protocol

KolibriAWG is a custom protocol based on WireGuard with:

### Obfuscation
- XOR encryption of headers
- Custom handshake
- Random parameters per connection

### TLS Masking
- Packets look like TLS 1.3
- Fake ClientHello/ServerHello
- SNI masking

### Jitter & Padding
- Random delays (jitter)
- Garbage bytes (padding)
- DPI bypass

### Parameters
```json
{
  "s1-s4": "32-byte seeds for obfuscation",
  "h1-h4": "32-byte hashes for verification",
  "jmin": 5,
  "jmax": 50,
  "jc": 6,
  "padding_max": 256,
  "tls_sni": "cloudflare.com"
}
```

## Installation on Servers

### Automatic Installation

```bash
# Install on all servers
./scripts/install-all.sh

# Or install on specific server
./scripts/install.sh <server-ip>
```

### Manual Installation

```bash
# 1. Copy binary to server
scp agent root@server:/usr/local/bin/kolibri-mesh-agent

# 2. Create systemd service
cat > /etc/systemd/system/kolibri-mesh-agent.service << EOF
[Unit]
Description=Kolibri Mesh Agent
After=network.target

[Service]
Type=simple
Environment=NODE_ID=server-01
Environment=NODE_NAME=main
Environment=COORDINATOR_URL=http://10.99.0.1:8080
Environment=NODE_IP=10.99.0.2
ExecStart=/usr/local/bin/kolibri-mesh-agent
Restart=always

[Install]
WantedBy=multi-user.target
EOF

# 3. Enable and start
systemctl enable kolibri-mesh-agent
systemctl start kolibri-mesh-agent
```

## MikroTik Integration

### Add MikroTik as Peer

```bash
# Add MikroTik node
./mesh add mikrotik-01 mikrotik 10.99.99.1

# Configure MikroTik WireGuard
# (via RouterOS API or WinBox)
```

### MikroTik WireGuard Config

```
/interface wireguard
add listen-port=51830 name=wg-kolibri private-key="<private-key>"

/interface wireguard peers
add allowed-address=10.99.0.0/24 endpoint-address=10.99.0.1 endpoint-port=51830 public-key="<public-key>"
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP server port | `8080` |
| `NODE_ID` | Unique node ID | required |
| `NODE_NAME` | Human-readable name | required |
| `COORDINATOR_URL` | Coordinator URL | required |
| `NODE_IP` | Node IP address | required |

### AWG Configuration

AWG configuration is stored in the coordinator and distributed to all agents.

## API Reference

### Coordinator API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/status` | GET | Get mesh status |
| `/api/nodes` | GET | List all nodes |
| `/api/nodes` | POST | Add node |
| `/api/heartbeat` | POST | Send heartbeat |
| `/api/config` | GET | Get AWG config |
| `/api/health` | GET | Health check |

### Agent API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/status` | GET | Get agent status |
| `/api/health` | GET | Health check |

## Development

### Build

```bash
# Build all binaries
make build

# Build specific binary
go build -o mesh ./cmd/cli
go build -o coordinator ./cmd/coordinator
go build -o agent ./cmd/agent
```

### Test

```bash
# Run tests
go test ./...

# Run specific tests
go test ./pkg/awg/...
go test ./pkg/mesh/...
```

### Lint

```bash
# Run linter
golangci-lint run
```

## License

MIT License

## Support

For issues and questions:
- GitHub Issues: kolibri-mesh/issues
- Documentation: docs/
