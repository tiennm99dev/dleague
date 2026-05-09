package ws

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/tiennm99/dleague/server/internal/game/wordle"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

const matchDuration = 5 * time.Minute

// Room holds the live state for one sync PvP match.
// HandleMove, HandleForfeit and HandleTimeout are all goroutine-safe;
// they acquire mu before mutating shared state.
type Room struct {
	MatchID   string
	Players   [2]*Conn
	Wordles   [2]*wordle.Wordle
	Solution  string
	StartedAt time.Time
	Deadline  time.Time

	mu       sync.Mutex
	resolved bool // set true after first resolution; further calls are no-ops
}

// NewRoom creates a Room for the two given players with a shared solution.
func NewRoom(matchID, solution string, p1, p2 *Conn) *Room {
	now := time.Now().UTC()
	return &Room{
		MatchID:   matchID,
		Players:   [2]*Conn{p1, p2},
		Wordles:   [2]*wordle.Wordle{wordle.New(solution), wordle.New(solution)},
		Solution:  solution,
		StartedAt: now,
		Deadline:  now.Add(matchDuration),
	}
}

// playerIndex returns the index (0 or 1) of c within the room.
// Returns -1 if c is not a player (e.g. after a reconnect swap).
func (r *Room) playerIndex(c *Conn) int {
	// Match by userID to survive reconnect pointer swap before rebind.
	for i, p := range r.Players {
		if p != nil && p.userID == c.userID {
			return i
		}
	}
	return -1
}

// opponentIndex returns the index of the player who is NOT i.
func opponentIndex(i int) int {
	return 1 - i
}

// HandleMove applies a guess for player c.
// It sends own full state back to c and the color-only progress to the opponent.
// If the move terminates the match, CompleteSync is called inside a transaction
// and both players receive MATCH_RESOLVED.
func (r *Room) HandleMove(ctx context.Context, c *Conn, guess string, deps *GameDeps) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.resolved {
		return nil // late frame; silently discard
	}

	idx := r.playerIndex(c)
	if idx < 0 {
		return fmt.Errorf("match_room: conn %q not a player in match %q", c.userID, r.MatchID)
	}

	w := r.Wordles[idx]
	if w.IsTerminal() {
		return nil // player already done; ignore extra moves
	}

	// Apply the move (caller already validated via Validate).
	if err := w.Apply(guess); err != nil {
		return fmt.Errorf("match_room: Apply: %w", err)
	}

	// Send full own state back to the moving player.
	ownState := w.ToProto()
	ownPayload, err := proto.Marshal(ownState)
	if err != nil {
		return fmt.Errorf("match_room: marshal own state: %w", err)
	}
	c.enqueue(&dleaguev1.Envelope{
		Type:    dleaguev1.MessageType_MESSAGE_TYPE_GAME_STATE,
		Payload: ownPayload,
	})

	// Send color-only progress to opponent (NEVER guess letters).
	latestHint := ownState.GetHints()
	var colors []dleaguev1.Color
	if len(latestHint) > 0 {
		colors = latestHint[len(latestHint)-1].GetColors()
	}
	progressPayload, err := proto.Marshal(&dleaguev1.MatchOpponentProgress{
		MatchId:    r.MatchID,
		AttemptNum: int32(len(ownState.GetGuesses())), //nolint:gosec
		Colors:     colors,
	})
	if err != nil {
		return fmt.Errorf("match_room: marshal opponent progress: %w", err)
	}
	opp := r.Players[opponentIndex(idx)]
	if opp != nil {
		opp.enqueue(&dleaguev1.Envelope{
			Type:    dleaguev1.MessageType_MESSAGE_TYPE_MATCH_OPPONENT_PROGRESS,
			Payload: progressPayload,
		})
	}

	// Check for resolution.
	if r.shouldResolve() {
		r.resolveUnlocked(ctx, deps)
	}
	return nil
}

// HandleForfeit resolves the match with the loser identified by loserUserID.
// Safe to call from the disconnect grace goroutine.
func (r *Room) HandleForfeit(ctx context.Context, loserUserID string, deps *GameDeps) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.resolved {
		return
	}
	winnerUID := ""
	for _, p := range r.Players {
		if p != nil && p.userID != loserUserID {
			winnerUID = p.userID
			break
		}
	}
	r.finishUnlocked(ctx, deps, winnerUID, "forfeit")
}

