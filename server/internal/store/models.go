package store

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// currentSchemaVersion is incremented when document shape changes.
// Lazy migration checks this on read (Option A from research §6).
const currentSchemaVersion = 1

// UserStats holds embedded gameplay statistics for a user.
type UserStats struct {
	Wins          int `bson:"wins"`
	Losses        int `bson:"losses"`
	CurrentStreak int `bson:"current_streak"`
	TotalGames    int `bson:"total_games"`
}

// User maps to the `users` collection. _id is the Firebase UID string.
type User struct {
	ID            string    `bson:"_id"`
	DisplayName   string    `bson:"display_name"`
	AvatarURL     string    `bson:"avatar_url"`
	Email         string    `bson:"email,omitempty"` // persisted when present in token (Phase 05 M4)
	CreatedAt     time.Time `bson:"created_at"`
	LastLogin     time.Time `bson:"last_login"`
	Verified      bool      `bson:"verified"`
	IsAnonymous   bool      `bson:"is_anonymous"`
	Stats         UserStats `bson:"stats"`
	SchemaVersion int       `bson:"schema_version"`
}

// GameConfig holds game-type configuration (e.g. attempts_max, word_length).
type GameConfig struct {
	AttemptsMax int `bson:"attempts_max"`
	WordLength  int `bson:"word_length"`
}

// Game maps to the `games` collection. _id is a short slug like "wordle".
type Game struct {
	ID            string     `bson:"_id"`
	Type          string     `bson:"type"`
	DisplayName   string     `bson:"display_name"`
	Active        bool       `bson:"active"`
	Config        GameConfig `bson:"config"`
	CreatedAt     time.Time  `bson:"created_at"`
	SchemaVersion int        `bson:"schema_version"`
}

// Match maps to the `matches` collection.
// Phase-08 async PvP fields added; old Players/Metadata fields kept for
// potential Phase-09 sync mode re-use.
type Match struct {
	ID     bson.ObjectID `bson:"_id,omitempty"`
	GameID string        `bson:"game_id"`
	// Legacy sync field; async challenges use ChallengerUID/ChallengeeUID below.
	Players []string `bson:"players,omitempty"`
	Mode    string   `bson:"mode"`  // "sync" | "async"
	State   string   `bson:"state"` // "pending" | "complete"
	Seed    int64    `bson:"seed"`
	// Async PvP fields (Phase 08).
	ChallengerUID string     `bson:"challenger_uid"`
	ChallengeeUID *string    `bson:"challengee_uid,omitempty"` // nil until joined
	ShareToken    string     `bson:"share_token"`
	WinnerUID     *string    `bson:"winner_uid,omitempty"`
	JoinedAt      *time.Time `bson:"joined_at,omitempty"`
	CompletedAt   *time.Time `bson:"completed_at,omitempty"`
	ExpiresAt     time.Time  `bson:"expires_at"`
	Metadata      bson.M     `bson:"metadata,omitempty"`
	CreatedAt     time.Time  `bson:"created_at"`
	SchemaVersion int        `bson:"schema_version"`
}

// Attempt maps to the `attempts` collection.
// One document per player per match.
type Attempt struct {
	ID            bson.ObjectID `bson:"_id,omitempty"`
	MatchID       bson.ObjectID `bson:"match_id"`
	PlayerUID     string        `bson:"player_uid"`
	Guesses       []string      `bson:"guesses"`         // word guesses in order
	Hints         [][]int32     `bson:"hints,omitempty"` // per-letter color codes
	TimeMs        int32         `bson:"time_ms"`         // milliseconds to solve
	Won           bool          `bson:"won"`
	Mode          string        `bson:"mode"` // "async" | "sync"
	CreatedAt     time.Time     `bson:"created_at"`
	SchemaVersion int           `bson:"schema_version"`
}

// DailyPuzzle maps to the `daily_puzzles` collection.
// _id is a date string "YYYY-MM-DD".
// Solution is stored in Mongo so the server can re-derive the answer after
// restart without recomputing from the seed. It is NEVER sent to clients
// until the game reaches a terminal state.
type DailyPuzzle struct {
	ID            string    `bson:"_id"` // "YYYY-MM-DD"
	GameID        string    `bson:"game_id"`
	Seed          int64     `bson:"seed"`
	Solution      string    `bson:"solution"`      // server-only; never sent pre-terminal
	SolutionHash  string    `bson:"solution_hash"` // sha256(solution) for audit
	Difficulty    string    `bson:"difficulty"`
	CreatedAt     time.Time `bson:"created_at"`
	SchemaVersion int       `bson:"schema_version"`
}

// LeaderboardRow is one entry in a leaderboard snapshot.
// Renamed from LeaderboardRanking to match Phase-08 proto naming.
type LeaderboardRow struct {
	Rank        int    `bson:"rank"`
	UID         string `bson:"uid"`
	DisplayName string `bson:"display_name"`
	Attempts    int32  `bson:"attempts"` // number of guesses used
	TimeMs      int32  `bson:"time_ms"`
}

// Leaderboard maps to the `leaderboards` collection.
// _id pattern: "{game}_{period}_{date}" e.g. "wordle_daily_2026-05-09".
type Leaderboard struct {
	ID            string           `bson:"_id"`
	GameID        string           `bson:"game_id"`
	Period        string           `bson:"period"` // "daily" | "all"
	Date          string           `bson:"date"`   // "YYYY-MM-DD" for daily
	Rankings      []LeaderboardRow `bson:"rankings"`
	RefreshedAt   time.Time        `bson:"refreshed_at"`
	SchemaVersion int              `bson:"schema_version"`
}
