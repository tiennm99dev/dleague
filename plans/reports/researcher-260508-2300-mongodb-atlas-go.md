# MongoDB Atlas + Go Driver Research
**Date:** 2026-05-08 | **For:** Dleague server rewrite (MySQL HeatWave → Mongo)

---

## 1. Driver Choice: `go.mongodb.org/mongo-driver/v2`

**Status:** Recommended. Latest stable version (v2.6.0+), requires Go 1.19+, MongoDB 4.2+.

### Installation & Basic Setup
```bash
go get go.mongodb.org/mongo-driver/v2/mongo
go get go.mongodb.org/mongo-driver/v2/mongo/options
```

### Client Initialization
```go
import (
    "context"
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
    "time"
)

// SRV connection string (Atlas recommended)
uri := "mongodb+srv://user:pass@cluster.mongodb.net/?retryWrites=true&w=majority"

client, err := mongo.Connect(options.Client().ApplyURI(uri))
defer client.Disconnect(context.Background())

// Verify connectivity on startup
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
err = client.Ping(ctx, nil)
```

### Connection Pooling Defaults
- **Min pool size:** 10 (auto-created at Connect)
- **Max pool size:** 100
- **Max idle time:** 30 seconds (connections dropped if unused)
- **Max connecting:** 2 (in-flight connections)

**Recommended for Atlas:**
```go
opts := options.Client().
    ApplyURI(uri).
    SetMaxPoolSize(100).
    SetMinPoolSize(10).
    SetMaxConnIdleTime(30 * time.Second).
    SetConnectTimeout(10 * time.Second).
    SetServerSelectionTimeout(5 * time.Second)
```

### Timeout Guidance
- **ConnectTimeout:** 10s (Atlas network latency + handshake)
- **ServerSelectionTimeout:** 5s (topology discovery, helps prevent hanging on connection failures)
- **Context per-operation:** Use `context.WithTimeout(ctx, 5*time.Second)` for CRUD operations

### Connection Lifecycle
1. `mongo.Connect()` → initializes pool, does NOT block for server discovery
2. `client.Ping(ctx)` → verify connectivity on startup (required check)
3. `client.Disconnect(ctx)` → graceful shutdown, drains pool
4. Create **one client per application**; share across handlers

### SRV Connection String Format
```
mongodb+srv://username:password@cluster.mongodb.net/database?retryWrites=true&w=majority
```
- DNS SRV record auto-discovers all replica set members
- No need to list individual host:port pairs
- Handles topology changes (new shards) without code changes
- **Important:** Password must be URL-encoded (`:`, `@`, etc. → `%3A`, `%40`)

---

## 2. MongoDB Atlas Free Tier (M0)

**Verdict:** Viable for MVP/dev, but tight for production. 10K MAU → upgrade to M10 or Flex.

### M0 Free Tier Specifications
| Limit | Value |
|-------|-------|
| **Storage** | 512 MB |
| **Connections** | 500 max |
| **Replication** | 3 nodes (fixed, no configuration) |
| **Throughput** | 100 ops/sec |
| **Data Transfer** | 10 GB in / 10 GB out per 7-day period |
| **Databases** | 100 max |
| **Collections** | 500 total |
| **In-Memory Sort Limit** | 32 MB (aggregations MUST fit in RAM) |
| **Indexes** | Unlimited, but constrained by storage |
| **Backups** | Not supported |
| **Sharding** | Not supported |
| **Encryption at Rest** | Not supported |
| **Private Link / VPC** | Not supported |

### Atlas-Specific Gotchas

**TLS Requirement:**
- M0 **requires** TLS 1.2+. Connection strings auto-append `?ssl=true`
- No option to disable; all traffic encrypted
- Driver handles transparently

**IP Allowlisting:**
- Must whitelist Fly.io IP ranges or use `0.0.0.0/0` (dev/staging only)
- For production: use Fly.io issuer-based allowlist or private endpoint (paid tier only)
- **Fly.io workaround:** IP can change; consider using `0.0.0.0/0` for dev, upgrade to M10 + VPC for prod

