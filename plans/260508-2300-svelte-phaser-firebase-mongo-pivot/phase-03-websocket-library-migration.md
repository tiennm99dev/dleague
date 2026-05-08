---
phase: 3
title: "Migrate nhooyr.io/websocket → github.com/coder/websocket"
status: pending
priority: P2
effort: 0.5w
dependencies: [2]
---

# Phase 03 — WebSocket library migration (nhooyr → coder)

## Context Links
- `plans/reports/security-review-260508-2300-phase1-foundation.md` (L1)
- `server/internal/ws/conn.go` (imports `nhooyr.io/websocket`)
- `server/go.mod` (`nhooyr.io/websocket v1.8.17`)

## Overview
Pure-mechanical migration: import path swap, `go mod tidy`. No behavior change. The library is API-compatible (same maintainer); the original `nhooyr.io/websocket` repo is archived.

## Key Insights
- `github.com/coder/websocket` is a maintained fork by the same author. API-identical for the calls Dleague uses (`Accept`, `Read`, `Write`, `Ping`, `CloseNow`).
- No known CVE pre-migration; this is preventive maintenance.
- Phase 02 just restructured `Conn`; doing the swap directly after avoids re-wrangling the same files.

## Requirements
**Functional:**
- All `nhooyr.io/websocket` imports replaced with `github.com/coder/websocket`.
- `go.mod` no longer references `nhooyr.io`.
- All existing tests pass with `-race`.
- WS endpoint behavior unchanged (manual smoke).

**Non-functional:**
- Diff is import-path only + `go.sum` updates; zero functional code change.
- CI green on first push.

## Architecture
No diagram needed — symbol-for-symbol substitution.

## Related Code Files
**Modify:**
- `server/internal/ws/conn.go` — `import "nhooyr.io/websocket"` → `import "github.com/coder/websocket"`
- `server/internal/ws/conn_test.go` — same
- `server/internal/ws/hub.go` — same (if it references the package)
- `server/internal/ws/hub_test.go` — same
- `server/go.mod` — drop `nhooyr.io/websocket`, add `github.com/coder/websocket`
- `server/go.sum` — `go mod tidy` regenerates

**Create:** none.
**Delete:** none.

## Implementation Steps
1. `grep -rn "nhooyr.io/websocket" server/` → enumerate every site.
2. Sed/IDE replace import path in each file (likely 4-6 lines total).
3. `cd server && go get github.com/coder/websocket@latest && go mod tidy`.
4. `go build ./...` — must pass with no symbol changes required.
5. `go test -race ./...` — must pass.
6. Smoke: `make dev` → connect via wscat → ping/pong round-trips.
7. Verify `go.mod` no longer lists `nhooyr.io/websocket` (only as indirect if at all; expect fully gone).
8. Commit: `chore: migrate nhooyr.io/websocket to github.com/coder/websocket (archived → maintained fork)`.

## Todo List
- [ ] Enumerate import sites
- [ ] Replace import paths
- [ ] `go mod tidy` + verify
- [ ] `go build ./...`
- [ ] `go test -race ./...`
- [ ] Manual ping/pong smoke
- [ ] Commit

## Success Criteria
- [ ] `grep -r "nhooyr.io" server/` returns no hits
- [ ] CI green
- [ ] `make dev` ping/pong works as before

## Risk Assessment
| Risk                                                   | Likelihood | Impact | Mitigation                                                       |
|--------------------------------------------------------|------------|--------|------------------------------------------------------------------|
| Hidden API drift between forks                         | Low        | Medium | Forks at v1.8.x point share full API; CI catches divergence.     |
| Indirect dep still pulls nhooyr (transitive)           | Low        | Low    | `go mod why nhooyr.io/websocket` after migration to confirm gone. |
| Race condition surfaces only under coder fork          | Low        | Medium | `-race` test catches; fix in coder/websocket-aware code.         |

## Security Considerations
- Closes security-review L1 (archived dependency).
- No new attack surface.

## Next Steps
- Phase 04 — MongoDB store rewrite — depends on this phase only via clean compile baseline.
