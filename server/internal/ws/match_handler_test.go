package ws

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// TestChallengeCreate_Unauthenticated rejects unauth conns with 401.
func TestChallengeCreate_Unauthenticated(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "", // unauthenticated
	}

	deps := &GameDeps{}

	payload, _ := proto.Marshal(&dleaguev1.ChallengeCreate{GameId: "wordle"})
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_CHALLENGE_CREATE,
		RequestId: "ch-1",
		Payload:   payload,
	}

	response, err := handleChallengeCreate(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleChallengeCreate: %v", err)
	}
	if response == nil {
		t.Fatal("response is nil")
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

// TestChallengeCreate_InvalidPayload returns 400 for malformed payload.
func TestChallengeCreate_InvalidPayload(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "test-user",
	}

	deps := &GameDeps{}

	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_CHALLENGE_CREATE,
		RequestId: "ch-2",
		Payload:   []byte{0xFF, 0xFE, 0xFD}, // garbage
	}

	response, err := handleChallengeCreate(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleChallengeCreate: %v", err)
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

// TestChallengeJoin_Unauthenticated rejects unauth conns with 401.
func TestChallengeJoin_Unauthenticated(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "", // unauthenticated
	}

	deps := &GameDeps{}

	payload, _ := proto.Marshal(&dleaguev1.ChallengeJoin{ShareToken: "token-123"})
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_CHALLENGE_JOIN,
		RequestId: "ch-3",
		Payload:   payload,
	}

	response, err := handleChallengeJoin(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleChallengeJoin: %v", err)
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

// TestChallengeJoin_MissingToken returns 400 when token is empty.
func TestChallengeJoin_MissingToken(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "test-user",
	}

	deps := &GameDeps{}

	payload, _ := proto.Marshal(&dleaguev1.ChallengeJoin{ShareToken: ""})
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_CHALLENGE_JOIN,
		RequestId: "ch-4",
		Payload:   payload,
	}

	response, err := handleChallengeJoin(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleChallengeJoin: %v", err)
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

// TestChallengeJoin_InvalidPayload returns 400 for malformed payload.
func TestChallengeJoin_InvalidPayload(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "test-user",
	}

	deps := &GameDeps{}

	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_CHALLENGE_JOIN,
		RequestId: "ch-5",
		Payload:   []byte{0xFF, 0xFE, 0xFD},
	}

	response, err := handleChallengeJoin(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleChallengeJoin: %v", err)
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
