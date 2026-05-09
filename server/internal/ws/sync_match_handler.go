package ws

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"log"

	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/protobuf/proto"

	"github.com/tiennm99/dleague/server/internal/game/wordle"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// Compile-time check: ensure dleaguev1.QueueAck is referenced so the import
// does not get pruned when ackPayload is unused. This reference is intentional
// because QueueAck is defined in match.proto and may be used in future phases.
var _ = (*dleaguev1.QueueAck)(nil)

// handleQueueJoin processes MESSAGE_TYPE_QUEUE_JOIN.
// Adds the connection to the matchmaking queue. If a pair is available,
// a match is created immediately and both players receive QUEUE_MATCHED.
func handleQueueJoin(ctx context.Context, c *Conn, env *dleaguev1.Envelope, deps *GameDeps) (*dleaguev1.Envelope, error) {
	if c.userID == "" {
		return errorEnvelope(env.GetRequestId(), 401, "unauthenticated"), nil
	}
	if deps.Queue == nil || deps.Rooms == nil {
		return errorEnvelope(env.GetRequestId(), 503, "matchmaking unavailable"), nil
	}

	var msg dleaguev1.QueueJoin
	if err := proto.Unmarshal(env.GetPayload(), &msg); err != nil {
		return errorEnvelope(env.GetRequestId(), 400, "invalid QueueJoin payload"), nil
	}
	gameID := msg.GetGameId()
	if gameID == "" {
		gameID = "wordle"
	}

	// Push before attempting to pop so the caller is always in the queue first.
	deps.Queue.Push(gameID, c)

	a, b, paired := deps.Queue.PopPair(gameID)
	if !paired {
		// Queued; no response needed — client shows "Searching…" UI.
		// QUEUE_MATCHED is a server-push type; there is no QUEUE_ACK envelope type.
		return nil, nil
	}

	// Pair found: a and b are the two players.
	if err := startSyncMatch(ctx, a, b, gameID, deps); err != nil {
		log.Printf("ws queue: startSyncMatch: %v", err)
		// Re-queue both players so they don't disappear silently.
		deps.Queue.Push(gameID, a)
		deps.Queue.Push(gameID, b)
		return errorEnvelope(env.GetRequestId(), 500, "match creation failed"), nil
	}
	return nil, nil
}

// handleQueueLeave processes MESSAGE_TYPE_QUEUE_LEAVE.
func handleQueueLeave(_ context.Context, c *Conn, _ *dleaguev1.Envelope, deps *GameDeps) (*dleaguev1.Envelope, error) {
	if deps.Queue != nil {
		deps.Queue.Remove(c)
	}
	return nil, nil
}

// handleMatchMove processes MESSAGE_TYPE_MATCH_MOVE.
func handleMatchMove(ctx context.Context, c *Conn, env *dleaguev1.Envelope, deps *GameDeps) (*dleaguev1.Envelope, error) {
	if c.userID == "" {
		return errorEnvelope(env.GetRequestId(), 401, "unauthenticated"), nil
	}

	var msg dleaguev1.MatchMove
	if err := proto.Unmarshal(env.GetPayload(), &msg); err != nil {
		return errorEnvelope(env.GetRequestId(), 400, "invalid MatchMove payload"), nil
	}
	if msg.GetMatchId() == "" || msg.GetGuess() == "" {
		return errorEnvelope(env.GetRequestId(), 400, "match_id and guess required"), nil
	}

	if deps.Rooms == nil {
		return errorEnvelope(env.GetRequestId(), 503, "game service unavailable"), nil
	}
	room := deps.Rooms.Get(msg.GetMatchId())
	if room == nil {
		return errorEnvelope(env.GetRequestId(), 404, "match not found"), nil
	}

	// Validate guess against dictionary (same as solo game).
	if err := validateSyncGuess(msg.GetGuess(), deps); err != nil {
		return errorEnvelope(env.GetRequestId(), 400, err.Error()), nil
	}

	if err := room.HandleMove(ctx, c, msg.GetGuess(), deps); err != nil {
		log.Printf("ws match_move: HandleMove matchID=%q uid=%q: %v", msg.GetMatchId(), c.userID, err)
		return errorEnvelope(env.GetRequestId(), 500, "move failed"), nil
	}
	// Response (own WordleState) is enqueued inside HandleMove; return nil here.
	return nil, nil
}

