# Operator scripts

## add-node.sh — add a VPN server yourself

Provisions a fresh VDS and registers it as a VPN node in one command.

```bash
scripts/add-node.sh <HOST_IP> <COUNTRY_ISO> "<Country Name>" [SSH_USER]
# e.g.
scripts/add-node.sh 203.0.113.20 NL "Netherlands"
```

It will:
1. generate WireGuard + Reality server keys,
2. ask for the node's root password **once** (to install an SSH key),
3. run the Ansible playbook (Docker + WireGuard + Xray/Reality + agent + node_exporter + firewall),
4. register the country and server in the platform database.

The new server appears in the Telegram bot and the API immediately — no restart needed.

### Requirements
- Docker running and the local stack up (`make up`).
- `deploy/ansible/secrets/credentials.env` with `AGENT_TOKEN` set (the same value as the
  platform's `PROVISIONER_AGENT_TOKEN`). All nodes share this token.
- A fresh Ubuntu 22.04/24.04 VDS reachable over SSH.

### Notes
- Re-running for an already-registered IP is refused (safe).
- The node's **private** keys are written only to `deploy/ansible/secrets/inv_<host>.ini`,
  which is git-ignored. Keep that file safe.
- On Windows run it from Git Bash.
