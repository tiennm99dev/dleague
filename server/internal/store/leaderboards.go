package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// LeaderboardRepo provides access to the `leaderboards` collection.
type LeaderboardRepo struct {
	coll        *mongo.Collection
	attemptColl *mongo.Collection
	userColl    *mongo.Collection
	matchColl   *mongo.Collection
}

// NewLeaderboardRepo returns a LeaderboardRepo backed by the "leaderboards"
// collection of db.
func NewLeaderboardRepo(db *mongo.Database) *LeaderboardRepo {
	return &LeaderboardRepo{
		coll:        db.Collection("leaderboards"),
		attemptColl: db.Collection("attempts"),
		userColl:    db.Collection("users"),
		matchColl:   db.Collection("matches"),
	}
}

// leaderboardID returns the canonical _id for a leaderboard document.
// Format: "{game}_{period}_{date}" e.g. "wordle_daily_2026-05-09".
func leaderboardID(gameID, period, date string) string {
	return gameID + "_" + period + "_" + date
}

// Refresh rebuilds the leaderboard for the given game, period, and date by
// fetching completed async matches for that date and joining with attempts +
// users in Go (pragmatic alternative to aggregation pipeline; avoids 32 MB
// in-memory sort limit on M0 while keeping the logic readable).
//
// Trade-off documented: a Mongo aggregation pipeline would be more efficient
// at scale (single round-trip), but Go-side join is simpler and adequate for
// MVP at <10K matches/day. Migrate to $lookup + $sort pipeline in Phase 10.
func (r *LeaderboardRepo) Refresh(ctx context.Context, gameID, period, date string) error {
	// Step 1: find all completed async matches for this date window.
	// We use the date portion of completed_at to scope the daily board.
	dayStart := mustParseDate(date)
	dayEnd := dayStart.Add(24 * time.Hour)

	matchFilter := bson.M{
		"game_id": gameID,
		"mode":    "async",
		"state":   "complete",
		"completed_at": bson.M{
			"$gte": dayStart,
			"$lt":  dayEnd,
		},
	}
	matchCur, err := r.matchColl.Find(ctx, matchFilter, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return fmt.Errorf("store: leaderboard refresh find matches: %w", err)
	}
	defer func() { _ = matchCur.Close(ctx) }()

	type matchIDDoc struct {
		ID bson.ObjectID `bson:"_id"`
	}
	var matchDocs []matchIDDoc
	if err := matchCur.All(ctx, &matchDocs); err != nil {
		return fmt.Errorf("store: leaderboard refresh decode matches: %w", err)
	}
	if len(matchDocs) == 0 {
		// Nothing to refresh; upsert empty rankings so Get returns a doc.
		return r.upsert(ctx, Leaderboard{
			ID:            leaderboardID(gameID, period, date),
			GameID:        gameID,
			Period:        period,
			Date:          date,
			Rankings:      []LeaderboardRow{},
			RefreshedAt:   time.Now().UTC(),
			SchemaVersion: currentSchemaVersion,
		})
	}

	matchIDs := make([]bson.ObjectID, 0, len(matchDocs))
	for _, d := range matchDocs {
		matchIDs = append(matchIDs, d.ID)
	}

	// Step 2: fetch all attempts for those matches.
	attemptCur, err := r.attemptColl.Find(ctx, bson.M{"match_id": bson.M{"$in": matchIDs}})
	if err != nil {
		return fmt.Errorf("store: leaderboard refresh find attempts: %w", err)
	}
	defer func() { _ = attemptCur.Close(ctx) }()

	var attempts []Attempt
	if err := attemptCur.All(ctx, &attempts); err != nil {
		return fmt.Errorf("store: leaderboard refresh decode attempts: %w", err)
	}

	// Step 3: collect unique player UIDs and fetch user docs (need is_anonymous + display_name).
	uidSet := map[string]struct{}{}
	for _, a := range attempts {
		uidSet[a.PlayerUID] = struct{}{}
	}
	uids := make([]string, 0, len(uidSet))
	for uid := range uidSet {
		uids = append(uids, uid)
	}

	type userDoc struct {
		ID          string `bson:"_id"`
		DisplayName string `bson:"display_name"`
		IsAnonymous bool   `bson:"is_anonymous"`
	}
	userCur, err := r.userColl.Find(ctx,
		bson.M{"_id": bson.M{"$in": uids}},
		options.Find().SetProjection(bson.M{"_id": 1, "display_name": 1, "is_anonymous": 1}),
	)
	if err != nil {
		return fmt.Errorf("store: leaderboard refresh find users: %w", err)
	}
	defer func() { _ = userCur.Close(ctx) }()

	userMap := map[string]userDoc{}
	var userDocs []userDoc
	if err := userCur.All(ctx, &userDocs); err != nil {
		return fmt.Errorf("store: leaderboard refresh decode users: %w", err)
	}
	for _, u := range userDocs {
		userMap[u.ID] = u
	}

	// Step 4: build one row per player; keep best attempt (won first, fewest
	// guesses, fastest time). Anonymous players are excluded.
	bestByUID := map[string]lbRow{}
	for _, a := range attempts {
		u, ok := userMap[a.PlayerUID]
		if !ok || u.IsAnonymous {
			continue
		}
		existing, seen := bestByUID[a.PlayerUID]
		cur := lbRow{
			uid:         a.PlayerUID,
			displayName: u.DisplayName,
			won:         a.Won,
			attempts:    int32(len(a.Guesses)),
			timeMs:      a.TimeMs,
		}
		if !seen || betterAttempt(cur, existing) {
			bestByUID[a.PlayerUID] = cur
		}
	}

	// Step 5: sort rows: won DESC, attempts ASC, time_ms ASC.
	rows := make([]lbRow, 0, len(bestByUID))
	for _, r := range bestByUID {
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		return betterAttempt(rows[i], rows[j])
	})
	if len(rows) > 100 {
		rows = rows[:100]
	}

	rankings := make([]LeaderboardRow, 0, len(rows))
	for i, rr := range rows {
		rankings = append(rankings, LeaderboardRow{
			Rank:        i + 1,
			UID:         rr.uid,
			DisplayName: rr.displayName,
			Attempts:    rr.attempts,
			TimeMs:      rr.timeMs,
		})
	}

	return r.upsert(ctx, Leaderboard{
		ID:            leaderboardID(gameID, period, date),
		GameID:        gameID,
		Period:        period,
		Date:          date,
		Rankings:      rankings,
		RefreshedAt:   time.Now().UTC(),
		SchemaVersion: currentSchemaVersion,
	})
}

