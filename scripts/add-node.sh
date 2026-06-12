#!/usr/bin/env bash
#
# add-node.sh — register and provision a new VPN node end-to-end.
#
# Usage:
#   scripts/add-node.sh <HOST_IP> <COUNTRY_ISO> "<Country Name>" [SSH_USER]
#
# Example:
#   scripts/add-node.sh 203.0.113.20 NL "Netherlands"
#
# What it does:
#   1. generates WireGuard + Reality server keys (cmd/keygen)
#   2. installs an SSH key on the node (asks for the root password once)
#   3. runs the Ansible playbook (WireGuard + Xray + agent + node_exporter + ufw)
#   4. registers the country and server in the platform database
#
# Requirements: Docker running, the local stack up (`make up`), and
# deploy/ansible/secrets/credentials.env containing AGENT_TOKEN (shared with the
# platform's PROVISIONER_AGENT_TOKEN). Run from the repo root or anywhere.
set -euo pipefail
export MSYS_NO_PATHCONV=1   # Git Bash on Windows: keep container paths intact

die() { echo "error: $*" >&2; exit 1; }

[ $# -ge 3 ] || die "usage: $0 <HOST_IP> <COUNTRY_ISO> \"<Country Name>\" [SSH_USER]"
HOST="$1"; ISO="$(echo "$2" | tr '[:lower:]' '[:upper:]')"; NAME="$3"; SSH_USER="${4:-root}"
WG_SUBNET="${WG_SUBNET:-10.7.0.0/24}"
COMPOSE="deploy/ansible/secrets"

# Move to repo root (this script lives in scripts/).
cd "$(dirname "$0")/.."
MOUNT="$(pwd -W 2>/dev/null || pwd)"
CREDS="deploy/ansible/secrets/credentials.env"
[ -f "$CREDS" ] || die "missing $CREDS"

AGENT_TOKEN="$(grep '^AGENT_TOKEN=' "$CREDS" | cut -d= -f2- | tr -d '\r')"
[ -n "$AGENT_TOKEN" ] || die "AGENT_TOKEN is empty in $CREDS"

echo ">> checking the node is not already registered…"
EXIST="$(docker compose -f deploy/docker-compose.yml exec -T postgres \
  psql -U vpn -d vpn -tAc "SELECT count(*) FROM servers WHERE public_host='$HOST'" | tr -d '[:space:]')"
[ "$EXIST" = "0" ] || die "a server with public_host=$HOST is already registered"

read -r -s -p "Root password for $SSH_USER@$HOST: " NODE_PW; echo
[ -n "$NODE_PW" ] || die "empty password"

echo ">> building the node agent…"
docker run --rm -v "$MOUNT:/src" -w /src -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=amd64 \
  golang:1.23-alpine go build -o deploy/ansible/files/agent ./cmd/agent >/dev/null

echo ">> generating node keys…"
KEYS="$(docker run --rm -v "$MOUNT:/src" -w /src golang:1.23-alpine go run ./cmd/keygen)"
field() { echo "$KEYS" | grep -o "\"$1\": *\"[^\"]*\"" | cut -d'"' -f4; }
WG_PRIV="$(field wg_private_key)";   WG_PUB="$(field wg_public_key)"
R_PRIV="$(field reality_private_key)"; R_PUB="$(field reality_public_key)"
SID="$(field reality_short_id)"
[ -n "$WG_PUB" ] && [ -n "$R_PUB" ] || die "key generation failed"

echo ">> installing SSH key and detecting WAN interface…"
WANIF="$(docker run --rm -e SSHPASS="$NODE_PW" -v "$MOUNT:/src" -w /src alpine:3.20 sh -s <<NODEPREP
apk add --no-cache openssh-client sshpass >/dev/null 2>&1
KEY=/src/deploy/ansible/secrets/id_node
[ -f "\$KEY" ] || ssh-keygen -t ed25519 -f "\$KEY" -N "" -q -C vpn-platform
cp "\$KEY" /tmp/k; chmod 600 /tmp/k
PUB="\$(cat "\$KEY.pub")"
sshpass -e ssh -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
  "mkdir -p ~/.ssh && chmod 700 ~/.ssh && touch ~/.ssh/authorized_keys && grep -qF \"\$PUB\" ~/.ssh/authorized_keys || printf '%s\n' \"\$PUB\" >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys" >/dev/null 2>&1
ssh -i /tmp/k -o IdentitiesOnly=yes -o StrictHostKeyChecking=no "$SSH_USER@$HOST" \
  "ip route get 1.1.1.1 | sed -n 's/.* dev \([^ ]*\).*/\1/p' | head -1"
NODEPREP
)"
WANIF="$(echo "$WANIF" | tr -d '[:space:]')"
[ -n "$WANIF" ] || die "could not reach the node over SSH"
echo "   WAN interface: $WANIF"

HOSTNAME_TAG="$(echo "$ISO" | tr '[:upper:]' '[:lower:]')-$(echo "$HOST" | tr '.' '-')"
INV="$COMPOSE/inv_${HOST}.ini"
cat > "$INV" <<INVENTORY
[vpn_nodes]
$HOSTNAME_TAG ansible_host=$HOST ansible_user=$SSH_USER

[vpn_nodes:vars]
ansible_ssh_private_key_file=/root/id_node
ansible_python_interpreter=/usr/bin/python3
ansible_host_key_checking=false
public_host=$HOST
wan_interface=$WANIF
wg_server_private_key=$WG_PRIV
wg_subnet=$WG_SUBNET
wg_address=10.7.0.1/24
wg_port=51820
reality_private_key=$R_PRIV
reality_short_id=$SID
reality_sni=www.microsoft.com
reality_dest=www.microsoft.com:443
reality_port=443
agent_port=8090
agent_token=$AGENT_TOKEN
INVENTORY

echo ">> running Ansible (this installs Docker/WireGuard/Xray/agent)…"
docker run --rm -v "$MOUNT:/src" -w /src python:3.12-alpine sh -s <<ANSIBLE
set -e
apk add --no-cache openssh-client >/dev/null 2>&1
pip install --quiet --disable-pip-version-check ansible >/dev/null 2>&1
cp /src/deploy/ansible/secrets/id_node /root/id_node && chmod 600 /root/id_node
cd /src/deploy/ansible
ANSIBLE_HOST_KEY_CHECKING=False ansible-playbook -i secrets/inv_${HOST}.ini site.yml
ANSIBLE

echo ">> registering the server in the database…"
docker compose -f deploy/docker-compose.yml exec -T postgres psql -U vpn -d vpn <<SQL
INSERT INTO countries (iso, name, enabled, priority)
VALUES ('$ISO', '$NAME', true, 50)
ON CONFLICT (iso) DO UPDATE SET name = EXCLUDED.name, enabled = true;

INSERT INTO servers (
  country_id, provider, hostname, public_host, agent_url, status, priority, capacity,
  wg_port, wg_public_key, wg_subnet, wg_dns,
  reality_port, reality_public_key, reality_sni, reality_short_id, reality_dest)
SELECT c.id, 'manual', '$HOSTNAME_TAG', '$HOST', 'http://$HOST:8090', 'active', 10, 1000,
  51820, '$WG_PUB', '$WG_SUBNET', '1.1.1.1',
  443, '$R_PUB', 'www.microsoft.com', '$SID', 'www.microsoft.com:443'
FROM countries c WHERE c.iso = '$ISO';
SQL

echo ""
echo "✅ Node $HOST ($NAME) provisioned and registered."
echo "   It will appear in the bot and API immediately."
echo "   Node private keys live only in $INV (git-ignored)."
