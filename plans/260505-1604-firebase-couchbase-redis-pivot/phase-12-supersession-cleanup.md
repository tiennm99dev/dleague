---
phase: 12
title: "Supersession + cleanup"
status: in_progress
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
7. ~~**Verify Couchbase Community Edition license terms** for the beta usage profile~~ — done 2026-05-06. Final verdict logged in `docs/migration-readiness.md` § License watchout; primary-source verification at `plans/reports/researcher-260506-1029-couchbase-ce-license-verbatim.md`. CE License Agreement permits commercial use; hard caps are 5 nodes / 4 cores per node / no XDCR. (Earlier report `researcher-260506-0939-couchbase-ce-license-fit.md` is superseded — it conflated BSL source license with CE binary license.)
8. Final commit: `chore: docs/cleanup post self-hosted couchbase+redis beta pivot`.

## Todo List

- [x] **Migration-export CLI** — `server/cmd/dleague-export/main.go` opens Couchbase from same env vars as the server, streams JSONL of all 4 collections to stdout. Redis state intentionally NOT exported (derivable from attempts).
- [x] PDR updated (`docs/project-overview-pdr.md` created)
- [x] System architecture diagram + doc (`docs/system-architecture.md` created with ASCII stack diagram)
- [ ] Deployment guide finalized — Phase 11 deferred; existing `docs/deployment-guide.md` covers compose + Coolify scaffold, but the live-VM walkthrough lands with Phase 11
- [x] Codebase summary updated (`docs/codebase-summary.md` created)
- [x] Roadmap updated (`docs/project-roadmap.md` created)
- [x] README tech stack section refreshed
- [x] Superseded plans cross-linked — frontmatter `superseded_by:` + top-of-doc "**Superseded by:**" banner added to both archived `plan.md` files
- [x] Legacy `client/` decision made — **delete** confirmed by user. Removed `client/cmd/`, `client/internal/`, `client/go.mod`, `client/go.sum` via `git rm`; `client/` now contains only `client/web/` (active). Docs (README, codebase-summary, project-roadmap) updated.
- [x] **GitHub CI rewritten** for Go 1.25.5 + Svelte/Phaser client (dropped WASM build + MySQL refs); separated server + client jobs. **File kept as `ci.yml.disabled`** per user — they will rename to `ci.yml` and verify on first PR after deploy.
- [x] Journal entry via `/ck:journal` — `plans/journals/260506-0933-phase-12-supersession-cleanup.md`
- [x] Migration-readiness doc (`docs/migration-readiness.md`)
- [x] **Couchbase CE license review** — 2026-05-06; final verdict logged in `docs/migration-readiness.md` § License watchout (primary-source verification at `plans/reports/researcher-260506-1029-couchbase-ce-license-verbatim.md`). CE License Agreement explicitly permits commercial use of Community Edition binaries; hard caps are 5 nodes / 4 cores per node / no XDCR. Earlier "non-commercial-only" framing in `researcher-260506-0939-couchbase-ce-license-fit.md` was wrong — that report conflated the BSL source license with the CE binary license and is **superseded**.

## Success Criteria

- [x] Fresh contributor can read `docs/` and bring up local dev in <30 min — verified 2026-05-07: README quickstart is 4 commands (`make tools`, `make proto-gen`, `docker compose up -d`, `cd client/web && npm i && npm run dev`); `docs/deployment-guide.md` covers the Firebase one-time setup; `docs/codebase-summary.md` tours the directories.
- [x] No references to MySQL HeatWave or Firestore in production code or live docs (only in archived plans) — audited 2026-05-07: residual hits in `docs/codebase-summary.md`, `docs/project-roadmap.md` are "removed / Strip MySQL phase" historical framing; only code hit is `cloud.google.com/go/firestore` as **indirect** transitive dep of Firebase Admin SDK (unavoidable, no direct import).
- [x] Roadmap reflects current state honestly — refreshed 2026-05-07.

## Risk Assessment

- **Dangling references** — search code+docs for `mysql`, `heatwave`, `firestore`. Any hits are bugs.
- **User-facing breaking change** — if any test domain users existed, notify (none expected for testing project).

## Security Considerations

- Final secrets audit: nothing committed to repo; all in Coolify env.
- `.env.example` contains placeholders only.

## Next Steps

Project enters maintenance / feature-add mode. Future phases land as new plans.

## Unresolved Questions

- ~~Does the user want to keep `client/` Ebitengine WASM as a historical artifact or delete?~~ — resolved 2026-05-06: delete. Recoverable via git history.
- Any docs (e.g. PDR) need user review before merge?
- Deployment guide finalization remains blocked on Phase 11 (Coolify live-VM walkthrough). Phase 12 carries the placeholder until then. **All non-deploy Phase 12 work shipped 2026-05-07; phase stays `in_progress` purely on the deploy-guide tail.**
