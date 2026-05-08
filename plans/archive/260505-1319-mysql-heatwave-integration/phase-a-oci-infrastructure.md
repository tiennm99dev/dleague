---
phase: A
title: "OCI infrastructure provisioning"
status: pending
priority: P1
effort: 0.5d
dependencies: []
---

# Phase A: OCI infrastructure provisioning

## Context Links

- Decision report: [`reports/researcher-260505-1308-mysql-heatwave-deep-dive.md`](../reports/researcher-260505-1308-mysql-heatwave-deep-dive.md)
- Networking refs: Oracle docs on [private subnet security lists](https://dasini.net/blog/2021/09/07/discovering-mysql-database-service-episode-6-update-the-private-subnet-security-list/), [NSGs for HeatWave](https://blogs.oracle.com/mysql/enhancing-security-in-oci-using-network-security-groups-heatwave-mysql)

## Overview

Provision the OCI MySQL HeatWave Always-Free DB system in a private subnet of the existing VCN, locked down by Network Security Group so only the Coolify VM can reach it on TCP 3306. No code is written in this phase.

## Requirements

**Functional**
- One MySQL HeatWave Always-Free DB system (`MySQL.Free` shape) reachable from Coolify VM private IP
- Schema `dleague` created (utf8mb4 + utf8mb4_0900_ai_ci collation)
- Two MySQL users: admin (Oracle-managed root) + `dleague_app` scoped to `dleague` schema only
- TLS-required for all client connections

**Non-functional**
- Sub-5 ms RTT from Coolify VM to DB private IP
- DB private IP not reachable from public internet
- Region matches Coolify VM region (BLOCKER — see Open Decisions)

## Architecture

```
OCI VCN (10.0.0.0/16)
├── Public subnet (10.0.1.0/24)
│   └── Coolify VM (NSG: dleague-app)
│       └── egress: TCP 3306 → dleague-db NSG
└── Private subnet (10.0.2.0/24, no IGW route)
    └── MySQL HeatWave (NSG: dleague-db)
        └── ingress: TCP 3306 ← dleague-app NSG
```

NSG-to-NSG rules (preferred over CIDR-based security list rules) — gives microsegmentation independent of subnet CIDR changes.

## Related Code Files

None. This phase touches OCI console / Terraform only.

**Optional follow-up** (out of scope this phase): commit Terraform / OCI Resource Manager stacks under `infra/oci/` for repeatable provisioning. Document as future work in Phase F.

## Implementation Steps

1. **Confirm OCI region with user** — must match existing Coolify VM region. Block subsequent steps until confirmed.
2. **Inventory existing VCN** — does the Coolify VM live in a VCN we control, or did Coolify auto-create one? If auto-created, plan adoption.
3. **Create private subnet** for the DB if not already present. CIDR `/24` is plenty.
4. **Create two NSGs:** `dleague-app` (attach to Coolify VM VNIC), `dleague-db` (will attach to HeatWave VNIC).
5. **NSG rules:**
   - `dleague-db` ingress: TCP 3306 from source NSG `dleague-app`. Optionally also TCP 33060 (X-Protocol) — skip unless needed.
   - `dleague-app` egress: TCP 3306 to destination NSG `dleague-db`.
6. **Provision MySQL HeatWave Always-Free DB system** via OCI console:
   - Shape: `MySQL.Free`
   - Storage: 50 GB (fixed on Always-Free)
   - Subnet: the private subnet from step 3
   - NSG: `dleague-db`
   - MySQL version: 8.x LTS (Oracle-default current LTS)
   - **Skip the "HeatWave Cluster" toggle** — we're OLTP-only
   - Set strong admin password, store in OCI Vault
7. **Wait for DB system to enter ACTIVE state.** Note its private IP.
8. **Connect from Coolify VM** via `mysql -h <private-ip> -u admin -p` to confirm reachability + TLS handshake.
9. **Run bootstrap SQL as admin:**
   - `CREATE DATABASE dleague CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;`
   - `CREATE USER 'dleague_app'@'%' IDENTIFIED BY '<random-strong-pw>' REQUIRE SSL;`
   - `GRANT SELECT, INSERT, UPDATE, DELETE, EXECUTE ON dleague.* TO 'dleague_app'@'%';`
   - `GRANT CREATE, ALTER, DROP, INDEX, REFERENCES ON dleague.* TO 'dleague_app'@'%';` (needed for migrator)
10. **Test as `dleague_app`:** can SELECT, INSERT, CREATE TABLE in `dleague`; cannot touch `mysql.*` or `information_schema` beyond default-readable views.
11. **Store both passwords in OCI Vault.** Reference vault secret OCID in Coolify env config (later phases).
12. **Document the runbook** as inline comments on each resource ("Why this NSG rule exists", "Recovery contact").

## Todo List

- [ ] Confirm OCI region (user blocker)
- [ ] Inventory existing VCN, identify private subnet candidate
- [ ] Create / verify private subnet CIDR
- [ ] Create NSG `dleague-app` and attach to Coolify VM
- [ ] Create NSG `dleague-db`
- [ ] Add ingress/egress rules (NSG-to-NSG)
- [ ] Provision `MySQL.Free` DB system, skip HeatWave cluster
- [ ] Bootstrap `dleague` schema + `dleague_app` user with least-privilege grants
- [ ] Smoke test from Coolify VM
- [ ] Store admin + app passwords in OCI Vault

## Success Criteria

- [ ] DB system in ACTIVE state, private IP reachable from Coolify VM
- [ ] `mysql -h <ip> -u dleague_app -p` connects with `--ssl-mode=REQUIRED`
- [ ] `dleague_app` cannot list other schemas (verify with `SHOW DATABASES`)
- [ ] DB private IP returns no response from any host outside `dleague-app` NSG
- [ ] Admin + app passwords stored in OCI Vault, NOT in plain-text config files

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Wrong region picked (mismatch with Coolify VM) | High | Block step 6 until user confirms region in writing |
| Always-Free quota already consumed by an existing DB system in this tenancy | Med | Inventory first; if exhausted, decide whether to delete the existing one or upgrade |
| Coolify VM auto-created VCN restricts what we can change | Med | Read-only inspect first; if too restrictive, plan migration to a fresh VCN before provisioning |
| TLS cert verification fails from Go (nhooyr-style strict mode) | Low | Document `tls=true` (verify-full) requires Oracle-issued CA bundle in client; Go's `mysql` driver pulls from system roots which already include Oracle's |

## Security Considerations

- **No public endpoint.** DB lives in private subnet without IGW route.
- **NSG-based microsegmentation** — even other VMs in the same VCN cannot reach the DB unless explicitly added to `dleague-app` NSG.
- **TLS-required** at MySQL level (`REQUIRE SSL` on user creation).
- **Least-privilege grants** for `dleague_app`: schema-scoped, no GRANT OPTION, no SUPER, no admin tables.
- **Password storage** in OCI Vault, never in repo or env files committed to git.

## Next Steps

After Phase A success criteria are met:
- **Phase B** (idle-reclaim test) starts immediately — leave the DB idle for 14 days while running other phases in parallel against it
- **Phase C** (Go scaffolding) can start in parallel using the live DB endpoint