// lbRow is a local working type for leaderboard computation.
type lbRow struct {
	uid         string
	displayName string
	won         bool
	attempts    int32
	timeMs      int32
}

// betterAttempt returns true if candidate is better than current
// (won first, fewer guesses, faster).
func betterAttempt(candidate, current lbRow) bool {
	if candidate.won != current.won {
		return candidate.won
	}
	if candidate.attempts != current.attempts {
		return candidate.attempts < current.attempts
	}
	return candidate.timeMs < current.timeMs
}

// Get fetches the leaderboard snapshot for the given game/period/date.
// Returns (nil, nil) when no document exists yet.
func (r *LeaderboardRepo) Get(ctx context.Context, gameID, period, date string) (*Leaderboard, error) {
	id := leaderboardID(gameID, period, date)
	var lb Leaderboard
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&lb)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: leaderboard Get %q: %w", id, err)
	}
	return &lb, nil
}

// upsert writes a leaderboard snapshot, replacing any existing document with
// the same _id.
func (r *LeaderboardRepo) upsert(ctx context.Context, lb Leaderboard) error {
	opts := options.UpdateOne().SetUpsert(true)
	update := bson.M{
		"$set": bson.M{
			"game_id":        lb.GameID,
			"period":         lb.Period,
			"date":           lb.Date,
			"rankings":       lb.Rankings,
			"refreshed_at":   lb.RefreshedAt,
			"schema_version": lb.SchemaVersion,
		},
	}
	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": lb.ID}, update, opts)
	if err != nil {
		return fmt.Errorf("store: leaderboard upsert %q: %w", lb.ID, err)
	}
	return nil
}

// mustParseDate parses "YYYY-MM-DD" as UTC midnight. Panics on bad input;
// callers control the value (always produced by time.Format).
func mustParseDate(date string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", date, time.UTC)
	if err != nil {
		panic(fmt.Sprintf("store: mustParseDate: bad date %q: %v", date, err))
	}
	return t
}