**Serverless vs Dedicated:**
- M0 is **shared cluster** (not serverless/Flex)
- Dedicated M10 available; no "shared M5" tier
- Flex Tier (newer, ~$8 base + usage) replaces deprecated serverless

**Pause & Reactivation:**
- M0 clusters auto-pause after 30 days of inactivity
- Restart causes 1-2 minute warm-up
- Dev-only acceptable; not for production

**Performance Baseline (Realistic for Dleague):**
- 5 games/day × 10K MAU = 50K game records/day
- If avg 5 attempts/game → 250K attempt records/day
- At 100 ops/sec, M0 handles ~8.6M ops/day (headroom ~17x)
- **But:** Concurrent spike during peak hours could exceed 100 ops/sec
- **Verdict:** M0 works for MVP (off-peak development), but upgrade to M10 for production

### Region Pairing with Fly.io
- Fly.io regions: SFO, IAD, LAX, LHR, CDG, AMS, NRT, SYD, SIN, MXP, GRU, etc.
- Atlas M0: Single region (chosen at creation, cannot change)
- **Recommendation:** Choose Atlas region closest to primary Fly.io deployment (e.g., SFO for US West, IAD for US East)
- Multi-region failover requires M30+ (not applicable to M0)

---

## 3. Document Schema Design

### Core Collections & Indexes

**`users` collection**
```json
{
  "_id": "firebase_uid_string",
  "display_name": "string",
  "avatar_url": "string",
  "created_at": "ISO8601",
  "stats": {
    "wins": 42,
    "losses": 18,
    "current_streak": 3,
    "total_games": 60
  },
  "verified": true,
  "last_login": "ISO8601"
}
```

**`games` collection** (game type registry)
```json
{
  "_id": "wordle",  // or ObjectId if prefer auto-generated
  "type": "wordle",
  "display_name": "Wordle",
  "active": true,
  "config": {
    "attempts_max": 6,
    "word_length": 5
  },
  "created_at": "ISO8601"
}
```

**`matches` collection**
```json
{
  "_id": ObjectId,
  "game_id": "wordle",
  "players": ["uid1", "uid2"],
  "mode": "sync",  // "sync" | "async"
  "state": "active",  // "pending" | "active" | "complete"
  "winner_uid": "uid1",  // null if in progress
  "created_at": "ISO8601",
  "ended_at": "ISO8601",  // null if in progress
  "seed": 12345,
  "metadata": {}
}
```

**`attempts` collection** (per-player guesses in a match)
```json
{
  "_id": ObjectId,
  "match_id": ObjectId,
  "player_uid": "uid1",
  "attempts": ["SLATE", "ROUTE", "SPORT"],
  "time_ms": 125000,  // total time to solve
  "result": "win",  // "win" | "loss" | "timeout"
  "created_at": "ISO8601"
}
```

**`daily_puzzles` collection**
```json
{
  "_id": "2026-05-08",  // date string; unique per day
  "game_id": "wordle",
  "seed": 67890,
  "solution_hash": "sha256_hash",  // don't store plaintext
  "difficulty": "medium",
  "created_at": "ISO8601"
}
```

**`leaderboards` collection** (pre-computed, refreshed daily)
```json
{
  "_id": "wordle_global_20260508",  // date-scoped leaderboard
  "game_id": "wordle",
  "period": "weekly",  // "daily" | "weekly" | "alltime"
  "period_end": "ISO8601",
  "rankings": [
    { "rank": 1, "uid": "uid1", "score": 450, "games_played": 60 },
    { "rank": 2, "uid": "uid2", "score": 440, "games_played": 62 }
  ],
  "updated_at": "ISO8601"
}
```

### Schema Patterns Applied
- **Embedding** (`users.stats`) for "one-to-one" tightly-coupled data
- **Referencing** (`matches.players` array, `matches.game_id`) for "one-to-many" relationships
- **Document per-record** (attempts) for high-volume, immutable event log
- **Pre-computed materialized view** (leaderboards) to avoid expensive aggregations on read

---

## 4. Critical Indexes

**Auto-generated by MongoDB:**
- `users._id` (primary key)
- `games._id`
- `matches._id`
- `attempts._id`

**Must create explicitly:**

