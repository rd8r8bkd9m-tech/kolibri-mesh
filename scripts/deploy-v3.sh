#!/bin/bash
# Kolibri Mesh — Deploy agents using SSH config
# Fixed version with proper env var passing

COORDINATOR_URL="http://178.207.11.90:8080"

deploy() {
  local host=$1
  local node_id=$2
  local node_ip=$3
  
  echo -n "$host... "
  
  # Create a start script on the remote server
  ssh -o ConnectTimeout=10 "$host" "cat > /tmp/start-kolibri-mesh.sh << 'SCRIPT'
#!/bin/bash
pkill -9 -f kolibri-mesh-agent 2>/dev/null
sleep 1
export NODE_ID=\"$node_id\"
export NODE_NAME=\"$node_id\"
export COORDINATOR_URL=\"$COORDINATOR_URL\"
export NODE_IP=\"$node_ip\"
nohup /tmp/kolibri-mesh-agent > /tmp/kolibri-mesh-agent.log 2>&1 &
SCRIPT
chmod +x /tmp/start-kolibri-mesh.sh
/tmp/start-kolibri-mesh.sh" 2>/dev/null
  
  echo "✅"
}

echo "Deploying agents to all servers..."
echo "Coordinator: $COORDINATOR_URL"
echo ""

deploy "kolibri-main" "main" "10.99.0.2"
deploy "kolibri-uiap" "uiap" "10.99.0.3"
deploy "kolibri-qjns" "qjns" "10.99.0.4"
deploy "kolibri-9fts" "9fts" "10.99.0.5"
deploy "kolibri-new" "new" "10.99.0.6"
deploy "kolibri-direct" "direct" "78.17.4.108"
deploy "kolibri-worker-backup" "worker-backup" "109.248.161.39"
deploy "reserve242" "reserve242" "31.57.26.242"
deploy "hostvds-highload" "highload" "45.38.139.182"
deploy "hostvds-agent-01" "agent-01" "31.57.27.128"
deploy "hostvds-agent-02" "agent-02" "213.232.204.223"
deploy "hostvds-agent-03" "agent-03" "188.130.206.204"
deploy "hostvds-agent-04" "agent-04" "31.59.41.146"
deploy "hostvds-agent-05" "agent-05" "31.56.196.10"
deploy "hostvds-agent-06" "agent-06" "45.39.33.252"
deploy "hostvds-agent-07" "agent-07" "46.8.225.34"
deploy "hostvds-agent-08" "agent-08" "31.59.105.200"
deploy "hostvds-agent-09" "agent-09" "95.182.84.254"
deploy "hostvds-agent-10" "agent-10" "217.60.38.191"
deploy "hostvds-paris-highload" "paris" "95.182.83.60"

echo ""
echo "Done! Check: curl $COORDINATOR_URL/api/nodes"
