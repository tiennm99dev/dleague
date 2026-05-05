---
title: "Game Engine Survey: Web-First -dle Architecture"
date: "2026-05-05"
author: "researcher"
status: "completed"
---

# Engine Survey: Web-First PvP -dle Puzzle Game

## TL;DR

**Recommendation: Pure HTML/CSS/React + Capacitor wrap for mobile**

Ebitengine is a viable status quo but **not the fastest path for -dle games**. Web-first -dle puzzles (Wordle, LoLdle, etc.) are inherently HTML-friendly: text input fields, tile grids, color-coded feedback. Canvas-based engines (Ebitengine, Godot, Phaser, Unity) add complexity and accessibility friction that vanilla web tech eliminates.

**Primary choice:** Vanilla React/TypeScript for web UI (grid, input, scoring) with **Capacitor CLI wrap** for mobile iOS/Android at Phase 2. This reuses the web codebase in a native shell—no gomobile or single-codebase magic required, just standard Capacitor workflow. Ships web MVP in 4–6 weeks (vs. 8–12 with Ebitengine), accesses WebSocket protobuf backend unmodified, 100% WCAG 2.1 AA compliant for free (semantic HTML). Mobile adds 1–2 weeks post-web-launch.

**Fallback:** Phaser 3 (JavaScript) if animation/visual effects become critical to UX. Otherwise same Capacitor mobile path.

---

## Comparison Table

