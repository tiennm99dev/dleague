# Engine Fitness for an "All-in-One -dle Games" Platform (Web + Mobile)

**Research Date:** 2026-05-05 17:28 (Asia/Saigon)
**Question:** Best engine for a multi-game `-dle` aggregator (Wordle, Quordle, Sumdle, etc.) running on web + mobile, written in **Go**, **JS**, or **Java/Kotlin**.
**Constraint context:** Active plan is React + Capacitor; user reconsidering after Ebitengine WASM was ruled out.

---

## TL;DR — **Use JS/TS. Stay on React + Capacitor + optional Phaser 4 scenes per variant. (= the current plan.)**

The honest finding: **Go and Java/Kotlin don't have competitive answers for "many small HTML5 puzzle games + Capacitor mobile."** This is a JS/TS workload.

Within JS/TS, the right architecture for an `-dle` aggregator is **a thin shell + per-game renderer interface**, not "one engine for everything":
- **Shell** (auth, lobby, profile, leaderboards, beta banner) → **React + DOM** (current plan).
- **DOM-shape `-dle` variants** (Wordle, Quordle, Octordle, NumberDle text-grid) → **plain React components** (~50 KB per variant). Most word/grid puzzles fit.
- **Canvas-shape variants** (animated tiles, particle wins, physics) → **Phaser 4 scene** plugged into the same `GameEngine<TState,TAction>` interface.
- **Mobile** → Capacitor WebView (web-first, ship to iOS/Android from same code).

Ranked verdict per candidate:

| Rank | Engine | Lang | Verdict |
|------|--------|------|---------|
| 🥇 | **React + DOM + Phaser 4 (per-variant)** | TS | Best fit for `-dle` aggregator. **= current plan.** |
| 🥈 | Cocos Creator | TS | Viable; native multi-platform export bypasses Capacitor. Heavier toolchain. |
| 🥉 | Phaser 4 alone (no DOM) | TS | Works but loses DOM a11y + bigger bundle for text puzzles |
| ❌ | libGDX | Java | Dated GWT + iOS path; action-game-focused; wrong ecosystem |
| ❌ | Compose Multiplatform | Kotlin | Compose-for-Web still **Beta** (Sept 2025); too immature for production |
| ❌ | Ebitengine WASM | Go | Bundle 5–15 MB ruled out in prior research |
| ❌ | Other Go options | Go | No mature alternative for cross-platform WASM + mobile |

---

## 1. Why JS/TS wins this category

For "many small puzzle games on web + mobile," the candidates split cleanly by language:

### Go
- **Ebitengine** is the only mature option. Already disqualified — WASM bundle 5–15 MB.
- No other Go engine has both production-grade WASM and mobile (gomobile only goes so far).

### Java / Kotlin
- **libGDX** — most-cited Java cross-platform engine. Compiles Java→JS via GWT for web. But:
  - GWT bundle size historically 2–5 MB compiled JS — heavier than Phaser, lighter than Ebitengine, but still hostile to a daily-puzzle UX.
  - GWT is *legacy tech* — Google moved focus to other paths years ago. Maintenance risk.
  - iOS path was RoboVM (now defunct as a free option) or Multi-OS Engine (low momentum). Mobile is pain.
  - Ecosystem is action-game heavy. Almost no `-dle`-style examples.
- **Korge** (Kotlin Multiplatform 2D engine) — exists but small community, fewer puzzle examples.
- **Compose Multiplatform** (Kotlin) — modern UI framework, NOT a game engine. Compose for Web on Kotlin/Wasm reached **Beta in Sept 2025** (v1.9.0). Promising future:
  - 100% code share across Android/iOS/web/desktop
  - Kotlin/Wasm "nearly 3× faster than JS in UI scenarios"
  - **But Beta means stability risk**, ecosystem is shallow, bundle weight of Compose runtime unmeasured for puzzle workloads. Wait 12+ months for GA.

### JavaScript / TypeScript
- Native to the web. Smallest bundles. Largest game-framework + UI ecosystem. Mobile via Capacitor or Cocos native export.
- Every Wordle clone in the wild (Listdle, Quordle, Octordle, Sedecordle) is JS/CSS — confirmed by survey of the aggregator category. None ship a game engine.
- **Conclusion:** for the workload shape, JS/TS is the natural choice.

---

## 2. Within JS/TS — the architectural choice

