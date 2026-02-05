#!/bin/bash

# V2bX Installation Script
# https://github.com/yuwan027/V2bX

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

GITHUB_REPO="yuwan027/V2bX"
SERVICE_NAME="V2bX"
INSTALL_DIR="/usr/local/V2bX"
CONFIG_DIR="/etc/V2bX"

# Check root
[[ $EUID -ne 0 ]] && echo -e "${red}Error: This script must be run as root!${plain}" && exit 1

# Check OS
check_os() {
    if [[ -f /etc/redhat-release ]]; then
        OS="centos"
    elif cat /etc/issue | grep -Eqi "debian"; then
        OS="debian"
    elif cat /etc/issue | grep -Eqi "ubuntu"; then
        OS="ubuntu"
    elif cat /etc/issue | grep -Eqi "centos|red hat|redhat"; then
        OS="centos"
    elif cat /proc/version | grep -Eqi "debian"; then
        OS="debian"
    elif cat /proc/version | grep -Eqi "ubuntu"; then
        OS="ubuntu"
    elif cat /proc/version | grep -Eqi "centos|red hat|redhat"; then
        OS="centos"
    elif cat /etc/os-release | grep -Eqi "alpine"; then
        OS="alpine"
    else
        echo -e "${red}Unsupported OS!${plain}"
        exit 1
    fi
}

# Check architecture
check_arch() {
    ARCH=$(uname -m)
    case $ARCH in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            echo -e "${red}Unsupported architecture: $ARCH${plain}"
            exit 1
            ;;
    esac
}

# Install dependencies
install_deps() {
    if [[ $OS == "centos" ]]; then
        yum install -y wget curl unzip
    elif [[ $OS == "alpine" ]]; then
        apk add --no-cache wget curl unzip
    else
        apt-get update
        apt-get install -y wget curl unzip
    fi
}

# Get latest version
get_latest_version() {
    VERSION=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ -z "$VERSION" ]]; then
        echo -e "${yellow}Failed to get latest version, using 'latest'${plain}"
        VERSION="latest"
    fi
}

# Download V2bX
download_v2bx() {
    echo -e "${green}Downloading V2bX...${plain}"

    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/V2bX-linux-${ARCH}"

    mkdir -p $INSTALL_DIR
    wget -q --show-progress -O ${INSTALL_DIR}/V2bX "$DOWNLOAD_URL"

    if [[ $? -ne 0 ]]; then
        echo -e "${red}Download failed!${plain}"
        exit 1
    fi

    chmod +x ${INSTALL_DIR}/V2bX
    echo -e "${green}V2bX downloaded successfully!${plain}"
}

# Download geo data
download_geo_data() {
    echo -e "${green}Downloading geo data...${plain}"

    wget -q --show-progress -O ${INSTALL_DIR}/geoip.dat \
        "https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/geoip.dat"
    wget -q --show-progress -O ${INSTALL_DIR}/geosite.dat \
        "https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/geosite.dat"

    echo -e "${green}Geo data downloaded!${plain}"
}

# Create systemd service
create_service() {
    if [[ $OS == "alpine" ]]; then
        # OpenRC for Alpine
        cat > /etc/init.d/V2bX << 'EOF'
#!/sbin/openrc-run

name="V2bX"
description="V2bX Service"
command="/usr/local/V2bX/V2bX"
command_args="server"
pidfile="/run/${RC_SVCNAME}.pid"
command_background="yes"

depend() {
    need net
    after firewall
}
EOF
        chmod +x /etc/init.d/V2bX
        rc-update add V2bX default
    else
        # Systemd for other distros
        cat > /etc/systemd/system/V2bX.service << 'EOF'
[Unit]
Description=V2bX Service
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/V2bX/V2bX server
Restart=always
RestartSec=10
LimitNOFILE=999999

[Install]
WantedBy=multi-user.target
EOF
        systemctl daemon-reload
        systemctl enable V2bX
    fi

    echo -e "${green}Service created!${plain}"
}

