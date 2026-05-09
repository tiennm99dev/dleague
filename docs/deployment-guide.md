# Deployment Guide

Dleague runs as a single Go binary on Fly.io, serving both the API and the
static SvelteKit web client. Persistent state lives in MongoDB Atlas M0 and
Firebase Auth (both external managed services).

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| `fly` CLI | latest | `curl -L https://fly.io/install.sh \| sh` |
| Go | 1.23+ | https://go.dev/dl/ |
| Node.js | 20+ | https://nodejs.org/ |
| MongoDB Atlas account | — | https://cloud.mongodb.com/ |
| Firebase project | — | https://console.firebase.google.com/ |

---

## 1 — MongoDB Atlas Setup

1. Create a free **M0** cluster in region **us-east-1** (paired with Fly.io `iad`).
2. Create database user `dleague_prod` with role **readWrite** on database `dleague`.
   - Avoid cluster-level admin roles (least-privilege).
3. Network Access → Add IP Address → **0.0.0.0/0** (allow all).
   - **Accepted-risk decision:** Atlas user/password is the actual security gate.
     Restricting IPs on Fly.io is impractical because outbound IPs are not stable
     per-machine. Revisit with M10 + VPC Peering when scale requires it.
4. Copy the SRV connection string:
   `mongodb+srv://dleague_prod:<PASSWORD>@<CLUSTER>.mongodb.net/dleague?retryWrites=true&w=majority`

### Atlas upgrade triggers (M0 → M10)
- Storage used > 70% of 512 MB, or
- Sustained ops/sec > 50, or
- Need private endpoint / VPC peering (security hardening).

### Atlas M0 notes
- No dedicated backups on M0 (Atlas free tier). Document accepted-risk.
- M0 pauses after 60 days of inactivity on some regions — use nightly CI ping
  or upgrade to M10 for production with real traffic.

---

## 2 — Firebase Project Setup

1. Create a project `dleague-prod` at https://console.firebase.google.com/.
2. Enable **Authentication** → Sign-in methods:
   - Email/Password
   - Google
   - Anonymous
3. Project Settings → Service Accounts → **Generate new private key** →
   download `serviceAccount-dleague-prod.json`.
4. Restrict Auth domain: Project Settings → Authorized domains →
   add `dleague.fly.dev` (and your custom domain if any).
5. Firebase web config (`apiKey` etc.) is public and committed in
   `web/src/lib/firebase.config.json`; restrict by domain in Firebase console.

---

## 3 — Create the Fly App

```bash
# Install fly CLI and authenticate.
fly auth login

# Create the app (first time only).
fly apps create dleague

# Optional: create staging app.
fly apps create dleague-staging
```

---

## 4 — Set Fly Secrets

**Never commit** these values. Copy each command from
`scripts/set-fly-secrets.sh` and run manually:

```bash
fly secrets set MONGO_URI="mongodb+srv://dleague_prod:<PASSWORD>@<CLUSTER>.mongodb.net/dleague?retryWrites=true&w=majority"
fly secrets set FIREBASE_PROJECT_ID="dleague-prod"
fly secrets set FIREBASE_SERVICE_ACCOUNT_B64="$(base64 -w0 serviceAccount-dleague-prod.json)"
fly secrets set DLEAGUE_WS_ORIGINS="https://dleague.fly.dev"
fly secrets set DLEAGUE_TRUSTED_PROXIES="<fly_proxy_cidr>"
```

The server boot decodes `FIREBASE_SERVICE_ACCOUNT_B64` to `/tmp/dleague-sa.json`
(mode 0600) and sets `GOOGLE_APPLICATION_CREDENTIALS` automatically. The JSON
file is never embedded in the container image.

---

## 5 — Deploy

```bash
# Deploy production.
make deploy
# equivalent to: fly deploy --remote-only

# Deploy staging.
make deploy-staging
```

Fly builds the image remotely using the `Dockerfile` at the repo root.
No local Docker required.

---

## 6 — Seed Wordlists (one-time)

```bash
MONGO_URI="mongodb+srv://dleague_prod:..." bash scripts/seed-wordlists.sh
```

Uploads the embedded 772-word placeholder list into the `wordlists` collection.
The server falls back to the embedded binary if the collection is empty, so this
step is optional but recommended for production.

---

## 7 — Verify

```bash
# Health check.
curl -s https://dleague.fly.dev/health | jq .
# → {"status":"ok","mongo":"ok"}

# View logs.
fly logs --app dleague

# Check machine status.
fly status --app dleague
```

---

## 8 — Rollback

```bash
# List recent releases.
fly releases --app dleague

# Roll back to a previous image.
fly deploy --image registry.fly.io/dleague:<version> --app dleague
```

---

## 9 — Promote Admin User

After the user signs in at least once (creating a Mongo document):

```bash
FIREBASE_PROJECT_ID=dleague-prod \
GOOGLE_APPLICATION_CREDENTIALS=/path/to/serviceAccount-dleague-prod.json \
bash scripts/promote-admin.sh <firebase-uid>
```

The admin claim takes effect on the user's next token refresh (~1 hour).

---

## 10 — Monitoring

| Signal | Where |
|--------|-------|
| Server logs | `fly logs --app dleague` |
| HTTP metrics | Fly.io dashboard → Metrics |
| Mongo metrics | Atlas → Monitor tab |
| Firebase usage | Firebase console → Usage & billing |
| Health check | `/health` endpoint (Fly monitors every 30 s) |

---

## Security Notes

- TLS enforced by Fly.io (`force_https = true` in `fly.toml`).
- Service-account JSON never on disk in the image; injected via env at boot.
- Prod Atlas user is `readWrite@dleague` only — no cluster admin perms.
- `0.0.0.0/0` IP allowlist is an accepted-risk decision documented above.
- Boot-time assertion: `DLEAGUE_WS_ORIGINS` must be non-empty in production.
- Container runs as distroless (no shell, no package manager, minimal attack surface).