### Phaser 4 (April 2026 GA, v4.1.0 "Salusa" April 2026)

**Strengths**
- Scenes are first-class — natural primitive for "one game per scene" aggregator.
- Sokoban tutorial published Jan 2026 — official puzzle-game starter.
- ESM-clean, AI-agent skill files bundled, plugin-friendly.
- Capacitor wrapping is documented and proven for mobile.

**Weaknesses for `-dle` shape**
- **Canvas-only by default.** Word grids in canvas lose DOM accessibility (screen readers, native keyboards, ARIA). Text-shape puzzles are CSS's home turf.
- ~600–900 KB bundle vs ~200–500 KB DOM bundle.
- Overkill for a Wordle keyboard + 5-letter row.

**When it's right:** if a `-dle` variant genuinely needs canvas (e.g. animated NumberDle with falling tiles, physics, particle win effects).

### Cocos Creator (TypeScript)

**Strengths**
- **Native export to Web + iOS + Android + Windows + Mac + HarmonyOS + WeChat/TikTok mini-games** from one project. Capacitor unnecessary.
- Built-in animation, physics, particle, complex UI — designed for instant-gaming distribution.
- TypeScript-scriptable; visual editor accelerates non-code workflows.
- Mature; used by big mobile-game houses (especially APAC).
- Strong 2D performance.

**Weaknesses for `-dle` shape**
- **Editor-driven workflow** — heavier than a Vite + React project. Bigger learning curve for code-first devs.
- Bigger runtime bundle than Phaser.
- TypeScript scripting in their editor environment, not standard Vite/React tooling — some friction with tree of mature React libraries.
- Ecosystem skew: action / casual mobile games, not text puzzles.

**When it's right:** if you want **native mobile export without Capacitor** and the team is OK adopting an editor-centric tool.

### React + DOM + Capacitor (current plan)

**Strengths**
- Smallest bundle (~200–500 KB) — best for daily-puzzle retention.
- Native CSS Grid for tile layouts; native `<button>` for keys; native ARIA for screen readers.
- React component is the natural plug-in primitive for new game variants.
- Vast ecosystem (Tailwind, framer-motion, shadcn/ui, etc.).
- Capacitor mobile path is well-trodden.
- **Mixes with Phaser cleanly:** mount a Phaser canvas inside a React component for the few variants that need it.

**Weaknesses**
- Canvas effects (particle wins, physics) need a separate renderer — addressed by the `GameEngine<TState,TAction>` interface in Phase 8.

**When it's right:** when most variants are HTML-shape (text grids, keyboards) — which is what `-dle` games actually are.

---

## 3. Architecture for "all-in-one -dle aggregator"

```
┌─────────────────────────────────────────┐
│  Shell (React + Capacitor)              │
│  ├── Auth / Profile / Lobby             │
│  ├── Leaderboards / Friends             │
│  ├── Beta banner / Settings             │
│  └── <GameRunner engine={...}/>         │
└─────────────────────────────────────────┘
                  │
                  ├─ <WordleEngine>      (DOM, ~30 KB)
                  ├─ <QuordleEngine>     (DOM, reuses Wordle code)
                  ├─ <SumdleEngine>      (DOM, ~30 KB)
                  ├─ <NumbleEngine>      (DOM)
                  └─ <CanvasEngine>      (Phaser 4 scene, ~600 KB only loaded when used)
```

**`GameEngine<TState,TAction>` interface (Phase 8 of active plan):**
```typescript
interface GameEngine<TState, TAction> {
  init(puzzle: Puzzle, prevAttempt?: Attempt): TState;
  step(state: TState, action: TAction): { state: TState; emit?: ServerEvent };
  isComplete(state: TState): boolean;
}
```

This is the migration seam. New variants ship as a folder under `client/web/src/games/`. Each variant exports an `engine.ts` (logic) + UI components (DOM-or-canvas). Code-split via Vite dynamic import — only the played variant downloads.

---

## 4. Why this answer is stable

- **Real-world `-dle` aggregators (Listdle, Wordle Hub, Floodgates list)** ship as DOM/CSS HTML5 sites, not game engines. The workload doesn't justify an engine for the majority of variants.
- **Wordle itself** is plain HTML/CSS/JS. NYT didn't reach for Phaser.
- **The `-dle` market signal** ("geography games win on engagement, not word games") suggests future variants may diverge from text-grid — at which point the per-variant Phaser 4 scene path activates without a rewrite.
- **Java/Kotlin paths** (libGDX, Compose-Web) are objectively immature for this workload in 2026.
- **Go path** is closed (Ebitengine bundle).

