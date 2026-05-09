package ws

import (
	"context"
	"log"

	"google.golang.org/protobuf/proto"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// handleLeaderboardQuery processes MESSAGE_TYPE_LEADERBOARD_QUERY.
// Reads the pre-computed leaderboard snapshot from the leaderboards collection.
// If no snapshot exists yet (scheduler not run), returns an empty rankings list
// and logs a warning rather than failing.
func handleLeaderboardQuery(ctx context.Context, c *Conn, env *dleaguev1.Envelope, deps *GameDeps) (*dleaguev1.Envelope, error) {
	if c.UserID() == "" {
		return errorEnvelope(env.GetRequestId(), 401, "unauthenticated"), nil
	}

	var msg dleaguev1.LeaderboardQuery
	if err := proto.Unmarshal(env.GetPayload(), &msg); err != nil {
		return errorEnvelope(env.GetRequestId(), 400, "invalid LeaderboardQuery payload"), nil
	}

	gameID := msg.GetGameId()
	if gameID == "" {
		gameID = "wordle"
	}
	period := msg.GetPeriod()
	if period == "" {
		period = "daily"
	}

	date := nowUTC().Format("2006-01-02")

	lb, err := deps.LeaderboardRepo.Get(ctx, gameID, period, date)
	if err != nil {
		log.Printf("ws leaderboard_handler: Get gameID=%q period=%q: %v", gameID, period, err)
		return errorEnvelope(env.GetRequestId(), 500, "failed to load leaderboard"), nil
	}

	snapshot := &dleaguev1.LeaderboardSnapshot{}
	if lb == nil {
		// Snapshot not yet generated — return empty; client can retry later.
		log.Printf("ws leaderboard_handler: no snapshot for %s/%s/%s — returning empty", gameID, period, date)
	} else {
		entries := make([]*dleaguev1.LeaderboardEntry, 0, len(lb.Rankings))
		for _, r := range lb.Rankings {
			entries = append(entries, &dleaguev1.LeaderboardEntry{
				Uid:         r.UID,
				DisplayName: r.DisplayName,
				Attempts:    r.Attempts,
				TimeMs:      r.TimeMs,
				Rank:        int32(r.Rank),
			})
		}
		snapshot.Rankings = entries
	}

	payload, err := proto.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	return &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_LEADERBOARD_SNAPSHOT,
		RequestId: env.GetRequestId(),
		Payload:   payload,
	}, nil
}
