package ws

import (
	"context"
	"errors"
	"log"

	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/protobuf/proto"

	"github.com/tiennm99/dleague/server/internal/game/wordle"
	"github.com/tiennm99/dleague/server/internal/store"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// handleChallengeCreate processes MESSAGE_TYPE_CHALLENGE_CREATE.
// Creates a pending async match and returns the share token + seed.
func handleChallengeCreate(ctx context.Context, c *Conn, env *dleaguev1.Envelope, deps *GameDeps) (*dleaguev1.Envelope, error) {
	if c.userID == "" {
		return errorEnvelope(env.GetRequestId(), 401, "unauthenticated"), nil
	}

	var msg dleaguev1.ChallengeCreate
	if err := proto.Unmarshal(env.GetPayload(), &msg); err != nil {
		return errorEnvelope(env.GetRequestId(), 400, "invalid ChallengeCreate payload"), nil
	}

	gameID := msg.GetGameId()
	if gameID == "" {
		gameID = "wordle"
	}

	// Always use today's daily seed (ignore seed_override for wordle; anti-cheat).
	seed, err := resolveDailySeed(ctx, deps)
	if err != nil {
		log.Printf("ws match_handler: resolveDailySeed uid=%q: %v", c.userID, err)
		return errorEnvelope(env.GetRequestId(), 500, "failed to load daily seed"), nil
	}

	matchID, shareToken, err := deps.MatchRepo.Create(ctx, store.Match{
		GameID:        gameID,
		ChallengerUID: c.userID,
		Seed:          seed,
	})
	if err != nil {
		log.Printf("ws match_handler: Create match uid=%q: %v", c.userID, err)
		return errorEnvelope(env.GetRequestId(), 500, "failed to create match"), nil
	}

	payload, err := proto.Marshal(&dleaguev1.ChallengeCreateAck{
		MatchId:    matchID,
		ShareToken: shareToken,
		Seed:       seed,
	})
	if err != nil {
		return nil, err
	}
	return &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_CHALLENGE_CREATE_ACK,
		RequestId: env.GetRequestId(),
		Payload:   payload,
	}, nil
}

// handleChallengeJoin processes MESSAGE_TYPE_CHALLENGE_JOIN.
// Atomically sets challengee_uid; rejects self-join and concurrent duplicates.
func handleChallengeJoin(ctx context.Context, c *Conn, env *dleaguev1.Envelope, deps *GameDeps) (*dleaguev1.Envelope, error) {
	if c.userID == "" {
		return errorEnvelope(env.GetRequestId(), 401, "unauthenticated"), nil
	}

	var msg dleaguev1.ChallengeJoin
	if err := proto.Unmarshal(env.GetPayload(), &msg); err != nil {
		return errorEnvelope(env.GetRequestId(), 400, "invalid ChallengeJoin payload"), nil
	}

	token := msg.GetShareToken()
	if token == "" {
		return errorEnvelope(env.GetRequestId(), 400, "share_token required"), nil
	}

	// Optimistic pre-check: validate challenger identity before the transaction.
	match, err := deps.MatchRepo.GetByShareToken(ctx, token)
	if err != nil {
		log.Printf("ws match_handler: GetByShareToken: %v", err)
		return errorEnvelope(env.GetRequestId(), 500, "internal error"), nil
	}
	if match == nil {
		return errorEnvelope(env.GetRequestId(), 404, "challenge not found"), nil
	}
	if match.ChallengerUID == c.userID {
		return errorEnvelope(env.GetRequestId(), 400, "cannot join your own challenge"), nil
	}

	// Atomic join inside a transaction: only one concurrent caller wins.
	var joined *store.Match
	session, sErr := deps.MongoClient.StartSession()
	if sErr != nil {
		log.Printf("ws match_handler: StartSession: %v", sErr)
		return errorEnvelope(env.GetRequestId(), 500, "transaction error"), nil
	}
	defer session.EndSession(ctx)

	// mongo-driver v2: WithTransaction callback is func(context.Context) (any, error).
	// The ctx passed to the callback already carries the session.
	_, txErr := session.WithTransaction(ctx, func(sc context.Context) (any, error) {
		var joinErr error
		joined, joinErr = deps.MatchRepo.JoinAsChallengee(sc, token, c.userID)
		return nil, joinErr
	})
	if txErr != nil {
		if errors.Is(txErr, store.ErrAlreadyJoined) {
			return errorEnvelope(env.GetRequestId(), 409, "challenge already taken"), nil
		}
		log.Printf("ws match_handler: JoinAsChallengee token=%q uid=%q: %v", token, c.userID, txErr)
		return errorEnvelope(env.GetRequestId(), 500, "join failed"), nil
	}

	payload, err := proto.Marshal(&dleaguev1.ChallengeJoinAck{
		MatchId: joined.ID.Hex(),
		Seed:    joined.Seed,
		GameId:  joined.GameID,
	})
	if err != nil {
		return nil, err
	}
	return &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_CHALLENGE_JOIN_ACK,
		RequestId: env.GetRequestId(),
		Payload:   payload,
	}, nil
}

