# VPN Node Provisioning (Ansible)

Deploys a VPN node: WireGuard, Xray (VLESS+Reality), the platform node agent
and node_exporter. Implements M2b of [the plan](../../docs/21_Implementation_Plan.md).

> The platform generates **client** configs centrally (M2a). This playbook
> prepares the **server** side and runs the agent the platform calls to add/remove
> peers and clients (`PROVISIONER_MODE=agent`).

## Prerequisites

- A fresh Ubuntu 22.04/24.04 VDS reachable via SSH.
- `ansible` on your workstation.
- The agent binary built for linux/amd64:
  ```bash
  make build-agent     # outputs bin/agent → copy to deploy/ansible/files/agent
  ```

## Variables (set per host in the inventory)

| Variable | Meaning |
|---|---|
| `public_host` | Client-facing IP/host (must match the `servers.public_host` row) |
| `wg_server_private_key` | WireGuard **server** private key (printed by `cmd/seed`) |
| `wg_subnet` | e.g. `10.7.0.0/24` (matches `servers.wg_subnet`) |
| `wg_port` | e.g. `51820` |
| `reality_private_key` | Reality **server** private key (printed by `cmd/seed`) |
| `reality_short_id` | Reality short id (printed by `cmd/seed`) |
| `reality_sni` / `reality_dest` | camouflage SNI / destination |
| `reality_port` | e.g. `443` |
| `agent_token` | shared secret = platform `PROVISIONER_AGENT_TOKEN` |

## Usage

```bash
cp inventory.example.ini inventory.ini   # edit with your host + vars
ansible-playbook -i inventory.ini site.yml
```

Then in the platform DB set the server's `agent_url`
(e.g. `https://NODE_IP:8090`) and run the API with `PROVISIONER_MODE=agent`
and a matching `PROVISIONER_AGENT_TOKEN`.

## Finishing the M2 DoD

After provisioning, create a VPN via `POST /api/v1/vpn/create`, import the
returned config/QR on a real device and confirm connectivity. That last,
hardware-dependent step is what remains to fully close M2.

## Security note

Expose the agent only on a private network or behind TLS + firewall. The agent
runs privileged operations (`wg set`). Restrict `8090/tcp` to the platform IP.