```go
// In migration/setup.go
indexes := []mongo.IndexModel{
    // users collection
    {
        Keys: bson.D{{Key: "display_name", Value: 1}},
        Options: options.Index().SetUnique(true),
    },

    // matches collection
    {
        Keys: bson.D{{Key: "players", Value: 1}},  // Array field query
    },
    {
        Keys: bson.D{{Key: "created_at", Value: -1}},  // Sort by recency
    },
    {
        Keys: bson.D{
            {Key: "state", Value: 1},
            {Key: "created_at", Value: -1},
        },  // ESR rule: state (equality), created_at (sort)
    },

    // attempts collection (high-volume)
    {
        Keys: bson.D{{Key: "match_id", Value: 1}},  // Find all attempts in a match
    },
    {
        Keys: bson.D{
            {Key: "match_id", Value: 1},
            {Key: "player_uid", Value: 1},
        },  // Lookup one player's attempts in a match
    },

    // daily_puzzles collection
    {
        Keys: bson.D{{Key: "_id", Value: -1}},  // Date range queries (recent puzzles)
    },

    // leaderboards collection
    {
        Keys: bson.D{
            {Key: "game_id", Value: 1},
            {Key: "period_end", Value: -1},
        },
    },
}

// Create all at once
_, err := collection.Indexes().CreateMany(ctx, indexes)
```

**Why these indexes?**
- `players` array index allows fast queries like `{players: uid}` for "find all matches user participated in"
- `match_id` compound index for attempts enables single-document lookup + sort optimization
- `state + created_at` compound follows **ESR rule** (Equality, Sort, Range)

---

## 5. Transactions for PvP Atomicity

**Requirement:** When a sync PvP match ends, atomically record winner + update both players' stats.

**Atlas M0 limitation:** Free tier **does** support transactions (requires 3-node replica set, which M0 has by default). ✓

**Transaction pattern (Go driver):**

```go
import "go.mongodb.org/mongo-driver/v2/mongo"

func RecordMatchResult(client *mongo.Client, matchID, winnerUID string) error {
    session, err := client.StartSession()
    if err != nil {
        return err
    }
    defer session.EndSession(context.Background())

    err = session.WithTransaction(context.Background(), func(ctx mongo.SessionContext) error {
        // 1. Update match as complete
        _, err := matchCollection.UpdateOne(ctx,
            bson.M{"_id": matchID},
            bson.M{"$set": bson.M{"state": "complete", "winner_uid": winnerUID, "ended_at": time.Now()}},
        )
        if err != nil {
            return err
        }

        // 2. Increment winner stats
        _, err = userCollection.UpdateOne(ctx,
            bson.M{"_id": winnerUID},
            bson.M{"$inc": bson.M{"stats.wins": 1}},
        )
        if err != nil {
            return err
        }

        // 3. Increment loser stats
        loserUID := getOtherPlayer(...)  // fetch from match doc
        _, err = userCollection.UpdateOne(ctx,
            bson.M{"_id": loserUID},
            bson.M{"$inc": bson.M{"stats.losses": 1}},
        )
        return err
    })
    return err
}
```

**Key points:**
- `session.WithTransaction()` handles retry logic automatically
- Default transaction options: `w: "majority"`, `readPreference: primary`
- All operations must pass `ctx` (type: `mongo.SessionContext`)
- Rollback is automatic if any operation returns error
- **Cost:** Transactions are ~10-15% overhead; use sparingly (only for critical atomicity needs)

---

## 6. Migration Strategy

**Why no SQL-style schema migrations?**
Mongo is schemaless. Documents can have different fields. Two approaches:

### Option A: Lazy Migration on Read (Simplest for MVP)
- Store `schema_version: 1` in each document
- On read, check version; if old, transform in memory and save updated doc
- **Pros:** No downtime, no bulk operations, handles partial migration
- **Cons:** Distributed logic, eventual consistency

