# Phase 1: Firebase project provisioning

## Context Links
- Research: `plans/reports/researcher-260505-1407-firebase-as-backend-feasibility.md`
- Parent plan: `plans/260505-1407-firebase-platform-pivot/plan.md`
- Locked decisions: sign-in = Google + Email/Pass + Anonymous; UID = PK; creds via env

## Overview
- **Priority:** P1 (blocker for phases 2–8)
- **Status:** pending
- **Effort:** 0.5d (mostly clicking through Firebase console)
- Manual provisioning of Firebase project, Auth providers, Firestore (Native mode), service-account JSON. **No code in this phase.**

## Key Insights
- Spark plan (free) DOES NOT include Cloud Functions — design avoids them entirely
- Service-account JSON is the single secret; whole content goes to one env var (`FIREBASE_CREDENTIALS_JSON`)
- Firestore Native mode (NOT Datastore mode) — choice is permanent per project
- Auth domain whitelist must include both `localhost:5173` (Vite dev) and final Firebase Hosting domain + `api.dleague.tld`
- Anonymous auth must be enabled BEFORE first client launch; otherwise guests see "this provider is disabled"

## Requirements

### Functional
- Firebase project named `dleague-prod` (or per user choice)
- Auth providers enabled: Google, Email/Password, Anonymous
- Firestore database in Native mode, region `asia-southeast1` (Singapore — closest to OCI Coolify VM if Tokyo unavailable; user confirms)
- Initial security rules: deny-all placeholder
- Service-account JSON downloaded + stored locally as `secrets/firebase-admin-sa.json` (gitignored)

### Non-functional
- Free Spark plan ONLY — verify "Spark" badge in Firebase console settings
- Budget alert: Cloud Console → Billing → set alert at $1 to catch accidental Blaze upgrade
- All values reproducible via screenshots

## Architecture
- Single Firebase project, single Firestore DB, single RTDB (provisioned but unused at this phase)
- No Cloud Functions, no Cloud Storage buckets (defer until needed)
- Service account principle: `firebase-adminsdk@<project>.iam.gserviceaccount.com` — bypasses security rules

## Related Code Files
- **Create:** `secrets/firebase-admin-sa.json` (gitignored — local dev only; prod uses env)
- **Create:** `secrets/.gitkeep`
- **Modify:** `.gitignore` add `secrets/*.json`
- **Modify:** `.env.example` add `FIREBASE_CREDENTIALS_JSON=` placeholder + `FIREBASE_PROJECT_ID=`

## Implementation Steps
1. Go to https://console.firebase.google.com → "Add project" → name `dleague-prod` → disable Google Analytics (free tier; reduce data exposure)
2. Build → Authentication → Get started → Sign-in method tab:
   - Enable **Google** (set support email)
   - Enable **Email/Password** (do NOT enable email link sign-in)
   - Enable **Anonymous**
3. Settings (gear) → Authorized domains: add `localhost`, `dleague.tld`, `api.dleague.tld` (TLD placeholder; user fills)
4. Build → Firestore Database → Create database → **Native mode** → region `asia-southeast1` → Production rules (deny all)
5. Build → Realtime Database → Create database → same region → "Locked mode" (used optionally in phase-7)
6. Project settings → Service accounts → Generate new private key → download JSON → save as `secrets/firebase-admin-sa.json`
7. Capture screenshots for repo: console "Spark" badge, Auth providers list, Firestore rules tab, RTDB rules tab. Store in `plans/260505-1407-firebase-platform-pivot/screenshots/` (gitignored).
8. Cloud Console → Billing → set budget alert: $1 threshold → email notification
9. Add to `.gitignore`: `secrets/*.json` and `screenshots/`
10. Update `.env.example` with placeholder values (described in next section)

## .env.example additions
```
# Firebase
FIREBASE_PROJECT_ID=dleague-prod
# Paste full JSON content as single-line string. Coolify env editor accepts multiline OK.
FIREBASE_CREDENTIALS_JSON={"type":"service_account",...}
# Optional: only set if migrating BACK to MySQL
STORE_BACKEND=firestore   # firestore (default) | mysql
```

## Todo List
- [ ] Create Firebase project (Spark plan)
- [ ] Enable Google Auth provider
- [ ] Enable Email/Password Auth provider
- [ ] Enable Anonymous Auth provider
- [ ] Add authorized domains
- [ ] Create Firestore in Native mode (region asia-southeast1)
- [ ] Create RTDB instance (locked mode, defer use)
- [ ] Generate + download service account JSON
- [ ] Save SA JSON to `secrets/firebase-admin-sa.json`
- [ ] Update `.gitignore` (`secrets/*.json`, `screenshots/`)
- [ ] Update `.env.example`
- [ ] Capture provisioning screenshots
- [ ] Set $1 budget alert in Cloud Console
- [ ] Verify Spark plan badge present

## Success Criteria
- [ ] `firebase projects:list` (Firebase CLI) shows `dleague-prod` with `--plan=Spark`
- [ ] All 3 auth providers visible as "Enabled" in Auth → Sign-in method
- [ ] Firestore rules tab shows default deny-all
- [ ] `secrets/firebase-admin-sa.json` parses as valid JSON with `type: "service_account"`
- [ ] `.env.example` committed; actual creds NOT committed

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Wrong region picked (latency to Coolify VM) | Med | Med | Confirm Coolify VM region with user before step 4; recreate is free but loses time |
| Service account JSON committed accidentally | Low | Critical | `.gitignore` step BEFORE downloading; verify with `git status` |
| Auth provider disabled at console; client gets cryptic "provider not enabled" error | Med | Low | Phase-1 success criteria includes visual verify; phase-4 client logs distinguish |
| Free tier surprise charges | Low | Med | $1 budget alert + verify Spark badge weekly during dev |

## Security Considerations
- Service account JSON = root credentials for Admin SDK; treat as production secret
- NEVER paste full JSON to chat, logs, or git history
- Coolify env injection over HTTPS only; rotate quarterly
- Firestore rules start at deny-all; phase-3 narrows allow-list (not the other direction)

## Next Steps
- **Unblocks:** phase-2 (server integration needs the SA JSON)
- **Unblocks:** phase-4 (web client needs Firebase config object — public, separate from SA JSON)
- Capture web SDK config (apiKey, authDomain, projectId, etc.) from Project settings → General → Your apps → "Add web app". Save to `web/.env.local.example` for phase-4.

## Unresolved Questions
1. Final domain TLD for `dleague.tld` — user confirms before phase-8 DNS setup
2. Firestore region: `asia-southeast1` (Singapore) vs `asia-northeast1` (Tokyo) — depends on Coolify VM region (per MySQL plan it was Tokyo). Confirm before step 4.
3. Should we also enable Apple Sign-In for iOS app store compliance later? (Defer to phase-4 mobile decision)