# Create config directory
create_config() {
    mkdir -p $CONFIG_DIR

    if [[ ! -f ${CONFIG_DIR}/config.yml ]]; then
        cat > ${CONFIG_DIR}/config.yml << 'EOF'
Log:
  Level: warning
  Output: ""
Cores:
  - Type: xray
    Log:
      Level: warning
    XrayConfig:
      InboundConfigPath:
      OutboundConfigPath:
      DnsConfigPath:
      RouteConfigPath:
      ConnectionConfig:
        Handshake: 4
        ConnIdle: 30
        UplinkOnly: 2
        DownlinkOnly: 4
        BufferSize: 64
  # - Type: sing
  #   Log:
  #     Level: warn
  #     Timestamp: true
  #   NtpConfig:
  #     Enable: false
  #     Server: time.apple.com
  #     ServerPort: 0
  #   OriginalPath:
Nodes:
  - ApiConfig:
      ApiHost: "https://your-panel.com"
      ApiKey: "your-api-key"
      NodeID: 1
      NodeType: V2ray
      Timeout: 30
      RuleListPath:
    Options:
      ListenIP: 0.0.0.0
      SendIP: 0.0.0.0
      LimitConfig:
        EnableRealtime: false
        SpeedLimit: 0
        IPLimit: 0
        ConnLimit: 0
        EnableTrigger: false
        TriggerSpeed: 0
        TriggerDuration: 0
        TriggerAction: 0
      CertConfig:
        CertMode: none
        CertDomain: ""
        CertFile: ""
        KeyFile: ""
        Provider: ""
        Email: ""
        DNSEnv:
EOF
        echo -e "${yellow}Config template created at ${CONFIG_DIR}/config.yml${plain}"
        echo -e "${yellow}Please edit the config file before starting the service.${plain}"
    fi
}

# Create management script
create_cli() {
    cat > /usr/bin/V2bX << 'EOFCLI'
#!/bin/bash

SERVICE_NAME="V2bX"
INSTALL_DIR="/usr/local/V2bX"
CONFIG_DIR="/etc/V2bX"

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

check_status() {
    if systemctl is-active --quiet $SERVICE_NAME 2>/dev/null; then
        echo -e "${green}V2bX is running${plain}"
    else
        echo -e "${red}V2bX is not running${plain}"
    fi
}

case "$1" in
    start)
        systemctl start $SERVICE_NAME
        echo -e "${green}V2bX started${plain}"
        ;;
    stop)
        systemctl stop $SERVICE_NAME
        echo -e "${green}V2bX stopped${plain}"
        ;;
    restart)
        systemctl restart $SERVICE_NAME
        echo -e "${green}V2bX restarted${plain}"
        ;;
    status)
        check_status
        ;;
    log)
        journalctl -u $SERVICE_NAME -f
        ;;
    config)
        ${EDITOR:-nano} ${CONFIG_DIR}/config.yml
        ;;
    update)
        bash <(curl -Ls https://raw.githubusercontent.com/yuwan027/V2bX/dev_new/install.sh)
        ;;
    uninstall)
        systemctl stop $SERVICE_NAME 2>/dev/null
        systemctl disable $SERVICE_NAME 2>/dev/null
        rm -rf $INSTALL_DIR
        rm -f /etc/systemd/system/V2bX.service
        rm -f /usr/bin/V2bX
        systemctl daemon-reload
        echo -e "${green}V2bX uninstalled${plain}"
        ;;
    *)
        echo "V2bX Management Script"
        echo ""
        echo "Usage: V2bX {start|stop|restart|status|log|config|update|uninstall}"
        echo ""
        echo "Commands:"
        echo "  start     - Start V2bX"
        echo "  stop      - Stop V2bX"
        echo "  restart   - Restart V2bX"
        echo "  status    - Show V2bX status"
        echo "  log       - Show V2bX logs"
        echo "  config    - Edit config file"
        echo "  update    - Update V2bX"
        echo "  uninstall - Uninstall V2bX"
        ;;
esac
EOFCLI
    chmod +x /usr/bin/V2bX
}

# Main
main() {
    echo -e "${green}V2bX Installation Script${plain}"
    echo ""

    check_os
    check_arch
    install_deps
    get_latest_version
    download_v2bx
    download_geo_data
    create_service
    create_config
    create_cli

    echo ""
    echo -e "${green}Installation completed!${plain}"
    echo ""
    echo -e "Usage: ${yellow}V2bX {start|stop|restart|status|log|config}${plain}"
    echo ""
    echo -e "Config file: ${yellow}${CONFIG_DIR}/config.yml${plain}"
    echo -e "Please edit the config file and then run: ${yellow}V2bX start${plain}"
}

main
