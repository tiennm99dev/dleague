# Ebitengine + WASM vs Phaser for dleague (web-first → mobile)

**Research Date:** 2026-05-05 16:48 (Asia/Saigon)
**Question:** Is Ebitengine + WASM suitable for dleague (web-first, mobile later)? If not, recommend Phaser path.

---

## TL;DR — **NO to Ebitengine WASM. YES to Phaser** (if you want a game-engine path at all).

But honest secondary question: **does dleague even need a game engine?** A Wordle-style game is grid + keyboard + simple animations — DOM/HTML/CSS + React (the current plan) ships smaller, faster, and simpler than either game engine for this workload shape. **My strongest recommendation: stay on the current React + Capacitor plan unless you have concrete game-feel needs Phaser uniquely solves.**

If you genuinely want a game-engine path: **Phaser 4 + Capacitor**, fits dleague's profile cleanly. Phaser 4 just shipped (April 2026) with a new rendering pipeline.

If you want one Go codebase across web + mobile: Ebitengine *technically* works but the **WASM bundle is 5–15+ MB** (Go runtime baggage). For a daily-puzzle game where load time matters, that's disqualifying.

---

## 1. Ebitengine + WASM — the brutal facts

### What works
- Mature 2D Go engine; well-maintained (active updates Oct 2025).
- Genuine cross-platform: Windows, macOS, Linux, FreeBSD, **iOS/Android via gomobile**, Web via WASM.
- Single Go codebase shared with the existing dleague server (DRY win on shared types).
- Production track record: Bear's Restaurant (1M+ downloads on mobile via gomobile).

### What kills it for dleague
1. **WASM bundle size: 5–15+ MB compressed.** Go's WASM runtime is heavy (Go's GC, scheduler, runtime are baked into the binary). Generic WASM minimum is ~50 KB; Go starts at ~2 MB and balloons fast. **WasmGC** (which slashes other-language WASM by 60–80%) **does not apply to Go** — Go ships its own GC.
   - For comparison: a React+DOM Wordle is ~200–500 KB total. A Phaser 4 Wordle is ~600–900 KB.
   - On 3G mobile this is the difference between 1s and 8–15s first-load.
2. **WASM rendering performance gap.** WASM is 10–800× faster than JS *for compute-heavy work*, but games are draw-heavy. Ebitengine WASM hits the canvas the same way Phaser does — the engine doesn't get the WASM speed-up where it matters.
3. **Mobile path via gomobile is "real integration work", not "wrap webview".** Ebitengine team explicitly recommends `gomobile bind` over `gomobile build` for store distribution — meaning you write iOS/Android shells in Swift/Kotlin around the bound library. **You don't get "free" mobile** the way you do with Capacitor.
4. **Tiny ecosystem for Wordle-style UI.** Ebitengine targets action games. There's no "Wordle keyboard" library, no battle-tested form / modal / animated-tile components. You build it all from primitives.
5. **DOM accessibility loses.** Ebitengine renders to a single canvas — screen readers, keyboard navigation, native input methods (especially mobile keyboards), CSS accessibility tools all degrade. dleague is text-heavy; this is a real penalty.

### Verdict
**Not suitable.** dleague's profile (puzzle game, daily + retention sensitive, web-first then mobile via webview) is the worst-case for Ebitengine WASM bundle weight, and the best-case for a DOM/HTML approach.

---

## 2. Phaser 4 (April 2026) — the honest assessment

### What's new in Phaser 4 (v4 GA April 2026)
- **New rendering pipeline:** node-based, modern WebGL state management, significantly faster than Phaser 3.
- **SpriteGPULayer:** 1M sprites in one draw call (irrelevant for Wordle but proves headroom).
- **Unified Filter system** (FX + Masks merged) — Blur, Glow, Shadow, Pixelate, ColorMatrix, Bloom, etc. baked in.
- **Improved lighting** (`sprite.setLighting(true)`, self-shadows).
- **WebGL spec compliance** — predictable behavior, custom shaders work cleanly.
- **AI agent skills bundled** (28 skill files) — including a Phaser 3→4 migration skill.
- API mostly compatible with Phaser 3 (existing tutorials translate).

