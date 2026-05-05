---
phase: E
title: "Backup automation"
status: pending
priority: P2
effort: 0.5d
dependencies: [A]
---

# Phase E: Backup automation

## Context Links

- OCI's free-tier backup gap (1-day retention, no PITR): [`reports/researcher-260505-1308-mysql-heatwave-deep-dive.md`](../reports/researcher-260505-1308-mysql-heatwave-deep-dive.md) — Q6
- OCI Object Storage Always-Free: 10 GB included

## Overview

Augment Oracle's 1-day automatic-backup retention with a weekly `mysqldump` cron on the Coolify VM that ships compressed dumps to OCI Object Storage. RPO target: max 1 week of data loss in the worst case (Oracle backup unavailable + most-recent weekly dump only). For dleague at <100 users this is adequate.

## Why

Always-Free retention is **1 day**. PITR is **disabled**. If you discover Sunday's bad migration on Tuesday, Oracle's automatic backup is already gone. Without an out-of-band copy, recovery is impossible.

## Requirements

**Functional**
- Weekly compressed `mysqldump` of `dleague` schema
- Upload to OCI Object Storage Always-Free bucket (`dleague-backups`)
- Retention: 6 weekly dumps + 7 daily ones (rolling)
- Documented restore procedure in `docs/runbooks/restore-mysql.md` (created in Phase F, declared here)

**Non-functional**
- Backup completes in <10 min for <500 MB schema
- Cron runs in low-traffic window (configurable; default Sunday 03:00 server time)
- Failure surfaces via Coolify notification or stderr that's caught by Coolify logging
- Backup script in repo for review, not in `/etc/cron.d` only

## Architecture

```
Coolify VM (cron, Sunday 03:00)
   └── /opt/dleague/scripts/backup-mysql.sh
       ├── mysqldump → /tmp/dleague-{date}.sql.gz
       ├── oci os object put → bucket: dleague-backups, prefix: weekly/
       └── (post) prune objects older than 6 weeks via lifecycle policy

OCI Object Storage Always-Free
   └── bucket: dleague-backups (10 GB cap)
       ├── weekly/dleague-2026-05-10.sql.gz
       ├── weekly/dleague-2026-05-17.sql.gz
       └── ...
```

Object Storage **lifecycle policy** handles retention (object older than 42 days → delete). No client-side prune logic.

## Related Code Files

**Create:**
- `scripts/backup-mysql.sh` — backup script in repo (committed for review)
- `scripts/restore-mysql.sh` — restore helper (used by runbook)
- `docs/runbooks/restore-mysql.md` — declared here, actual content authored in Phase F

**Coolify VM** (out of repo):
- `/opt/dleague/scripts/backup-mysql.sh` — copy of `scripts/backup-mysql.sh`
- `/etc/cron.d/dleague-backup` — cron entry pointing at the script
- `/opt/dleague/.backup-env` — `DB_HOST`, `DB_USER`, `DB_PASS`, `OCI_BUCKET` (mode 0600)

## Implementation Steps

1. **Provision OCI Object Storage bucket** `dleague-backups` in same region as DB system; standard storage tier.
2. **Lifecycle rule:** delete objects older than 42 days, prefix `weekly/`. Separate rule for `daily/` if/when daily dumps are added.
3. **Author `scripts/backup-mysql.sh`** (committed to repo):
   ```bash
   #!/usr/bin/env bash
   set -euo pipefail
   : "${DB_HOST:?}" "${DB_USER:?}" "${DB_PASS:?}" "${OCI_BUCKET:?}"
   STAMP=$(date -u +%Y-%m-%d)
   OUT="/tmp/dleague-${STAMP}.sql.gz"
   trap 'rm -f "$OUT"' EXIT
   mysqldump --single-transaction --routines --triggers --set-gtid-purged=OFF \
     -h "$DB_HOST" -u "$DB_USER" -p"$DB_PASS" --ssl-mode=REQUIRED dleague \
     | gzip -9 > "$OUT"
   oci os object put -bn "$OCI_BUCKET" --file "$OUT" --name "weekly/$(basename "$OUT")"
   echo "uploaded weekly/$(basename "$OUT") ($(stat -c %s "$OUT") bytes)"
   ```