// handleAttemptSubmit processes MESSAGE_TYPE_ATTEMPT_SUBMIT.
// Records the attempt; when both sides have submitted, computes and persists
// the winner atomically via a Mongo transaction.
func handleAttemptSubmit(ctx context.Context, c *Conn, env *dleaguev1.Envelope, deps *GameDeps) (*dleaguev1.Envelope, error) {
	if c.userID == "" {
		return errorEnvelope(env.GetRequestId(), 401, "unauthenticated"), nil
	}

	var msg dleaguev1.AttemptSubmit
	if err := proto.Unmarshal(env.GetPayload(), &msg); err != nil {
		return errorEnvelope(env.GetRequestId(), 400, "invalid AttemptSubmit payload"), nil
	}

	// Anti-cheat: reject impossibly fast (<500ms) or replay-attack (>24h) times.
	tms := msg.GetTimeMs()
	if tms < 500 || tms > 86_400_000 {
		return errorEnvelope(env.GetRequestId(), 422, "time_ms out of range [500, 86400000]"), nil
	}

	matchID := msg.GetMatchId()
	if matchID == "" {
		return errorEnvelope(env.GetRequestId(), 400, "match_id required"), nil
	}

	// Idempotency: return existing result if already submitted.
	prior, err := deps.AttemptRepo.GetByMatchAndPlayer(ctx, matchID, c.userID)
	if err != nil {
		return errorEnvelope(env.GetRequestId(), 500, "internal error"), nil
	}
	if prior != nil {
		existing, mErr := deps.MatchRepo.GetByID(ctx, matchID)
		if mErr != nil || existing == nil {
			return errorEnvelope(env.GetRequestId(), 500, "internal error"), nil
		}
		return buildAttemptAck(env.GetRequestId(), existing), nil
	}

	matchOID, oErr := bson.ObjectIDFromHex(matchID)
	if oErr != nil {
		return errorEnvelope(env.GetRequestId(), 400, "invalid match_id"), nil
	}

	attempt := store.Attempt{
		MatchID:   matchOID,
		PlayerUID: c.userID,
		Guesses:   msg.GetGuesses(),
		TimeMs:    tms,
		Won:       msg.GetWon(),
		Mode:      "async",
	}

	var winnerUID string
	var completed bool

	session, sErr := deps.MongoClient.StartSession()
	if sErr != nil {
		return errorEnvelope(env.GetRequestId(), 500, "transaction error"), nil
	}
	defer session.EndSession(ctx)

	_, txErr := session.WithTransaction(ctx, func(sc context.Context) (any, error) {
		if iErr := deps.AttemptRepo.Insert(sc, attempt); iErr != nil {
			if !errors.Is(iErr, store.ErrAttemptExists) {
				return nil, iErr
			}
			// Idempotent retry path: attempt was committed by an earlier try
			// of this WithTransaction callback. Fall through so winnerUID /
			// completed get populated from the current match state instead
			// of returning a stale "pending" ack.
		}

		match, mErr := deps.MatchRepo.GetByID(sc, matchID)
		if mErr != nil || match == nil {
			return nil, mErr
		}
		// If the match was already completed in a previous tx attempt, lift
		// the result and skip re-doing Complete/IncrementStats.
		if match.State == "complete" && match.WinnerUID != nil {
			winnerUID = *match.WinnerUID
			completed = true
			return nil, nil
		}
		if match.ChallengeeUID == nil {
			return nil, nil // challengee has not joined yet
		}

		allAttempts, lErr := deps.AttemptRepo.ListByMatch(sc, matchID)
		if lErr != nil {
			return nil, lErr
		}
		if len(allAttempts) < 2 {
			return nil, nil // still waiting for the other side
		}

		winnerUID = decideWinner(match, allAttempts)
		completed = true

		if cErr := deps.MatchRepo.Complete(sc, matchID, winnerUID); cErr != nil {
			return nil, cErr
		}

		// Update stats (best-effort: log but don't abort on failure).
		for _, uid := range []string{match.ChallengerUID, *match.ChallengeeUID} {
			if sErr := deps.UserRepo.IncrementStats(sc, uid, uid == winnerUID); sErr != nil {
				log.Printf("ws match_handler: IncrementStats uid=%q: %v", uid, sErr)
			}
		}
		return nil, nil
	})

	if txErr != nil {
		log.Printf("ws match_handler: AttemptSubmit tx matchID=%q uid=%q: %v", matchID, c.userID, txErr)
		return errorEnvelope(env.GetRequestId(), 500, "submit failed"), nil
	}

	status := "pending"
	if completed {
		status = "completed"
	}
	payload, err := proto.Marshal(&dleaguev1.AttemptSubmitAck{
		WinnerUid: winnerUID,
		Status:    status,
	})
	if err != nil {
		return nil, err
	}
	return &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_ATTEMPT_SUBMIT_ACK,
		RequestId: env.GetRequestId(),
		Payload:   payload,
	}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// resolveDailySeed returns today's Wordle seed, ensuring the puzzle exists.
