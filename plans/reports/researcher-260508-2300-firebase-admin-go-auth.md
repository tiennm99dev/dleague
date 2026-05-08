# Firebase Admin Go Auth Integration Research

**Date:** 2026-05-08 | **Status:** DONE | **Scope:** Firebase ID token auth for Dleague WS + Mongo user sync

---

## 1. Firebase Admin SDK for Go Setup

### Package & Initialization
- **Import:** `firebase.google.com/go/v4` (v4.19.0+); auth subpackage: `firebase.google.com/go/v4/auth`
- **Credential loading (3 patterns):**
  
  **(a) Service Account JSON via env var** (simplest for Phase 3):
  ```bash
  export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
  ```
  
  **(b) Explicit service account in code:**
  ```go
  opt := option.WithCredentialsFile("/path/to/service-account.json")
  app, err := firebase.NewApp(ctx, nil, opt)
  ```
  
  **(c) Workload Identity (Fly.io/GCP best practice):**
  ```bash
  # On Fly.io with OIDC Token Federation:
  export GOOGLE_APPLICATION_CREDENTIALS=  # unset; uses ambient credentials
  export GOOGLE_CLOUD_PROJECT=your-project-id
  # Firebase SDK auto-uses Fly's OIDC token at refresh time
  ```

### Initialization Pattern
```go
app, err := firebase.NewApp(context.Background(), nil)
if err != nil {
    log.Fatal(err)
}

authClient, err := app.Auth(context.Background())
if err != nil {
    log.Fatal(err)
}
```

**Recommendation:** Use env var for dev (`GOOGLE_APPLICATION_CREDENTIALS` → local service account), Workload Identity for production (Fly.io native, zero credential management).

---

## 2. WebSocket Auth Gate — ID Token Verification

### Two Upgrade Patterns

#### Pattern A: Token in `Sec-WebSocket-Protocol` header (recommended for Dleague)
Client sends ID token as a WebSocket subprotocol parameter during upgrade:

```
GET /ws HTTP/1.1
Sec-WebSocket-Protocol: firebase.idtoken={base64-encoded-id-token}
```

Server extracts and verifies at upgrade time:

```go
func UpgradeHandler(hub *Hub, authClient *auth.Client, opts UpgradeOptions) http.HandlerFunc {
	accept := websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
		OriginPatterns:  opts.AllowedOrigins,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract ID token from Sec-WebSocket-Protocol
		proto := r.Header.Get("Sec-WebSocket-Protocol")
		idToken := extractTokenFromProtocol(proto)
		
		// Verify token
		token, err := authClient.VerifyIDToken(r.Context(), idToken)
		if err != nil {
			http.Error(w, "auth failed", http.StatusUnauthorized)
			return
		}
		
		// Upgrade after auth passes
		c, err := websocket.Accept(w, r, &accept)
		if err != nil {
			log.Printf("ws accept: %v", err)
			return
		}
		c.SetReadLimit(readLimit)

		conn := &Conn{
			ws:     c,
			hub:    hub,
			userID: token.UID,  // Store Firebase UID on connection
		}
		hub.register(conn)
		defer hub.unregister(conn)
		defer c.CloseNow()

		conn.readLoop(r.Context())
	}
}
```

#### Pattern B: First message post-upgrade is `AuthRequest{id_token}`
Less preferred (adds message dispatch overhead before auth, potential DDoS vector).

**Choose Pattern A:** Token validation happens at HTTP upgrade, prevents any authenticated message handling until verified.

---

## 3. Modified Conn Type (Auth-Aware)

```go
type Conn struct {
	ws     *websocket.Conn
	hub    *Hub
	userID string  // Firebase UID; "" means guest/unauthenticated
}

func (c *Conn) handle(ctx context.Context, data []byte) error {
	var env dleaguev1.Envelope
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}

	// Gate dispatch: some messages require auth
	if !c.isAuthenticated() && requiresAuth(&env) {
		return fmt.Errorf("message requires authentication")
	}

	resp, err := c.hub.dispatch(&env, time.Now().UnixMilli(), c.userID)
	if err != nil {
		return err
	}
	if resp == nil {
		return nil
	}

	out, err := proto.Marshal(resp)
	if err != nil {
		return err
	}
	logSend(resp)

	writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return c.ws.Write(writeCtx, websocket.MessageBinary, out)
}

func (c *Conn) isAuthenticated() bool {
	return c.userID != ""
}
```

---

## 4. Token Refresh & Re-auth for Long-Lived Connections

