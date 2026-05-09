# Archived plans

This directory holds superseded plans. They are kept for historical reference only — do not resurrect without an explicit decision recorded in a new plan.

Active plan: [`../260509-1331-improvement-plan/plan.md`](../260509-1331-improvement-plan/plan.md) (post-MVP hardening; Phase 06 still pending).

| Plan | Status | Why archived |
|------|--------|---------------|
| [`260508-2300-svelte-phaser-firebase-mongo-pivot/`](260508-2300-svelte-phaser-firebase-mongo-pivot/) | completed (2026-05-09) | All 10 phases shipped (commits `5f837ea`..`8bc05d9`). Stack pivot Ebitengine→Svelte+Phaser + Firebase Auth + Mongo Atlas + sync/async PvP. Pre-Phase-04 reports + Phase-01 shipped journal moved into this plan's `reports/` and `journals/`. |
| [`260505-0947-dleague-pvp-game/`](260505-0947-dleague-pvp-game/) | cancelled (2026-05-08) | Phase 1 shipped (commit `9937c7d`) and is retained as history. Phases 2-6 superseded by stack pivot: Ebitengine → Svelte+Phaser, MySQL → MongoDB Atlas, session-cookie auth → Firebase Auth. |
| [`260505-1319-mysql-heatwave-integration/`](260505-1319-mysql-heatwave-integration/) | cancelled (2026-05-08) | DB choice changed from MySQL HeatWave to MongoDB Atlas M0. Provisioning + schema work no longer applies. |
| [`260505-1407-firebase-platform-pivot/`](260505-1407-firebase-platform-pivot/) | cancelled (2026-05-08) | Stack scope narrowed: Firebase scope reduced to Auth only (no Firestore, no RTDB). Client framework changed from React/Capacitor to Svelte+Phaser. |

Internal links inside archived `plan.md` / `phase-*.md` files may point to other archived files or to live phase files that have been replaced. Treat them as frozen — do not edit.
