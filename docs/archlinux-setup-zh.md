# nft-ui Arch Linux 安装指南

本指南将指导您在 Arch Linux 上安装和配置 nft-ui。

## 前置要求

- 具有 root 权限的 Arch Linux 系统
- 互联网连接
- 已安装 `curl`（通常预装）

## 步骤 1: 安装 nftables

nft-ui 需要系统上安装并运行 nftables。

```bash
# 安装 nftables
sudo pacman -S nftables

# 启用并启动 nftables 服务
sudo systemctl enable nftables.service
sudo systemctl start nftables.service

# 验证 nftables 正在运行
sudo systemctl status nftables.service
```

## 步骤 2: 安装 nft-ui

使用官方安装脚本下载并安装最新版本：

### 稳定版本（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/grey-org/nft-ui/main/install.sh | sudo bash
```

### Beta/预发布版本

如果您想测试最新功能：

```bash
curl -fsSL https://raw.githubusercontent.com/grey-org/nft-ui/main/install.sh | sudo bash -s -- --beta
```

### 指定版本

安装特定版本：

```bash
curl -fsSL https://raw.githubusercontent.com/grey-org/nft-ui/main/install.sh | sudo bash -s -- --tag v1.0.0
```

安装脚本将：
- 自动检测系统架构（amd64/arm64）
- 下载相应的二进制文件
- 安装到 `/usr/local/bin/nft-ui`
- 设置可执行权限

## 步骤 3: 配置 systemd 服务

### 下载服务文件

```bash
# 下载 systemd 服务文件
sudo curl -fsSL https://raw.githubusercontent.com/grey-org/nft-ui/main/nft-ui.service \
    -o /etc/systemd/system/nft-ui.service
```

### 配置服务（可选）

编辑服务文件以自定义设置：

```bash
sudo nano /etc/systemd/system/nft-ui.service
```

根据需要取消注释并修改环境变量：

```ini
Environment=NFT_UI_LISTEN_ADDR=localhost:8080
Environment=NFT_UI_AUTH_USER=admin
Environment=NFT_UI_AUTH_PASSWORD=changeme
Environment=NFT_UI_READ_ONLY=false
```

**重要安全设置：**
- `NFT_UI_LISTEN_ADDR`：如果需要远程访问，改为 `0.0.0.0:8080`（谨慎使用）
- `NFT_UI_AUTH_USER` 和 `NFT_UI_AUTH_PASSWORD`：设置强密码
- `NFT_UI_READ_ONLY`：如果只需要监控功能，设置为 `true`

或者，您可以使用环境变量文件：

```bash
# 创建环境变量文件
sudo mkdir -p /etc/nft-ui
sudo nano /etc/nft-ui/env
```

添加您的配置：

```
NFT_UI_LISTEN_ADDR=localhost:8080
NFT_UI_AUTH_USER=admin
NFT_UI_AUTH_PASSWORD=your-secure-password
NFT_UI_READ_ONLY=false
NFT_UI_TOKEN_SALT=your-random-salt-string
```

然后在服务文件中取消注释 `EnvironmentFile` 行：

```ini
EnvironmentFile=/etc/nft-ui/env
```

### 重载 systemd

创建或修改服务文件后：

```bash
sudo systemctl daemon-reload
```

## 步骤 4: 启动并启用服务

```bash
# 启用服务以在启动时自动运行
sudo systemctl enable nft-ui.service

# 启动服务
sudo systemctl start nft-ui.service

# 检查服务状态
sudo systemctl status nft-ui.service
```

## 步骤 5: 验证安装

### 检查服务是否正在运行：

```bash
sudo systemctl status nft-ui.service
```

### 测试 Web 界面：

```bash
curl http://localhost:8080
```

如果启用了身份验证，使用：

```bash
curl -u admin:changeme http://localhost:8080
```

### 查看日志：

```bash
# 查看最近的日志
sudo journalctl -u nft-ui.service -n 50

# 实时跟踪日志
sudo journalctl -u nft-ui.service -f
```

## 访问 Web 界面

- **本地访问**：在浏览器中打开 `http://localhost:8080`
- **远程访问**：如已配置，通过 `http://your-server-ip:8080` 访问

## 防火墙配置（可选）

如果需要从其他机器访问 nft-ui：

```bash
# 允许端口 8080 通过防火墙
sudo nft add rule inet filter input tcp dport 8080 accept
```

**安全警告**：远程访问应该通过以下方式保护：
- 强身份验证凭据
- 使用反向代理（nginx/caddy）配置 HTTPS
- VPN 或 SSH 隧道
- IP 白名单限制

## 故障排除

### 服务启动失败

检查日志中的错误：

```bash
sudo journalctl -u nft-ui.service -n 100 --no-pager
```

### 权限问题

验证服务是否具有正确的权限：

```bash
sudo systemctl show nft-ui.service | grep Capabilities
```

### nftables 无响应

确保 nftables 正在运行并已配置：

```bash
sudo systemctl status nftables.service
sudo nft list ruleset
```

### 端口已被占用

检查是否有其他服务正在使用端口 8080：

```bash
sudo ss -tlnp | grep 8080
```

如有需要，在服务配置中更改监听地址。

## 更新 nft-ui

要更新到最新版本，只需再次运行安装脚本：

```bash
curl -fsSL https://raw.githubusercontent.com/grey-org/nft-ui/main/install.sh | sudo bash
```

然后重启服务：

```bash
sudo systemctl restart nft-ui.service
```

## 卸载

```bash
# 停止并禁用服务
sudo systemctl stop nft-ui.service
sudo systemctl disable nft-ui.service

# 删除服务文件
sudo rm /etc/systemd/system/nft-ui.service

# 删除二进制文件
sudo rm /usr/local/bin/nft-ui

# 删除配置（可选）
sudo rm -rf /etc/nft-ui

# 重载 systemd
sudo systemctl daemon-reload
```

## 其他资源

- [GitHub 仓库](https://github.com/grey-org/nft-ui)
- [nftables 文档](https://wiki.nftables.org/)
- [Arch Linux nftables Wiki](https://wiki.archlinux.org/title/Nftables)

## 支持

如有问题、疑问或功能请求，请访问 [GitHub Issues](https://github.com/grey-org/nft-ui/issues) 页面。
