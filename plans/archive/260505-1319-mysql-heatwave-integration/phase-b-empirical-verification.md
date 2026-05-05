---
phase: B
title: "Empirical verification (idle-reclaim + max_connections)"
status: pending
priority: P1
effort: 14d-elapsed-passive
dependencies: [A]
---

# Phase B: Empirical verification

## Context Links

- Deep dive: [`reports/researcher-260505-1308-mysql-heatwave-deep-dive.md`](../reports/researcher-260505-1308-mysql-heatwave-deep-dive.md) — questions Q4 and Q5 are explicitly unverified

## Overview

Run two empirical tests against the provisioned `MySQL.Free` instance to resolve the two **unverified** questions from the deep-dive research. Results gate the **production cutover**, not development scaffolding — so phases C/D/E/F can run in parallel with the 14-day idle observation.

## Why this phase exists

Oracle docs do not say whether `MySQL.Free` has an idle-reclaim policy (Autonomous Database has 7-day stop / 90-day reclaim). Default `max_connections` for Always-Free is also undocumented. Going to production without knowing either is operational malpractice.

## Requirements

**Functional**
- Confirm whether `MySQL.Free` reclaims/stops idle instances
- Record actual `max_connections` value
- Validate restore-from-backup workflow end-to-end

**Non-functional**
- Each verification step independently re-runnable
- Results recorded in this phase file (append a "Findings" section at the end before marking the phase complete)

## Architecture

```
Day 0:    Provision (Phase A done)
Day 0+1h: Run quick checks (max_connections, version, tls)
Day 0..14: Leave idle (no SELECT 1 cron during this window)
Day 14:   Check state. RUNNING → idle-reclaim does NOT exist for MySQL.Free
                       STOPPED → reclaim exists; need keepalive
Day 14+1h: Restore drill (clone DB system from automatic backup; time it)
```

## Related Code Files

None. This phase is observational + console operations.

May produce:
- A new entry under `## Findings` in this file (markdown only)
- A new keepalive cron in Phase E if reclaim is observed

## Implementation Steps

### Quick checks (Day 0, run once)

1. Connect from Coolify VM as `admin` (or as `dleague_app` if grants allow):
   ```sql
   SHOW VARIABLES LIKE 'max_connections';
   SHOW VARIABLES LIKE 'wait_timeout';
   SHOW VARIABLES LIKE 'interactive_timeout';
   SELECT VERSION();
   SHOW STATUS LIKE 'Ssl_cipher';   -- confirm TLS active
   ```
2. Record values in this file under "Findings" (subsection "Day 0 checks").

### Idle observation (Day 0 → Day 14)

3. **Do not connect to the DB during this window.** No keepalive, no health check loops, no scripted SELECTs. (Phase C scaffolding can be developed locally / against a separate `mysql:8` Docker for dev, OR you accept that running Phase C against this instance disqualifies the idle test — see "Parallelism caveat" below.)
4. Calendar reminder for Day 14.

### Day 14 check

5. Open OCI console, find DB system.
6. Record state:
   - **ACTIVE / RUNNING** → no idle-reclaim observed; document in Findings; production cutover unblocked
   - **STOPPED / IDLE** → reclaim exists; document trigger threshold; add keepalive cron in Phase E (5-min `SELECT 1`); note recovery procedure (manual start via console or `oci mysql db-system start`)
7. Reconnect via `mysql -h <ip>` to confirm DB is healthy after the dormancy.

### Restore drill (Day 14 + 1 hour)

8. Take a final backup manually:
   ```
   oci mysql backup create --db-system-id <ocid> --display-name drill-260519
   ```
9. Wait for backup to reach ACTIVE state.
10. Provision a new DB system from this backup (also `MySQL.Free`-shaped). Time the operation.
11. Connect to the new DB, verify `dleague` schema + `dleague_app` user are restored.
12. Delete the drill DB system to free Always-Free quota.
13. Document restore time in Findings.

## Parallelism caveat

Phases C/D/E/F can develop in **parallel** with Phase B if you use a **local Docker `mysql:8`** for dev (zero impact on the Always-Free instance). Migrations and Go store layer can be validated against the local container; the live HeatWave is touched only for production smoke tests.

If you instead run all dev against the live DB, the 14-day idle observation is invalidated and you must either:
- Restart the 14-day clock, or
- Accept that idle-reclaim status is "unverified by us" and assume worst case (add the keepalive cron unconditionally)

## Todo List

- [ ] Day 0: capture `max_connections`, `wait_timeout`, `interactive_timeout`, MySQL version, TLS status
- [ ] Day 0–14: do not connect to live DB (or accept caveat above)
- [ ] Day 14: check state in OCI console
- [ ] Day 14: record idle-reclaim verdict in Findings
- [ ] Restore drill: take backup → restore to new DB system → measure time → delete drill instance
- [ ] If reclaim observed: add Phase E todo "deploy 5-min keepalive cron"

## Success Criteria

- [ ] `max_connections` value recorded
- [ ] Idle-reclaim verdict (yes / no / threshold) recorded in Findings section below
- [ ] Restore drill completed, time measured (minutes-to-hours), schema integrity verified
- [ ] If keepalive needed, ticket created or todo added in Phase E

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Idle observation invalidated by accidental connection | Med | Use local `mysql:8` for dev work or restart clock |
| Restore drill consumes Always-Free quota (multi-DB-system-during-drill) | Low | Free-tier limit is 1 DB system per tenancy; OCI may allow temporary 2-instance overlap during restore — verify before drilling |
| `max_connections` is significantly lower than expected (e.g. <50) | Low | Adjust `db.SetMaxOpenConns` in Phase C accordingly |
| 14-day window pushes production cutover past dev expectations | Low | Communicate up front; dev work proceeds locally in parallel |

## Security Considerations

- Manual restore creates a new DB system — verify NSG attachment is correct on the new instance (rules don't auto-copy)
- Backup includes user passwords (hashed); the drill instance contains real credential material — delete it immediately after the drill

## Next Steps

After Day 14 results recorded:
- If `max_connections >= 50` and no idle-reclaim → Phase C pool sizing stays at 25; production cutover unblocked
- If reclaim → add keepalive cron in Phase E; production cutover unblocked
- If `max_connections < 30` → revisit Phase C `SetMaxOpenConns` and document upper bound

## Findings

_(populate after Day 14)_

### Day 0 checks
- `max_connections`: __TBD__
- `wait_timeout`: __TBD__
- `interactive_timeout`: __TBD__
- MySQL version: __TBD__
- TLS active: __TBD__

### Day 14 idle verdict
- State: __ACTIVE | STOPPED__
- Action: __none | add keepalive cron__

### Restore drill
- Time to restore: __TBD minutes__
- Schema integrity: __OK | issues observed__
