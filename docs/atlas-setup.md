# MongoDB Atlas setup runbook

One-page, repeatable setup for the dleague Atlas cluster. Stack is
[Atlas](https://cloud.mongodb.com) M0 free tier, AWS Singapore.

## 1. Create the cluster

1. Sign in to [cloud.mongodb.com](https://cloud.mongodb.com) and create the
   project `dleague-beta`.
2. **Build a Database** → **M0 Free** → Provider **AWS**, Region
   **Singapore (ap-southeast-1)**, cluster name `dleague-m0`. Submit.

## 2. Create the database user

1. Project → **Database Access** → **Add New Database User**.
2. Method: **Password**.
3. User: `dleague-app`. Password: `openssl rand -base64 24` (save in 1Password).
4. **Database User Privileges** → **Specific Privileges**:
   - role `readWrite` on database `dleague`.
5. Save.

## 3. Configure network access

1. Project → **Network Access** → **Add IP Address**.
2. **Allow access from anywhere (0.0.0.0/0)** is acceptable during beta —
   SCRAM auth still gates every request. Add a comment:
   `beta only — replace with static-IP allowlist or PrivateLink before launch`.
3. Save.

## 4. Get the connection string

1. Cluster → **Connect** → **Drivers** → **Go** → version **2.6 or later**.
2. Copy the SRV string. Replace `<password>` with the real DB user password.
3. Result looks like:
   ```
   mongodb+srv://dleague-app:<password>@dleague-m0.xxxxx.mongodb.net/?retryWrites=true&w=majority&appName=dleague-server
   ```

## 5. Inject the URI into runtime configuration

- **Local dev:** copy `.env.example` to `.env` and set `MONGODB_URI=...`.
- **Coolify:** Project → Environment → add secret env var `MONGODB_URI` with
  the same value. Mark as secret so it does not leak into logs.

Never commit the URI. `.env` is gitignored.

## 6. Smoke test

Run the Atlas smoke test from a machine with the URI set:

```sh
MONGODB_URI='mongodb+srv://...' go run ./server/cmd/atlas-smoke
```

Expected output: `ping ok (db=dleague)`.

If it fails: re-check the IP allowlist, the password URL-encoding, and the
SRV DNS resolution from the source network.

## 7. Rotate the password

1. **Database Access** → user `dleague-app` → **Edit** → **Edit Password**.
2. Generate a new password, save in 1Password.
3. Update the `MONGODB_URI` env var in Coolify and `.env` locally.
4. The cluster accepts both old and new passwords briefly during rotation;
   restart the server within 5 minutes to flush old connections.

## 8. Pre-launch hardening (deferred; revisit before non-beta release)

- Replace `0.0.0.0/0` with: (a) Coolify static egress IP allowlist, or
  (b) AWS PrivateLink (M10+ tier).
- Enable Atlas alerts: connection-count breach, slow-query log, disk-full
  warning. All free on M0.
- Schedule daily `mongodump` to OCI Object Store.