```go
func (r *UserRepo) GetUser(ctx context.Context, uid string) (*User, error) {
    var doc bson.M
    err := r.coll.FindOne(ctx, bson.M{"_id": uid}).Decode(&doc)
    if err != nil {
        return nil, err
    }
    
    // Migrate if old schema
    if v, ok := doc["schema_version"].(int32); !ok || v < 2 {
        doc["avatar_url"] = doc["avatar_url"].(string) + "?size=large"  // add default param
        doc["schema_version"] = 2
        r.coll.UpdateOne(ctx, bson.M{"_id": uid}, bson.M{"$set": doc})
    }
    
    var user User
    // ... unmarshal doc to User
    return &user, nil
}
```

### Option B: One-Time Bulk Migration Script
- Run once during deployment
- Use `collection.UpdateMany()` with `$set`, `$rename`, `$unset`
- **Pros:** Schema is uniform immediately, no read-time overhead
- **Cons:** Needs downtime or careful timing, more risky

```go
// cmd/migrate/main.go
_, err := usersCollection.UpdateMany(ctx,
    bson.M{},  // all docs
    bson.A{
        bson.M{"$set": bson.M{"schema_version": 2}},
        bson.M{"$rename": bson.M{"old_field": "new_field"}},
    },
)
```

**Recommendation for Dleague:** Option A (lazy migration). Lowest risk, no deployment downtime.

---

## 7. Local Development Setup

**docker-compose.yml** (Go app + Mongo + Mongo Express):
```yaml
version: "3.8"

services:
  mongodb:
    image: mongo:7
    restart: always
    environment:
      MONGO_INITDB_ROOT_USERNAME: admin
      MONGO_INITDB_ROOT_PASSWORD: admin
    ports:
      - "27017:27017"
    volumes:
      - mongo_data:/data/db
    healthcheck:
      test: echo 'db.runCommand("ping").ok' | mongosh localhost:27017/test -u admin -p admin
      interval: 10s
      timeout: 5s
      retries: 5

  mongo-express:
    image: mongo-express
    restart: always
    ports:
      - "8081:8081"
    environment:
      ME_CONFIG_MONGODB_ADMINUSERNAME: admin
      ME_CONFIG_MONGODB_ADMINPASSWORD: admin
      ME_CONFIG_MONGODB_URL: mongodb://admin:admin@mongodb:27017/
    depends_on:
      - mongodb

  # Optional: Go app service
  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      MONGO_URI: mongodb://admin:admin@mongodb:27017
      GO_ENV: development
    depends_on:
      - mongodb

volumes:
  mongo_data:
```

**Local connection string:**
```
mongodb://admin:admin@localhost:27017/?authSource=admin
```

**Mongo Express UI:** http://localhost:8081 (default username/password: admin/admin)

**Production (Fly.io → Atlas):**
```
MONGO_URI=mongodb+srv://dleague_user:${DB_PASSWORD}@cluster.mongodb.net/dleague?retryWrites=true&w=majority
```

Use environment variables; never hardcode credentials.

---

## 8. Repository Pattern (Recommended)

**Structure: Per-collection repository, dependency injection**

```go
// store/user_repo.go
package store

import (
    "context"
    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
)

type UserRepo struct {
    coll *mongo.Collection
}

func NewUserRepo(db *mongo.Database) *UserRepo {
    return &UserRepo{
        coll: db.Collection("users"),
    }
}

func (r *UserRepo) CreateUser(ctx context.Context, user *User) error {
    _, err := r.coll.InsertOne(ctx, user)
    return err
}

func (r *UserRepo) GetUser(ctx context.Context, uid string) (*User, error) {
    var user User
    err := r.coll.FindOne(ctx, bson.M{"_id": uid}).Decode(&user)
    if err == mongo.ErrNoDocuments {
        return nil, nil  // User not found; handle gracefully
    }
    return &user, err
}

func (r *UserRepo) IncrementWins(ctx context.Context, uid string) error {
    _, err := r.coll.UpdateOne(ctx,
        bson.M{"_id": uid},
        bson.M{"$inc": bson.M{"stats.wins": 1}},
    )
    return err
}
```

