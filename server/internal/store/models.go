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
	CreatedAt     time.Time `bson:"created_at"`
	LastLogin     time.Time `bson:"last_login"`
	Verified      bool      `bson:"verified"`
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
type Match struct {
	ID            bson.ObjectID `bson:"_id,omitempty"`
	GameID        string        `bson:"game_id"`
	Players       []string      `bson:"players"` // Firebase UIDs
	Mode          string        `bson:"mode"`    // "sync" | "async"
	State         string        `bson:"state"`   // "pending" | "active" | "complete"
	WinnerUID     string        `bson:"winner_uid,omitempty"`
	Seed          int64         `bson:"seed"`
	Metadata      bson.M        `bson:"metadata,omitempty"`
	CreatedAt     time.Time     `bson:"created_at"`
	EndedAt       *time.Time    `bson:"ended_at,omitempty"`
	SchemaVersion int           `bson:"schema_version"`
}

// Attempt maps to the `attempts` collection.
// One document per player per match.
type Attempt struct {
	ID            bson.ObjectID `bson:"_id,omitempty"`
	MatchID       bson.ObjectID `bson:"match_id"`
	PlayerUID     string        `bson:"player_uid"`
	Attempts      []string      `bson:"attempts"` // word guesses in order
	TimeMs        int64         `bson:"time_ms"`  // milliseconds to solve
	Result        string        `bson:"result"`   // "win" | "loss" | "timeout"
	CreatedAt     time.Time     `bson:"created_at"`
	SchemaVersion int           `bson:"schema_version"`
}

// DailyPuzzle maps to the `daily_puzzles` collection.
// _id is a date string "YYYY-MM-DD".
type DailyPuzzle struct {
	ID            string    `bson:"_id"` // "YYYY-MM-DD"
	GameID        string    `bson:"game_id"`
	Seed          int64     `bson:"seed"`
	SolutionHash  string    `bson:"solution_hash"` // sha256; never store plaintext
	Difficulty    string    `bson:"difficulty"`
	CreatedAt     time.Time `bson:"created_at"`
	SchemaVersion int       `bson:"schema_version"`
}

// LeaderboardRanking is one entry in a leaderboard snapshot.
type LeaderboardRanking struct {
	Rank        int    `bson:"rank"`
	UID         string `bson:"uid"`
	Score       int    `bson:"score"`
	GamesPlayed int    `bson:"games_played"`
}

// Leaderboard maps to the `leaderboards` collection.
// _id pattern: "{game}_{period}_{period_end_date}".
type Leaderboard struct {
	ID            string               `bson:"_id"`
	GameID        string               `bson:"game_id"`
	Period        string               `bson:"period"` // "daily" | "weekly" | "alltime"
	PeriodEnd     time.Time            `bson:"period_end"`
	Rankings      []LeaderboardRanking `bson:"rankings"`
	UpdatedAt     time.Time            `bson:"updated_at"`
	SchemaVersion int                  `bson:"schema_version"`
}
