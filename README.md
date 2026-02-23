# DNS Tunnel Client (dnstc)

A cross-platform CLI tool for managing DNS tunnel connections. Supports multiple transport and backend types with both interactive TUI and CLI interfaces.

## Features

- **Transports**: Slipstream and DNSTT
- **Backends**: SOCKS (standalone) and Shadowsocks (SIP003 plugin)
- **Gateway proxy**: Single SOCKS port routing to the active tunnel, switchable at runtime
- **DNS proxy**: Local caching DNS proxy with health-aware upstream selection
- **Running modes**: Interactive TUI, headless foreground (`dnstc up`), systemd service (Linux)
- **Auto-download**: Required binaries (slipstream-client, dnstt-client, sslocal) fetched on first use
- **Named tunnels**: Auto-generated adjective-noun tags (e.g. `swift-tunnel`)
- **Per-tunnel ports**: Each tunnel gets its own local port, auto-assigned if not specified
- **Process management**: Child processes cleaned up automatically when dnstc exits

## Installation

```bash
go install github.com/net2share/dnstc@latest
```

Or build from source:

```bash
git clone https://github.com/net2share/dnstc.git
cd dnstc
go build -o dnstc .
```

## Transport + Backend Combinations

| Transport  | Backend     | Description                             | Required Config                |
|------------|-------------|-----------------------------------------|--------------------------------|
| slipstream | shadowsocks | Slipstream as Shadowsocks SIP003 plugin | domain, ss-server, ss-password |
| slipstream | socks       | Slipstream standalone SOCKS proxy       | domain                         |
| dnstt      | socks       | DNSTT standalone SOCKS proxy            | domain, pubkey (64-char hex)   |

> **Note**: `dnstt + shadowsocks` is not a supported combination.

## Usage

### Interactive TUI

Run without arguments:

```bash
dnstc
```

Full-screen menu with Connect/Disconnect, tunnel management, and configuration. Changes take effect immediately.

### Headless Mode

Start all enabled tunnels and the gateway in the foreground:

```bash
dnstc up
```

Runs until interrupted with Ctrl+C. All child processes stop when dnstc exits.

### Systemd Service (Linux only)

```bash
sudo dnstc service install    # Install and start systemd service
sudo dnstc service status     # Show service status
sudo dnstc service uninstall  # Stop and remove service
```

The service runs `dnstc up` under systemd with auto-restart on failure.

### CLI Commands

#### Tunnel Management

```bash
# Add a tunnel
dnstc tunnel add --transport slipstream --backend socks -d tunnel.example.com
dnstc tunnel add --transport dnstt --backend socks -d tunnel.example.com --pubkey <64-char-hex>
dnstc tunnel add --transport slipstream --backend shadowsocks -d tunnel.example.com \
  --ss-server 127.0.0.1:8388 --ss-password secret

# Add with a specific local port (auto-assigned if omitted)
dnstc tunnel add --transport slipstream --backend socks -d tunnel.example.com -p 9050

# List tunnels
dnstc tunnel list

# Show tunnel status
dnstc tunnel status -t <tag>

# Switch active tunnel (gateway routes to this tunnel)
dnstc tunnel activate -t <tag>

# Remove a tunnel
dnstc tunnel remove -t <tag> --force
```

#### Configuration

```bash
dnstc config show              # Display current config
dnstc config edit              # Open config in $EDITOR
dnstc config gateway-port -p 1080  # Set gateway proxy port
```

#### Uninstall

```bash
dnstc uninstall --force
```

Removes config, state, downloaded binaries, and logs.

## Architecture

```
┌──────────────────────────┐
│         Gateway          │  Single port (default :1080)
│    SOCKS proxy entry     │  Accepts client connections
└───────────┬──────────────┘
            │ routes to active tunnel
            ▼
┌──────────────────────────┐
│     Active Tunnel        │  Per-tunnel local port
│   (e.g. :41023)          │  Running transport binary
└───────────┬──────────────┘
            │ DNS queries
            ▼
┌──────────────────────────┐
│       DNS Proxy          │  Local caching proxy
│   Health-aware routing   │  Fastest healthy upstream
└───────────┬──────────────┘
            │
            ▼
       Remote Server
```

- The **gateway** is a TCP relay that listens on a single configurable port and forwards each connection to whichever tunnel is currently active.
- The **DNS proxy** is a local caching resolver that routes queries to the fastest healthy upstream. It monitors upstream health and latency, automatically avoiding failed resolvers.
- **Switching** the active tunnel takes effect on the next connection — no restart needed.
- Each **tunnel** runs as a child process (slipstream-client, dnstt-client, or sslocal) on its own local port.

## Configuration

Stored in `~/.config/dnstc/config.json`:

```json
{
  "listen": {
    "socks": "127.0.0.1:1080"
  },
  "resolvers": ["1.1.1.1:53"],
  "tunnels": [
    {
      "tag": "swift-tunnel",
      "transport": "slipstream",
      "backend": "shadowsocks",
      "domain": "tunnel.example.com",
      "port": 41023,
      "shadowsocks": {
        "server": "127.0.0.1:8388",
        "password": "your-password",
        "method": "chacha20-ietf-poly1305"
      }
    },
    {
      "tag": "cool-proxy",
      "transport": "dnstt",
      "backend": "socks",
      "domain": "dns.example.com",
      "port": 41024,
      "dnstt": {
        "pubkey": "0123456789abcdef..."
      }
    }
  ],
  "route": {
    "active": "swift-tunnel"
  }
}
```

- `listen.socks` — Gateway port. Auto-assigned if the default (1080) is unavailable.
- `resolvers` — Upstream DNS resolvers for the local DNS proxy.
- `tunnels[].port` — Per-tunnel local SOCKS port. Auto-assigned when adding a tunnel.
- `route.active` — Tag of the tunnel the gateway routes to.

## File Locations

| Purpose       | Path                          |
|---------------|-------------------------------|
| Configuration | `~/.config/dnstc/config.json` |
| State/PIDs    | `~/.config/dnstc/state.json`  |
| Binaries      | `~/.local/share/dnstc/bin/`   |
| Logs          | `~/.local/share/dnstc/logs/`  |

Automatic migration from YAML config (`config.yaml`) to JSON is performed on first run.

## Related Projects

- [dnstm](https://github.com/net2share/dnstm) — DNS Tunnel Manager (server-side)
- [go-corelib](https://github.com/net2share/go-corelib) — Shared Go library