### Phaser + Capacitor for mobile
- **Officially supported, documented, production-proven.** Capacitor wraps Phaser in iOS/Android WebView with a native bridge for push, IAP, camera, etc.
- WebView performance: **excellent for typical 2D games** on iOS; **Android can drop frames on heavy animation** (WebView limitation, not Phaser/Capacitor).
- Wordle-style is well below any mobile WebView performance ceiling.

### dleague-specific fit
- Bundle ~600–900 KB minified+gzipped (Phaser core ~500 KB; Wordle game logic ~50–100 KB; ui/glue ~100 KB).
- Acceptable cold-start.
- Mobile via Capacitor = same codebase, well-trodden path. Same as the current React+Capacitor plan, just swap the renderer.
- **One real downside:** Phaser 4 is fresh — third-party plugins from the Phaser 3 ecosystem may lag. For a Wordle workload, plugins are barely used; low risk.
- **Hybrid pattern is common and clean:** Phaser canvas for the game grid + DOM for menus/forms/auth. Tutorials demonstrate Phaser+Bootstrap layouts.

### Verdict
**Yes, viable.** But it's the right answer only if you genuinely want game-engine features (animated tile flips, particle wins, multiple `-dle` variants where canvas pays off). Otherwise it's overkill vs DOM.

---

## 3. The third option you didn't ask about — **stay on the React + Capacitor plan**

Your current Phase 7+8 plan is React + TS + Capacitor with a `GameEngine<TState,TAction>` interface. This is the workload-optimal pick for Wordle-style:

| Criterion | DOM + React (current plan) | Phaser 4 + Capacitor | Ebitengine WASM |
|-----------|----------------------------|----------------------|-----------------|
| Bundle (web, gzipped) | **~200–500 KB** | ~600–900 KB | **5–15+ MB** ❌ |
| Cold start (mobile 3G) | ~1s | ~2–3s | ~8–15s ❌ |
| Wordle UI fit | **native (CSS grid + buttons)** | OK (canvas tiles + DOM keyboard hybrid) | bad (everything is canvas) |
| Mobile path | Capacitor WebView (battle-tested) | Capacitor WebView (battle-tested) | gomobile bind (real native shell work) |
| Accessibility | **first-class** (DOM + ARIA) | partial (canvas needs custom a11y) | poor |
| Animation richness ceiling | medium (CSS+framer-motion is plenty for tile flips) | **high** (filters, particles, lighting) | high (but not used at this scale) |
| Ecosystem (Wordle-like UI parts) | **massive** (every UI lib) | medium (Phaser-specific) | tiny |
| Code shared with Go server | none | none | **shared types via Go modules** |
| Dev velocity for puzzle UI | **fastest** | medium | slowest |

Strong answer: **DOM + React for Wordle and any text-grid `-dle` game.** Phaser shines if you add a `-dle` variant with rich graphics (e.g. a tile-physics or particle game). Ebitengine shines for action / arcade — not your shape.

---

## 4. Decision tree

```
Does dleague need rich canvas animation
beyond CSS-feasible (e.g. particle systems,
filters, real-time graphics)?
│
├── No  → STAY on React + Capacitor (current plan).
│         No change needed. Ship faster.
│
├── Yes → Will some -dle variants need it
│         and others not?
│         ├── Yes → Phaser 4 for canvas variants
│         │         + DOM/React for the rest
│         │         (hybrid in same Capacitor app)
│         └── No, ALL games need it
│              → Phaser 4 + Capacitor end-to-end
│
└── "I want Go everywhere" → Ebitengine native (mobile)
                              + accept WASM bundle pain
                              for web (or skip web).
                              Don't ship Ebitengine WASM
                              for a daily-puzzle game.
```

---

## 5. Recommendation

**For dleague's stated workload (Wordle daily + leaderboards + sync/async PvP):**