func resolveDailySeed(ctx context.Context, deps *GameDeps) (int64, error) {
	if _, err := wordle.EnsureToday(ctx, deps.DailyRepo, deps.Answers, nowUTC()); err != nil {
		return 0, err
	}
	// DailyPuzzleStore is the minimal interface (wordle pkg). The richer
	// *store.DailyPuzzleRepo also has GetByDate; assert at runtime.
	type dailyGetter interface {
		GetByDate(ctx context.Context, date string) (*store.DailyPuzzle, error)
	}
	getter, ok := deps.DailyRepo.(dailyGetter)
	if !ok {
		return 0, nil // test stub without GetByDate; seed 0 is acceptable
	}
	dp, err := getter.GetByDate(ctx, nowUTC().Format("2006-01-02"))
	if err != nil || dp == nil {
		return 0, err
	}
	return dp.Seed, nil
}

// decideWinner returns the UID of the winner.
// Tie-break: won DESC → guesses ASC → time_ms ASC → challenger wins ties.
func decideWinner(match *store.Match, attempts []store.Attempt) string {
	byUID := make(map[string]store.Attempt, len(attempts))
	for _, a := range attempts {
		byUID[a.PlayerUID] = a
	}
	ca := byUID[match.ChallengerUID]
	var challengeeUID string
	if match.ChallengeeUID != nil {
		challengeeUID = *match.ChallengeeUID
	}
	ea := byUID[challengeeUID]

	if ca.Won != ea.Won {
		if ca.Won {
			return match.ChallengerUID
		}
		return challengeeUID
	}
	cg, eg := int32(len(ca.Guesses)), int32(len(ea.Guesses))
	if cg != eg {
		if cg < eg {
			return match.ChallengerUID
		}
		return challengeeUID
	}
	if ca.TimeMs < ea.TimeMs {
		return match.ChallengerUID
	}
	if ea.TimeMs < ca.TimeMs {
		return challengeeUID
	}
	return match.ChallengerUID // perfect tie → challenger wins
}

// buildAttemptAck constructs an idempotent AttemptSubmitAck from an existing match.
func buildAttemptAck(reqID string, match *store.Match) *dleaguev1.Envelope {
	status := "pending"
	winner := ""
	if match.State == "complete" {
		status = "completed"
		if match.WinnerUID != nil {
			winner = *match.WinnerUID
		}
	}
	payload, _ := proto.Marshal(&dleaguev1.AttemptSubmitAck{WinnerUid: winner, Status: status})
	return &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_ATTEMPT_SUBMIT_ACK,
		RequestId: reqID,
		Payload:   payload,
	}
}
