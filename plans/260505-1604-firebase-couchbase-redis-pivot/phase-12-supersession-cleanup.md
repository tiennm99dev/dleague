---
phase: 12
title: "Supersession + cleanup"
status: pending
priority: P3
effort: "0.5d"
dependencies: [11]
---

# Phase 12: Supersession + cleanup

## Context Links

- Plan: [plan.md](plan.md)
- Plans being closed: `260505-1407-firebase-platform-pivot/`, `260505-1319-mysql-heatwave-integration/`

## Overview

Final docs sweep + ship the **migration-export CLI** (`cmd/dleague-export`) that wraps `store.Export()` from Phase 4 — the migration-readiness deliverable. Mark prior plans archived. Remove dead code (legacy Ebitengine WASM client if user agrees). Update `docs/`.

## Key Insights

- The two superseded plans had useful research that we should preserve as historical context (DO NOT delete the report files).
- The legacy `client/` Ebitengine WASM directory is now unused if `client/web/` is the active frontend. Decision deferred to user — for now, retain with deprecated note.
- `docs/` should reflect: stack overview, deploy guide, env-var schema, architecture diagram.

## Requirements

- Functional: docs reflect implemented stack; superseded plans archived; journal entry written.
- Non-functional: any reader can rebuild the dev environment from docs alone.

## Architecture

`docs/` final shape:
```
docs/
├── project-overview-pdr.md         # what dleague is; the elevator pitch
├── system-architecture.md          # stack diagram + sequence flows
├── deployment-guide.md             # Phase 11 prod setup + Phase 1 free-tier provisioning
├── code-standards.md               # Go + TS conventions
├── codebase-summary.md             # tour of dirs
├── design-guidelines.md            # UI guidelines (web)
└── project-roadmap.md              # what's next
```

## Related Code Files

- Create:
  - `server/cmd/dleague-export/main.go` — CLI wrapping `store.Export()`; outputs JSONL of all persistent keys
  - `docs/migration-readiness.md` — migration playbook: store-interface boundary, key conventions, export tool usage, target-backend swap recipe
- Modify:
  - `docs/project-overview-pdr.md` — update stack (Firebase Auth + self-hosted Couchbase 8.0 + Redis 8.4) + beta posture + Couchbase CE license review
  - `docs/system-architecture.md` — new diagram (single store, store-interface seam highlighted)
  - `docs/deployment-guide.md` — produced in Phase 11; finalize
  - `docs/codebase-summary.md` — describe `internal/store/` (interface + couchbase impl + redis impl + composed glue + memstore impl), `internal/auth`, `client/web/` (Svelte 5 shell + Phaser 4 game scenes)
  - `docs/project-roadmap.md` — update progress + add "post-beta tech-stack reassessment" milestone with explicit migration-target evaluation criteria
  - `README.md` — update tech stack section + beta notice + migration-readiness statement
- Optionally delete:
  - `client/` (Ebitengine WASM legacy) — only with user confirmation
- Frontmatter already updated (during planning):
  - `plans/260505-1407-firebase-platform-pivot/plan.md` — `status: superseded`
  - `plans/260505-1319-mysql-heatwave-integration/plan.md` — `status: superseded`

## Implementation Steps

1. Update each doc file listed above to reflect final stack.
2. Generate architecture diagram (mermaid, embed in `system-architecture.md`).
3. Cross-link plans: each superseded `plan.md` adds top-of-file note: "**Superseded by**: `260505-1604-firebase-couchbase-redis-pivot/`".
4. Decision on `client/` legacy: present to user; if delete, `git rm -r client/` (keep `client/web/`).
5. Run `/ck:journal` to write the technical journal entry covering the pivot.
6. Update `docs/project-roadmap.md` with completion date and next-up items.
7. **Verify Couchbase Community Edition license terms** for the beta usage profile (incl. early-adopter rewards). Document outcome in `docs/migration-readiness.md`.
8. Final commit: `chore: docs/cleanup post self-hosted couchbase+redis beta pivot`.

## Todo List

- [ ] PDR updated
- [ ] System architecture diagram + doc
- [ ] Deployment guide finalized
- [ ] Codebase summary updated
- [ ] Roadmap updated
- [ ] README tech stack section refreshed
- [ ] Superseded plans cross-linked
- [ ] Legacy `client/` decision made (delete vs retain)
- [ ] Journal entry via `/ck:journal`

## Success Criteria

- [ ] Fresh contributor can read `docs/` and bring up local dev in <30 min
- [ ] No references to MySQL HeatWave or Firestore in production code or live docs (only in archived plans)
- [ ] Roadmap reflects current state honestly

## Risk Assessment

- **Dangling references** — search code+docs for `mysql`, `heatwave`, `firestore`. Any hits are bugs.
- **User-facing breaking change** — if any test domain users existed, notify (none expected for testing project).

## Security Considerations

- Final secrets audit: nothing committed to repo; all in Coolify env.
- `.env.example` contains placeholders only.

## Next Steps

Project enters maintenance / feature-add mode. Future phases land as new plans.

## Unresolved Questions

- Does the user want to keep `client/` Ebitengine WASM as a historical artifact or delete?
- Any docs (e.g. PDR) need user review before merge?