1. **Reject Ebitengine WASM.** Bundle weight kills the daily-puzzle-retention loop on mobile cellular.
2. **Keep the current React + Capacitor plan as the primary path.** It's already in the plan (Phase 7 + 8). Lowest cost, fastest ship, best fit for puzzle workload.
3. **Hold Phaser 4 in reserve.** Spike it during Phase 8 (already a plan-listed spike) only if a future `-dle` variant has graphics needs DOM can't satisfy. Low-cost insurance: the `GameEngine<TState,TAction>` interface in Phase 8 already abstracts the renderer, so swapping in a Phaser-backed game variant later costs ~1 sprint, not a rewrite.

**No plan change needed today.** The Phase 8 spike already covers the question. This research validates the current direction and rules out the Ebitengine WASM path explicitly.

---

## 6. Plan implications

If you accept the recommendation:
- **Plan stays as-is.** Phase 7 + 8 already specify React + Capacitor + pluggable game-engine interface.
- Update Phase 8's Phaser-3 mention to **Phaser 4** (just released; if you do spike, use the new version).
- Optionally: drop the `client/` Ebitengine WASM directory in Phase 12 cleanup — it's now confirmed dead weight (this research is the deciding evidence).

If you want to switch primary renderer to Phaser 4 (against my recommendation):
- Phase 7: replace plain React-DOM Wordle with Phaser 4 game scene + React-DOM shell for auth/lobby
- Phase 8: rewrite the engine interface for Phaser scene primitives
- Effort delta: +2–3 days; bundle +400 KB
- Worth it ONLY if you have concrete graphics needs

---

## 7. Unresolved questions

1. **Future `-dle` variants** — do any envisioned games (Sumdle, NumberDle, etc.) need particle effects, animated physics, or canvas-only rendering? If yes, Phaser becomes load-bearing.
2. **Phaser 4 third-party plugin maturity** — verify that any plugins you'd need (e.g. Rex UI, virtual joystick) have migrated to v4 before committing.
3. **Capacitor 7 + Phaser 4 integration story** — search showed Phaser+Capacitor tutorials but mostly Phaser 3 era. Spike compatibility before betting on it.
4. **Legacy `client/` Ebitengine WASM dir fate** — confirm delete in Phase 12.

---

## Sources

- [Ebitengine WebAssembly docs](https://ebitengine.org/en/documents/webassembly.html)
- [Ebitengine Mobile docs](https://ebitengine.org/en/documents/mobile.html)
- [Ebitengine GitHub](https://github.com/ebitengine)
- [Ebitengine gomobile fork](https://github.com/ebitengine/gomobile)
- [WebAssembly for Game Development: Complete Guide 2026 (Reintech)](https://reintech.io/blog/webassembly-game-development-complete-guide-2026)
- [WebAssembly Ecosystem 2026 (Reintech)](https://reintech.io/blog/webassembly-ecosystem-2026-tools-frameworks-runtimes)
- [Phaser 4 Renderer announcement (April 2026)](https://phaser.io/news/2026/04/phaser-4-renderer-faster-cleaner-and-built-for-modern-games)
- [Phaser 4 Release Candidate 7 (March 2026)](https://phaser.io/news/2026/03/phaser-v4-release-candidate-7)
- [Phaser 3 → 4 Migration guide](https://phaser.io/news/2026/04/migrating-from-phaser-3-to-phaser-4-what-you-need-to-know)
- [Bring your Phaser game to iOS and Android with Capacitor](https://phaser.io/tutorials/bring-your-phaser-game-to-ios-and-android-with-capacitor)
- [Capacitor Games guide](https://capacitorjs.com/docs/guides/games)
- [Build a HTML5 game like Wordle using Phaser (tutorial series)](https://emanueleferonato.com/2022/02/20/build-a-html5-game-like-wordle-using-phaser-step-1-keyboard-input-and-result-management/)
- [JS game rendering benchmark (Phaser/Pixi/DOM/Canvas)](https://github.com/Shirajuki/js-game-rendering-benchmark)
- [Can You Make Mobile Games with Phaser? (2025 guide)](https://generalistprogrammer.com/tutorials/phaser-mobile-games-guide)
- [Phaser 3 Mobile Performance discussion](https://phaser.discourse.group/t/phaser-3-mobile-performance-ios-android/1435)