4. **Cron entry** (server-side `/etc/cron.d/dleague-backup`):
   ```
   0 3 * * 0  dleague  /opt/dleague/scripts/backup-mysql.sh > /var/log/dleague-backup.log 2>&1
   ```
5. **First-run smoke test:** invoke the script manually as the `dleague` cron user; verify object lands in bucket; download + `gunzip -t` to confirm not corrupted.
6. **Restore drill** (links to Phase B drill — same procedure). Document outcome.
7. **Phase F deliverable hook:** declare `docs/runbooks/restore-mysql.md` for Phase F to author.

## Conditional sub-step (Phase B feeds back here)

If Phase B determines that `MySQL.Free` reclaims idle instances, add:

8. **Keepalive cron** on Coolify VM, every 5 minutes:
   ```
   */5 * * * *  dleague  mysql -h "$DB_HOST" -u "$DB_USER" -p"$DB_PASS" --ssl-mode=REQUIRED -e "SELECT 1" >/dev/null 2>&1
   ```
   Or, equivalently, an in-process Go ticker in the server.

This sub-step is conditional; default state is "do nothing" until Phase B verifies.

## Todo List

- [ ] Create OCI Object Storage bucket `dleague-backups`
- [ ] Configure 42-day lifecycle rule on `weekly/` prefix
- [ ] Commit `scripts/backup-mysql.sh` and `scripts/restore-mysql.sh`
- [ ] Deploy script to Coolify VM at `/opt/dleague/scripts/`
- [ ] Configure `/opt/dleague/.backup-env` with secrets (mode 0600)
- [ ] Install cron entry
- [ ] Smoke test: manual run, confirm object in bucket, gunzip integrity
- [ ] Restore drill: download dump, restore to local `mysql:8`, verify schema integrity
- [ ] (Conditional) Add keepalive cron if Phase B observes reclaim

## Success Criteria

- [ ] First weekly backup successful + retrievable
- [ ] Lifecycle rule visible in OCI console; predicted next-deletion date matches expectation
- [ ] Restore drill: dump → fresh `mysql:8` Docker → all 5 tables present, FKs intact, sample row INSERT/SELECT works
- [ ] Backup script log path tailable; failures don't silently swallow

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| `mysqldump` blocks writes during backup | Low | `--single-transaction` uses consistent snapshot, no table locks for InnoDB |
| Cron runs while server is mid-deploy → inconsistent | Low | Sunday 03:00 is off-peak; use `--single-transaction` for write-during-dump tolerance |
| Object Storage 10 GB cap exceeded | Low | 6 weekly dumps × ~50 MB compressed each = 300 MB. Plenty of headroom. |
| OCI CLI auth fails on cron user | Med | Use OCI Resource Principal or instance-principal auth; document setup |
| Bucket world-readable by accident | High | Verify bucket visibility = "Private" (default); audit ACL after creation |
| Backup encryption at rest | Med | OCI Object Storage encrypts at rest by default; for sensitive payloads, additionally GPG-encrypt the dump before upload |
| Restore overwrites production by mistake | High | `restore-mysql.sh` MUST require `--target-host` arg + interactive `yes-i-mean-it` confirmation; never default to production host |

## Security Considerations

- `.backup-env` contains plain-text DB password — file mode 0600, owned by `dleague` user
- Prefer OCI Vault retrieval over plain-text env file; if Vault is overkill at this scale, document the trade-off
- Backup files contain `bcrypt` password hashes — sufficient if dumps leak, but treat dump as sensitive
- OCI Object Storage IAM: Coolify VM's instance-principal needs `manage object-family` on bucket, nothing more
- Audit: log every put to a separate audit prefix or via OCI Audit service

## Next Steps

After Phase E success criteria are met:
- `docs/runbooks/restore-mysql.md` content authored in Phase F using this script as the source of truth
- Phase B's restore drill should reuse this exact restore script