// handleMatchRejoin processes MESSAGE_TYPE_MATCH_REJOIN.
// Rebinds the conn to the live room and sends back current state.
func handleMatchRejoin(_ context.Context, c *Conn, env *dleaguev1.Envelope, deps *GameDeps) (*dleaguev1.Envelope, error) {
	if c.userID == "" {
		return errorEnvelope(env.GetRequestId(), 401, "unauthenticated"), nil
	}

	var msg dleaguev1.MatchRejoin
	if err := proto.Unmarshal(env.GetPayload(), &msg); err != nil {
		return errorEnvelope(env.GetRequestId(), 400, "invalid MatchRejoin payload"), nil
	}
	if msg.GetMatchId() == "" {
		return errorEnvelope(env.GetRequestId(), 400, "match_id required"), nil
	}

	if deps.Rooms == nil {
		return errorEnvelope(env.GetRequestId(), 404, "match not found"), nil
	}
	room := deps.Rooms.Get(msg.GetMatchId())
	if room == nil {
		// Match no longer active (already resolved or never existed).
		return errorEnvelope(env.GetRequestId(), 404, "match not found or already resolved"), nil
	}

	room.mu.Lock()
	// Find and rebind the player slot.
	playerIdx := -1
	for i, p := range room.Players {
		if p != nil && p.userID == c.userID {
			playerIdx = i
			room.Players[i] = c // rebind to new conn
			break
		}
	}
	if playerIdx < 0 {
		room.mu.Unlock()
		return errorEnvelope(env.GetRequestId(), 403, "not a participant in this match"), nil
	}

	// Build own state and opponent progress for the ack.
	ownState := room.Wordles[playerIdx].ToProto()
	oppIdx := opponentIndex(playerIdx)
	oppWordle := room.Wordles[oppIdx]
	oppAttempts := int32(len(oppWordle.ToProto().GetGuesses())) //nolint:gosec
	oppHints := oppWordle.ToProto().GetHints()
	room.mu.Unlock()

	// Cancel disconnect grace timer (player is back).
	if deps.GraceTimers != nil {
		deps.GraceTimers.Cancel(msg.GetMatchId(), c.userID)
	}
	c.setActiveMatchID(msg.GetMatchId())

	ackPayload, err := proto.Marshal(&dleaguev1.MatchRejoinAck{
		MatchId:          msg.GetMatchId(),
		OwnState:         ownState,
		OpponentAttempts: oppAttempts,
		OpponentHints:    oppHints,
	})
	if err != nil {
		return nil, err
	}
	return &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_MATCH_REJOIN_ACK,
		RequestId: env.GetRequestId(),
		Payload:   ackPayload,
	}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// startSyncMatch creates the Mongo match doc, builds the room, registers it,
// and pushes QUEUE_MATCHED to both players.
func startSyncMatch(ctx context.Context, a, b *Conn, gameID string, deps *GameDeps) error {
	seed := cryptoSeed()

	// Derive solution from seed using the answer word list.
	solution := ""
	if len(deps.Answers) > 0 {
		idx := int(seed % int64(len(deps.Answers))) //nolint:gosec
		if idx < 0 {
			idx = -idx
		}
		solution = deps.Answers[idx]
	}
	if solution == "" {
		solution = "CRANE" // last-resort fallback (answers list is always populated in prod)
	}

	// Pre-generate the match ID so both conns can claim activeMatchID BEFORE
	// the Mongo round-trip. Closes the H2 orphan-match window: if a player
	// disconnects mid-CreateSync the defer hook now sees a non-empty match ID
	// and schedules the grace timer. The grace path is idempotent if the room
	// never gets registered (Get returns nil and HandleForfeit no-ops).
	matchOID := bson.NewObjectID()
	matchID := matchOID.Hex()

	a.setActiveMatchID(matchID)
	b.setActiveMatchID(matchID)

	// Persist the match document (may be nil in tests).
	if deps.MatchRepo != nil {
		if err := deps.MatchRepo.CreateSyncWithID(ctx, matchOID, a.userID, b.userID, seed, gameID); err != nil {
			// Rollback the activeMatchID claim so the conns aren't haunted by
			// a never-registered match.
			a.setActiveMatchID("")
			b.setActiveMatchID("")
			return err
		}
	}

	room := NewRoom(matchID, solution, a, b)
	deps.Rooms.Add(matchID, room)

	// Resolve display names (best-effort; fallback to userID).
	aName, bName := displayName(a), displayName(b)

	sendQueueMatched(a, matchID, seed, bName, "")
	sendQueueMatched(b, matchID, seed, aName, "")
	return nil
}

// sendQueueMatched enqueues a QUEUE_MATCHED push on the given conn.
func sendQueueMatched(c *Conn, matchID string, seed int64, opponentName, reqID string) {
	payload, err := proto.Marshal(&dleaguev1.QueueMatched{
		MatchId:             matchID,
		Seed:                seed,
		OpponentDisplayName: opponentName,
	})
	if err != nil {
		log.Printf("ws: marshal QueueMatched: %v", err)
		return
	}
	c.enqueue(&dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_QUEUE_MATCHED,
		RequestId: reqID,
		Payload:   payload,
	})
}

// displayName returns the user's display name or their userID as fallback.
func displayName(c *Conn) string {
	// Conn does not store display name — hub.userRepo would be needed for a
	// DB lookup. For MVP we return the userID as the display label.
	// Phase 10 can enrich this with a cached profile store.
	if c.userID != "" {
		return c.userID
	}
	return "anonymous"
}

// validateSyncGuess validates a guess for a sync match (length + dictionary).
func validateSyncGuess(guess string, deps *GameDeps) error {
	// Build a temporary Wordle just for validation — no state mutation.
	tmp := wordle.New("CRANE")
	return tmp.Validate(guess, deps.Dictionary)
}

// cryptoSeed returns a random int64 seed using crypto/rand.
func cryptoSeed() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Should never happen; fall back to a deterministic value.
		return 42
	}
	v := int64(binary.LittleEndian.Uint64(b[:]))
	if v < 0 {
		v = -v
	}
	return v
}