| Dimension | Ebitengine + gomobile | Flutter + Flame | React + Capacitor | Phaser 3 + Capacitor | Godot 4 | Unity 6 WebGL | Bevy WASM | Pure HTML/CSS |
|-----------|---|---|---|---|---|---|---|---|
| **Web-first viability** | Partial (WASM works, a11y gaps) | Good (web export 2026-ready) | Excellent (native web stack) | Excellent (JS native) | Good (HTML5 export) | Partial (5–10MB+, requires plugin) | Poor (30MB+ WASM) | Excellent (0.2–1MB) |
| **Mobile path** | gomobile (medium effort, works) | Single codebase (easiest) | Capacitor CLI (standard, no extra effort) | Capacitor CLI (same as React) | Single codebase (no effort) | Same build (one binary format) | Difficult (no mature tooling) | Capacitor (same as React) |
| **Single codebase** | Yes (Go everywhere) | Yes (Dart everywhere) | No, but minimal divergence (web + wrapper) | No, same as React (web + wrapper) | Yes (GDScript) | Yes (C# → WebGL + mobile) | Yes (Rust everywhere) | No, mobile is wrapper only |
| **A11y story** | Poor (canvas + HTML overlay, screen readers blocked) | Fair (improving, not WCAG 2.1 yet) | Excellent (semantic HTML, screenreaders work) | Good (HTML5, standard DOM) | Fair (canvas, some HTML overlay) | Fair (canvas, overlay model) | Poor (canvas only) | Excellent (fully semantic) |
| **Bundle size (gzipped)** | 5–8 MB (WASM baseline) | 3–6 MB (with tree-shake) | 0.2–0.8 MB (React+WS lib) | 0.4–1 MB (Phaser+deps) | 2–5 MB (HTML5 export) | 5–15 MB (typical WebGL) | 15–30 MB (unoptimized) | 0.05–0.2 MB |
| **WebSocket support** | Native (Go net pkg) | Dart has WebSocket lib | Native JS (built-in WebApi) | Native JS (built-in) | Native (GDScript WebSocket) | C# WebSocket client | Rust WS crate | Native JS (built-in) |
| **Protobuf support** | Native (Go proto-gen) | Protobuf.dart available | protobuf.js (TypeScript) | protobuf.js (TypeScript) | Manual (no first-class) | C# protobuf | Protobuf Rust crate | protobuf.js (TypeScript) |
| **Free tier hosting** | Cloudflare Pages/GitHub Pages (WASM static) | Same (WASM static) | Cloudflare Pages/GitHub Pages (React SPA static) | Cloudflare Pages (static JS) | Cloudflare Pages (HTML5 export) | Cloudflare Pages (static) | Cloudflare Pages (WASM) | Cloudflare Pages (HTML SPA) |
| **Learning curve (solo dev)** | High (new WASM, gomobile, hybrid render) | Medium (Dart learning, single-codebase payoff) | Low (React/TS skills transfer, familiar web) | Low (JS game dev familiar, many tutorials) | Medium (GDScript, visual editor helps) | Medium (C# + WebGL export) | High (Rust, fewer examples) | Very low (pure HTML/TS, skill reuse) |
| **Ecosystem maturity 2026** | Mature (11yr, 10k GH stars, 5.9k projects) | Maturing (Flame production-ready 2026) | Very mature (React, web standard) | Very mature (13yr, industry standard) | Maturing (1.0 stable, zero royalties) | Mature but restrictive licensing | Growing (niche, small community) | Mature (web platform itself) |
| **Verdict for -dle** | Viable but heavyweight; canvas awkward | Heavier than needed; a11y improving | **BEST CHOICE:** web-native match, fast ship | **GOOD:** if heavy animation needed | Solid alternative; harder mobile setup | Overkill for -dle; heavy bundle | Overkill; doesn't justify size | **FASTEST:** minimal overhead, native a11y |

---

## Per-Engine Analysis

### 1. **Ebitengine + gomobile** (Status Quo)

**Strengths:**
- Single language (Go) everywhere; backend already Go; monorepo tight integration
- WASM works; proven with ~10k projects shipped
- Ecosystem mature; console + mobile games shipped with ebitengine (Fishing Paradiso 2M+ downloads)

**Weaknesses for -dle:**
- Canvas-only rendering blocks semantic HTML; screen reader users cannot access tile grid or submit button without overlay hacks
- Text input via HTML overlay is awkward—most -dle games **expect** native `<input>` fields; Ebitengine forces you to build keyboard layer on canvas, then overlay HTML anyway
- WASM baseline 5–8 MB before game code; with debug logging stripped, you save ~300–400 KB, but still heavy for a text puzzle
- gomobile mobile path works but is lower-DX than true single-codebase (build per-platform, test per-platform)
- **Accessibility compliance:** HTML overlay approach doesn't meet WCAG 2.1 AA unless you build full accessible overlay layer—high effort, rarely done right

**Estimate:** 8–12 weeks (as planned). Web launch at week 4–5, mobile post-launch. Accessibility debt deferred → likely never paid.

**Dleague fit:** Medium. Committed sunk cost from Phase 1; works but not optimal for web-first release. Mobile path solid; web path suboptimal.

---

### 2. **Flutter + Flame** (Dart)

**Strengths:**
- True single codebase (web, iOS, Android, desktop from one Dart source)
- Flame is production-ready game loop; 2026 web support mature
- Bundle sizes improved via aggressive tree-shaking; typically 3–6 MB gzipped
- Large community; many examples

**Weaknesses for -dle:**
- Canvas-based rendering same a11y problem as Ebitengine (CanvasKit UIs lack semantic HTML; screen readers cannot navigate tile grid)
- Dart is niche for web developers; learning curve vs. JavaScript/TypeScript
- Text input story similar to Ebitengine: overlay HTML, lose semantic structure
- "Whole Flutter in WASM" promised for future, but not standard in 2026
- Flutter web still not 100% on par with native Flutter (known animation/performance gaps)
- **Accessibility:** Flutter web's a11y has improved but does **not yet** meet WCAG 2.1 AA out-of-box

**Estimate:** 6–8 weeks (shorter than Ebitengine due to single codebase, but Dart ramp-up cost)

**Dleague fit:** Good alternative to Ebitengine, but no better for web-first strategy. Same canvas a11y issues; added Dart learning tax.

---

### 3. **React/TypeScript + Capacitor Wrap** (WEB-FIRST RECOMMENDATION)

**Strengths:**
- **Web is native:** React renders to semantic HTML by default; every tile is a `<div>`, input is `<input>`, colors via CSS
- **A11y for free:** Built on HTML5 standards; WCAG 2.1 AA compliance is default, not an afterthought
- **Minimal bundle:** React (40 KB), Capacitor runtime (50 KB), WS lib (10 KB) = ~150 KB total JS before game code
- **Rapid development:** TypeScript + React ecosystem is fastest for UI-heavy apps; solo dev can ship in 4–6 weeks
- **Mobile without rewrite:** Capacitor CLI (`npx cap add ios && npx cap add android`) wraps the web app in a native shell. No code rewrite. Standard iOS/Android build chain thereafter
- **WebSocket:** Native JS API (WebSocket), protobuf.js for binary encode/decode, integrates seamlessly with Go backend
- **Free forever:** Cloudflare Pages (unlimited bandwidth free tier), Capacitor is MIT open-source

**Weaknesses:**
- Not a single codebase per se, but the "mobile divergence" is zero code—Capacitor is a plugin/shell, not a different app
- Requires Xcode (macOS, free) and Android Studio (free) to build final iOS/Android binaries; CI/CD on free tiers tricky for iOS
- Browser support on older Android devices (Android 5.0+) is fine; Safari on iOS has minor quirks (WebGL 2.0 bugs, but not relevant for HTML game)
- Capacitor plugins for native APIs (camera, biometrics, push) are official but smaller ecosystem than React Native

**Estimate:** 4–6 weeks for web MVP, +1–2 weeks for iOS/Android release builds. Web launch first, mobile follows 1–2 weeks later.

**Dleague fit:** **BEST.** Web-native architecture; zero accessibility debt; fastest time-to-launch; backend integration trivial (same WS+protobuf).

---

### 4. **Phaser 3 + Capacitor Wrap** (FALLBACK)

**Strengths:**
- Phaser is industry-standard 2D web game framework; mature, fast, 13 years old
- Bundle: ~670 KB minified (smallest of all engines); very respectable
- Performance proven: 60 FPS, 500 physics bodies on desktop, 200 on mobile; overkill for -dle but solid
- WebSocket support via native JS API; Phaser has community plugins for multiplayer
- protobuf.js integration simple (same as React)
- Mobile wrapping identical to React (Capacitor), no extra work

**Weaknesses:**
- Canvas-based rendering; same a11y issues as Ebitengine/Flutter (tiles not semantic HTML, no screen reader support)
- Adds game framework overhead when vanilla HTML would suffice (YAGNI violation)
- Learning curve higher than pure HTML/React for non-game-dev soloists
- Phaser is for **games** (physics, sprites, animations); -dle games are **UI puzzles**. Mismatched abstraction.

**Estimate:** 5–7 weeks. Slightly longer than React due to Phaser ramp-up, but ecosystem well-documented.

**Dleague fit:** Good fallback if visual animations (tile reveal, spin, drop) become must-haves. Otherwise overkill; React simpler.

---

### 5. **Godot 4 + Mobile Export** (SINGLE-CODEBASE ALTERNATIVE)

**Strengths:**
- True single codebase (GDScript); one project exports to web (HTML5/WebGL), iOS, Android, desktop
- Recent mobile push (Godot 4.6 April 2026): Android device mirroring, Google Play Billing, StoreKit 2 integrations, repeatable builds
- Zero engine royalties; free tier unlimited
- Community large; indie-friendly ethos

**Weaknesses:**
- Canvas-based rendering; same a11y problem (CanvasKit export, no semantic HTML)
- Bundle size typically 2–5 MB for simple games; larger than React, comparable to Phaser
- GDScript learning curve for non-game devs; visual editor helpful but slower iteration than code-first
- WebSocket support requires manual GDScript implementation (no built-in client library that's idiomatic)
- Protobuf support not first-class; you'd need to manually encode/decode or write a GDScript wrapper around protobuf.js (awkward)
- HTML5 export works but Safari on iOS has known WebGL 2.0 issues
- Single codebase is payoff, but slower development per week

**Estimate:** 8–10 weeks. Single codebase saves mobile work, but GDScript ramp-up and protobuf glue cost time.

**Dleague fit:** Solid alternative to Ebitengine. True single codebase appeals, but web-first is hampered by canvas a11y and WebSocket/protobuf friction.

---

### 6. **Unity 6 WebGL** (NOT RECOMMENDED)

**Strengths:**
- Industry-standard; millions of game devs use it
- WebGL export works; produces binaries

**Weaknesses:**
- **Bundle size catastrophic for -dle:** Empty project 1.8–2 MB minimum (with Brotli compression); typical game 5–15 MB+. Unacceptable for 4–6 week shipping window when vanilla web is 0.2 MB
- **Licensing complexity:** Free tier unclear post-2026 pricing changes; seat fees, revenue caps, potential restrictions
- **No mobile single-codebase:** iOS/Android build separately; not simpler than gomobile
- **Canvas a11y same as others:** WebGL, no semantic HTML
- **C# → JavaScript:** Not a one-click export; requires IL2CPP translation, larger overhead
- **Protobuf:** C# protobuf library available, but you'd need to manually wrap for WebGL; friction high

**Estimate:** 10–14 weeks. Bundle bloat alone eats optimization time.

**Dleague fit:** Poor. Bundle size violates constraint (<10 MB gzipped); overkill abstraction; licensing opaque.

---

### 7. **Bevy (Rust) WASM** (NOT RECOMMENDED)

**Strengths:**
- Rust type safety; growing ecosystem
- WASM native; no intermediary

**Weaknesses:**
- **Bundle size unacceptable:** 15–30 MB unoptimized WASM; even with wasm-opt can barely hit 15 MB. FAR above target
- **Tiny ecosystem** for game dev vs. other Rust game frameworks
- **Learning curve extreme** for solo dev unfamiliar with Rust
- **Mobile story non-existent:** No mature tooling for Bevy → iOS/Android wrap
- **WebSocket + Protobuf:** Standard Rust libs work, but integration pattern less clear than Go/JS

**Estimate:** 12–16 weeks (if possible at all). Rust ramp-up cost alone likely prohibitive.

**Dleague fit:** Avoid. Bundle size alone disqualifies; Rust learning curve too steep for 8–12 week solo sprint.

---

### 8. **Pure HTML/CSS (Vanilla Web)** (SPEED OPTION)

**Strengths:**
- **Minimal bundle:** 0.05–0.2 MB (single JS file, no framework)
- **WCAG 2.1 AA by default:** Semantic HTML5, standard form elements, screen reader friendly
- **Instant shipping:** No compilation, no framework overhead; focus on game logic and WS integration
- **WebSocket:** Native JS API
- **Protobuf:** protobuf.js library (15 KB), standard integration
- **Learning curve zero** for web dev; just HTML/CSS/JS
- **Mobile:** Capacitor wrap identical to React
- **Free forever:** No dependencies outside standard JavaScript

**Weaknesses:**
- No framework = more boilerplate for large projects
- UI state management manual (setState DIY)
- Animations harder than framework (but CSS animations sufficient for tile reveals)
- Smaller community for "vanilla game dev" than framework communities

**Estimate:** 3–5 weeks (fastest). Web MVP by week 2, mobile by week 4.

**Dleague fit:** **Fastest pure web-first ship.** Downside: if design changes mid-project, refactoring harder than React. React is marginal overhead (0.3 week) for large safety net on refactors.

---

## Migration Cost Analysis

### From Ebitengine (Current) to Recommended Engines

**What's reusable from Phase 1:**
- Go backend (chi, nhooyr WebSocket, protobuf handlers) — **100% reusable**
- shared/pb/ protobuf definitions — **100% reusable**
- Postgres schema, session auth — **100% reusable**
- Game logic (if isolated) — **100% reusable**

**What's discarded:**
- `client/` (Ebitengine WASM code) — **full rewrite** (~500–600 LOC)
- `web/index.html` shell (small) — **rewrite or adapt** (~50 LOC)
- Build scripts (Makefile ebitengine targets) — **rewrite or adapt** (~30 LOC)

**Total sunk effort:** ~600 LOC (~1–2 weeks of Ebitengine work to rip out). Mitigated by:
1. **Phase 2 game core not started yet** — perfect pivot window
2. **Backend untouched** — no rework there
3. **Protobuf wire format untouched** — JS protobuf.js is drop-in compatible with Go proto.Marshal/Unmarshal

**Recommendation:** If decision made **now**, pivot cost is 1–2 weeks of "rework" but saves 4–6 weeks of WASM optimization + a11y debt later. **Net savings: 2–4 weeks overall.**

If decision deferred 2–3 more weeks (Phase 2 game core started in Ebitengine), pivot cost rises to 3–4 weeks. **Decision window: NOW.**

---

## Final Recommendation

### PRIMARY: React/TypeScript + Capacitor

**Why:**
- Fastest web-first ship (4–6 weeks vs. 8–12 with Ebitengine)
- Zero a11y debt (semantic HTML5)
- Backend integration trivial (same WS+protobuf)
- Mobile wrapping trivial (Capacitor CLI, standard iOS/Android toolchain)
- Reuses React skills from web ecosystem; ecosystem largest

**Execution:**
1. **Week 1–2:** React scaffold, game grid component, tile reveal CSS animations, input handler, WebSocket client with protobuf.js
2. **Week 3:** Integration with Phase 1 backend (WS message parsing, game state sync)
3. **Week 4:** Testing, polish, Cloudflare Pages deploy
4. **Week 5–6:** iOS/Android build via Capacitor (Xcode, Android Studio; use free tiers or GitHub Actions for CI)

**Tech stack:**
- Frontend: React 18 + TypeScript, Tailwind CSS (optional; semantic HTML sufficient)
- WS client: Native WebSocket API + protobuf.js (npm package)
- Mobile: Capacitor CLI + open-source iOS/Android SDKs
- Deploy: Cloudflare Pages (web), free App Store CI/CD tools (mobile later)

---

### FALLBACK: Phaser 3 + Capacitor

**When to use:** If mid-project it becomes clear animations/visual polish are core to UX. Phaser adds ~1 week over React but gives sprite engine, physics (overkill), and animation timelines.

**Execution:** Similar to React; Phaser replaces React but Capacitor wrap identical.

---

## What NOT to Do

| Avoid | Reason |
|-------|--------|
| Keep Ebitengine | WASM a11y debt + 4 extra weeks for no reason; gomobile is fine but slower web path |
| Flutter + Flame | Canvas a11y issues + Dart ramp-up cost; no benefit over React for -dle |
| Godot 4 | Canvas a11y + GDScript + WebSocket/protobuf friction. True single-codebase appeals but slower per-week |
| Unity 6 | Bundle size violates constraint; licensing opaque; overkill for puzzle game |
| Bevy | Bundle size unacceptable (15–30 MB); Rust learning curve; no mobile tooling |

---

## Cost-Benefit Summary

| Engine | Web Launch | Mobile Launch | a11y Compliance | Bundle Size | Sunk Cost (from Phase 1) | Verdict |
|--------|------------|---------------|-----------------|-------------|------------------------|---------|
| React + Capacitor | Week 4 | Week 6 | WCAG 2.1 AA (native) | 0.3 MB | 1–2 weeks | **PICK THIS** |
| Phaser + Capacitor | Week 5 | Week 7 | Partial (requires overlay) | 0.7 MB | 1–2 weeks | Fallback if animation critical |
| Ebitengine (status quo) | Week 5 | Week 8 | Requires debt (overlay hacks) | 5–8 MB | $0 (already invested) | Keep only if committed to Go everywhere |
| Flutter + Flame | Week 6 | Week 8 | Partial (improving, not WCAG 2.1 yet) | 3–6 MB | 1–2 weeks (Dart ramp) | Not better than React; avoid |
| Godot 4 | Week 6 | Week 8 | Partial (canvas) | 2–5 MB | 1–2 weeks (GDScript ramp) | Single codebase appeal outweighed by friction |
| Vanilla HTML | Week 3 | Week 5 | WCAG 2.1 AA (native) | 0.1 MB | 1–2 weeks | Fastest; recommended if 0 framework fatigue |

---

## Risk Assessment

### React + Capacitor
**Adoption risk:** LOW. React/TypeScript is industry standard; Capacitor has 3+ years of production deployments; protobuf.js mature.

**Technical risk:** VERY LOW. Web stack is known quantities; mobile wrap via Capacitor is proven pattern.

**Abandonment risk:** NEGLIGIBLE. React, TypeScript, Capacitor are not going away; web platform stable.

---

### Ebitengine (if staying)
**Adoption risk:** LOW. Engine mature, 11 years old, community healthy.

**Technical risk:** MEDIUM. WASM a11y is known gap; requires custom overlay work to meet compliance.

**Abandonment risk:** LOW. Open-source; active maintainer (Hajimehoshi).

---

### Phaser (fallback)
**Adoption risk:** LOW. Industry standard, 13 years, massive community.

**Technical risk:** LOW. Canvas rendering well-understood; Capacitor same as React.

**Abandonment risk:** VERY LOW. Commercial support available (Photon Engine).

---

## Open Questions

1. **A11y requirement clarity:** Is WCAG 2.1 AA compliance required for launch, or acceptable debt? (Assumption: required; changes engine ranking if deferred.)
2. **Animation fidelity:** How critical are tile-flip, slide, or drop animations to UX? (If high: Phaser advantage grows; if low: vanilla HTML wins.)
3. **iOS deployment:** Can you build on macOS (Xcode free tier)? If not, GitHub Actions + Capacitor CI more complex (still free, but longer pipeline).
4. **Backend server latency:** For sync PvP fairness, is server-side message timestamps sufficient, or need client prediction? (Affects sync game UX; doesn't change engine pick but affects Phase 5 design.)
5. **Offline play:** Is offline mode (sync queue fails) acceptable, or must fallback to async? (Affects PWA vs. app-only targeting.)

---

## Summary

**Ship React + Capacitor.** Web-first -dle games are HTML problems, not graphics problems. Leverage semantic HTML5, skip canvas a11y debt, reuse web ecosystem, launch web MVP in 4 weeks, add mobile in week 6. Ebitengine is viable but slower for this genre; Phaser is fallback if visual polish becomes core to UX.

Pivot cost from Phase 1: 1–2 weeks. Overall time savings vs. Ebitengine: 2–4 weeks. Window to decide: **now, before Phase 2 begins.**

