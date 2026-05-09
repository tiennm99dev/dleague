package ws

import (
	"context"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/protobuf/proto"

	"github.com/tiennm99/dleague/server/internal/game/wordle"
	"github.com/tiennm99/dleague/server/internal/store"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// nowUTC is a package-level var so tests can stub time.
var nowUTC = func() time.Time { return time.Now().UTC() }

// GameDeps holds the runtime dependencies needed by WS game handlers.
// Constructed once in main and attached to the Hub before serving traffic.
type GameDeps struct {
	// Wordle game deps.
	DailyRepo  wordle.DailyPuzzleStore // accepts *store.DailyPuzzleRepo or test mock
	Dictionary []string                // valid-guess word list (superset of answers)
	Answers    []string                // answer word list (used to seed daily puzzles)

	// Phase 08: async PvP + leaderboard repos.
	MatchRepo       *store.MatchRepo
	AttemptRepo     *store.AttemptRepo
	LeaderboardRepo *store.LeaderboardRepo
	UserRepo        *store.UserRepo

	// MongoClient is the raw client used to open transaction sessions.
	// May be nil in unit tests that don't exercise transactional paths.
	MongoClient *mongo.Client

	// Phase 09: sync PvP — in-memory state.
	Queue       *Queue         // matchmaking queue
	Rooms       *RoomsRegistry // active match rooms
	GraceTimers *GraceTimers   // disconnect grace timers

	// Phase 10 / security: per-UID rate limiter (defence-in-depth above per-conn).
	// Nil-safe: if unset, per-UID limiting is skipped.
	UIDLimiter *UIDLimiter
}

// wordleSession holds one user's in-progress Wordle game.
// Sessions are in-memory only; they are lost on server restart (Phase 08
// will add durable persistence via the attempts collection).
type wordleSession struct {
	mu     sync.Mutex
	game   *wordle.Wordle
	loaded bool // true once solution has been loaded from daily repo
}

// sessions maps Firebase UID → *wordleSession for solo daily play.
// sync.Map chosen for concurrent access without a coarse hub-level lock.
var sessions sync.Map //nolint:gochecknoglobals

// handleGameMove processes a MESSAGE_TYPE_GAME_MOVE envelope.
// It validates the guess, applies it to the player's session, and returns
// a MESSAGE_TYPE_GAME_STATE envelope. The solution is omitted from the
// response until the game reaches a terminal state.
func handleGameMove(ctx context.Context, c *Conn, env *dleaguev1.Envelope, deps *GameDeps) (*dleaguev1.Envelope, error) {
	// Defensive auth re-check (upstream requiresAuth already gate-keeps,
	// but belt-and-suspenders for direct unit-test callers).
	uid := c.UserID()
	if uid == "" {
		return errorEnvelope(env.GetRequestId(), 401, "unauthenticated"), nil
	}

	// Unmarshal the move payload.
	var move dleaguev1.WordleMove
	if err := proto.Unmarshal(env.GetPayload(), &move); err != nil {
		return errorEnvelope(env.GetRequestId(), 400, "invalid WordleMove payload"), nil
	}

	guess := move.GetGuess()

	// Load or create the session for this user.
	sess := loadOrCreateSession(uid)
	sess.mu.Lock()
	defer sess.mu.Unlock()

	// Lazy-init: load today's solution on first move.
	if !sess.loaded {
		solution, err := wordle.EnsureToday(ctx, deps.DailyRepo, deps.Answers, nowUTC())
		if err != nil {
			log.Printf("ws game_handler: EnsureToday uid=%s: %v", RedactUID(uid), err)
			return errorEnvelope(env.GetRequestId(), 500, "failed to load daily puzzle"), nil
		}
		sess.game = wordle.New(solution)
		sess.loaded = true
	}

	// Validate: length + dictionary.
	if err := sess.game.Validate(guess, deps.Dictionary); err != nil {
		return errorEnvelope(env.GetRequestId(), 400, err.Error()), nil
	}

	// Apply the guess.
	if err := sess.game.Apply(guess); err != nil {
		return errorEnvelope(env.GetRequestId(), 400, err.Error()), nil
	}

	// Build response.
	stateProto := sess.game.ToProto()
	payload, err := proto.Marshal(stateProto)
	if err != nil {
		return nil, err
	}

	// Evict the session once the game reaches a terminal state so memory is
	// reclaimed promptly. The next GAME_MOVE reloads today's puzzle via
	// EnsureToday — safe because the daily seed is deterministic. Phase 07 M3 fix.
	// Trade-off: in-progress state is lost on disconnect, but for solo daily play
	// the puzzle regenerates from the same seed, so no user-visible data is lost.
	if stateProto.GetWon() || stateProto.GetLost() {
		sessions.Delete(uid)
	}

	return &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_GAME_STATE,
		RequestId: env.GetRequestId(),
		Payload:   payload,
	}, nil
}

// deleteSession removes the solo game session for the given userID.
// Called on conn disconnect to free memory. Phase 07 M3 fix.
func deleteSession(uid string) {
	if uid != "" {
		sessions.Delete(uid)
	}
}

// loadOrCreateSession returns the existing session for uid, or creates a new
// empty one. The returned session is not yet locked; callers must lock it.
func loadOrCreateSession(uid string) *wordleSession {
	v, _ := sessions.LoadOrStore(uid, &wordleSession{})
	return v.(*wordleSession) //nolint:forcetypeassert
}
