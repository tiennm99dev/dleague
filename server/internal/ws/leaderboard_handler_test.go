package ws

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// TestLeaderboardQuery_Unauthenticated rejects unauth conns with 401.
func TestLeaderboardQuery_Unauthenticated(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "", // unauthenticated
	}

	deps := &GameDeps{}

	queryPayload, _ := proto.Marshal(&dleaguev1.LeaderboardQuery{})
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_LEADERBOARD_QUERY,
		RequestId: "lb-1",
		Payload:   queryPayload,
	}

	response, err := handleLeaderboardQuery(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleLeaderboardQuery: %v", err)
	}

	if response.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_ERROR {
		t.Fatalf("response type = %v, want ERROR", response.GetType())
	}

	var errMsg dleaguev1.Error
	if err := proto.Unmarshal(response.GetPayload(), &errMsg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if errMsg.GetCode() != 401 {
		t.Fatalf("error code = %d, want 401", errMsg.GetCode())
	}
}

// TestLeaderboardQuery_InvalidPayload returns 400 for malformed payload.
func TestLeaderboardQuery_InvalidPayload(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "test-user",
	}

	deps := &GameDeps{}

	// Malformed payload.
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_LEADERBOARD_QUERY,
		RequestId: "lb-2",
		Payload:   []byte{0xFF, 0xFE, 0xFD}, // garbage bytes
	}

	response, err := handleLeaderboardQuery(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleLeaderboardQuery: %v", err)
	}

	if response.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_ERROR {
		t.Fatalf("response type = %v, want ERROR", response.GetType())
	}

	var errMsg dleaguev1.Error
	if err := proto.Unmarshal(response.GetPayload(), &errMsg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if errMsg.GetCode() != 400 {
		t.Fatalf("error code = %d, want 400", errMsg.GetCode())
	}
}

