# nft-ui Setup Guide for Arch Linux

This guide will walk you through installing and configuring nft-ui on Arch Linux.

## Prerequisites

- Arch Linux system with root access
- Internet connection
- `curl` installed (usually pre-installed)

## Step 1: Install nftables

nft-ui requires nftables to be installed and running on your system.

```bash
# Install nftables
sudo pacman -S nftables

# Enable and start nftables service
sudo systemctl enable nftables.service
sudo systemctl start nftables.service

# Verify nftables is running
sudo systemctl status nftables.service
```

## Step 2: Install nft-ui

Use the official installation script to download and install the latest release:

### Stable Release (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/grey-org/nft-ui/main/install.sh | sudo bash
```

### Beta/Pre-release Version

If you want to test the latest features:

```bash
curl -fsSL https://raw.githubusercontent.com/grey-org/nft-ui/main/install.sh | sudo bash -s -- --beta
```

### Specific Version

To install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/grey-org/nft-ui/main/install.sh | sudo bash -s -- --tag v1.0.0
```

The script will:
- Detect your system architecture (amd64/arm64)
- Download the appropriate binary
- Install it to `/usr/local/bin/nft-ui`
- Make it executable

## Step 3: Set Up systemd Service

### Download the Service File

```bash
# Download the systemd service file
sudo curl -fsSL https://raw.githubusercontent.com/grey-org/nft-ui/main/nft-ui.service \
    -o /etc/systemd/system/nft-ui.service
```

### Configure the Service (Optional)

Edit the service file to customize settings:

```bash
sudo nano /etc/systemd/system/nft-ui.service
```

Uncomment and modify environment variables as needed:

```ini
Environment=NFT_UI_LISTEN_ADDR=localhost:8080
Environment=NFT_UI_AUTH_USER=admin
Environment=NFT_UI_AUTH_PASSWORD=changeme
Environment=NFT_UI_READ_ONLY=false
```

**Important Security Settings:**
- `NFT_UI_LISTEN_ADDR`: Change to `0.0.0.0:8080` if you need remote access (use with caution)
- `NFT_UI_AUTH_USER` and `NFT_UI_AUTH_PASSWORD`: Set strong credentials
- `NFT_UI_READ_ONLY`: Set to `true` if you only need monitoring capabilities

Alternatively, you can use an environment file:

```bash
# Create environment file
sudo mkdir -p /etc/nft-ui
sudo nano /etc/nft-ui/env
```

Add your configuration:

```
NFT_UI_LISTEN_ADDR=localhost:8080
NFT_UI_AUTH_USER=admin
NFT_UI_AUTH_PASSWORD=your-secure-password
NFT_UI_READ_ONLY=false
NFT_UI_TOKEN_SALT=your-random-salt-string
```

Then uncomment the `EnvironmentFile` line in the service file:

```ini
EnvironmentFile=/etc/nft-ui/env
```

### Reload systemd

After creating or modifying the service file:

```bash
sudo systemctl daemon-reload
```

## Step 4: Start and Enable the Service

```bash
# Enable the service to start on boot
sudo systemctl enable nft-ui.service

# Start the service
sudo systemctl start nft-ui.service

# Check service status
sudo systemctl status nft-ui.service
```

## Step 5: Verify Installation

### Check if the service is running:

```bash
sudo systemctl status nft-ui.service
```

### Test the web interface:

```bash
curl http://localhost:8080
```

If authentication is enabled, use:

```bash
curl -u admin:changeme http://localhost:8080
```

### View logs:

```bash
# View recent logs
sudo journalctl -u nft-ui.service -n 50

# Follow logs in real-time
sudo journalctl -u nft-ui.service -f
```

## Accessing the Web Interface

- **Local access**: Open `http://localhost:8080` in your browser
- **Remote access**: If configured, access via `http://your-server-ip:8080`

## Firewall Configuration (Optional)

If you need to access nft-ui from other machines:

```bash
# Allow port 8080 through the firewall
sudo nft add rule inet filter input tcp dport 8080 accept
```

**Security Warning**: Remote access should be secured with:
- Strong authentication credentials
- HTTPS with a reverse proxy (nginx/caddy)
- VPN or SSH tunnel
- IP whitelist restrictions

## Troubleshooting

### Service fails to start

Check logs for errors:

```bash
sudo journalctl -u nft-ui.service -n 100 --no-pager
```

### Permission issues

Verify the service has proper capabilities:

```bash
sudo systemctl show nft-ui.service | grep Capabilities
```

### nftables not responding

Ensure nftables is running and configured:

```bash
sudo systemctl status nftables.service
sudo nft list ruleset
```

### Port already in use

Check if another service is using port 8080:

```bash
sudo ss -tlnp | grep 8080
```

Change the listen address in the service configuration if needed.

## Updating nft-ui

To update to the latest version, simply run the installation script again:

```bash
curl -fsSL https://raw.githubusercontent.com/grey-org/nft-ui/main/install.sh | sudo bash
```

Then restart the service:

```bash
sudo systemctl restart nft-ui.service
```

## Uninstalling

```bash
# Stop and disable the service
sudo systemctl stop nft-ui.service
sudo systemctl disable nft-ui.service

# Remove service file
sudo rm /etc/systemd/system/nft-ui.service

# Remove binary
sudo rm /usr/local/bin/nft-ui

# Remove configuration (optional)
sudo rm -rf /etc/nft-ui

# Reload systemd
sudo systemctl daemon-reload
```

## Additional Resources

- [GitHub Repository](https://github.com/grey-org/nft-ui)
- [nftables Documentation](https://wiki.nftables.org/)
- [Arch Linux nftables Wiki](https://wiki.archlinux.org/title/Nftables)

## Support

For issues, questions, or feature requests, please visit the [GitHub Issues](https://github.com/grey-org/nft-ui/issues) page.