### Problem
ID tokens expire in **1 hour**. For PvP matches lasting >1 hour, connection becomes invalid mid-game.

### Solution: Client-Driven Refresh

**Client side (Svelte, via Firebase JS SDK):**
```typescript
import { getAuth, signInWithEmailAndPassword, onAuthStateChanged } from "firebase/auth";

const auth = getAuth();
let currentIdToken: string;

onAuthStateChanged(auth, async (user) => {
	if (user) {
		currentIdToken = await user.getIdToken();
		// Send to server via WS AuthRefresh message every 50 min
		scheduleRefresh();
	}
});

async function scheduleRefresh() {
	setInterval(async () => {
		const newToken = await auth.currentUser.getIdToken(true);
		// Send AuthRefresh message to server
		ws.send(encodeMessage({ type: "auth_refresh", id_token: newToken }));
	}, 50 * 60_000);
}
```

**Server side (in `conn.handle()` dispatch):**
```go
case dleaguev1.MessageType_MESSAGE_TYPE_AUTH_REFRESH:
	var authMsg dleaguev1.AuthRefresh
	if err := proto.Unmarshal(env.Payload, &authMsg); err != nil {
		return err
	}

	// Re-verify new token
	newToken, err := hub.authClient.VerifyIDToken(ctx, authMsg.IdToken)
	if err != nil {
		return fmt.Errorf("token refresh failed: %w", err)
	}

	// Update connection's user context
	c.userID = newToken.UID
	c.tokenExpiresAt = time.Unix(newToken.Expires, 0)

	// Send confirmation
	return sendAuthRefreshAck(c, newToken.Expires)
```

**New protobuf messages:**
```protobuf
message AuthRefresh {
	string id_token = 1;
}

message AuthRefreshAck {
	int64 expires_at_unix = 1;
}
```

---

## 5. User Identity → MongoDB Auto-Create Pattern