---

## 5. Plan implications

**No change required to the active plan.** Current Phase 7 + 8 already specify:
- React + Capacitor (Phase 7)
- `GameEngine<TState,TAction>` interface + Wordle-as-DOM as first variant (Phase 8)
- Phaser 4 spike already listed in Phase 8

If you accept this recommendation, suggested tiny edits:
1. **Phase 8 description** — explicitly note the architecture is "DOM-first, Phaser 4 per-variant when needed" (it implies it already; make it explicit).
2. **Phase 12 cleanup** — confirmed delete of `./client/` Ebitengine WASM dir (legacy, dead per prior research).

If you want to reverse course to a single-engine approach (against my recommendation):
- **Phaser 4 throughout** (drop React, all UI in Phaser scenes) → +5 days effort, lose DOM a11y, bundle +400 KB
- **Cocos Creator** (drop React + Capacitor) → +1–2 weeks effort to learn editor tooling, gain native mobile export
- **Either path** would mean rewriting Phase 7 + 8 of the active plan

---

## 6. Decision matrix (quick reference)

| If your priority is… | Pick |
|---|---|
| Smallest bundle, fastest load, best a11y | **React + DOM** (current) |
| Future canvas variants without rewriting shell | React + DOM + Phaser 4 per-variant (current) |
| Single-engine "everything in canvas" web+mobile | Phaser 4 + Capacitor |
| Native mobile export without Capacitor | Cocos Creator |
| Single Java codebase across all platforms | libGDX (with caveats) |
| Bleeding-edge Kotlin Wasm UI | Compose Multiplatform (wait for GA) |
| One Go codebase for everything | None — workload mismatch |

---

## 7. Unresolved questions

1. **Future `-dle` variant graphics needs** — do envisioned games need canvas (animated tiles, physics, particles)? If yes for ≥3 variants, consider promoting Phaser 4 to default. If no/few, DOM-first stays optimal.
2. **Mobile install size budget** — what's the acceptable APK/IPA size? Capacitor adds ~5–10 MB to the WebView shell baseline.
3. **WeChat / TikTok mini-games** — if APAC mini-game distribution is a goal, Cocos Creator becomes more compelling (it's the dominant tool for those channels).
4. **Team familiarity bias** — strongest factor. If the team is faster in Phaser than React, swap. If unsure, default to the JS/TS+React mainstream.

---

## Sources

- [Phaser 4.1 "Salusa" release (April 2026)](https://phaser.io/news/2026/04/phaser-4-1-0-salusa-release)
- [Build a Sokoban Puzzle Game in Phaser (Jan 2026)](https://phaser.io/news/2026/01/build-a-sokoban-puzzle-game-in-phaser)
- [Phaser Scenes documentation](https://docs.phaser.io/phaser/concepts/scenes)
- [Phaser Plugins directory](https://phaserplugins.com/)
- [Cocos Creator official site](https://www.cocos.com/en/creator)
- [Cocos Engine GitHub](https://github.com/cocos/cocos-engine)
- [libGDX official site](https://libgdx.com/)
- [libGDX HTML5/GWT specifics](https://github.com/libgdx/libgdx/wiki/HTML5-Backend-and-GWT-Specifics)
- [libGDX Wikipedia](https://en.wikipedia.org/wiki/LibGDX)
- [Kotlin Multiplatform overview](https://kotlinlang.org/multiplatform/)
- [Kotlin/Wasm + Compose Multiplatform getting started](https://kotlinlang.org/docs/wasm-get-started.html)
- [Compose Multiplatform 1.9.0 — Compose for Web Beta (Sept 2025)](https://blog.jetbrains.com/kotlin/2025/09/compose-multiplatform-1-9-0-compose-for-web-beta/)
- [Listdle — daily-game aggregator](https://listdle.com/)
- [Comprehensive list of Wordle variants (Maxspero gist)](https://gist.github.com/maxspero/0a2f536b9561d829caf6bd994a34193d)
- [State of Wordle Alternatives 2025](https://wordlealternative.com/state-of-wordle-alternatives-2025)
- [Quordle acquired by Merriam-Webster (TechCrunch)](https://techcrunch.com/2023/01/20/wordle-clone-quordle-acquired-by-merriam-webster/)
