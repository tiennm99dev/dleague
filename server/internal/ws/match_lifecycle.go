package ws

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/tiennm99/dleague/server/internal/store"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

const matchOpTimeout = 5 * time.Second

// handleJoinRoom verifies the conn is authenticated AND in the match's
// player list, then registers the conn into the room. Returns a
// JoinRoomAck (ok or error) — the conn-side dispatch writes it back.
func (h *Hub) handleJoinRoom(c *Conn, env *dleaguev1.Envelope) (*dleaguev1.Envelope, error) {
	if h.store == nil {
		return ackJoin(env.GetRequestId(), false, "", "", "", "sync-pvp disabled"), nil
	}
	if c.uid == "" {
		return ackJoin(env.GetRequestId(), false, "", "", "", "unauthenticated"), nil
	}

	var req dleaguev1.JoinRoom
	if err := proto.Unmarshal(env.GetPayload(), &req); err != nil {
		return nil, fmt.Errorf("join_room unmarshal: %w", err)
	}
	matchID := req.GetMatchId()
	if matchID == "" {
		return ackJoin(env.GetRequestId(), false, "", "", "", "missing match_id"), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), matchOpTimeout)
	defer cancel()
	m, err := h.store.GetMatch(ctx, matchID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ackJoin(env.GetRequestId(), false, matchID, "", "", "match not found"), nil
		}
		return nil, fmt.Errorf("get match: %w", err)
	}

	role := roleOf(c.uid, m.Players)
	if role == "" {
		return ackJoin(env.GetRequestId(), false, matchID, "", "", "uid not in match.players"), nil
	}
	opponent := opponentOf(c.uid, m.Players)

	// Register into the room. Lock order: hub mu → room mu (always).
	h.mu.Lock()
	room, ok := h.rooms[matchID]
	if !ok {
		room = newRoom(matchID)
		h.rooms[matchID] = room
	}
	c.matchID = matchID
	h.mu.Unlock()
	room.add(c)

	return ackJoin(env.GetRequestId(), true, matchID, role, opponent, ""), nil
}

// handleTurn forwards the TURN frame to the room's other conn(s). Returns
// an error if the frame can't be parsed or the conn isn't in a room.
func (h *Hub) handleTurn(c *Conn, env *dleaguev1.Envelope) error {
	if c.matchID == "" {
		return fmt.Errorf("turn: conn not in a room")
	}
	var req dleaguev1.Turn
	if err := proto.Unmarshal(env.GetPayload(), &req); err != nil {
		return fmt.Errorf("turn unmarshal: %w", err)
	}
	if req.GetMatchId() != c.matchID {
		// Defensive: prevent cross-room frame injection.
		return fmt.Errorf("turn: match_id mismatch")
	}
	h.broadcastRoom(c.matchID, env, c)
	return nil
}

// handleMatchEnd validates and persists the final result, ZADDs both
// leaderboards, then echoes the (validated) MATCH_END to room peers.
func (h *Hub) handleMatchEnd(c *Conn, env *dleaguev1.Envelope) (*dleaguev1.Envelope, error) {
	if h.store == nil {
		return nil, fmt.Errorf("match_end: store unwired")
	}
	if c.matchID == "" {
		return nil, fmt.Errorf("match_end: conn not in a room")
	}
	var req dleaguev1.MatchEnd
	if err := proto.Unmarshal(env.GetPayload(), &req); err != nil {
		return nil, fmt.Errorf("match_end unmarshal: %w", err)
	}
	if req.GetMatchId() != c.matchID {
		return nil, fmt.Errorf("match_end: match_id mismatch")
	}

	ctx, cancel := context.WithTimeout(context.Background(), matchOpTimeout)
	defer cancel()

	m, err := h.store.GetMatch(ctx, c.matchID)
	if err != nil {
		return nil, fmt.Errorf("match_end get: %w", err)
	}
	if req.GetWinnerUid() != "" && roleOf(req.GetWinnerUid(), m.Players) == "" {
		return nil, fmt.Errorf("match_end: winner not in players")
	}

	m.State = "ended"
	m.Winner = req.GetWinnerUid()
	m.Turns = int(maxScoreToTurns(req.GetScoreP1(), req.GetScoreP2()))
	m.EndedAt = time.Now().UTC()
	if err := h.store.UpsertMatch(ctx, m); err != nil {
		return nil, fmt.Errorf("match_end upsert: %w", err)
	}

	// Best-effort leaderboard updates: score per uid, both daily + global.
	if len(m.Players) >= 1 && req.GetScoreP1() > 0 {
		_ = h.store.SubmitScore(ctx, "lb:daily:"+m.PuzzleDate, m.Players[0], req.GetScoreP1())
		_ = h.store.SubmitScore(ctx, "lb:global:alltime", m.Players[0], req.GetScoreP1())
	}
	if len(m.Players) >= 2 && req.GetScoreP2() > 0 {
		_ = h.store.SubmitScore(ctx, "lb:daily:"+m.PuzzleDate, m.Players[1], req.GetScoreP2())
		_ = h.store.SubmitScore(ctx, "lb:global:alltime", m.Players[1], req.GetScoreP2())
	}

	// Notify the opponent. The sender's response ack travels via the
	// dispatch return value.
	h.broadcastRoom(c.matchID, env, c)
	return env, nil
}

func ackJoin(requestID string, ok bool, matchID, role, opponent, errMsg string) *dleaguev1.Envelope {
	body := &dleaguev1.JoinRoomAck{
		Ok:          ok,
		MatchId:     matchID,
		Role:        role,
		OpponentUid: opponent,
		Error:       errMsg,
	}
	payload, _ := proto.Marshal(body)
	return &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_JOIN_ROOM_ACK,
		RequestId: requestID,
		Payload:   payload,
	}
}

func roleOf(uid string, players []string) string {
	for i, p := range players {
		if p == uid {
			if i == 0 {
				return "p1"
			}
			return "p2"
		}
	}
	return ""
}

func opponentOf(uid string, players []string) string {
	for _, p := range players {
		if p != uid {
			return p
		}
	}
	return ""
}

// maxScoreToTurns is a placeholder mapping until the scoring/turn schema
// stabilizes; for now, just records "some turns happened" as the count.
func maxScoreToTurns(s1, s2 int64) int64 {
	if s1 > s2 {
		return s1
	}
	return s2
}
