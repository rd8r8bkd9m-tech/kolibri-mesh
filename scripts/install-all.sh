#!/bin/bash
# Kolibri Mesh - Install on all servers
# This script installs the Kolibri Mesh agent on all servers

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Coordinator URL
COORDINATOR_URL="http://10.99.0.1:8080"

# Server list
declare -A SERVERS=(
  ["main"]="10.99.0.2"
  ["uiap"]="10.99.0.3"
  ["qjns"]="10.99.0.4"
  ["9fts"]="10.99.0.5"
  ["new"]="10.99.0.6"
  ["reserve242"]="31.57.26.242"
  ["hostvds-highload"]="45.38.139.182"
  ["hostvds-agent-01"]="31.57.27.128"
  ["hostvds-agent-02"]="213.232.204.223"
  ["hostvds-agent-03"]="188.130.206.204"
  ["hostvds-agent-04"]="31.59.41.146"
  ["hostvds-agent-05"]="31.56.196.10"
  ["hostvds-agent-06"]="45.39.33.252"
  ["hostvds-agent-07"]="46.8.225.34"
  ["hostvds-agent-08"]="31.59.105.200"
  ["hostvds-agent-09"]="95.182.84.254"
  ["hostvds-agent-10"]="217.60.38.191"
  ["hostvds-paris-highload"]="95.182.83.60"
  ["worker-backup"]="109.248.161.39"
)

# Function to install on a server
install_on_server() {
  local name=$1
  local ip=$2
  
  echo -e "${YELLOW}Installing on ${name} (${ip})...${NC}"
  
  # Copy binary
  scp -o ConnectTimeout=10 agent root@${ip}:/usr/local/bin/kolibri-mesh-agent 2>/dev/null || {
    echo -e "${RED}Failed to copy binary to ${name}${NC}"
    return 1
  }
  
  # Create systemd service
  ssh -o ConnectTimeout=10 root@${ip} "cat > /etc/systemd/system/kolibri-mesh-agent.service << EOF
[Unit]
Description=Kolibri Mesh Agent
After=network.target

[Service]
Type=simple
Environment=NODE_ID=${name}
Environment=NODE_NAME=${name}
Environment=COORDINATOR_URL=${COORDINATOR_URL}
Environment=NODE_IP=${ip}
ExecStart=/usr/local/bin/kolibri-mesh-agent
Restart=always

[Install]
WantedBy=multi-user.target
EOF
" 2>/dev/null || {
    echo -e "${RED}Failed to create service on ${name}${NC}"
    return 1
  }
  
  # Enable and start
  ssh -o ConnectTimeout=10 root@${ip} "systemctl daemon-reload && systemctl enable kolibri-mesh-agent && systemctl start kolibri-mesh-agent" 2>/dev/null || {
    echo -e "${RED}Failed to start service on ${name}${NC}"
    return 1
  }
  
  echo -e "${GREEN}Installed on ${name}${NC}"
  return 0
}

# Main
echo -e "${GREEN}Kolibri Mesh - Installing on all servers${NC}"
echo ""

# Build binaries
echo "Building binaries..."
go build -o agent ./cmd/agent || {
  echo -e "${RED}Failed to build agent${NC}"
  exit 1
}

# Install on each server
SUCCESS=0
FAILED=0

for name in "${!SERVERS[@]}"; do
  ip=${SERVERS[$name]}
  if install_on_server "$name" "$ip"; then
    ((SUCCESS++))
  else
    ((FAILED++))
  fi
done

echo ""
echo -e "${GREEN}Installation complete!${NC}"
echo -e "Success: ${SUCCESS}"
echo -e "Failed: ${FAILED}"
echo ""
echo "Check status with: mesh nodes"
