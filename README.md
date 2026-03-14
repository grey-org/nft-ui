<div align="center">
  <img src="https://avatars.githubusercontent.com/u/258744129?s=200&v=4" alt="nft-ui logo" width="200"/>
  
  # nft-ui
  
  [![Release](https://github.com/nft-ui/nft-ui/actions/workflows/release.yml/badge.svg)](https://github.com/nft-ui/nft-ui/actions/workflows/release.yml)
</div>

A web-based management tool for nftables firewall rules, including outbound traffic quotas, inbound port access control, and port forwarding management.

## Features

- **Quota Management** — View, add, edit, delete, and reset quota rules with visual progress
- **Inbound Port Control** — Manage allowed ports with status indicators
- **Authentication & Access** — Basic auth, read-only mode, auto-refresh

## Installation

### Quick Install

If GitHub access is slow or blocked in your region, fetch the installer through a GitHub proxy and pass `--github-proxy` so the script also proxies GitHub API and release downloads. `https://gh-proxy.com/` is one supported example; you can replace it with any compatible prefix-style GitHub proxy.

**Stable (main branch):**
```bash
curl -fsSL https://raw.githubusercontent.com/nft-ui/nft-ui/main/install.sh | sudo bash
```

**Stable via GitHub proxy:**
```bash
curl -fsSL https://gh-proxy.com/https://raw.githubusercontent.com/nft-ui/nft-ui/main/install.sh | sudo bash -s -- --github-proxy https://gh-proxy.com/
```

**Beta (dev branch):**
```bash
curl -fsSL https://raw.githubusercontent.com/nft-ui/nft-ui/dev/install.sh | sudo bash -s -- --beta
```

**Beta via GitHub proxy:**
```bash
curl -fsSL https://gh-proxy.com/https://raw.githubusercontent.com/nft-ui/nft-ui/dev/install.sh | sudo bash -s -- --beta --github-proxy https://gh-proxy.com/
```

**Specific version:**
```bash
curl -fsSL https://raw.githubusercontent.com/nft-ui/nft-ui/main/install.sh | sudo bash -s -- --tag v0.4.5
```

**Specific version via GitHub proxy:**
```bash
curl -fsSL https://gh-proxy.com/https://raw.githubusercontent.com/nft-ui/nft-ui/main/install.sh | sudo bash -s -- --tag v0.4.5 --github-proxy https://gh-proxy.com/
```

### Manual Install

Download the binary from [Releases](https://github.com/nft-ui/nft-ui/releases):

If you need a GitHub proxy for manual installation too, prepend the original GitHub URL with the proxy base, for example `https://gh-proxy.com/https://github.com/...` or `https://gh-proxy.com/https://api.github.com/...`.

**Stable (latest release):**
```bash
# Linux amd64
curl -fsSL -o nft-ui-stable https://github.com/nft-ui/nft-ui/releases/latest/download/nft-ui-linux-amd64

# Linux arm64
curl -fsSL -o nft-ui-stable https://github.com/nft-ui/nft-ui/releases/latest/download/nft-ui-linux-arm64

chmod +x nft-ui-stable
sudo mv nft-ui-stable /usr/local/bin/nft-ui
```

**Beta/Alpha (pre-release):**
```bash
# Get latest pre-release tag
TAG=$(curl -s https://api.github.com/repos/nft-ui/nft-ui/releases | jq -r '[.[] | select(.prerelease==true)][0].tag_name')

# Linux amd64
curl -fsSL -o nft-ui-beta https://github.com/nft-ui/nft-ui/releases/download/${TAG}/nft-ui-linux-amd64

# Linux arm64
curl -fsSL -o nft-ui-beta https://github.com/nft-ui/nft-ui/releases/download/${TAG}/nft-ui-linux-arm64

chmod +x nft-ui-beta
sudo mv nft-ui-beta /usr/local/bin/nft-ui
```

## Usage

```bash
# Run with default settings (port 8080)
sudo nft-ui

# With environment variables
sudo NFT_UI_LISTEN_ADDR=:3000 NFT_UI_AUTH_USER=admin NFT_UI_AUTH_PASSWORD=secret nft-ui
```

## Run in Docker (manage host nftables)

nftables depends on the **host kernel**. To let a container manage **host** rules, run it in the **host network namespace** and grant net-admin capabilities.

```bash
# build image
docker build -t nft-ui .

# run (host rules)
docker run -d --name nft-ui \
  --network host \
  --cap-add NET_ADMIN --cap-add NET_RAW \
  -e NFT_UI_LISTEN_ADDR=127.0.0.1:8080 \
  nft-ui
```

Notes:
- Requires **rootful** Docker/Podman (rootless cannot add NET_ADMIN).
- If you bind to `127.0.0.1`, use SSH tunnel as below for remote access.
- `--privileged` also works, but is broader than needed.

### docker-compose

```yaml
services:
  nft-ui:
    build: .
    network_mode: host
    cap_add:
      - NET_ADMIN
      - NET_RAW
    environment:
      NFT_UI_LISTEN_ADDR: 127.0.0.1:8080
    restart: unless-stopped
```

### Podman

```bash
# build image
podman build -t nft-ui .

# run (host rules)
podman run -d --name nft-ui \
  --network host \
  --cap-add NET_ADMIN --cap-add NET_RAW \
  -e NFT_UI_LISTEN_ADDR=127.0.0.1:8080 \
  nft-ui
```

## Remote Access via SSH Tunnel

For security, the default configuration binds to `localhost:8080` (not exposed to public network). 

To access the web interface from your local machine:

```bash
# Forward remote localhost:8080 to your local port 8080
ssh -L 8080:localhost:8080 user@remote-server

# Or use a different local port (e.g., 9090)
ssh -L 9090:localhost:8080 user@remote-server

# Keep the tunnel alive with connection monitoring
ssh -L 8080:localhost:8080 -o ServerAliveInterval=60 user@remote-server
```

Then open your browser: `http://localhost:8080` (or `http://localhost:9090`)

**Tip:** Add the SSH tunnel to your `~/.ssh/config`:

```
Host nft-ui-tunnel
    HostName remote-server
    User your-username
    LocalForward 8080 localhost:8080
    ServerAliveInterval 60
```

Then simply run: `ssh nft-ui-tunnel`

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `NFT_UI_LISTEN_ADDR` | `localhost:8080` | Server listen address |
| `NFT_UI_AUTH_USER` | - | Basic auth username |
| `NFT_UI_AUTH_PASSWORD` | - | Basic auth password |
| `NFT_UI_READ_ONLY` | `false` | Disable write operations |
| `NFT_UI_REFRESH_INTERVAL` | `5` | Auto-refresh interval (seconds) |
| `NFT_UI_NFT_BINARY` | `/usr/sbin/nft` | Path to nft binary |
| `NFT_UI_TABLE_FAMILY` | `inet` | nftables family |
| `NFT_UI_TABLE_NAME` | `filter` | nftables table name |
| `NFT_UI_CHAIN_NAME` | `output` | nftables chain name |

## Systemd Service

```bash
sudo tee /etc/systemd/system/nft-ui.service > /dev/null <<EOF
[Unit]
Description=nft-ui - nftables quota management web interface
After=network.target nftables.service

[Service]
Type=simple
ExecStart=/usr/local/bin/nft-ui
Environment=NFT_UI_LISTEN_ADDR=localhost:8080
Restart=on-failure
RestartSec=5
AmbientCapabilities=CAP_NET_ADMIN

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now nft-ui
```

## Files Modified by nft-ui

**Runtime ruleset changes:**
- nft-ui modifies active nftables rules directly via the `nft` command
- Changes are applied **in-memory only** by default
- **Does NOT automatically modify** `/etc/nftables.conf` or `/etc/nftables.d/` files

**Persistent files written by nft-ui:**
- `/var/lib/nft-ui/ruleset.nft` - Backup of complete ruleset (saved after each modification)
- `/var/lib/nft-ui/disabled-forwards.json` - State file for disabled port forwarding rules

**To make changes persistent across reboots:**
You must manually save the ruleset to your system's nftables configuration:
```bash
# Option 1: Save to main config
sudo nft list ruleset > /etc/nftables.conf

# Option 2: Save to a drop-in file
sudo nft list ruleset > /etc/nftables.d/nft-ui.conf
```

Or configure your system to restore from nft-ui's backup on boot.

## Expected nftables Rule Format

Quota rules in the `output` chain:

```
meta l4proto { tcp, udp } th sport 18444 quota over 100000 mbytes drop comment "block 18444 after 100GB"
```

Allowed port rules in the `input` chain:

```
tcp dport 8080 accept comment "nft-ui managed"
```

## Build from Source

```bash
# Requirements: Go 1.21+, Node.js 18+
make
```

## License

GPL-3.0