### Schema
```go
// MongoDB users collection
type User struct {
	ID        string    `bson:"_id"`  // Firebase UID (canonical ID)
	Email     string    `bson:"email"`
	DisplayName string  `bson:"display_name"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
	// Avatar, premium flag, etc. added in Phase 4
}
```

### First-Login Auto-Create
On every authenticated connection, check if user exists; create if missing:

```go
// In dispatcher, after token verified
func (h *Hub) ensureUserExists(ctx context.Context, token *auth.Token) error {
	userID := token.UID
	
	// Check if user exists
	existingUser := h.userStore.GetUser(ctx, userID)
	if existingUser != nil {
		return nil // Already created
	}

	// First login: create user doc
	newUser := &User{
		ID:        userID,
		Email:     token.Claims["email"].(string),
		DisplayName: token.Claims["name"].(string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return h.userStore.CreateUser(ctx, newUser)
}
```

**Never store passwords:** Firebase owns auth; user docs are identity + profile data only.

---

## 6. Frontend Integration (Svelte)

### Sign-In Options (MVP Phase 3)

#### (a) Email/Password (mandatory)
```svelte
<script>
	import { auth } from "./firebase-config";
	import { signInWithEmailAndPassword } from "firebase/auth";

	async function login(email, password) {
		try {
			const { user } = await signInWithEmailAndPassword(auth, email, password);
			const idToken = await user.getIdToken();
			connectWS(idToken);
		} catch (error) {
			console.error(error);
		}
	}
</script>

<form on:submit|preventDefault={() => login(email, password)}>
	<input bind:value={email} type="email" />
	<input bind:value={password} type="password" />
	<button type="submit">Sign In</button>
</form>
```

#### (b) Google Sign-In (recommended for friction reduction)
```svelte
<script>
	import { GoogleAuthProvider, signInWithPopup } from "firebase/auth";

	async function signInWithGoogle() {
		const provider = new GoogleAuthProvider();
		const { user } = await signInWithPopup(auth, provider);
		const idToken = await user.getIdToken();
		connectWS(idToken);
	}
</script>

<button on:click={signInWithGoogle}>Sign in with Google</button>
```

#### (c) Anonymous Auth (guest play — YES, recommended)
**Pattern:** Allow unauthenticated guests to play local leaderboards; offer "sign in to save score" upgrade path post-game.

```svelte
<script>
	import { signInAnonymously } from "firebase/auth";

	async function playAsGuest() {
		const { user } = await signInAnonymously(auth);
		const idToken = await user.getIdToken();
		connectWS(idToken);  // Still sends token; server treats as guest (can detect via claims)
	}
</script>

<button on:click={playAsGuest}>Play as Guest</button>
```

**Server-side detection of anonymous user:**
```go
// In token.Claims
if claims["firebase"].(map[string]interface{})["sign_in_provider"] == "anonymous" {
	c.userID = token.UID
	c.isAnonymous = true
}
```

---

## 7. Security Considerations

### Token Verification
- **Standard case (MVP):** Use `authClient.VerifyIDToken(ctx, idToken)`
  - Validates signature (RS256), `exp`, `iat`, `aud` (project ID), `iss` (securetoken.google.com)
  - **Does NOT check revocation** (acceptable for MVP; user session only revoked on logout)

- **Revocation support (Phase 4+):** Use `authClient.VerifyIDTokenAndCheckRevoked(ctx, idToken)`
  - Fetches user from Firebase to check `UserRecord.Disabled` or session revocation
  - ~100ms latency per call; rate-limit on high-traffic connections

### Admin/Role Claims (Phase 4)
Custom claims set via Admin SDK propagate to ID token:

```go
// Server: set custom claim (e.g., during user creation or admin action)
claims := map[string]interface{}{
	"admin": true,
	"role": "moderator",
}
err := authClient.SetCustomUserClaims(ctx, userID, claims)

// In token verification, access custom claims:
if token.Claims["admin"] == true {
	// Admin-only action
}
```

### Rate-Limiting `VerifyIDToken` Calls
Firebase has quotas (~20K verify/min free tier). For high-throughput:
- Cache verification result per token (expires_at - 10sec buffer)
- Or verify only at connection upgrade, validate refresh tokens client-side before sending

### Token Leakage
- **In headers:** Use HTTPS always (Fly.io enforces)
- **In logs:** Never log full tokens; log hash or suffix only
- **In WASM:** Token stays in browser memory; WASM can't access if sandboxed properly

---

## 8. Firebase Auth Free Tier & Pricing

### Free Tier (Blaze plan minimum)
- **50,000 Monthly Active Users (MAU)** — no cost
- **Email/Password + Google + Apple + Microsoft + Twitter/GitHub/Yahoo** included
- SMS auth charges separately ($0.02/SMS)
- **Definition:** One MAU = one unique user who signs in or is created in a calendar month

### Pricing Beyond 50K MAU
- **$0.0055 per MAU** ($5.50 per 1,000 users)
- SMS: separate charge, varies by region

**For Dleague MVP:** Assume <5K DAU → <50K MAU → **$0.** Lock in free tier unless mobile launch explosively grows player base.

---

## 9. Local Development — Firebase Emulator

### Setup
```bash
# One-time install
npm install -g firebase-tools
firebase init emulators --project dleague-dev

# In your firebase.json:
{
  "emulators": {
    "auth": {
      "host": "127.0.0.1",
      "port": 9099
    }
  }
}
```

### Run Emulator
```bash
firebase emulators:start --only auth
```

This starts a local auth service on `127.0.0.1:9099` + Emulator Suite UI on `127.0.0.1:4000`.

### Server Configuration (Go)
```go
// In config, check for emulator env var:
if emulatorHost := os.Getenv("FIREBASE_AUTH_EMULATOR_HOST"); emulatorHost != "" {
	// Emulator mode: no GOOGLE_APPLICATION_CREDENTIALS needed
	log.Printf("Using Firebase emulator at %s", emulatorHost)
	// SDK auto-routes to emulator when env var is set
}
```

### Client Configuration (Svelte)
```typescript
import { getAuth, connectAuthEmulator } from "firebase/auth";
const auth = getAuth();

if (import.meta.env.DEV) {
	connectAuthEmulator(auth, "http://127.0.0.1:9099");
}
```

### CI/CD Integration
```yaml
# .github/workflows/test.yml
- name: Start Firebase emulator
  run: firebase emulators:start --only auth &
  
- name: Run Go tests
  env:
    FIREBASE_AUTH_EMULATOR_HOST: 127.0.0.1:9099
  run: go test ./...
```

**Critical:** Emulator must omit `http://` in `FIREBASE_AUTH_EMULATOR_HOST` env var (unlike client SDK).

---

## 10. Migration Path: Session Cookies → Firebase ID Tokens

### Current State (Phase 1)
- Single WS endpoint (`/ws`)
- No auth gate; all connections treated as unauthenticated
- Message dispatch ignores identity

### Phase 3 Changes
1. Add `Conn.userID` field (Firebase UID or empty)
2. Wrap `UpgradeHandler` to verify ID token from `Sec-WebSocket-Protocol` header
3. Gate dispatch: some messages require `conn.isAuthenticated()`
4. Add `AuthRefresh` message type for 1-hour token refresh
5. Auto-create user doc on first login

### No Breaking Changes
- Clients send `Sec-WebSocket-Protocol` during upgrade (HTTP-level, transparent)
- Message format unchanged (existing `Envelope` works as-is)
- **Anonymous play supported:** Guests can still connect (empty `userID` → guest mode)

---

## Adoption Risk & Constraints

| Dimension | Assessment | Mitigation |
|-----------|-----------|-----------|
| **Maturity** | Firebase Auth v1 stable; Admin SDK Go v4 mature (v4.19+) | Use released versions; follow official guides |
| **Team skill** | Requires Firebase project setup, service account creds mgmt | Document emulator-first dev; share creds template in oncall |
| **Latency** | VerifyIDToken ~5-10ms; `VerifyIDTokenAndCheckRevoked` ~100ms | Use simple `VerifyIDToken` MVP; cache or defer revocation to Phase 4 |
| **Dependency** | Hard dependency on Google Firebase API (outage = auth broken) | Emulator + cached tokens in browser mitigate; fallback login retry |
| **Config management** | Service account JSON is secret; CI/CD needs secure storage | Use GitHub secrets + Fly.io secret env vars; never commit JSON |
| **Browser security** | ID token in memory; WASM <-> JS boundary attack surface | Validate CORS headers; token stays in top-level JS scope |

---

## Implementation Checklist (Phase 3)

- [ ] Create Firebase project (or reuse existing)
- [ ] Generate service account JSON; commit reference to `.gitignore`
- [ ] Add `firebase.google.com/go/v4` to `server/go.mod`
- [ ] Add `auth.Client` field to `Hub`; initialize in `main.go`
- [ ] Update `Conn` struct: add `userID`, `tokenExpiresAt`, `isAnonymous` fields
- [ ] Update `UpgradeHandler`: extract & verify `Sec-WebSocket-Protocol` token
- [ ] Add `AuthRefresh` message type to protobuf schema
- [ ] Update `dispatch()` to accept `userID` parameter
- [ ] Gate messages: `requiresAuth()` check before dispatch
- [ ] Implement user auto-create: `ensureUserExists()` on first message
- [ ] Add to `schema/user.go` (Mongo/Postgres): Firebase UID as `_id`
- [ ] Svelte: Add Firebase JS SDK sign-in (Email/Password, Google, Anonymous)
- [ ] Setup Firebase emulator for CI/CD
- [ ] Test: E2E sign-in → token verify → auth refresh → re-auth
- [ ] Docs: How to obtain service account, set `GOOGLE_APPLICATION_CREDENTIALS` locally

---

## Unresolved Questions

1. **Session durability across reconnects:** If client drops mid-match and reconnects, does server keep `Conn.userID` in match state, or reset? (Answer: Phase 4 durability; MVP resets on drop.)
2. **Refreshing at exact expiry:** Should client preempt refresh (50 min), or only on server's `TokenExpiringSoon` message? (Recommend both: client proactive, server defensive.)
3. **Custom claims for game roles:** Do we need `player`, `moderator`, `tester` roles in Phase 3, or defer to Phase 4+? (Recommend defer; anonymous + user ID sufficient for MVP.)
4. **Fallback auth for emulator CI:** If Firebase emulator crashes during test, does test fail hard or skip auth? (Recommend fail fast; CI must be reliable.)
5. **Cost tracking:** Any tooling to monitor MAU growth & alert before hitting paid tier? (Firebase console shows graphs; defer automated alerting to later.)

---

**Status:** DONE

**Summary:** Firebase Admin SDK provides `auth.Client.VerifyIDToken(ctx, idToken)` for ID token verification at WS upgrade via `Sec-WebSocket-Protocol` header. Token refresh every 50 min via client-sent `AuthRefresh` message; server re-verifies and updates `Conn.userID`. First-login auto-creates MongoDB user doc keyed by Firebase UID. Frontend: Svelte integrates `firebase/auth` JS SDK with Email/Password (mandatory), Google (recommended), and Anonymous (yes—friction-free guest play). Emulator suite (`firebase emulators:start --only auth` + `FIREBASE_AUTH_EMULATOR_HOST=127.0.0.1:9099`) enables local dev without prod Firebase. Free tier: 50K MAU at no cost. No breaking changes to WS protocol; migration is HTTP-level only.