**Struct definition:**
```go
// store/models.go
type User struct {
    ID           string `bson:"_id"`
    DisplayName  string `bson:"display_name"`
    AvatarURL    string `bson:"avatar_url"`
    CreatedAt    time.Time `bson:"created_at"`
    Stats        Stats `bson:"stats"`
}

type Stats struct {
    Wins         int `bson:"wins"`
    Losses       int `bson:"losses"`
    CurrentStreak int `bson:"current_streak"`
}
```

**Benefits:**
- One repo per entity (single responsibility)
- All methods accept `context.Context` (cancellation, timeouts)
- Testable (mock `*mongo.Collection` with interfaces)
- No God-object store pattern

---

## 9. Cost Projections

### M0 Free Tier → M10 Upgrade Path

**M0 Limits:** 100 ops/sec, 512 MB storage, 500 connections
- Suitable for: dev, MVP with <1K DAU
- **Cost:** $0

**M10 Dedicated Cluster**
- **Cost:** ~$57–65/month (AWS, US regions)
- **Throughput:** 1,500 ops/sec per node (15x more headroom)
- **Storage:** 10 GB included (scale to TB)
- **Connections:** 1,500 per node
- **Backups:** Daily automated snapshots included
- **Sharding:** Available if needed (M30+)

**Flex Tier (Newer, Hybrid Pricing)**
- **Base:** ~$8–10/month
- **Usage:** Per read/write unit + storage overage
- **RPU (Read Processing Unit):** $0.10 per million (first 50M/day tier)
- **WPU (Write Processing Unit):** $1.00 per million
- **Storage:** $0.25 per GB-month beyond 100 GB free

### Cost for 10K MAU @ 5 games/day scenario

**Metrics:**
- 10K MAU × 5 games/day = 50K games/day
- ~250K records created/day (attempts, matches, leaderboard refresh)
- Assume 30% read amplification (stats lookups, leaderboards, game history) → ~325K total ops/day
- Peak hour: ~15K ops/hour = 4.2 ops/sec (well under M0's 100 ops/sec)

**Cost estimate:**

| Tier | Monthly Cost | Headroom | Notes |
|------|--------------|----------|-------|
| M0 | $0 | 50x | MVP only; no backups, auto-pause risk |
| M10 | $57–65 | 300x | Production-ready, backups, SLA |
| Flex | $8 + variable | ~100x (base) | If truly sparse traffic, could be cheaper |

**Recommendation:** Start M0 (dev), upgrade to M10 (production). Flex is not cost-effective for this workload; overhead on WPU is high.

---

## Unresolved Questions

1. **PvP match timeout handling:** How long before an "active" sync match times out → loser assigned? Affects transaction lifecycle and cleanup jobs.
2. **Leaderboard refresh frequency:** Daily, hourly, or on-demand? If daily (1 aggregation pipeline/day), pre-computed collection is best; if real-time, may need on-the-fly aggregation with caching layer.
3. **Async challenge storage:** Are "async" match attempts stored separately, or same `attempts` collection with mode flag? Affects indexing strategy.
4. **Data retention policy:** Delete old daily_puzzles/leaderboards after N days, or keep forever for historical analysis?
5. **Fly.io IP stability:** Confirmed whether Fly.io NAT IPs are stable enough for IP allowlist, or must use `0.0.0.0/0` + network peering?
6. **Search feature:** Does Dleague need full-text search (user display name, game titles)? If yes, MongoDB Search adds cost; prefer Redis cache for small result sets.
7. **Change Streams for real-time sync:** Do web clients need live leaderboard updates (WebSocket)? Affects whether to use Change Streams for incremental updates.

---

## Summary & Next Steps

✅ **Proceed with:** `go.mongodb.org/mongo-driver/v2`, M0 free tier (dev), migrate to M10 (production)

✅ **Implementation order:**
1. Create `server/internal/store/client.go` → singleton mongo client
2. Create per-collection repos (`user_repo.go`, `match_repo.go`, `attempt_repo.go`, etc.)
3. Create `server/internal/store/schema.go` → index creation on startup
4. Write integration tests against local Mongo (docker-compose)
5. Add env-var toggle between local (mongodb://admin:admin@localhost) and Atlas (mongodb+srv://...)

✅ **Delete old SQL layer:** `server/internal/store/{store.go, migrate.go, users.go}`

---

**Status:** DONE