// HandleTimeout resolves the match due to the 5-minute hard cap.
// Safe to call from the rooms-tick goroutine.
func (r *Room) HandleTimeout(ctx context.Context, deps *GameDeps) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.resolved {
		return
	}
	// Winner is whoever has solved; if neither, both lose (empty winner).
	winnerUID := r.timeoutWinner()
	r.finishUnlocked(ctx, deps, winnerUID, "timeout")
}

// ── internals (called with mu held) ──────────────────────────────────────────

// shouldResolve returns true when either player has won, or both are terminal.
func (r *Room) shouldResolve() bool {
	w0, w1 := r.Wordles[0], r.Wordles[1]
	if w0.IsTerminal() && w0.Result().Won {
		return true
	}
	if w1.IsTerminal() && w1.Result().Won {
		return true
	}
	return w0.IsTerminal() && w1.IsTerminal()
}

// resolveUnlocked determines the winner and delegates to finishUnlocked.
// Must be called with r.mu held.
func (r *Room) resolveUnlocked(ctx context.Context, deps *GameDeps) {
	w0, w1 := r.Wordles[0], r.Wordles[1]
	r0, r1 := w0.Result(), w1.Result()

	var reason string
	var winnerUID string

	switch {
	case r0.Won && !r1.Won:
		winnerUID = r.Players[0].userID
		reason = "solved"
	case r1.Won && !r0.Won:
		winnerUID = r.Players[1].userID
		reason = "solved"
	case r0.Won && r1.Won:
		// Both solved: tie-break by attempts ASC then time ASC (time approximated
		// by attempt count at this layer; sub-second tie-break deferred to Phase 10).
		if r0.AttemptsUsed < r1.AttemptsUsed {
			winnerUID = r.Players[0].userID
		} else if r1.AttemptsUsed < r0.AttemptsUsed {
			winnerUID = r.Players[1].userID
		} else {
			winnerUID = r.Players[0].userID // perfect tie → player 0
		}
		reason = "solved"
	default:
		// Both lost: no winner.
		reason = "exhausted"
	}

	r.finishUnlocked(ctx, deps, winnerUID, reason)
}

// finishUnlocked persists the result, broadcasts MATCH_RESOLVED, and removes
// the room from the registry. Must be called with r.mu held.
func (r *Room) finishUnlocked(ctx context.Context, deps *GameDeps, winnerUID, reason string) {
	r.resolved = true

	if deps != nil && deps.MongoClient != nil && deps.MatchRepo != nil {
		if err := deps.MatchRepo.CompleteSync(ctx, deps.MongoClient, r.MatchID, winnerUID, reason); err != nil {
			log.Printf("match_room: CompleteSync matchID=%q: %v", r.MatchID, err)
		}
		// Best-effort stats update (outside transaction; acceptable for MVP).
		if deps.UserRepo != nil {
			for _, p := range r.Players {
				if p != nil {
					won := p.userID == winnerUID
					if sErr := deps.UserRepo.IncrementStats(ctx, p.userID, won); sErr != nil {
						log.Printf("match_room: IncrementStats uid=%q: %v", p.userID, sErr)
					}
				}
			}
		}
	}

	resolvedPayload, err := proto.Marshal(&dleaguev1.MatchResolved{
		MatchId:   r.MatchID,
		WinnerUid: winnerUID,
		Reason:    reason,
	})
	if err != nil {
		log.Printf("match_room: marshal MatchResolved: %v", err)
		return
	}
	env := &dleaguev1.Envelope{
		Type:    dleaguev1.MessageType_MESSAGE_TYPE_MATCH_RESOLVED,
		Payload: resolvedPayload,
	}
	for _, p := range r.Players {
		if p != nil {
			p.setActiveMatchID("")
			p.enqueue(env)
		}
	}

	// Remove from registry asynchronously to avoid deadlock (registry lock ≠ room lock).
	if deps != nil && deps.Rooms != nil {
		go deps.Rooms.Remove(r.MatchID)
	}
}

// timeoutWinner returns the userID of whoever solved (or "" for both-lose).
func (r *Room) timeoutWinner() string {
	w0, w1 := r.Wordles[0], r.Wordles[1]
	if w0.IsTerminal() && w0.Result().Won {
		return r.Players[0].userID
	}
	if w1.IsTerminal() && w1.Result().Won {
		return r.Players[1].userID
	}
	return ""
}
